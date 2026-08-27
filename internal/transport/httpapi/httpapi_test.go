package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/domain/auth"
	"github.com/OWNER/aos/internal/transport/httpapi"
)

type echoInput struct {
	ID   string `json:"id" jsonschema:"The record to touch." validate:"required,notblank"`
	Text string `json:"text,omitempty" jsonschema:"Anything."`
	Boom bool   `json:"boom,omitempty" jsonschema:"Panic instead of answering."`
	Fail bool   `json:"fail,omitempty" jsonschema:"Return a classified error."`

	command.Reasoning
}

type echoOutput struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Workspace string `json:"workspace"`
	Agent     string `json:"agent"`
	User      string `json:"user"`
	RequestID string `json:"requestId"`
}

// fakeAuth resolves one token and refuses everything else.
type fakeAuth struct {
	token string
	user  auth.User
	calls int
}

func (f *fakeAuth) Authenticate(_ context.Context, bearer string) (*auth.User, error) {
	f.calls++
	if bearer == "" || bearer != f.token {
		return nil, apperr.New("TEST_UNAUTHENTICATED").
			Status(apperr.StatusUnauthorized).
			Msgf("no").
			CTA(apperr.CallToAction{Label: "present a token"})
	}
	u := f.user
	return &u, nil
}

func registry(t *testing.T) *command.Registry {
	t.Helper()
	reg := command.NewRegistry()
	err := command.Register(reg, command.Command[echoInput, echoOutput]{
		Group: "records", Name: "touch", Summary: "Touch a record.", Doc: "Touch it.",
		Registry: true,
		Handler: func(ctx context.Context, in echoInput) (echoOutput, error) {
			if in.Boom {
				panic("the handler exploded")
			}
			if in.Fail {
				return echoOutput{}, apperr.New("TEST_REFUSED").
					Causer("test").Msgf("no").Status(apperr.StatusConflict).
					CTA(apperr.CallToAction{Label: "try later"})
			}
			id := identity.From(ctx)
			return echoOutput{
				ID: in.ID, Text: in.Text, Workspace: id.WorkspaceID, Agent: id.AgentID,
				User: id.UserID, RequestID: id.RequestID,
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	reg.DescribeGroup(command.GroupDoc{Name: "records", Tool: "Record", Summary: "Records."})
	return reg
}

type harness struct {
	server *httptest.Server
	auth   *fakeAuth
}

func newHarness(t *testing.T, tweak ...func(*httpapi.Config)) *harness {
	t.Helper()
	fake := &fakeAuth{token: "aos_valid", user: auth.User{ID: "u-1", Username: "vitor", Role: auth.Super}}
	cfg := httpapi.Config{
		Registry:        registry(t),
		Auth:            fake,
		SecurityEnabled: func() bool { return true },
		Log:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:             func() time.Time { return time.Unix(0, 0) },
		NewID:           func() string { return "req-generated" },
	}
	for _, f := range tweak {
		f(&cfg)
	}
	srv := httptest.NewServer(httpapi.New(cfg).Handler())
	t.Cleanup(srv.Close)
	return &harness{server: srv, auth: fake}
}

func (h *harness) do(t *testing.T, method, path string, body any, prepare ...func(*http.Request)) *http.Response {
	t.Helper()
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, h.server.URL+path, payload)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer aos_valid")
	for _, p := range prepare {
		p(req)
	}
	res, err := h.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	return res
}

func decode[T any](t *testing.T, res *http.Response) T {
	t.Helper()
	var envelope struct {
		Data  T              `json:"data"`
		Error map[string]any `json:"error"`
	}
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("body is not an envelope: %v\n%s", err, raw)
	}
	return envelope.Data
}

func errorOf(t *testing.T, res *http.Response) map[string]any {
	t.Helper()
	var envelope struct {
		Error map[string]any `json:"error"`
	}
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("body is not an envelope: %v\n%s", err, raw)
	}
	if envelope.Error == nil {
		t.Fatalf("no error in the envelope:\n%s", raw)
	}
	return envelope.Error
}

// TestEveryCommandOfTheRegistryHasARoute is the property the whole layer exists
// for: the original maintains 26 controllers by hand, and the drift between
// them and the published tools is what this replaces.
func TestEveryCommandOfTheRegistryHasARoute(t *testing.T) {
	reg := registry(t)
	srv := httpapi.New(httpapi.Config{Registry: reg, SecurityEnabled: func() bool { return false }})

	routes := srv.Routes()
	if len(routes) != len(reg.Sorted()) {
		t.Fatalf("%d routes for %d commands", len(routes), len(reg.Sorted()))
	}
	for i, d := range reg.Sorted() {
		if want := "/api/" + d.Group() + "/" + d.Name(); routes[i] != want {
			t.Errorf("route = %q, want %q", routes[i], want)
		}
	}
}

func TestACommandRoundTripsThroughHTTP(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/records/touch", echoInput{
		ID: "r-1", Text: "hello", Reasoning: command.Reasoning{Reasoning: "checking the transport"},
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	got := decode[echoOutput](t, res)
	if got.Text != "hello" {
		t.Fatalf("out = %+v", got)
	}
}

// TestTheRequestIdIsOnEveryResponse: the failures are where somebody has to
// find the matching log line.
func TestTheRequestIdIsOnEveryResponse(t *testing.T) {
	h := newHarness(t)
	for _, c := range []struct {
		name string
		path string
		body any
	}{
		{"success", "/api/records/touch", echoInput{ID: "r-1", Reasoning: command.Reasoning{Reasoning: "r"}}},
		{"domain error", "/api/records/touch", echoInput{ID: "r-1", Fail: true, Reasoning: command.Reasoning{Reasoning: "r"}}},
		{"not found", "/api/records/nope", echoInput{}},
		{"health", "/api/health", nil},
	} {
		method := http.MethodPost
		if c.body == nil {
			method = http.MethodGet
		}
		res := h.do(t, method, c.path, c.body)
		if res.Header.Get(httpapi.HeaderRequestID) == "" {
			t.Errorf("%s: no %s header", c.name, httpapi.HeaderRequestID)
		}
	}
}

func TestTheCallersRequestIdIsKept(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, "/api/health", nil, func(r *http.Request) {
		r.Header.Set(httpapi.HeaderRequestID, "mine-42")
	})
	if got := res.Header.Get(httpapi.HeaderRequestID); got != "mine-42" {
		t.Fatalf("requestId = %q", got)
	}
}

