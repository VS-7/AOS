package netguard_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OWNER/aos/internal/adapters/netguard"
	"github.com/OWNER/aos/internal/domain/toolset"
)

// recordingTransport answers every request with 200 and remembers the host
// it saw, so a test can tell whether Transport let a request through to it
// or refused it beforehand.
type recordingTransport struct{ sawHost string }

func (r *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.sawHost = req.URL.Hostname()
	rec := httptest.NewRecorder()
	rec.WriteHeader(http.StatusOK)
	return rec.Result(), nil
}

func TestTransportRunsNextUnmodifiedWithNoRestrictionAttached(t *testing.T) {
	next := &recordingTransport{}
	rt := netguard.Transport(context.Background(), next)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://anywhere.example.com/", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unrestricted context refused a request: %v", err)
	}
	_ = resp.Body.Close()
	if next.sawHost != "anywhere.example.com" {
		t.Fatalf("the request never reached the wrapped transport: sawHost = %q", next.sawHost)
	}
}

func TestTransportAllowsAnAllowedHost(t *testing.T) {
	ctx := toolset.WithAllowedHosts(context.Background(), []string{"api.example.com"})
	next := &recordingTransport{}
	rt := netguard.Transport(ctx, next)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.example.com/v1", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("an allowed host was refused: %v", err)
	}
	_ = resp.Body.Close()
	if next.sawHost != "api.example.com" {
		t.Fatalf("sawHost = %q", next.sawHost)
	}
}

func TestTransportAllowsAHostRegardlessOfCase(t *testing.T) {
	ctx := toolset.WithAllowedHosts(context.Background(), []string{"API.Example.com"})
	next := &recordingTransport{}
	rt := netguard.Transport(ctx, next)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.example.com/v1", nil)
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatalf("a host differing only in case was refused: %v", err)
	}
}

// TestTransportRefusesAHostNotDeclared is the exact scenario the design doc
// names: an OpenAPI document (or an MCP server's own redirect) pointing at
// a host the skill never declared under permissions.network — not only a
// mismatch against the toolset's own BaseURL, since this check runs per
// request, not once at Connect.
func TestTransportRefusesAHostNotDeclared(t *testing.T) {
	ctx := toolset.WithAllowedHosts(context.Background(), []string{"api.example.com"})
	next := &recordingTransport{}
	rt := netguard.Transport(ctx, next)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://attacker.example.com/steal", nil)
	if _, err := rt.RoundTrip(req); err == nil {
		t.Fatal("a request to an undeclared host must be refused")
	}
	if next.sawHost != "" {
		t.Fatal("a refused request must never reach the wrapped transport")
	}
}

// TestTransportRefusesEveryRequestWhenTheAllowlistIsEmpty: an explicitly
// empty allowlist restricts to nothing, rather than being read as "no
// restriction" — see toolset.WithAllowedHosts' own doc on why the two must
// not collapse into each other.
func TestTransportRefusesEveryRequestWhenTheAllowlistIsEmpty(t *testing.T) {
	ctx := toolset.WithAllowedHosts(context.Background(), nil)
	next := &recordingTransport{}
	rt := netguard.Transport(ctx, next)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://anything.example.com/", nil)
	if _, err := rt.RoundTrip(req); err == nil {
		t.Fatal("a request must be refused when the allowlist is empty")
	}
	if next.sawHost != "" {
		t.Fatal("the refused request must never reach the wrapped transport")
	}
}

func TestTransportDefaultsToDefaultTransportWhenNextIsNil(t *testing.T) {
	ctx := toolset.WithAllowedHosts(context.Background(), []string{"127.0.0.1"})
	rt := netguard.Transport(ctx, nil)
	if rt == nil {
		t.Fatal("Transport returned nil")
	}
}
