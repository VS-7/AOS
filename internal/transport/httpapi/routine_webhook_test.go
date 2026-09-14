package httpapi_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/domain/routine"
	"github.com/OWNER/aos/internal/transport/httpapi"
)

// hooks accepts one token for one routine and records what it was asked.
type hooks struct {
	mu        sync.Mutex
	verified  []routine.WebhookInput
	workspace []string
	fired     chan routine.WebhookInput
	cancelled chan bool
}

func newHooks() *hooks {
	return &hooks{fired: make(chan routine.WebhookInput, 4), cancelled: make(chan bool, 4)}
}

func (h *hooks) VerifyWebhook(ctx context.Context, in routine.WebhookInput) error {
	h.mu.Lock()
	h.verified = append(h.verified, in)
	h.workspace = append(h.workspace, identity.From(ctx).WorkspaceID)
	h.mu.Unlock()
	if in.ID != "r-1" || in.Token != "hook-token" {
		return apperr.New("ROUTINE_FIRE_INVALID_TOKEN").
			Msgf("this request is not authorised to fire that routine").
			Status(apperr.StatusUnauthorized).
			CTA(apperr.CallToAction{Label: "rotate it"})
	}
	return nil
}

func (h *hooks) FireWebhook(ctx context.Context, in routine.WebhookInput) (*routine.Run, error) {
	// The run outlives the request that asked for it; give the handler time to
	// answer and the client time to hang up before looking at the context.
	time.Sleep(50 * time.Millisecond)
	h.cancelled <- ctx.Err() != nil
	h.fired <- in
	return &routine.Run{ID: "run-1"}, nil
}

func withHooks(h *hooks) func(*httpapi.Config) {
	return func(c *httpapi.Config) { c.RoutineWebhooks = h }
}

func hookRequest(t *testing.T, srv *harness, method, path, body, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, srv.server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := srv.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	return res
}

// TestAWebhookFiresARoutineWithItsTokenAlone. The token is the credential: the
// sender is a deploy script or a form service with no session on this daemon,
// so the route sits outside the authenticated group and asks for nothing else.
func TestAWebhookFiresARoutineWithItsTokenAlone(t *testing.T) {
	h := newHooks()
	srv := newHarness(t, withHooks(h))

	res := hookRequest(t, srv, http.MethodPost, "/api/hooks/routines/r-1?workspace=vs",
		`{"ref":"main","commit":"abc"}`, "hook-token")
	if res.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("status = %d: %s", res.StatusCode, raw)
	}
	if srv.auth.calls != 0 {
		t.Fatal("the webhook route consulted the session authenticator")
	}
	accepted := decode[map[string]any](t, res)
	if accepted["routine"] != "r-1" {
		t.Fatalf("answer = %+v", accepted)
	}

	select {
	case in := <-h.fired:
		if in.Payload["ref"] != "main" || in.Token != "hook-token" {
			t.Fatalf("fired with %+v", in)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the routine was never fired")
	}
	// The run is a whole model turn and continues after the answer went out.
	if <-h.cancelled {
		t.Fatal("the run was handed a context that ends with the request")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.workspace) != 1 || h.workspace[0] != "vs" {
		t.Fatalf("addressed workspace %v", h.workspace)
	}
}

// TestAWebhookWithABadTokenFiresNothing.
func TestAWebhookWithABadTokenFiresNothing(t *testing.T) {
	h := newHooks()
	srv := newHarness(t, withHooks(h))

	for _, token := range []string{"guessed", ""} {
		res := hookRequest(t, srv, http.MethodPost, "/api/hooks/routines/r-1", `{}`, token)
		if res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("token %q: status = %d", token, res.StatusCode)
		}
	}
	select {
	case in := <-h.fired:
		t.Fatalf("a refused webhook fired %+v", in)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestAWebhookIsAPostWithABoundedBody.
func TestAWebhookIsAPostWithABoundedBody(t *testing.T) {
	h := newHooks()
	srv := newHarness(t, withHooks(h))

	if res := hookRequest(t, srv, http.MethodGet, "/api/hooks/routines/r-1", "", "hook-token"); res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET: status = %d", res.StatusCode)
	}
	huge := `{"x":"` + strings.Repeat("x", 2<<20) + `"}`
	if res := hookRequest(t, srv, http.MethodPost, "/api/hooks/routines/r-1", huge, "hook-token"); res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("huge body: status = %d", res.StatusCode)
	}
	select {
	case <-h.fired:
		t.Fatal("a refused webhook fired")
	case <-time.After(100 * time.Millisecond):
	}
}

// TestAWebhookBodyThatIsNotAnObjectIsHandedOverAsIs. Senders post form fields,
// plain text, arrays; the routine gets what arrived rather than a refusal.
func TestAWebhookBodyThatIsNotAnObjectIsHandedOverAsIs(t *testing.T) {
	h := newHooks()
	srv := newHarness(t, withHooks(h))

	res := hookRequest(t, srv, http.MethodPost, "/api/hooks/routines/r-1", "status=deployed", "hook-token")
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d", res.StatusCode)
	}
	select {
	case in := <-h.fired:
		if in.Payload["body"] != "status=deployed" {
			t.Fatalf("payload = %+v", in.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the routine was never fired")
	}
}

// TestTheWebhookRouteIsAbsentWhenUnconfigured.
func TestTheWebhookRouteIsAbsentWhenUnconfigured(t *testing.T) {
	srv := newHarness(t)
	if res := hookRequest(t, srv, http.MethodPost, "/api/hooks/routines/r-1", `{}`, "hook-token"); res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", res.StatusCode)
	}
}