// TestAPanicDegradesOneRequest is defect #16. In the original an unhandled
// rejection takes the process down, so one bad request ends every other one in
// flight.
func TestAPanicDegradesOneRequest(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/records/touch", echoInput{
		ID: "r-1", Boom: true, Reasoning: command.Reasoning{Reasoning: "r"},
	})
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if e := errorOf(t, res); e["code"] != "AOS_HTTP_HANDLER_PANIC" {
		t.Errorf("error = %+v", e)
	}

	// And the daemon is still serving.
	after := h.do(t, http.MethodPost, "/api/records/touch", echoInput{
		ID: "r-1", Text: "still here", Reasoning: command.Reasoning{Reasoning: "r"},
	})
	if after.StatusCode != http.StatusOK {
		t.Fatalf("the server died with the request: %d", after.StatusCode)
	}
}

// TestATokenInTheQueryStringIsIgnored is defect #4. A credential in a URL is in
// every proxy log between here and the caller, in the browser history, and in
// the Referer of the next request.
func TestATokenInTheQueryStringIsIgnored(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost,
		"/api/records/touch?token=aos_valid&access_token=aos_valid",
		echoInput{ID: "r-1", Reasoning: command.Reasoning{Reasoning: "r"}},
		func(r *http.Request) { r.Header.Del("Authorization") },
	)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d — the query string authenticated the request", res.StatusCode)
	}
}

func TestTheTokenIsAcceptedFromHeadersAndCookie(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(*http.Request)
	}{
		{"authorization bearer", func(r *http.Request) { r.Header.Set("Authorization", "Bearer aos_valid") }},
		{"lowercase bearer", func(r *http.Request) { r.Header.Set("Authorization", "bearer aos_valid") }},
		{"x-auth-token", func(r *http.Request) { r.Header.Set(httpapi.HeaderAuthToken, "aos_valid") }},
		{"session cookie", func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "sessionToken", Value: "aos_valid"}) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			res := h.do(t, http.MethodPost, "/api/records/touch",
				echoInput{ID: "r-1", Reasoning: command.Reasoning{Reasoning: "r"}},
				func(r *http.Request) { r.Header.Del("Authorization") },
				c.prepare,
			)
			if res.StatusCode != http.StatusOK {
				t.Fatalf("status = %d", res.StatusCode)
			}
		})
	}
}

func TestAnUnauthenticatedRequestIsRefusedWithACallToAction(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/records/touch",
		echoInput{ID: "r-1", Reasoning: command.Reasoning{Reasoning: "r"}},
		func(r *http.Request) { r.Header.Del("Authorization") },
	)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}
	e := errorOf(t, res)
	if e["cta"] == nil {
		t.Errorf("a 401 must say what to do: %+v", e)
	}
}

