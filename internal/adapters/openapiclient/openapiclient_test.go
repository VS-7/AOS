package openapiclient_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/adapters/openapiclient"
	"github.com/OWNER/aos/internal/domain/testsuite"
	"github.com/OWNER/aos/internal/domain/toolset"
)

// fixtureSpec is a minimal OpenAPI 3.0 document: one path parameter to read,
// one request body to write. It declares no "servers" on purpose — the
// adapter's own fallback (the origin the document was fetched from) is what
// every test below exercises, since that is what a spec with no servers
// block, or none reachable from wherever this test runs, actually looks like.
const fixtureSpec = `{
  "openapi": "3.0.3",
  "info": {"title": "Fixture API", "version": "1.0.0"},
  "paths": {
    "/users/{id}": {
      "get": {
        "operationId": "get_user",
        "summary": "Get one user.",
        "parameters": [
          {"name": "id", "in": "path", "required": true, "schema": {"type": "string"}}
        ],
        "responses": {"200": {"description": "ok"}}
      }
    },
    "/users": {
      "post": {
        "operationId": "create_user",
        "summary": "Create a user.",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {"name": {"type": "string"}},
                "required": ["name"]
              }
            }
          }
        },
        "responses": {"201": {"description": "created"}}
      }
    }
  }
}`

// newFixtureServer serves fixtureSpec at /openapi.json and implements the two
// operations it declares — the same server plays both roles, spec host and
// API host, which is what resolveServer's fallback to the document's own
// origin is for.
func newFixtureServer(t *testing.T, onRequest func(*http.Request)) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		if onRequest != nil {
			onRequest(r)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureSpec))
	})
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if onRequest != nil {
			onRequest(r)
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "new-1", "name": body["name"]})
	})
	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		if onRequest != nil {
			onRequest(r)
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/users/")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "name": "Ada"})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func fixtureToolset(url string) toolset.Toolset {
	return toolset.Toolset{
		ID: "rest-fixture", Type: toolset.RESTAPI, Status: toolset.StatusEnabled,
		BaseURL: url + "/openapi.json",
	}
}

// TestRESTSatisfiesTheAdapterContract runs the identical behavioural suite
// stdio's and http's own contract tests do, against a real OpenAPI document
// and the two operations it declares.
func TestRESTSatisfiesTheAdapterContract(t *testing.T) {
	ts := newFixtureServer(t, nil)
	testsuite.RunAdapterContract(t, testsuite.AdapterContract{
		Name:    "rest-api",
		New:     func(t *testing.T) toolset.Adapter { t.Helper(); return openapiclient.New() },
		Toolset: fixtureToolset(ts.URL),
		Invalid: toolset.Toolset{
			ID: "unreachable", Type: toolset.RESTAPI, Status: toolset.StatusEnabled,
			BaseURL: "http://127.0.0.1:1/openapi.json",
		},
		ExpectTool: "get_user",
	})
}

// TestRESTCallsAToolWithAPathParameter proves a path parameter named in the
// input JSON actually lands in the URL Call sends, not just anywhere in the
// request.
func TestRESTCallsAToolWithAPathParameter(t *testing.T) {
	ts := newFixtureServer(t, nil)
	a := openapiclient.New()
	ctx := context.Background()
	if err := a.Connect(ctx, fixtureToolset(ts.URL)); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = a.Close() }()

	out, err := a.Call(ctx, "get_user", []byte(`{"id":"u-42"}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("result is not JSON: %s", out)
	}
	if got["id"] != "u-42" {
		t.Fatalf("id = %v, want u-42 — the path parameter did not reach the request", got["id"])
	}
}

// TestRESTCallsAToolWithARequestBody proves the reserved "body" key reaches
// the target as the operation's JSON request body, and that the response
// comes back through Call unmodified.
func TestRESTCallsAToolWithARequestBody(t *testing.T) {
	ts := newFixtureServer(t, nil)
	a := openapiclient.New()
	ctx := context.Background()
	if err := a.Connect(ctx, fixtureToolset(ts.URL)); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = a.Close() }()

	out, err := a.Call(ctx, "create_user", []byte(`{"body":{"name":"Ada Lovelace"}}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("result is not JSON: %s", out)
	}
	if got["name"] != "Ada Lovelace" {
		t.Fatalf("name = %v, want Ada Lovelace — the body did not reach the request", got["name"])
	}
}

