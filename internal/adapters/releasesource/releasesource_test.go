package releasesource_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OWNER/aos/internal/adapters/releasesource"
	"github.com/OWNER/aos/internal/domain/update"
)

func TestLatestDecodesAPublishedRelease(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/stable.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(update.Release{
			Version:      "v0.10.0",
			ChecksumsURL: "https://example.test/checksums.txt",
			SignatureURL: "https://example.test/checksums.txt.sig",
			Assets:       []update.Asset{{Binary: "aos", Platform: "linux/amd64"}},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	src := releasesource.New(srv.URL)
	release, err := src.Latest(context.Background(), update.ChannelStable)
	if err != nil {
		t.Fatal(err)
	}
	if release == nil || release.Version != "v0.10.0" {
		t.Fatalf("got %+v", release)
	}
	if release.Channel != update.ChannelStable {
		t.Fatalf("expected the requested channel stamped on the result, got %q", release.Channel)
	}
}

func TestLatestOnAnUnpublishedChannelReportsNoRelease(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/beta.json", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	src := releasesource.New(srv.URL)
	release, err := src.Latest(context.Background(), update.ChannelBeta)
	if err != nil {
		t.Fatalf("a 404 channel should not be an error, got %v", err)
	}
	if release != nil {
		t.Fatalf("expected no release, got %+v", release)
	}
}

// An unconfigured BaseURL is what an installation with no release
// infrastructure set up yet looks like — Check's own contract needs this
// told apart from a real failure to reach a configured one.
func TestLatestWithNoBaseURLReportsNoReleaseWithoutARequest(t *testing.T) {
	src := releasesource.New("")
	release, err := src.Latest(context.Background(), update.ChannelStable)
	if err != nil {
		t.Fatalf("an unconfigured source should not error, got %v", err)
	}
	if release != nil {
		t.Fatalf("expected no release, got %+v", release)
	}
}

func TestLatestOnAServerErrorFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/stable.json", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	src := releasesource.New(srv.URL)
	if _, err := src.Latest(context.Background(), update.ChannelStable); err == nil {
		t.Fatal("expected an error on a 500")
	}
}

func TestFetchDownloadsBytesWhole(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/asset", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("binary contents"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	src := releasesource.New(srv.URL)
	data, err := src.Fetch(context.Background(), srv.URL+"/asset")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "binary contents" {
		t.Fatalf("got %q", data)
	}
}

func TestFetchOnA404Fails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/missing", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	src := releasesource.New(srv.URL)
	if _, err := src.Fetch(context.Background(), srv.URL+"/missing"); err == nil {
		t.Fatal("expected an error fetching a missing asset")
	}
}