// TestSecurityOffLetsARequestThrough is the original's behaviour and what makes
// a loopback installation usable without ceremony. Binding beyond loopback with
// it off is refused at boot, not here.
func TestSecurityOffLetsARequestThrough(t *testing.T) {
	h := newHarness(t, func(c *httpapi.Config) { c.SecurityEnabled = func() bool { return false } })
	res := h.do(t, http.MethodPost, "/api/records/touch",
		echoInput{ID: "r-1", Reasoning: command.Reasoning{Reasoning: "r"}},
		func(r *http.Request) { r.Header.Del("Authorization") },
	)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if h.auth.calls != 0 {
		t.Error("authentication ran even though security is off")
	}
}

// TestHealthIsReachableWithoutACredential: the gateway polls it to decide
// whether the daemon it just started is actually serving, and it has no
// credential at that moment.
func TestHealthIsReachableWithoutACredential(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, "/api/health", nil, func(r *http.Request) {
		r.Header.Del("Authorization")
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var body map[string]any
	raw, _ := io.ReadAll(res.Body)
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("health = %s", raw)
	}
}

// TestTheDocsRouteIsAuthenticatedAndOptional is defect #3: the original serves
// its playground with `security: () => true`.
func TestTheDocsRouteIsAuthenticatedAndOptional(t *testing.T) {
	off := newHarness(t)
	if res := off.do(t, http.MethodGet, "/api/docs", nil); res.StatusCode != http.StatusNotFound {
		t.Fatalf("with docs disabled, status = %d", res.StatusCode)
	}

	on := newHarness(t, func(c *httpapi.Config) { c.DocsEnabled = true })
	res := on.do(t, http.MethodGet, "/api/docs", nil, func(r *http.Request) {
		r.Header.Del("Authorization")
	})
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated docs, status = %d", res.StatusCode)
	}
	if res := on.do(t, http.MethodGet, "/api/docs", nil); res.StatusCode != http.StatusOK {
		t.Fatalf("authenticated docs, status = %d", res.StatusCode)
	}
}

// TestTheDocsRouteStaysGuardedEvenWithSecurityOff: the playground lists every
// schema, and turning off authentication for convenience on loopback is not a
// decision to publish the surface.
func TestTheDocsRouteStaysGuardedEvenWithSecurityOff(t *testing.T) {
	h := newHarness(t, func(c *httpapi.Config) {
		c.DocsEnabled = true
		c.SecurityEnabled = func() bool { return false }
	})
	res := h.do(t, http.MethodGet, "/api/docs", nil, func(r *http.Request) {
		r.Header.Del("Authorization")
	})
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestTheAmbientIdentityReachesTheHandler(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/records/touch",
		echoInput{ID: "r-1", Reasoning: command.Reasoning{Reasoning: "r"}},
		func(r *http.Request) {
			r.Header.Set(httpapi.HeaderWorkspace, "project-alpha")
			r.Header.Set(httpapi.HeaderAgent, "ATLAS")
			r.Header.Set(httpapi.HeaderRequestID, "req-7")
		},
	)
	got := decode[echoOutput](t, res)
	if got.Workspace != "project-alpha" {
		t.Errorf("workspace = %q", got.Workspace)
	}
	if got.Agent != "atlas" {
		t.Errorf("agent = %q, want it lowercased like every other slug", got.Agent)
	}
	if got.User != "u-1" {
		t.Errorf("user = %q", got.User)
	}
	if got.RequestID != "req-7" {
		t.Errorf("requestId = %q", got.RequestID)
	}
}

// TestTheWorkspaceMayComeFromTheQueryString: it is not a credential, and a link
// that opens one workspace is useful.
func TestTheWorkspaceMayComeFromTheQueryString(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/records/touch?workspace=from-query",
		echoInput{ID: "r-1", Reasoning: command.Reasoning{Reasoning: "r"}})
	if got := decode[echoOutput](t, res).Workspace; got != "from-query" {
		t.Fatalf("workspace = %q", got)
	}
}

func TestADomainErrorKeepsItsStatusAndItsCallToAction(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/records/touch", echoInput{
		ID: "r-1", Fail: true, Reasoning: command.Reasoning{Reasoning: "r"},
	})
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d", res.StatusCode)
	}
	e := errorOf(t, res)
	if e["code"] != "AOS_TEST_REFUSED" || e["cta"] == nil {
		t.Fatalf("error = %+v", e)
	}
}