// TestRESTMissingRequiredBodyFailsBeforeAnyRequest proves Call refuses a
// required body's absence itself rather than sending an empty one and
// relaying whatever error the target happens to answer with.
func TestRESTMissingRequiredBodyFailsBeforeAnyRequest(t *testing.T) {
	requested := false
	ts := newFixtureServer(t, func(*http.Request) { requested = true })
	a := openapiclient.New()
	ctx := context.Background()
	if err := a.Connect(ctx, fixtureToolset(ts.URL)); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = a.Close() }()
	requested = false // Connect itself fetched the spec; reset before the call under test.

	if _, err := a.Call(ctx, "create_user", nil); err == nil {
		t.Fatal("expected an error for a missing required body")
	}
	if requested {
		t.Error("Call reached the server despite the missing required body")
	}
}

// TestRESTHeadersReachEveryRequest proves Toolset.Headers travels on both the
// spec fetch and every subsequent call — a toolset behind auth would fail to
// even discover its own tools otherwise.
func TestRESTHeadersReachEveryRequest(t *testing.T) {
	var authOnSpec, authOnCall string
	ts := newFixtureServer(t, func(r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "openapi.json") {
			authOnSpec = r.Header.Get("Authorization")
		} else {
			authOnCall = r.Header.Get("Authorization")
		}
	})

	tsConfig := fixtureToolset(ts.URL)
	tsConfig.Headers = map[string]string{"Authorization": "Bearer test-token"}

	a := openapiclient.New()
	ctx := context.Background()
	if err := a.Connect(ctx, tsConfig); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = a.Close() }()
	if authOnSpec != "Bearer test-token" {
		t.Errorf("Authorization on the spec fetch = %q", authOnSpec)
	}

	if _, err := a.Call(ctx, "get_user", []byte(`{"id":"u-1"}`)); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if authOnCall != "Bearer test-token" {
		t.Errorf("Authorization on the call = %q", authOnCall)
	}
}

