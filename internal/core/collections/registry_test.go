package collections_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/collections"
)

// TestThirteenNativeCollections pins the set. The original has fourteen
// collection files; artifacts is the one missing here and arrives in phase 8,
// and workspaces is not a collection there either — it is a hand-written store
// over ~/.fractal/workspaces.
func TestThirteenNativeCollections(t *testing.T) {
	want := []string{
		"agents", "chats", "comments", "goals", "instructions", "memories",
		"projects", "routines", "runs", "skills", "tasks", "templates", "todos",
	}
	got := collections.Natives()
	if len(got) != len(want) {
		t.Fatalf("%d native collections, want %d", len(got), len(want))
	}
	for i, desc := range got {
		if desc.Name != want[i] {
			t.Errorf("collection %d = %q, want %q", i, desc.Name, want[i])
		}
	}
}

// TestEveryPatternIsRootedInTheStateDirectory guards the rename: nothing may
// point at the original product's directory, which exists on this machine.
func TestEveryPatternIsRootedInTheStateDirectory(t *testing.T) {
	for _, desc := range collections.Natives() {
		for _, p := range desc.Patterns {
			if !strings.HasPrefix(p.Raw(), build.StateDir+"/") {
				t.Errorf("%s: pattern %q is not under %q", desc.Name, p.Raw(), build.StateDir)
			}
			if strings.Contains(p.Raw(), ".fractal") {
				t.Errorf("%s: pattern %q points at the original product", desc.Name, p.Raw())
			}
		}
	}
}

func TestEveryCollectionHasAWritablePattern(t *testing.T) {
	for _, desc := range collections.Natives() {
		m, err := collections.ModelOf[probe](desc.Name)
		if err != nil {
			t.Fatalf("%s: %v", desc.Name, err)
		}
		if _, err := m.WritePattern(); err != nil {
			t.Errorf("%s: %v", desc.Name, err)
		}
	}
}

// TestSkillScopedVariantsExistWhereTheOriginalHasThem: without the second
// pattern, installing a skill does not install the team that comes with it.
func TestSkillScopedVariantsExistWhereTheOriginalHasThem(t *testing.T) {
	for _, name := range []string{"agents", "memories", "routines", "templates", "instructions", "goals", "runs"} {
		desc, ok := collections.Lookup(name)
		if !ok {
			t.Fatalf("%s is not registered", name)
		}
		found := false
		for _, p := range desc.Patterns {
			if strings.Contains(p.Raw(), "/skills/") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s has no skill-scoped pattern", name)
		}
	}
}

func TestCascadeIsDeclaredForDirectoryBackedCollections(t *testing.T) {
	cascading := map[string]bool{
		"agents": true, "skills": true, "tasks": true,
		"routines": true, "projects": true, "goals": true,
	}
	for _, desc := range collections.Natives() {
		if desc.CascadeDelete != cascading[desc.Name] {
			t.Errorf("%s: cascade = %v, want %v", desc.Name, desc.CascadeDelete, cascading[desc.Name])
		}
	}
}

// TestFormatMatchesTheOriginal, with two deliberate exceptions.
//
// The original stores todos and comments as JSON. They are Markdown here
// because a comment body is prose, and prose in a JSON string is one line with
// escaped newlines — the exact thing ADR-0004 rejects. The divergence is
// recorded in the Todo (Go) and Comment (Go) notes.
func TestFormatMatchesTheOriginal(t *testing.T) {
	jsonBacked := map[string]bool{"chats": true, "runs": true}
	for _, desc := range collections.Natives() {
		want := collections.FormatMarkdown
		if jsonBacked[desc.Name] {
			want = collections.FormatJSON
		}
		if desc.Format != want {
			t.Errorf("%s: format = %s, want %s", desc.Name, desc.Format, want)
		}
	}
}

