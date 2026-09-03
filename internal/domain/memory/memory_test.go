package memory_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/domain/fakes"
	"github.com/OWNER/aos/internal/domain/memory"
)

var refTime = time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

func newService(t *testing.T) (*memory.Service, *fakes.Repo[memory.Memory]) {
	t.Helper()
	repo := fakes.NewRepo[memory.Memory]("memories")
	svc := memory.NewService(memory.Deps{
		Repo: repo,
		// Stepping, not Fixed: several memories created in one test must not
		// share a timestamp, or ordering by it is untestable.
		Clock: &clockx.Stepping{At: refTime, Step: time.Minute},
		IDs:   &ids.Sequence{Prefix: "m"},
	})
	return svc, repo
}

// ctx is the agent execution context. Memories are personal, so almost every
// call needs one.
func ctx() context.Context {
	return identity.With(context.Background(), identity.Identity{AgentID: "atlas"})
}

func humanCtx() context.Context {
	return identity.With(context.Background(), identity.Identity{UserID: "vitor"})
}

func store(t *testing.T, svc *memory.Service, in memory.StoreInput) memory.StoreOutput {
	t.Helper()
	if in.Title == "" {
		in.Title = "A title"
	}
	if in.Description == "" {
		in.Description = "A description"
	}
	if in.Category == "" {
		in.Category = memory.CatFact
	}
	out, err := svc.Store(ctx(), in)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestStoreStampsIdentityAndDefaults(t *testing.T) {
	svc, _ := newService(t)
	out := store(t, svc, memory.StoreInput{
		Title: "Gateway restarts nightly", Description: "Observed on three consecutive days",
		Category: memory.CatObservation,
	})

	m := out.Memory
	if m.ID != "m-1" {
		t.Errorf("id = %q", m.ID)
	}
	if m.Agent != "atlas" {
		t.Errorf("agent = %q, want the ambient identity", m.Agent)
	}
	if m.Status != memory.StatusActive {
		t.Errorf("status = %q", m.Status)
	}
	if m.Confidence != 1 {
		t.Errorf("confidence = %v, want the default of 1", m.Confidence)
	}
	if m.CreatedAt.IsZero() || !m.UpdatedAt.Equal(m.CreatedAt) {
		t.Errorf("timestamps = %v / %v", m.CreatedAt, m.UpdatedAt)
	}
}

// TestAMemoryNeedsAnOwner: a memory with no owner belongs to nobody and would
// be recalled by everybody.
func TestAMemoryNeedsAnOwner(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.Store(humanCtx(), memory.StoreInput{
		Title: "x", Description: "y", Category: memory.CatFact,
	})
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("error = %v", err)
	}
	e, _ := apperr.As(err)
	if len(e.Actions) == 0 {
		t.Error("a 403 must carry a CTA")
	}
}

