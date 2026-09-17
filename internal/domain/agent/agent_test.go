package agent_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/fakes"
)

var refTime = time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

// newService runs on fakes: no disk, no network. A failure here means the rule
// under test is wrong, not that the machine is slow.
func newService(t *testing.T) (*agent.Service, *fakes.Repo[agent.Agent]) {
	t.Helper()
	repo := fakes.NewRepo[agent.Agent]("agents")
	return agent.NewService(repo, clockx.Fixed{At: refTime}), repo
}

func ctx() context.Context { return context.Background() }

func TestCreateStampsAndDefaultsTheName(t *testing.T) {
	svc, _ := newService(t)
	got, err := svc.Create(ctx(), agent.CreateInput{ID: "atlas"})
	if err != nil {
		t.Fatal(err)
	}
	// The original defaults the display name to the slug on create.
	if got.Name != "atlas" {
		t.Errorf("name = %q, want the slug", got.Name)
	}
	if !got.CreatedAt.Equal(refTime) || !got.UpdatedAt.Equal(refTime) {
		t.Errorf("timestamps = %v/%v", got.CreatedAt, got.UpdatedAt)
	}
}

// TestSlugIsLowercased: the file name carries the identity, so "Atlas" and
// "atlas" must not become two agents.
func TestSlugIsLowercased(t *testing.T) {
	svc, repo := newService(t)
	if _, err := svc.Create(ctx(), agent.CreateInput{ID: "  ATLAS  ", Leader: "BOSS"}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx(), collections.Key{"id": "atlas"})
	if err != nil {
		t.Fatalf("the record is not under the normalised slug: %v", err)
	}
	if got.Leader != "boss" {
		t.Errorf("the leader reference was not normalised: %q", got.Leader)
	}

	// And reading it back by any casing finds the same agent.
	found, err := svc.Get(ctx(), agent.GetInput{ID: "ATLAS"})
	if err != nil || found.ID != "atlas" {
		t.Fatalf("get = %+v, %v", found, err)
	}
}

func TestCreateRejectsAnEmptySlugWithACallToAction(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.Create(ctx(), agent.CreateInput{ID: "   "})
	if err == nil {
		t.Fatal("an empty slug has no identity")
	}
	if !errors.Is(err, apperr.ErrInvalid) {
		t.Fatalf("error = %v", err)
	}
	e, _ := apperr.As(err)
	if len(e.Actions) == 0 {
		t.Error("the caller must be told what a usable slug looks like")
	}
}