func TestModelOfRejectsAnUnknownCollection(t *testing.T) {
	if _, err := collections.ModelOf[probe]("nope"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestWorkspaceDirsCoversEveryCollection(t *testing.T) {
	dirs := collections.WorkspaceDirs()
	if len(dirs) == 0 {
		t.Fatal("no directories derived from the patterns")
	}
	for _, want := range []string{
		build.StateDir + "/agents", build.StateDir + "/skills",
		build.StateDir + "/tasks", build.StateDir + "/chats",
	} {
		found := false
		for _, d := range dirs {
			if d == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%q missing from %v", want, dirs)
		}
	}
}

// A collection an agent invents has to be usable in the same session — that is
// the whole point of the original's autoWatch. The registry is therefore an
// instance with a lock, not package state computed once at init.
func TestADynamicCollectionIsVisibleAsSoonAsItIsRegistered(t *testing.T) {
	reg := collections.NewRegistry()

	if _, ok := reg.Lookup("contacts"); ok {
		t.Fatal("contacts existed before anybody registered it")
	}
	desc := collections.Descriptor{
		Name:     "contacts",
		Patterns: []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/contacts/records/{id}.md")},
		Format:   collections.FormatMarkdown,
	}
	if err := reg.Register(desc); err != nil {
		t.Fatal(err)
	}
	got, ok := reg.Lookup("contacts")
	if !ok {
		t.Fatal("the collection is not there right after being registered")
	}
	if got.Name != "contacts" {
		t.Fatalf("name = %q", got.Name)
	}
}

// A custom collection that shadowed a native one would break the engine's own
// registry: two things would claim the same name and the loser would be
// whichever was asked for second.
func TestARegisteredNameMayNotShadowANative(t *testing.T) {
	reg := collections.NewRegistry()

	for _, name := range []string{"agents", "skills", "memories", "tasks", "chats", "routines"} {
		err := reg.Register(collections.Descriptor{
			Name:     name,
			Patterns: []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/x/records/{id}.md")},
		})
		if err == nil {
			t.Fatalf("registering %q as a custom collection was allowed", name)
		}
		if code := codeOfErr(t, err); code != "AOS_COLLECTION_NAME_RESERVED" {
			t.Fatalf("%s: code = %q, want AOS_COLLECTION_NAME_RESERVED", name, code)
		}
	}
}

func TestANativeCannotBeUnregistered(t *testing.T) {
	reg := collections.NewRegistry()

	if err := reg.Unregister("agents"); err == nil {
		t.Fatal("a native collection was unregistered")
	}
	if _, ok := reg.Lookup("agents"); !ok {
		t.Fatal("the native disappeared anyway")
	}
}

func TestUnregisteringADynamicRemovesIt(t *testing.T) {
	reg := collections.NewRegistry()
	desc := collections.Descriptor{
		Name:     "contacts",
		Patterns: []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/contacts/records/{id}.md")},
	}
	if err := reg.Register(desc); err != nil {
		t.Fatal(err)
	}
	if err := reg.Unregister("contacts"); err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Lookup("contacts"); ok {
		t.Fatal("the collection survived being unregistered")
	}
}

// Registering the same name twice is the uninstall-then-reinstall path of a
// skill-scoped collection. It replaces rather than failing, because failing
// would leave a workspace that cannot reinstall what it just removed.
func TestRegisteringTheSameNameTwiceReplaces(t *testing.T) {
	reg := collections.NewRegistry()
	first := collections.Descriptor{
		Name:     "contacts",
		Patterns: []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/contacts/records/{id}.md")},
		Format:   collections.FormatMarkdown,
	}
	second := first
	second.Format = collections.FormatJSON
	second.Patterns = []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/contacts/records/{id}.json")}

	if err := reg.Register(first); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(second); err != nil {
		t.Fatal(err)
	}
	got, _ := reg.Lookup("contacts")
	if got.Format != collections.FormatJSON {
		t.Fatalf("format = %v, want the second registration to win", got.Format)
	}
}

// The registry is read by whatever is running and written by the watcher and by
// the create path. An unguarded map races on the first test that exercises one.
func TestTheRegistryIsSafeUnderConcurrentUse(t *testing.T) {
	reg := collections.NewRegistry()
	done := make(chan struct{})

	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			_ = reg.Register(collections.Descriptor{
				Name:     "contacts",
				Patterns: []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/contacts/records/{id}.md")},
			})
			_ = reg.Unregister("contacts")
		}
	}()
	for i := 0; i < 200; i++ {
		reg.Lookup("contacts")
		reg.Names()
	}
	<-done
}

func codeOfErr(t *testing.T, err error) string {
	t.Helper()
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, not an apperr: %v", err, err)
	}
	return app.Code
}
