package skillfetch_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/aos/internal/adapters/skillfetch"
	"github.com/OWNER/aos/internal/core/apperr"
)

func ctx() context.Context { return context.Background() }

// TestFetchReadsTheCommittedFixture proves the adapter reads the exact
// fixture Task 11's delivery test uses, at internal/app/testdata/crm-skill —
// built once, here, and not duplicated anywhere else.
func TestFetchReadsTheCommittedFixture(t *testing.T) {
	pkg, err := skillfetch.New().Fetch(ctx(), filepath.Join("..", "..", "app", "testdata", "crm-skill"), "")
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Manifest.Name != "crm" {
		t.Fatalf("manifest name = %q", pkg.Manifest.Name)
	}
	if len(pkg.Manifest.Permissions.Agents) != 1 || pkg.Manifest.Permissions.Agents[0] != "closer" {
		t.Fatalf("permissions.agents = %v", pkg.Manifest.Permissions.Agents)
	}
	if len(pkg.Collections) != 1 || pkg.Collections[0].ID != "contacts" {
		t.Fatalf("collections = %+v", pkg.Collections)
	}
	if len(pkg.Views) != 1 || pkg.Views[0].ID != "contacts-table" {
		t.Fatalf("views = %+v", pkg.Views)
	}
	var sawAgent bool
	for _, f := range pkg.Files {
		if f.Path == "agents/closer/AGENT.md" {
			sawAgent = true
		}
	}
	if !sawAgent {
		t.Fatalf("agents/closer/AGENT.md is missing from the raw files: %+v", pkg.Files)
	}
	// Version, Source and Commit are declared empty for a local fetch: there
	// is no provenance to record, and inventing one would be worse than
	// leaving it blank.
	if pkg.Manifest.Source != "" {
		t.Fatalf("source = %q, want empty: a local directory has no provenance", pkg.Manifest.Source)
	}
}

// TestFetchRefusesANonEmptyRef is the local fetcher's only opinion about
// versions: it has none, and says so rather than silently ignoring one.
func TestFetchRefusesANonEmptyRef(t *testing.T) {
	dir := writePackage(t, "name: x\n", nil)
	_, err := skillfetch.New().Fetch(ctx(), dir, "v1.2.3")
	if code := codeOf(t, err); code != "AOS_SKILLFETCH_REF_NOT_SUPPORTED" {
		t.Fatalf("code = %q", code)
	}
}

func TestFetchRefusesADirectoryWithNoSkillMD(t *testing.T) {
	dir := t.TempDir()
	_, err := skillfetch.New().Fetch(ctx(), dir, "")
	if code := codeOf(t, err); code != "AOS_SKILLFETCH_NOT_A_PACKAGE" {
		t.Fatalf("code = %q", code)
	}
}

// TestAResourceThatClimbsOutOfThePackageIsRefused is the property the whole
// package exists to hold: a SKILL.md naming a path that climbs out is not
// followed. A package that could read outside itself could read the
// workspace's .env.
func TestAResourceThatClimbsOutOfThePackageIsRefused(t *testing.T) {
	front := "name: escapee\nresources:\n  - uri: \"../../../etc/passwd\"\n"
	dir := writePackage(t, front, nil)

	_, err := skillfetch.New().Fetch(ctx(), dir, "")
	if code := codeOf(t, err); code != "AOS_SKILLFETCH_RESOURCE_OUTSIDE_PACKAGE" {
		t.Fatalf("code = %q, err = %v", code, err)
	}
}

// TestAResourceInsideThePackageIsRead is the same path confined the other
// way: a relative reference that stays inside the package is read normally.
func TestAResourceInsideThePackageIsRead(t *testing.T) {
	front := "name: bounded\nresources:\n  - uri: \"references/notes.md\"\n"
	dir := writePackage(t, front, map[string]string{
		"references/notes.md": "some notes",
	})

	pkg, err := skillfetch.New().Fetch(ctx(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, f := range pkg.Files {
		if f.Path == "references/notes.md" && string(f.Content) == "some notes" {
			found = true
		}
	}
	if !found {
		t.Fatalf("references/notes.md was not read: %+v", pkg.Files)
	}
}

func TestFetchDecodesAToolsetsCommandAndHost(t *testing.T) {
	front := "name: with-tools\n"
	dir := writePackage(t, front, map[string]string{
		"toolsets/gh.toolset.md": "---\ntype: cli\ncommand: gh\n---\n",
	})

	pkg, err := skillfetch.New().Fetch(ctx(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkg.Toolsets) != 1 {
		t.Fatalf("toolsets = %+v", pkg.Toolsets)
	}
	ts := pkg.Toolsets[0]
	if ts.ID != "gh" || ts.Type != "cli" || ts.Command != "gh" {
		t.Fatalf("toolset decoded as %+v", ts)
	}
	if ts.RawFile.Path != "toolsets/gh.toolset.md" || len(ts.RawFile.Content) == 0 {
		t.Fatalf("toolset raw file = %+v", ts.RawFile)
	}
}

func TestFetchWithNoCollectionsOrViewsDirectoryBringsNone(t *testing.T) {
	dir := writePackage(t, "name: bare\n", nil)
	pkg, err := skillfetch.New().Fetch(ctx(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkg.Collections) != 0 || len(pkg.Views) != 0 || len(pkg.Toolsets) != 0 {
		t.Fatalf("pkg = %+v, want nothing beyond the manifest", pkg)
	}
}

// writePackage builds a minimal skill package under a temp directory: a
// SKILL.md with the given front matter, plus any extra files.
func writePackage(t *testing.T, front string, extra map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	skillMD := "---\n" + front + "---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillMD), 0o600); err != nil {
		t.Fatal(err)
	}
	for rel, content := range extra {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, not *apperr.Error: %v", err, err)
	}
	return app.Code
}