// The interface's "New agent" form asks for a name, not a slug — it was
// ported from an app whose server minted the id. Requiring one here is what
// made every create from the application fail with "id is required".
func TestCreateDerivesTheSlugFromTheName(t *testing.T) {
	svc, _ := newService(t)
	got, err := svc.Create(ctx(), agent.CreateInput{Name: "Luara Ávila"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "luara-avila" {
		t.Errorf("id = %q, want the slug of the name", got.ID)
	}
	if got.Name != "Luara Ávila" {
		t.Errorf("name = %q, want the name as it was written", got.Name)
	}
}

// A name that slugs to nothing is still nothing to name a file after.
func TestCreateRejectsANameThatSlugsToNothing(t *testing.T) {
	svc, _ := newService(t)
	if _, err := svc.Create(ctx(), agent.CreateInput{Name: "  ***  "}); err == nil {
		t.Fatal("an agent with no usable slug has no identity")
	}
}

// The orchestrator's instructions tell it to create focused specialists, and
// the settings screen's New Agent form has no sandbox field. Every agent
// either produced was read-only with no execution, so its first Write or Bash
// was refused and the specialist was useless from the moment it existed.
func TestACreatedAgentCanActInTheWorkspace(t *testing.T) {
	svc, _ := newService(t)
	got, err := svc.Create(ctx(), agent.CreateInput{ID: "helper"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Sandbox == nil {
		t.Fatal("a created agent has no sandbox, so it can read and nothing else")
	}
	for _, want := range []string{"read", "write", "execute"} {
		if !slices.Contains(got.Sandbox.Permissions, want) {
			t.Errorf("permissions %v do not include %q", got.Sandbox.Permissions, want)
		}
	}
	if got.Sandbox.Exec == nil || got.Sandbox.Exec.Policy != "allowlist" {
		t.Errorf("exec policy = %+v, want an allowlist (ADR-0006)", got.Sandbox.Exec)
	}
	if got.Sandbox.Exec != nil && got.Sandbox.Exec.AllowShell {
		t.Error("the default handed out a shell, which makes the allowlist a suggestion")
	}
}

// A caller that says what the agent may do gets exactly that, and nothing is
// added to it.
func TestADeclaredSandboxIsNotWidened(t *testing.T) {
	svc, _ := newService(t)
	got, err := svc.Create(ctx(), agent.CreateInput{
		ID:      "reader",
		Sandbox: &agent.Sandbox{Permissions: []string{"read"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sandbox.Permissions) != 1 || got.Sandbox.Permissions[0] != "read" {
		t.Errorf("permissions = %v, want exactly what was declared", got.Sandbox.Permissions)
	}
}

func TestCreateRefusesADuplicate(t *testing.T) {
	svc, _ := newService(t)
	if _, err := svc.Create(ctx(), agent.CreateInput{ID: "atlas"}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Create(ctx(), agent.CreateInput{ID: "atlas"})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("error = %v", err)
	}
}

// TestUpdateTouchesOnlyWhatWasSent: an omitted field is left alone rather than
// blanked, which is the difference between a patch and a replace.
func TestUpdateTouchesOnlyWhatWasSent(t *testing.T) {
	svc, _ := newService(t)
	if _, err := svc.Create(ctx(), agent.CreateInput{
		ID: "atlas", Name: "Atlas", Role: "Orchestrator",
		Provider: "openai", Model: "gpt-5.5", Content: "# Instructions\n",
	}); err != nil {
		t.Fatal(err)
	}

	role := "Lead"
	got, err := svc.Update(ctx(), agent.UpdateInput{ID: "atlas", Role: &role})
	if err != nil {
		t.Fatal(err)
	}
	if got.Role != "Lead" {
		t.Errorf("role = %q", got.Role)
	}
	for _, kept := range []struct{ name, got, want string }{
		{"name", got.Name, "Atlas"},
		{"provider", got.Provider, "openai"},
		{"model", got.Model, "gpt-5.5"},
		{"content", got.Content, "# Instructions\n"},
	} {
		if kept.got != kept.want {
			t.Errorf("%s was blanked: %q, want %q", kept.name, kept.got, kept.want)
		}
	}
}

func TestUpdateOfAMissingAgentIsNotFound(t *testing.T) {
	svc, _ := newService(t)
	name := "x"
	_, err := svc.Update(ctx(), agent.UpdateInput{ID: "ghost", Name: &name})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestDeleteReportsWhatHappened(t *testing.T) {
	svc, _ := newService(t)
	if _, err := svc.Create(ctx(), agent.CreateInput{ID: "atlas"}); err != nil {
		t.Fatal(err)
	}
	out, err := svc.Delete(ctx(), agent.DeleteInput{ID: "atlas"})
	if err != nil || !out.Deleted {
		t.Fatalf("out = %+v, err = %v", out, err)
	}

	// Deleting what is not there is reported, not silently accepted: the caller
	// asked to remove a specific agent and it did not exist.
	out, err = svc.Delete(ctx(), agent.DeleteInput{ID: "atlas"})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
	if out.Deleted {
		t.Error("nothing was deleted")
	}
}

func TestListIsOrderedAndFiltered(t *testing.T) {
	svc, _ := newService(t)
	for _, in := range []agent.CreateInput{
		{ID: "zeta", Role: "reviewer"},
		{ID: "alpha", Role: "orchestrator"},
		{ID: "mu", Role: "reviewer"},
	} {
		if _, err := svc.Create(ctx(), in); err != nil {
			t.Fatal(err)
		}
	}

	all, err := svc.List(ctx(), agent.ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 3 {
		t.Fatalf("total = %d", all.Total)
	}
	for i, want := range []string{"alpha", "mu", "zeta"} {
		if all.Agents[i].ID != want {
			t.Fatalf("order = %v, want alphabetical", ids(all.Agents))
		}
	}

	reviewers, err := svc.List(ctx(), agent.ListInput{Role: "reviewer"})
	if err != nil {
		t.Fatal(err)
	}
	if reviewers.Total != 2 {
		t.Fatalf("reviewers = %v", ids(reviewers.Agents))
	}

	// The limit cuts the page but Total still reports the whole match, so the
	// caller knows there is more.
	page, err := svc.List(ctx(), agent.ListInput{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Agents) != 2 || page.Total != 3 {
		t.Fatalf("page = %d of %d", len(page.Agents), page.Total)
	}
}

func ids(as []agent.Agent) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.ID
	}
	return out
}

func TestDisplayNameFallsBackToTheSlug(t *testing.T) {
	if got := (agent.Agent{ID: "atlas"}).DisplayName(); got != "atlas" {
		t.Errorf("DisplayName = %q", got)
	}
	if got := (agent.Agent{ID: "atlas", Name: "Atlas"}).DisplayName(); got != "Atlas" {
		t.Errorf("DisplayName = %q", got)
	}
}

// TestRegisterPublishesTheWholeGroup covers the projection: the domain declares
// the commands, and every surface is derived from that declaration.
func TestRegisterPublishesTheWholeGroup(t *testing.T) {
	svc, _ := newService(t)
	reg := command.NewRegistry()
	agent.Register(reg, svc)

	want := []string{"agents_create", "agents_delete", "agents_get", "agents_list", "agents_me", "agents_update"}
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

	// The read-only actions must announce themselves as such: the approval
	// channel derives its risk level from the annotations.
	for _, d := range reg.Sorted() {
		switch d.Key() {
		case "agents_list", "agents_get", "agents_me":
			if !d.Annotations().ReadOnlyHint {
				t.Errorf("%s must be announced read-only", d.Key())
			}
		case "agents_delete":
			if !d.Annotations().DestructiveHint {
				t.Errorf("%s must be announced destructive", d.Key())
			}
		}
	}
}

// The settings form has edited the avatar since the port, and neither input
// carried it: the field existed in the interface, in the record, and nowhere
// in between, so every save dropped it silently.
func TestTheAvatarIsWritable(t *testing.T) {
	svc, _ := newService(t)
	created, err := svc.Create(ctx(), agent.CreateInput{ID: "luara", Image: "https://example.test/a.png"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Image != "https://example.test/a.png" {
		t.Errorf("image = %q, want the one that was sent", created.Image)
	}

	blank := ""
	cleared, err := svc.Update(ctx(), agent.UpdateInput{ID: "luara", Image: &blank})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Image != "" {
		t.Errorf("image = %q, want it cleared", cleared.Image)
	}
}

// The Channels tab saves a Telegram binding with `agents update`, and the
// input had no field for it: the decoder dropped the list and the save
// answered with an unchanged agent. Channels are replaced whole, like the
// sandbox, so removing a binding really removes it.
func TestChannelsAreWritableAndReplacedWhole(t *testing.T) {
	svc, _ := newService(t)
	if _, err := svc.Create(ctx(), agent.CreateInput{
		ID:       "luara",
		Role:     "Orchestrator",
		Channels: []agent.Channel{{Provider: "discord", Data: map[string]any{"guild": "1"}}},
	}); err != nil {
		t.Fatal(err)
	}

	telegram := []agent.Channel{{Provider: "telegram", Data: map[string]any{"token": "123:abc", "allowedIds": []any{"42"}}}}
	got, err := svc.Update(ctx(), agent.UpdateInput{ID: "luara", Channels: &telegram})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Channels) != 1 || got.Channels[0].Provider != "telegram" {
		t.Fatalf("channels = %+v, want only the telegram binding that was sent", got.Channels)
	}
	if got.Role != "Orchestrator" {
		t.Errorf("role = %q, want it left alone", got.Role)
	}

	role := "Lead"
	kept, err := svc.Update(ctx(), agent.UpdateInput{ID: "luara", Role: &role})
	if err != nil {
		t.Fatal(err)
	}
	if len(kept.Channels) != 1 {
		t.Errorf("channels = %+v, want them left alone by an update that did not send any", kept.Channels)
	}

	none := []agent.Channel{}
	cleared, err := svc.Update(ctx(), agent.UpdateInput{ID: "luara", Channels: &none})
	if err != nil {
		t.Fatal(err)
	}
	if len(cleared.Channels) != 0 {
		t.Errorf("channels = %+v, want them cleared", cleared.Channels)
	}
}

// A person creating an agent from the settings screen types a name, and the
// id is made from it. A name with nothing to make an id from was refused as
// `"" is not a usable agent slug` with a CTA to run `aos agents create atlas`
// — an empty quote, a field the form does not have, and a terminal command.
func TestANameWithNothingToMakeAnIDFromIsRefusedByName(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.Create(ctx(), agent.CreateInput{Name: "!!!"})
	if !errors.Is(err, apperr.ErrInvalid) {
		t.Fatalf("error = %v", err)
	}
	e, _ := apperr.As(err)
	if !strings.Contains(e.Message, `"!!!"`) || strings.Contains(e.Message, `""`) {
		t.Errorf("message = %q, want it to quote the name that was typed", e.Message)
	}
	if len(e.Actions) == 0 || !strings.Contains(e.Actions[0].Label, "letters or digits") {
		t.Errorf("actions = %+v, want to be told what a usable name has", e.Actions)
	}
}

// The duplicate reached the caller as the storage layer's sentence — `record
// id=luara already exists in collection "agents"` — which names a collection
// and a key rather than the agent that is in the way.
func TestCreateNamesTheAgentThatAlreadyHasTheID(t *testing.T) {
	svc, _ := newService(t)
	if _, err := svc.Create(ctx(), agent.CreateInput{Name: "Luara"}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Create(ctx(), agent.CreateInput{Name: "Luára"})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("error = %v", err)
	}
	e, _ := apperr.As(err)
	if e.Code != "AOS_AGENT_ALREADY_EXISTS" {
		t.Errorf("code = %q, want the agent domain's own", e.Code)
	}
	if !strings.Contains(e.Message, "Luara") || !strings.Contains(e.Message, "luara") {
		t.Errorf("message = %q, want the existing agent and the id", e.Message)
	}
}

type recordingNotifier struct{ got []agent.Agent }

func (r *recordingNotifier) AgentChanged(_ context.Context, _ string, a *agent.Agent) {
	r.got = append(r.got, *a)
}

// The activity for a deletion said "Deleted agent: zeta-del": the notifier
// was handed a record with only the id, so the name was already gone.
func TestDeleteReportsTheAgentByName(t *testing.T) {
	notes := &recordingNotifier{}
	svc := agent.NewService(fakes.NewRepo[agent.Agent]("agents"), clockx.Fixed{At: refTime}, agent.WithNotifier(notes))
	if _, err := svc.Create(ctx(), agent.CreateInput{Name: "Zeta Del"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Delete(ctx(), agent.DeleteInput{ID: "zeta-del"}); err != nil {
		t.Fatal(err)
	}
	last := notes.got[len(notes.got)-1]
	if last.ID != "zeta-del" || last.Name != "Zeta Del" {
		t.Errorf("deleted = %+v, want the agent as it was", last)
	}
}
