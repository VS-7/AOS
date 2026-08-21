package marketplacehttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/domain/marketplace"
	"github.com/OWNER/aos/internal/domain/skill"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/listings", func(w http.ResponseWriter, r *http.Request) {
		out := []marketplace.Listing{{
			Source: "acme/crm", Name: "CRM", Description: "A tiny CRM skill",
			Version: "1.0.0", Tags: []string{"crm"}, UpdatedAt: time.Now(),
			Permissions: skill.Permissions{Agents: []string{"sales"}},
		}}
		if r.URL.Query().Get("owner") == "nobody" {
			out = nil
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("/packages/acme/crm", func(w http.ResponseWriter, r *http.Request) {
		pkg := skill.Package{Manifest: skill.Manifest{Name: "crm", Version: "1.0.0"}, Content: "# CRM"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pkg)
	})
	mux.HandleFunc("/packages/nobody/nothing", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestHTTPSearchReturnsWhatTheIndexServes(t *testing.T) {
	srv := newTestServer(t)
	reg := New(srv.URL)

	got, err := reg.Search(context.Background(), marketplace.SearchQuery{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Source != "acme/crm" {
		t.Fatalf("Search = %+v, want one listing for acme/crm", got)
	}
	if len(got[0].Permissions.Agents) != 1 {
		t.Fatalf("Permissions = %+v, want the manifest's own agents, visible at discovery time", got[0].Permissions)
	}
}

func TestHTTPSearchPassesFiltersAsQueryParams(t *testing.T) {
	srv := newTestServer(t)
	reg := New(srv.URL)

	got, err := reg.Search(context.Background(), marketplace.SearchQuery{Owner: "nobody"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Search by owner=nobody = %d listings, want 0", len(got))
	}
}

func TestHTTPFetchDecodesThePackage(t *testing.T) {
	srv := newTestServer(t)
	reg := New(srv.URL)

	pkg, err := reg.Fetch(context.Background(), "acme/crm", "")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if pkg.Manifest.Name != "crm" {
		t.Fatalf("Manifest.Name = %q, want crm", pkg.Manifest.Name)
	}
}

func TestHTTPFetchOfAnUnknownSourceIsRefused(t *testing.T) {
	srv := newTestServer(t)
	reg := New(srv.URL)

	if _, err := reg.Fetch(context.Background(), "nobody/nothing", ""); err == nil {
		t.Fatal("Fetch of a 404 source succeeded, want an error")
	}
}

func TestHTTPRegistryUnreachableDegradesWithAClearError(t *testing.T) {
	reg := New("http://127.0.0.1:1") // nothing listens here
	reg.Timeout = 500 * time.Millisecond
	if _, err := reg.Search(context.Background(), marketplace.SearchQuery{}); err == nil {
		t.Fatal("Search against an unreachable registry succeeded, want a clear error")
	}
}