func TestAnInvalidPayloadIsRejectedByTheCommandLayer(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/records/touch", map[string]any{
		"text": "no identifier", "_reasoning": "r",
	})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if e := errorOf(t, res); e["cta"] == nil {
		t.Errorf("a 400 must say what to do: %+v", e)
	}
}

// TestReasoningIsNotRequiredOverHTTP records the surface boundary rather than
// discovering it later. `_reasoning` is a tool-surface obligation: it exists so
// a model states why it is calling, and it is enforced on MCP and on the agent
// registry. HTTP is what the terminal and the desktop app talk to, and neither
// is a model. Requiring it here would mean every `curl` carried a sentence
// explaining itself to nobody.
func TestReasoningIsNotRequiredOverHTTP(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/records/touch", map[string]any{"id": "r-1"})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestAnUnknownRouteAndAWrongMethodAreExplained(t *testing.T) {
	h := newHarness(t)
	missing := h.do(t, http.MethodPost, "/api/records/nope", map[string]any{})
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", missing.StatusCode)
	}
	if e := errorOf(t, missing); e["cta"] == nil {
		t.Errorf("a 404 must say what to do: %+v", e)
	}

	wrong := h.do(t, http.MethodGet, "/api/records/touch", nil)
	if wrong.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", wrong.StatusCode)
	}
}

func TestABodyLargerThanTheLimitIsRefused(t *testing.T) {
	h := newHarness(t)
	huge := strings.Repeat("x", 5<<20)
	res := h.do(t, http.MethodPost, "/api/records/touch", echoInput{
		ID: "r-1", Text: huge, Reasoning: command.Reasoning{Reasoning: "r"},
	})
	if res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestAnEmptyBodyIsTreatedAsAnEmptyObject(t *testing.T) {
	// Not for convenience: a command whose every field is optional is a
	// legitimate call with no payload, and curl sends no body by default.
	h := newHarness(t, func(c *httpapi.Config) { c.SecurityEnabled = func() bool { return false } })
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		h.server.URL+"/api/records/touch", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := h.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	// It reaches validation rather than failing to parse, which is the point.
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

// TestCorsAllowsOnlyTheDeclaredOrigins: the daemon listens on the machine the
// browser is running on, so a permissive policy means any page the person opens
// can talk to it.
func TestCorsAllowsOnlyTheDeclaredOrigins(t *testing.T) {
	h := newHarness(t, func(c *httpapi.Config) {
		c.AllowedOrigins = []string{"http://localhost:5173"}
	})

	allowed := h.do(t, http.MethodOptions, "/api/records/touch", nil, func(r *http.Request) {
		r.Header.Set("Origin", "http://localhost:5173")
	})
	if got := allowed.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("allowed origin = %q", got)
	}

	refused := h.do(t, http.MethodOptions, "/api/records/touch", nil, func(r *http.Request) {
		r.Header.Set("Origin", "https://evil.example")
	})
	if got := refused.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("an unlisted origin was allowed: %q", got)
	}
}

func TestNoOriginIsAllowedByDefault(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodOptions, "/api/records/touch", nil, func(r *http.Request) {
		r.Header.Set("Origin", "http://localhost:5173")
	})
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("origin allowed with an empty allowlist: %q", got)
	}
}

// TestARenamedCommandKeepsAnsweringAtItsOldPath: the consumer of these paths is
// an integration written months ago, and it has to fail the same way on every
// surface (ADR-0011).
func TestARenamedCommandKeepsAnsweringAtItsOldPath(t *testing.T) {
	reg := registry(t)
	reg.MustAlias(command.Alias{
		From: "records_poke", To: "records_touch",
		Since: "0.4.0", RemoveAt: "1.0.0", Deprecated: true,
	})
	srv := httptest.NewServer(httpapi.New(httpapi.Config{
		Registry: reg, SecurityEnabled: func() bool { return false },
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}).Handler())
	defer srv.Close()

	raw, err := json.Marshal(echoInput{ID: "r-1", Text: "via the old name", Reasoning: command.Reasoning{Reasoning: "r"}})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/api/records/poke", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var envelope struct {
		Data       echoOutput                 `json:"data"`
		Deprecated *command.DeprecationNotice `json:"_deprecated"`
	}
	body, _ := io.ReadAll(res.Body)
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Text != "via the old name" {
		t.Fatalf("data = %+v", envelope.Data)
	}
	if envelope.Deprecated == nil || envelope.Deprecated.Use != "records_touch" {
		t.Fatalf("the response does not say where the command moved to: %s", body)
	}
}