// TestRESTNonJSONResponseIsWrappedNotCorrupted proves Call never hands back
// bytes that are not themselves valid JSON — CallOutput.Result is documented
// as opaque JSON, and a REST target answering with plain text would otherwise
// corrupt whatever envelope Result is later marshaled into.
func TestRESTNonJSONResponseIsWrappedNotCorrupted(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureSpec))
	})
	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "not json at all")
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	a := openapiclient.New()
	ctx := context.Background()
	if err := a.Connect(ctx, fixtureToolset(ts.URL)); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = a.Close() }()

	out, err := a.Call(ctx, "get_user", []byte(`{"id":"u-1"}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !json.Valid(out) {
		t.Fatalf("result is not valid JSON: %s", out)
	}
}

// searchSpec declares one operation with no operationId — toolName must
// synthesize a name from its method and path — and a query and a header
// parameter instead of fixtureSpec's path parameter, so both reach Call
// through the case that reads them, not just the case that substitutes a
// path segment.
const searchSpec = `{
  "openapi": "3.0.3",
  "info": {"title": "Search Fixture", "version": "1.0.0"},
  "paths": {
    "/search": {
      "get": {
        "summary": "Search.",
        "parameters": [
          {"name": "q", "in": "query", "required": true, "schema": {"type": "string"}},
          {"name": "X-Trace", "in": "header", "required": false, "schema": {"type": "string"}}
        ],
        "responses": {"200": {"description": "ok"}}
      }
    }
  }
}`

func newSearchServer(t *testing.T, onSearch func(*http.Request)) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(searchSpec))
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if onSearch != nil {
			onSearch(r)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"q": r.URL.Query().Get("q")})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// TestRESTToolNameFallsBackToMethodAndPath proves an operation with no
// operationId is still published — under a name built from its method and
// path — rather than silently dropped from the toolset.
func TestRESTToolNameFallsBackToMethodAndPath(t *testing.T) {
	ts := newSearchServer(t, nil)
	a := openapiclient.New()
	ctx := context.Background()
	if err := a.Connect(ctx, toolset.Toolset{
		ID: "search-fixture", Type: toolset.RESTAPI, Status: toolset.StatusEnabled,
		BaseURL: ts.URL + "/openapi.json",
	}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = a.Close() }()

	tools, err := a.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "get_search" {
		t.Fatalf("tools = %+v, want one named get_search", tools)
	}
}

// TestRESTQueryAndHeaderParametersReachTheRequest proves a query parameter
// lands in the URL and a header parameter lands on the request — the two
// parameter locations fixtureSpec's path parameter does not exercise.
func TestRESTQueryAndHeaderParametersReachTheRequest(t *testing.T) {
	var gotQuery, gotTrace string
	ts := newSearchServer(t, func(r *http.Request) {
		gotQuery = r.URL.Query().Get("q")
		gotTrace = r.Header.Get("X-Trace")
	})
	a := openapiclient.New()
	ctx := context.Background()
	if err := a.Connect(ctx, toolset.Toolset{
		ID: "search-fixture", Type: toolset.RESTAPI, Status: toolset.StatusEnabled,
		BaseURL: ts.URL + "/openapi.json",
	}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = a.Close() }()

	if _, err := a.Call(ctx, "get_search", []byte(`{"q":"ada","X-Trace":"abc-123"}`)); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if gotQuery != "ada" {
		t.Errorf("query q = %q, want ada", gotQuery)
	}
	if gotTrace != "abc-123" {
		t.Errorf("header X-Trace = %q, want abc-123", gotTrace)
	}
}

// TestRESTMissingRequiredQueryParameterFailsBeforeAnyRequest mirrors the
// missing-required-body test for a non-body parameter: Call must refuse it
// itself, before a request ever reaches the target.
func TestRESTMissingRequiredQueryParameterFailsBeforeAnyRequest(t *testing.T) {
	requested := false
	ts := newSearchServer(t, func(*http.Request) { requested = true })
	a := openapiclient.New()
	ctx := context.Background()
	if err := a.Connect(ctx, toolset.Toolset{
		ID: "search-fixture", Type: toolset.RESTAPI, Status: toolset.StatusEnabled,
		BaseURL: ts.URL + "/openapi.json",
	}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = a.Close() }()

	if _, err := a.Call(ctx, "get_search", nil); err == nil {
		t.Fatal("expected an error for a missing required query parameter")
	}
	if requested {
		t.Error("Call reached the server despite the missing required parameter")
	}
}

// TestRESTServerFieldIsJoinedWithOperationPaths proves a document that
// declares a relative "servers" entry has that prefix applied to every
// operation's path — the branch resolveServer's fallback to the document's
// own origin does not exercise, since fixtureSpec declares no servers at all.
func TestRESTServerFieldIsJoinedWithOperationPaths(t *testing.T) {
	spec := `{
  "openapi": "3.0.3",
  "info": {"title": "Prefixed Fixture", "version": "1.0.0"},
  "servers": [{"url": "/api"}],
  "paths": {
    "/users/{id}": {
      "get": {
        "operationId": "get_user",
        "parameters": [
          {"name": "id", "in": "path", "required": true, "schema": {"type": "string"}}
        ],
        "responses": {"200": {"description": "ok"}}
      }
    }
  }
}`
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(spec))
	})
	var requestedPath string
	mux.HandleFunc("/api/users/", func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	a := openapiclient.New()
	ctx := context.Background()
	if err := a.Connect(ctx, toolset.Toolset{
		ID: "prefixed-fixture", Type: toolset.RESTAPI, Status: toolset.StatusEnabled,
		BaseURL: ts.URL + "/openapi.json",
	}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = a.Close() }()

	if _, err := a.Call(ctx, "get_user", []byte(`{"id":"u-1"}`)); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if requestedPath != "/api/users/u-1" {
		t.Fatalf("requested path = %q, want /api/users/u-1 — the servers prefix was not applied", requestedPath)
	}
}

// TestRESTConnectFailsOnANonOpenAPIDocument proves a document that does not
// parse as OpenAPI fails Connect with a diagnosable error, rather than
// succeeding with an adapter that has silently indexed zero operations.
func TestRESTConnectFailsOnANonOpenAPIDocument(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "this is not an OpenAPI document")
	}))
	defer ts.Close()

	a := openapiclient.New()
	err := a.Connect(context.Background(), toolset.Toolset{
		ID: "garbage", Type: toolset.RESTAPI, Status: toolset.StatusEnabled,
		BaseURL: ts.URL,
	})
	if err == nil {
		t.Fatal("expected Connect to fail on a non-OpenAPI document")
	}
}

// TestRESTConnectFailsWhenTheDocumentFetchIs4xx proves a document endpoint
// that answers with an HTTP error status fails Connect instead of trying to
// parse whatever error body came back as if it were the spec.
func TestRESTConnectFailsWhenTheDocumentFetchIs4xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	a := openapiclient.New()
	err := a.Connect(context.Background(), toolset.Toolset{
		ID: "missing-doc", Type: toolset.RESTAPI, Status: toolset.StatusEnabled,
		BaseURL: ts.URL + "/openapi.json",
	})
	if err == nil {
		t.Fatal("expected Connect to fail when the document fetch returns 404")
	}
}

// TestRESTConnectRejectsAnEmptyBaseURL proves the "must point at the
// document" error fires before any network call, for the one configuration
// mistake that would otherwise be an unhelpful "unsupported protocol scheme"
// from net/http.
func TestRESTConnectRejectsAnEmptyBaseURL(t *testing.T) {
	a := openapiclient.New()
	err := a.Connect(context.Background(), toolset.Toolset{
		ID: "no-url", Type: toolset.RESTAPI, Status: toolset.StatusEnabled,
	})
	if err == nil {
		t.Fatal("expected Connect to reject a toolset with no baseUrl")
	}
}

// TestRESTConnectRefusesAHostNotInTheAttachedAllowlist proves the guard
// toolset.Service.Call attaches to ctx (see toolset.WithAllowedHosts) is
// actually wired into this adapter's client, not only unit-tested in
// isolation — even the initial OpenAPI document fetch is refused, before any
// operation the document might declare is ever reached.
func TestRESTConnectRefusesAHostNotInTheAttachedAllowlist(t *testing.T) {
	ts := newFixtureServer(t, nil)
	a := openapiclient.New()
	restricted := toolset.WithAllowedHosts(context.Background(), []string{"some-other-host.example.com"})

	if err := a.Connect(restricted, fixtureToolset(ts.URL)); err == nil {
		t.Fatal("Connect must refuse a host outside the attached allowlist")
	}
}

func TestRESTConnectSucceedsWhenTheHostIsInTheAttachedAllowlist(t *testing.T) {
	ts := newFixtureServer(t, nil)
	a := openapiclient.New()
	restricted := toolset.WithAllowedHosts(context.Background(), []string{"127.0.0.1"})

	if err := a.Connect(restricted, fixtureToolset(ts.URL)); err != nil {
		t.Fatalf("Connect refused a host the allowlist actually declares: %v", err)
	}
}
