package marketplacegit

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/OWNER/aos/internal/domain/marketplace"
	"github.com/OWNER/aos/internal/domain/marketplace/registrytest"
)

// TestRegistryObeysTheContract runs the fixture-independent port contract
// (docs/07 "Testes de Contrato de Port") against a real Git registry — see
// newTestRegistry's own doc for the fixture it seeds.
func TestRegistryObeysTheContract(t *testing.T) {
	registrytest.Contract(t, newTestRegistry(t))
}

const testSkillMD = `---
name: crm
description: A tiny CRM skill, for tests.
version: 1.0.0
permissions:
  agents: [sales]
  collections: [contacts]
---

# CRM

A tiny CRM skill.
`

// newTestRegistry builds a real local Git repository — no central service,
// no network — with one listing at "acme/crm", and returns a Registry
// pointed at it. Cloning a local path is what makes this adapter testable
// without a network, per Registry.URL's own doc.
func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(context.Background(), "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "-q", "-b", "main")

	pkgDir := filepath.Join(dir, "packages", "crm")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "SKILL.md"), []byte(testSkillMD), 0o644); err != nil {
		t.Fatal(err)
	}

	idx := indexFile{Listings: []indexEntry{
		{Source: "acme/crm", Path: "packages/crm", Name: "CRM", Description: "A tiny CRM skill", Version: "1.0.0", Tags: []string{"crm", "sales"}, Stars: 7},
	}}
	b, err := json.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	run("add", "-A")
	run("commit", "-q", "-m", "seed")

	return New(dir)
}

func TestSearchWithNoFilterReturnsEveryListing(t *testing.T) {
	reg := newTestRegistry(t)
	got, err := reg.Search(context.Background(), marketplace.SearchQuery{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Source != "acme/crm" {
		t.Fatalf("Search = %+v, want one listing for acme/crm", got)
	}
}

func TestSearchFiltersByOwner(t *testing.T) {
	reg := newTestRegistry(t)
	got, err := reg.Search(context.Background(), qOwner("acme"))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Search by owner acme = %d listings, want 1", len(got))
	}
	if got, err := reg.Search(context.Background(), qOwner("nobody")); err != nil || len(got) != 0 {
		t.Fatalf("Search by owner nobody = %v, %v, want 0 listings", got, err)
	}
}

func TestSearchFiltersByTag(t *testing.T) {
	reg := newTestRegistry(t)
	if got, err := reg.Search(context.Background(), qTag("sales")); err != nil || len(got) != 1 {
		t.Fatalf("Search by tag sales = %v, %v, want 1 listing", got, err)
	}
	if got, err := reg.Search(context.Background(), qTag("unrelated")); err != nil || len(got) != 0 {
		t.Fatalf("Search by tag unrelated = %v, %v, want 0 listings", got, err)
	}
}

func TestSearchSurfacesPermissionsAtDiscoveryTime(t *testing.T) {
	reg := newTestRegistry(t)
	got, err := reg.Search(context.Background(), marketplace.SearchQuery{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Search = %d listings, want 1", len(got))
	}
	if len(got[0].Permissions.Agents) != 1 || got[0].Permissions.Agents[0] != "sales" {
		t.Fatalf("Permissions = %+v, want the manifest's own agents:[sales], visible before Fetch (ADR-0015)", got[0].Permissions)
	}
}

func TestFetchReturnsTheClonedPackage(t *testing.T) {
	reg := newTestRegistry(t)
	pkg, err := reg.Fetch(context.Background(), "acme/crm", "")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if pkg.Manifest.Name != "crm" {
		t.Fatalf("Manifest.Name = %q, want crm", pkg.Manifest.Name)
	}
	if pkg.Manifest.Version != "1.0.0" {
		t.Fatalf("Manifest.Version = %q, want 1.0.0", pkg.Manifest.Version)
	}
}

func TestFetchOfAnUnlistedSourceIsRefused(t *testing.T) {
	reg := newTestRegistry(t)
	if _, err := reg.Fetch(context.Background(), "nobody/nothing", ""); err == nil {
		t.Fatal("Fetch of an unlisted source succeeded, want an error naming the missing listing")
	}
}

func TestSearchAgainstAnUnreachableRegistryDegradesWithAClearError(t *testing.T) {
	reg := New(filepath.Join(t.TempDir(), "does-not-exist"))
	reg.Timeout = 0 // exercise the default too
	if _, err := reg.Search(context.Background(), marketplace.SearchQuery{}); err == nil {
		t.Fatal("Search against an unreachable registry succeeded, want a clear error")
	}
}

// query helpers, kept local to the test file so the assertions above read as
// one line each rather than a struct literal every time.
func qOwner(owner string) marketplace.SearchQuery { return marketplace.SearchQuery{Owner: owner} }
func qTag(tag string) marketplace.SearchQuery     { return marketplace.SearchQuery{Tag: tag} }