// stubHandler is a minimal http.Handler for testing this package's own
// routing and auth decisions around a mount — not the real handler's own
// protocol, which is that handler's own test suite's job
// (internal/transport/mcpserver's for MCP).
func stubHandler(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) })
}

// TestMCPMountIsUnreachableWhenUnconfigured mirrors Realtime's own nil case:
// a build with no MCP-over-HTTP transport wired leaves /mcp unmounted, not
// mounted-and-erroring.
func TestMCPMountIsUnreachableWhenUnconfigured(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/mcp", nil)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 with no MCP handler configured", res.StatusCode)
	}
}

// TestMCPMountRequiresAuthentication is the property /mcp exists to prove
// over Realtime's own "/ws authorises itself" shape: this mount reaches the
// exact same command registry every guarded /api route does, so it must sit
// behind the same bearer-token check, not its own.
func TestMCPMountRequiresAuthentication(t *testing.T) {
	h := newHarness(t, func(c *httpapi.Config) { c.MCP = stubHandler(http.StatusOK) })

	res := h.do(t, http.MethodPost, "/mcp", nil, func(r *http.Request) {
		r.Header.Del("Authorization")
	})
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /mcp, status = %d", res.StatusCode)
	}

	res = h.do(t, http.MethodPost, "/mcp", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("authenticated /mcp, status = %d", res.StatusCode)
	}
}

// TestMCPMountFollowsSecurityEnabledTheSameWayTheGuardedAPIGroupDoes: /mcp
// reaches the exact same command registry the guarded /api group does, so it
// shares that group's posture on the loopback convenience toggle too —
// unlike /api/docs, which stays guarded regardless (see
// TestTheDocsRouteStaysGuardedEvenWithSecurityOff), /mcp is not a second,
// stricter door onto a surface the REST routes already leave open when
// security is off — guardExposure (serve.go) is what stops that
// configuration from ever reaching beyond loopback in the first place.
func TestMCPMountFollowsSecurityEnabledTheSameWayTheGuardedAPIGroupDoes(t *testing.T) {
	h := newHarness(t, func(c *httpapi.Config) {
		c.MCP = stubHandler(http.StatusOK)
		c.SecurityEnabled = func() bool { return false }
	})
	res := h.do(t, http.MethodPost, "/mcp", nil, func(r *http.Request) {
		r.Header.Del("Authorization")
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want /mcp reachable with no credential once security is off, like /api/records/touch already is", res.StatusCode)
	}
}

// The event channel carries the workspace's whole life — every message, every
// task move, every approval request — and it used to be mounted bare.
//
// realtime.Upgrade does authorise, but only by workspace, and a workspace
// with no explicit members admits everybody (workspace.Service.
// AuthorizeWorkspace) — which is every workspace this system creates. So the
// upgrade completed for a caller with no credential at all: `curl` on the
// loopback interface, or any page on the same origin in the browser flavour,
// read the stream. It answers to the same middleware the API does now.
func TestTheEventChannelRequiresACredential(t *testing.T) {
	h := newHarness(t, func(c *httpapi.Config) { c.Realtime = stubHandler(http.StatusOK) })

	res := h.do(t, http.MethodGet, "/ws", nil, func(r *http.Request) {
		r.Header.Del("Authorization")
	})
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /ws, status = %d, want 401", res.StatusCode)
	}

	res = h.do(t, http.MethodGet, "/ws", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("authenticated /ws, status = %d", res.StatusCode)
	}
}

// With security off, /ws is reachable without one — the same posture every
// other surface takes, and the reason guardExposure refuses to bind beyond
// loopback in that configuration.
func TestTheEventChannelFollowsSecurityEnabled(t *testing.T) {
	h := newHarness(t, func(c *httpapi.Config) {
		c.Realtime = stubHandler(http.StatusOK)
		c.SecurityEnabled = func() bool { return false }
	})
	res := h.do(t, http.MethodGet, "/ws", nil, func(r *http.Request) {
		r.Header.Del("Authorization")
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want /ws reachable with security off", res.StatusCode)
	}
}

// A daemon that requires authentication and has nobody to perform it must
// refuse, not wave everything through. The check used to read
// `a == nil || !enabled()`, so a wiring fault produced an open API rather
// than a loud failure.
func TestAMissingAuthenticatorFailsClosedWhileSecurityIsOn(t *testing.T) {
	h := newHarness(t, func(c *httpapi.Config) {
		c.Auth = nil
		c.SecurityEnabled = func() bool { return true }
	})
	res := h.do(t, http.MethodPost, "/api/records/touch", map[string]any{"id": "r-1", "_reasoning": "why"})
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want the request refused rather than served", res.StatusCode)
	}
}