// The application's own "Record a memory" is a person writing on an agent's
// behalf. Store was the only one of the four memory commands with no `agent`
// field, so it could resolve an owner from the ambient identity alone — and
// the desktop window is a user identity. Every write from the agent's
// Memories tab came back AOS_MEMORY_AGENT_REQUIRED, telling the person to
// pass `--agent` on a command line they were not using.
func TestAMemoryCanNameItsAgent(t *testing.T) {
	svc, _ := newService(t)
	out, err := svc.Store(humanCtx(), memory.StoreInput{
		Agent: "luara", Title: "x", Description: "y", Category: memory.CatFact,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Memory.Agent != "luara" {
		t.Errorf("agent = %q, want the one that was named", out.Memory.Agent)
	}
}

func TestStoreValidatesCategoryAndConfidence(t *testing.T) {
	svc, _ := newService(t)

	_, err := svc.Store(ctx(), memory.StoreInput{
		Title: "x", Description: "y", Category: memory.Category("vibes"),
	})
	if !errors.Is(err, apperr.ErrInvalid) {
		t.Fatalf("category: error = %v", err)
	}
	e, _ := apperr.As(err)
	if !strings.Contains(e.Issues["allowed"].(string), "decision") {
		t.Error("the error should list the categories that exist")
	}

	for _, bad := range []float64{-0.1, 1.5} {
		_, err := svc.Store(ctx(), memory.StoreInput{
			Title: "x", Description: "y", Category: memory.CatFact, Confidence: &bad,
		})
		if !errors.Is(err, apperr.ErrInvalid) {
			t.Errorf("confidence %v: error = %v", bad, err)
		}
	}
}

// TestTheBodySitsFlushAgainstTheFrontMatter reproduces the original's
// formatting rule: a body starting with a blank line renders as one everywhere
// it is shown.
func TestTheBodySitsFlushAgainstTheFrontMatter(t *testing.T) {
	svc, _ := newService(t)
	out := store(t, svc, memory.StoreInput{Content: "\n\n   # Heading\nbody\n"})
	if !strings.HasPrefix(out.Memory.Content, "# Heading") {
		t.Fatalf("content = %q", out.Memory.Content)
	}
}

// TestSupersedeWritesTheReplacementFirst is the protocol, in order: verify,
// write the new one, deprecate the old ones pointing at it.
func TestSupersedeWritesTheReplacementFirst(t *testing.T) {
	svc, _ := newService(t)
	old := store(t, svc, memory.StoreInput{Title: "Old decision", Category: memory.CatDecision}).Memory

	out := store(t, svc, memory.StoreInput{
		Title: "New decision", Category: memory.CatDecision,
		Supersedes: []memory.Super{{ID: old.ID, Reason: "The premise it rested on no longer holds."}},
	})
	if len(out.Deprecated) != 1 || out.Deprecated[0] != old.ID {
		t.Fatalf("deprecated = %v", out.Deprecated)
	}
	if len(out.Incomplete) != 0 {
		t.Fatalf("incomplete = %v", out.Incomplete)
	}

	got, err := svc.Reflect(ctx(), memory.ReflectInput{Memory: old.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != memory.StatusDeprecated {
		t.Errorf("status = %q", got.Status)
	}
	if got.DeprecatedAt == nil || got.DeprecatedReason == "" {
		t.Errorf("lineage = %v / %q", got.DeprecatedAt, got.DeprecatedReason)
	}
	// This is the divergence: the original writes the *agent* slug into
	// deprecatedBy, so its lineage records who deprecated rather than what
	// replaced, and the chain cannot be walked.
	if got.DeprecatedBy != out.Memory.ID {
		t.Errorf("deprecatedBy = %q, want the replacement memory %q", got.DeprecatedBy, out.Memory.ID)
	}
}

// TestSupersedingSomethingThatIsNotThereWritesNothing: a lineage pointing at
// nothing is worse than a failed write.
func TestSupersedingSomethingThatIsNotThereWritesNothing(t *testing.T) {
	svc, repo := newService(t)
	_, err := svc.Store(ctx(), memory.StoreInput{
		Title: "New", Description: "d", Category: memory.CatDecision,
		Supersedes: []memory.Super{{ID: "ghost", Reason: "It never existed."}},
	})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
	if repo.Len() != 0 {
		t.Fatal("the replacement was written even though its target was missing")
	}
}

func TestSupersedeNeedsARealReason(t *testing.T) {
	svc, repo := newService(t)
	old := store(t, svc, memory.StoreInput{Title: "Old"}).Memory

	_, err := svc.Store(ctx(), memory.StoreInput{
		Title: "New", Description: "d", Category: memory.CatDecision,
		Supersedes: []memory.Super{{ID: old.ID, Reason: "nope"}},
	})
	if !errors.Is(err, apperr.ErrInvalid) {
		t.Fatalf("error = %v", err)
	}
	if repo.Len() != 1 {
		t.Fatal("the replacement was written despite the rejected reason")
	}
}

func TestForgetNeedsARealReason(t *testing.T) {
	svc, _ := newService(t)
	m := store(t, svc, memory.StoreInput{}).Memory

	_, err := svc.Forget(ctx(), memory.ForgetInput{Memory: m.ID, Reason: "old"})
	if !errors.Is(err, apperr.ErrInvalid) {
		t.Fatalf("error = %v", err)
	}
	e, _ := apperr.As(err)
	if len(e.Actions) == 0 || !strings.Contains(e.Actions[0].Label, "lower its confidence") {
		t.Error("the caller should be told the alternative to forgetting")
	}
}

func TestForgettingTwiceIsAConflict(t *testing.T) {
	svc, _ := newService(t)
	m := store(t, svc, memory.StoreInput{}).Memory
	reason := "This turned out to be wrong after the migration."

	if _, err := svc.Forget(ctx(), memory.ForgetInput{Memory: m.ID, Reason: reason}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Forget(ctx(), memory.ForgetInput{Memory: m.ID, Reason: reason})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("error = %v", err)
	}
}

// TestThereIsNoDelete is a structural assertion: the aggregate does not offer
// erasure on any surface, which is the decision the whole model rests on.
func TestThereIsNoDelete(t *testing.T) {
	svc, _ := newService(t)
	reg := command.NewRegistry()
	memory.Register(reg, svc)
	for _, d := range reg.Sorted() {
		if strings.Contains(d.Key(), "delete") || strings.Contains(d.Key(), "remove") {
			t.Errorf("the memory group publishes %q", d.Key())
		}
		if d.Annotations().DestructiveHint {
			t.Errorf("%q is announced destructive; nothing here destroys", d.Key())
		}
	}
}

func TestRecallExcludesDeprecatedByDefault(t *testing.T) {
	svc, _ := newService(t)
	keep := store(t, svc, memory.StoreInput{Title: "Still true"}).Memory
	drop := store(t, svc, memory.StoreInput{Title: "No longer true"}).Memory
	if _, err := svc.Forget(ctx(), memory.ForgetInput{
		Memory: drop.ID, Reason: "Superseded by the redesign.",
	}); err != nil {
		t.Fatal(err)
	}

	out, err := svc.Recall(ctx(), memory.RecallInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 || out.Memories[0].ID != keep.ID {
		t.Fatalf("recall = %v", idsOf(out.Memories))
	}

	// It is not gone: asking for it by status finds it.
	out, err = svc.Recall(ctx(), memory.RecallInput{Status: memory.StatusDeprecated})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 || out.Memories[0].ID != drop.ID {
		t.Fatalf("deprecated recall = %v", idsOf(out.Memories))
	}
}

// TestRecallRequiresEveryWord: a result containing one word out of three is
// worse than no result, because the caller acts on what comes back.
func TestRecallRequiresEveryWord(t *testing.T) {
	svc, _ := newService(t)
	store(t, svc, memory.StoreInput{
		Title: "UUID migration decision", Description: "Chose v4 over auto-increment",
		Category: memory.CatDecision,
	})
	store(t, svc, memory.StoreInput{Title: "Postgres connection pooling", Description: "pgbouncer"})

	hit, err := svc.Recall(ctx(), memory.RecallInput{Query: "uuid migration"})
	if err != nil {
		t.Fatal(err)
	}
	if hit.Total != 1 {
		t.Fatalf("matches = %v", idsOf(hit.Memories))
	}

	miss, err := svc.Recall(ctx(), memory.RecallInput{Query: "uuid postgres"})
	if err != nil {
		t.Fatal(err)
	}
	if miss.Total != 0 {
		t.Fatalf("a query whose words are split across two memories matched %v", idsOf(miss.Memories))
	}
}

func TestRecallFiltersByCategory(t *testing.T) {
	svc, _ := newService(t)
	store(t, svc, memory.StoreInput{Title: "a", Category: memory.CatDecision})
	store(t, svc, memory.StoreInput{Title: "b", Category: memory.CatLearning})
	store(t, svc, memory.StoreInput{Title: "c", Category: memory.CatDecision})

	out, err := svc.Recall(ctx(), memory.RecallInput{Category: memory.CatDecision})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 2 {
		t.Fatalf("total = %d", out.Total)
	}
}

func TestRecallRejectsAnUnknownCategoryOrStatus(t *testing.T) {
	svc, _ := newService(t)
	if _, err := svc.Recall(ctx(), memory.RecallInput{Category: "vibes"}); !errors.Is(err, apperr.ErrInvalid) {
		t.Errorf("category: error = %v", err)
	}
	if _, err := svc.Recall(ctx(), memory.RecallInput{Status: "pending"}); !errors.Is(err, apperr.ErrInvalid) {
		t.Errorf("status: error = %v", err)
	}
}

// TestRecallPagesWithoutLosingTheTotal: the limit cuts the page, and the total
// still says how much there was, so the caller knows to ask for more.
func TestRecallPagesWithoutLosingTheTotal(t *testing.T) {
	svc, _ := newService(t)
	for i := 0; i < 5; i++ {
		store(t, svc, memory.StoreInput{Title: "memory"})
	}
	out, err := svc.Recall(ctx(), memory.RecallInput{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Memories) != 2 || out.Total != 5 {
		t.Fatalf("page = %d of %d", len(out.Memories), out.Total)
	}

	second, err := svc.Recall(ctx(), memory.RecallInput{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Memories) != 2 {
		t.Fatalf("second page = %d", len(second.Memories))
	}
	if second.Memories[0].ID == out.Memories[0].ID {
		t.Error("the offset did not move the window")
	}
}

func TestRecallDefaultsToTenLikeTheOriginal(t *testing.T) {
	svc, _ := newService(t)
	for i := 0; i < 12; i++ {
		store(t, svc, memory.StoreInput{Title: "memory"})
	}
	out, err := svc.Recall(ctx(), memory.RecallInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Memories) != 10 || out.Total != 12 {
		t.Fatalf("page = %d of %d", len(out.Memories), out.Total)
	}
}

func TestRecallOrdersByConfidenceWhenAsked(t *testing.T) {
	svc, _ := newService(t)
	for _, c := range []float64{0.4, 0.95, 0.7} {
		conf := c
		store(t, svc, memory.StoreInput{Title: "memory", Confidence: &conf})
	}
	out, err := svc.Recall(ctx(), memory.RecallInput{OrderBy: "confidence", Desc: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.Memories[0].Confidence != 0.95 || out.Memories[2].Confidence != 0.4 {
		t.Fatalf("order = %v", confidencesOf(out.Memories))
	}
}

// TestRelevanceRanksTheTitleAboveTheBody: without an index, the fallback still
// has to order results the way an index would.
func TestRelevanceRanksTheTitleAboveTheBody(t *testing.T) {
	svc, _ := newService(t)
	buried := store(t, svc, memory.StoreInput{
		Title: "Unrelated", Description: "nothing here", Content: "the gateway is mentioned once",
	}).Memory
	titled := store(t, svc, memory.StoreInput{Title: "Gateway restart protocol", Description: "d"}).Memory

	out, err := svc.Recall(ctx(), memory.RecallInput{Query: "gateway"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Memories) != 2 {
		t.Fatalf("matches = %v", idsOf(out.Memories))
	}
	if out.Memories[0].ID != titled.ID || out.Memories[1].ID != buried.ID {
		t.Fatalf("order = %v, want the title match first", idsOf(out.Memories))
	}
	if out.Indexed {
		t.Error("no index is wired, so the answer came from a scan")
	}
}

func TestScopesStrictExcludesTheUnscoped(t *testing.T) {
	svc, _ := newService(t)
	scoped := store(t, svc, memory.StoreInput{
		Title: "scoped", Scopes: []string{"internal/domain/memory/service.go"},
	}).Memory
	store(t, svc, memory.StoreInput{Title: "unscoped"})

	strict, err := svc.Recall(ctx(), memory.RecallInput{
		Scopes: []string{"internal/domain/**"}, ScopesMode: memory.ScopesStrict,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strict.Total != 1 || strict.Memories[0].ID != scoped.ID {
		t.Fatalf("strict = %v", idsOf(strict.Memories))
	}

	lax, err := svc.Recall(ctx(), memory.RecallInput{
		Scopes: []string{"internal/domain/**"}, ScopesMode: memory.ScopesLax,
	})
	if err != nil {
		t.Fatal(err)
	}
	if lax.Total != 2 {
		t.Fatalf("lax = %v", idsOf(lax.Memories))
	}
}

// TestGlobsBehaveAsExpected pins the pattern semantics, including the case the
// original's hand-rolled regex gets wrong.
func TestGlobsBehaveAsExpected(t *testing.T) {
	cases := []struct {
		pattern, scope string
		want           bool
	}{
		{"src/**/*.go", "src/a/b.go", true},
		{"src/**/*.go", "src/a/b.ts", false},
		{"src/**/*.go", "other/a/b.go", false},
		// The original translates ** to .* with the surrounding slashes intact,
		// so this one fails there. doublestar matches zero directories too,
		// which is the behaviour every other tool with globstar has.
		{"src/**/*.go", "src/b.go", true},
		{"internal/domain/**", "internal/domain/memory/service.go", true},
		{"**/*", "anything/at/all.md", true},
	}
	for _, c := range cases {
		svc, _ := newService(t)
		store(t, svc, memory.StoreInput{Title: "m", Scopes: []string{c.scope}})

		out, err := svc.Recall(ctx(), memory.RecallInput{
			Scopes: []string{c.pattern}, ScopesMode: memory.ScopesStrict,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := out.Total == 1; got != c.want {
			t.Errorf("pattern %q against scope %q = %v, want %v", c.pattern, c.scope, got, c.want)
		}
	}
}

func TestAMalformedPatternDoesNotBreakTheOthers(t *testing.T) {
	svc, _ := newService(t)
	store(t, svc, memory.StoreInput{Title: "m", Scopes: []string{"src/a.go"}})

	out, err := svc.Recall(ctx(), memory.RecallInput{
		Scopes: []string{"[unclosed", "src/*.go"}, ScopesMode: memory.ScopesStrict,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 {
		t.Fatalf("one bad pattern made the good one useless: %v", idsOf(out.Memories))
	}
}

// TestMemoriesAreScopedToTheirOwner: two agents in one workspace do not read
// each other's traces through recall.
func TestMemoriesAreScopedToTheirOwner(t *testing.T) {
	svc, _ := newService(t)
	store(t, svc, memory.StoreInput{Title: "mine"})

	other := identity.With(context.Background(), identity.Identity{AgentID: "reviewer"})
	if _, err := svc.Store(other, memory.StoreInput{
		Title: "theirs", Description: "d", Category: memory.CatFact,
	}); err != nil {
		t.Fatal(err)
	}

	out, err := svc.Recall(ctx(), memory.RecallInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 || out.Memories[0].Title != "mine" {
		t.Fatalf("recall = %v", titlesOf(out.Memories))
	}
}

func TestReflectFromATerminalFindsAnyOwnersMemory(t *testing.T) {
	svc, _ := newService(t)
	m := store(t, svc, memory.StoreInput{Title: "atlas remembers"}).Memory

	got, err := svc.Reflect(humanCtx(), memory.ReflectInput{Memory: m.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != m.ID {
		t.Fatalf("reflect = %+v", got)
	}
}

func TestReflectOfAMissingMemoryIsNotFound(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.Reflect(ctx(), memory.ReflectInput{Memory: "ghost"})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestRegisterPublishesTheFiveVerbs(t *testing.T) {
	svc, _ := newService(t)
	reg := command.NewRegistry()
	memory.Register(reg, svc)

	want := []string{
		"memories_forget", "memories_graph", "memories_recall",
		"memories_reflect", "memories_store",
	}
	got := make([]string, 0, len(want))
	for _, d := range reg.Sorted() {
		got = append(got, d.Key())
	}
	if len(got) != len(want) {
		t.Fatalf("commands = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("commands = %v, want %v", got, want)
		}
	}

	doc, ok := reg.GroupOf("memories")
	if !ok {
		t.Fatal("the group was not described")
	}
	// The globality warning is behaviour, not decoration: it is what stops an
	// agent from treating its memories as private scratch space.
	if !strings.Contains(doc.Hint, "GLOBAL") {
		t.Error("the group hint dropped the warning about parallel selves")
	}
}

// TestVersionIsCapturedOnRead guards the concurrency story: two parallel selves
// updating the same memory must not silently overwrite each other.
func TestTwoParallelSelvesCannotBothWin(t *testing.T) {
	svc, repo := newService(t)
	m := store(t, svc, memory.StoreInput{Title: "contested"}).Memory

	key := collections.Key{"agent": "atlas", "id": m.ID}
	stale, err := repo.Version(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}

	// The first writer lands.
	if _, err := svc.Forget(ctx(), memory.ForgetInput{
		Memory: m.ID, Reason: "The first self decided this was wrong.",
	}); err != nil {
		t.Fatal(err)
	}

	// The second holds the version it read before that, and is refused.
	current, err := repo.Get(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	current.Title = "changed"
	if err := repo.Update(context.Background(), current, stale); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("error = %v, want a conflict", err)
	}
}

func idsOf(ms []memory.Memory) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.ID
	}
	return out
}

func titlesOf(ms []memory.Memory) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Title
	}
	return out
}

func confidencesOf(ms []memory.Memory) []float64 {
	out := make([]float64, len(ms))
	for i, m := range ms {
		out[i] = m.Confidence
	}
	return out
}

func keyOf(m memory.Memory) collections.Key {
	return collections.Key{"agent": m.Agent, "id": m.ID}
}

// versionZero expresses "no expectation": the test is arranging state, not
// exercising the compare-and-swap.
func versionZero() collections.Version { return collections.Version{} }
