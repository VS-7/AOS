package releasesource_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

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

// A configured feed with no manifest at the channel's address is not "no
// release": it was read that way, and Check answered "you are on the newest
// release" for a feed pointed at the wrong address.
func TestLatestOnAnUnpublishedChannelSaysNothingIsPublished(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/beta.json", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	src := releasesource.New(srv.URL)
	release, err := src.Latest(context.Background(), update.ChannelBeta)
	if !errors.Is(err, update.ErrNotPublished) {
		t.Fatalf("a 404 channel should wrap ErrNotPublished, got %v", err)
	}
	if release != nil {
		t.Fatalf("expected no release, got %+v", release)
	}
}

// An unconfigured BaseURL is what an installation with no release
// infrastructure set up yet looks like — Check's own contract needs this
// told apart from a real failure to reach a configured one, and it asks
// Configured rather than inferring it from an empty answer.
func TestNoBaseURLIsNotConfiguredAndMakesNoRequest(t *testing.T) {
	src := releasesource.New("")
	if src.Configured() {
		t.Fatal("an empty base URL is not a configured feed")
	}
	if _, err := src.Latest(context.Background(), update.ChannelStable); err == nil {
		t.Fatal("asking an unconfigured source for a release should say it is not configured")
	}
	if !releasesource.New("https://example.test/feed/").Configured() {
		t.Fatal("a base URL is a configured feed")
	}
}

func TestAnAddressThatDoesNotAnswerIsUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()

	src := releasesource.New(url)
	if _, err := src.Latest(context.Background(), update.ChannelStable); !errors.Is(err, update.ErrUnreachable) {
		t.Fatalf("expected ErrUnreachable, got %v", err)
	}
	if _, err := src.Fetch(context.Background(), url+"/checksums.txt"); !errors.Is(err, update.ErrUnreachable) {
		t.Fatalf("expected ErrUnreachable, got %v", err)
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
	_, err := src.Latest(context.Background(), update.ChannelStable)
	if err == nil {
		t.Fatal("expected an error on a 500")
	}
	if errors.Is(err, update.ErrNotPublished) || errors.Is(err, update.ErrUnreachable) {
		t.Fatalf("a server error is neither missing nor unreachable, got %v", err)
	}
}

func TestLatestOnABodyThatIsNotAManifestFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/stable.json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>not json</html>"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	if _, err := releasesource.New(srv.URL).Latest(context.Background(), update.ChannelStable); err == nil {
		t.Fatal("expected an error decoding a body that is not a manifest")
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
	if _, err := src.Fetch(context.Background(), srv.URL+"/missing"); !errors.Is(err, update.ErrNotPublished) {
		t.Fatalf("a missing file should wrap ErrNotPublished, got %v", err)
	}
}

// trickle serves size bytes in chunks, pausing between them, and then — when
// stall is set — stops sending without closing.
func trickle(t *testing.T, size, chunks int, pause, stall time.Duration) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(size))
		flusher := w.(http.Flusher)
		chunk := size / chunks
		for n := 0; n < chunks; n++ {
			if n == chunks-1 {
				chunk = size - chunk*(chunks-1)
			}
			if n == chunks-1 && stall > 0 {
				select {
				case <-time.After(stall):
				case <-r.Context().Done():
					return
				}
			}
			_, _ = w.Write(bytes.Repeat([]byte{'x'}, chunk))
			flusher.Flush()
			time.Sleep(pause)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/aosd_v0.10.0_linux_amd64"
}

// A release asset is tens of megabytes: aosd alone is about 50. The client's
// 30-second timeout covered reading the body, so on any link slower than
// about 1.7 MB/s every download failed partway, reported as the release
// channel answering with an error. The clock that matters is how long the
// transfer goes without progress, not how long it takes.
func TestFetchOfAReleaseStillArrivingIsNotCutOff(t *testing.T) {
	url := trickle(t, 64<<10, 8, 60*time.Millisecond, 0)
	src := releasesource.New("http://unused.test")
	src.Timeout = 200 * time.Millisecond

	data, err := src.Fetch(context.Background(), url)
	if err != nil {
		t.Fatalf("a transfer that keeps making progress for longer than the timeout should finish, got %v", err)
	}
	if len(data) != 64<<10 {
		t.Fatalf("got %d bytes", len(data))
	}
}

// A transfer that stops making progress is still given up on, and it is the
// network's failure — what the domain tells the person to check.
func TestFetchOfATransferThatStallsGivesUp(t *testing.T) {
	url := trickle(t, 64<<10, 4, 0, 2*time.Second)
	src := releasesource.New("http://unused.test")
	src.Timeout = 200 * time.Millisecond

	start := time.Now()
	_, err := src.Fetch(context.Background(), url)
	if !errors.Is(err, update.ErrUnreachable) {
		t.Fatalf("a stalled transfer should wrap ErrUnreachable, got %v", err)
	}
	if waited := time.Since(start); waited > 1500*time.Millisecond {
		t.Fatalf("gave up after %s, not after the stall", waited)
	}
}

// The feed is not signed, and Fetch buffers what it reads: an answer larger
// than any release asset is refused rather than read into memory whole.
func TestFetchRefusesAnAnswerLargerThanAnyRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		chunk := bytes.Repeat([]byte{'x'}, 1<<20)
		for n := 0; n < 8; n++ {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)
	src := releasesource.New("http://unused.test")
	src.MaxSize = 4 << 20

	if _, err := src.Fetch(context.Background(), srv.URL+"/huge"); err == nil || errors.Is(err, update.ErrUnreachable) {
		t.Fatalf("an 8 MB answer over a 4 MB limit should be refused as a bad answer, got %v", err)
	}
}
