// Package openapiclient is the rest-api implementation of toolset.Adapter: it
// reads a target's OpenAPI document and turns every operation in it into a
// callable tool — the "generates tools automatically" half of rest-api's own
// design, rather than asking a skill to hand-declare each endpoint's shape.
package openapiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/OWNER/aos/internal/adapters/netguard"
	"github.com/OWNER/aos/internal/domain/toolset"
)

// errNotConnected fires when ListTools or Call runs before Connect — the
// adapter has no document, so there is nothing to look an operation up in.
var errNotConnected = errors.New("rest-api adapter: not connected")

// operation is one path+method the connected document declared, keyed by the
// tool name ListTools published it under.
type operation struct {
	method string
	path   string
	op     *openapi3.Operation
}

// Adapter is the rest-api implementation of toolset.Adapter. A fresh one is
// built per call — see toolset.Factory's own doc comment — so the document it
// parsed and the *http.Client it built never outlive the one Service.Call
// that owns them.
type Adapter struct {
	server  *url.URL
	headers map[string]string
	ops     map[string]operation
	client  *http.Client
}

// New builds a fresh rest-api adapter. Its signature is already
// func() toolset.Adapter, so it is used directly as a toolset.Factory —
// Adapters{toolset.RESTAPI: openapiclient.New} — the same shape mcpclient.NewHTTP
// gives toolset.MCPHTTP.
func New() toolset.Adapter { return &Adapter{} }

// Connect fetches the OpenAPI document ts.BaseURL names, parses it, and
// indexes every operation it declares by the tool name ListTools will later
// publish. ts arrives with every ${env.VAR} placeholder already resolved —
// this adapter never sees the literal placeholder, only the header value or
// setting it named.
//
// It does not hold the connection open — there is nothing to hold open, a
// document fetch is one round trip — so ListTools and Call each reuse the
// index this builds rather than the network call itself.
func (a *Adapter) Connect(ctx context.Context, ts toolset.Toolset) error {
	if strings.TrimSpace(ts.BaseURL) == "" {
		return fmt.Errorf("rest-api toolset %q has no baseUrl — it must point at the target's OpenAPI document", ts.ID)
	}
	specURL, err := url.Parse(ts.BaseURL)
	if err != nil {
		return fmt.Errorf("rest-api toolset %q: baseUrl is not a valid URL: %w", ts.ID, err)
	}

	// netguard.Transport enforces the installing skill's permissions.network
	// allowlist, when Service.Call attached one to ctx — on this fetch and on
	// every subsequent Call this client makes, including a request an
	// OpenAPI document's own servers[] points somewhere other than BaseURL.
	client := &http.Client{Transport: netguard.Transport(ctx, nil)}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.BaseURL, nil)
	if err != nil {
		return fmt.Errorf("rest-api toolset %q: building the request for its OpenAPI document: %w", ts.ID, err)
	}
	for k, v := range ts.Headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching the OpenAPI document for toolset %q: %w", ts.ID, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("fetching the OpenAPI document for toolset %q: HTTP %d", ts.ID, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading the OpenAPI document for toolset %q: %w", ts.ID, err)
	}

	doc, err := openapi3.NewLoader().LoadFromDataWithPath(body, specURL)
	if err != nil {
		return fmt.Errorf("parsing the OpenAPI document for toolset %q: %w", ts.ID, err)
	}

	a.server = resolveServer(doc, specURL)
	a.headers = ts.Headers
	a.ops = indexOperations(doc)
	a.client = client
	return nil
}

// resolveServer picks the base URL every operation's path is joined to. A
// document's own servers[0].url wins when it names one and does not carry an
// unresolved {variable} — this adapter does not implement server variable
// substitution, only the common case of a literal URL. Anything else falls
// back to the origin the document itself was fetched from, which is right
// for the frequent case of a spec that declares no servers at all, or one
// relative to itself.
func resolveServer(doc *openapi3.T, specURL *url.URL) *url.URL {
	if len(doc.Servers) > 0 {
		raw := strings.TrimSpace(doc.Servers[0].URL)
		if raw != "" && !strings.Contains(raw, "{") {
			if u, err := specURL.Parse(raw); err == nil {
				return u
			}
		}
	}
	origin := *specURL
	origin.Path, origin.RawQuery, origin.Fragment = "", "", ""
	return &origin
}

// indexOperations walks every path and method the document declares and
// names each one the way ListTools will publish it — see toolName.
func indexOperations(doc *openapi3.T) map[string]operation {
	ops := map[string]operation{}
	if doc.Paths == nil {
		return ops
	}
	for _, path := range doc.Paths.InMatchingOrder() {
		item := doc.Paths.Find(path)
		if item == nil {
			continue
		}
		for method, op := range item.Operations() {
			ops[toolName(op, method, path)] = operation{method: method, path: path, op: op}
		}
	}
	return ops
}

// toolName is operationId when the document declares one — the identifier an
// OpenAPI author already gave this exact operation — and a name synthesized
// from the method and path otherwise, so an undeclared operationId does not
// exclude the operation from the toolset.
func toolName(op *openapi3.Operation, method, path string) string {
	if op.OperationID != "" {
		return op.OperationID
	}
	cleaned := strings.Trim(strings.NewReplacer("/", "_", "{", "", "}", "").Replace(path), "_")
	if cleaned == "" {
		return strings.ToLower(method)
	}
	return strings.ToLower(method) + "_" + cleaned
}

// ListTools reports one ToolSpec per operation the connected document
// declared, sorted by name for a stable, diffable listing.
func (a *Adapter) ListTools(context.Context) ([]toolset.ToolSpec, error) {
	if a.ops == nil {
		return nil, errNotConnected
	}
	names := make([]string, 0, len(a.ops))
	for n := range a.ops {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]toolset.ToolSpec, 0, len(names))
	for _, n := range names {
		e := a.ops[n]
		out = append(out, toolset.ToolSpec{
			Name:        n,
			Description: describe(e.op),
			InputSchema: inputSchema(e.op),
		})
	}
	return out, nil
}

// describe prefers an operation's summary — the short label an OpenAPI
// author writes for exactly this purpose — falling back to its longer
// description when no summary exists.
func describe(op *openapi3.Operation) string {
	if op.Summary != "" {
		return op.Summary
	}
	return op.Description
}

// inputSchema builds the JSON Schema ListTools publishes for one operation:
// one property per declared parameter, named exactly as the parameter is, and
// one more named "body" when the operation declares a request body — the
// reserved key Call reads it back from. Path, query and header parameters
// share one flat namespace because OpenAPI itself never lets an operation
// declare two parameters of the same name in different locations.
func inputSchema(op *openapi3.Operation) map[string]any {
	props := map[string]any{}
	var required []string

	for _, ref := range op.Parameters {
		p := ref.Value
		if p == nil || p.Name == "" {
			continue
		}
		props[p.Name] = schemaJSON(p.Schema, p.Description)
		if p.Required {
			required = append(required, p.Name)
		}
	}

	if op.RequestBody != nil && op.RequestBody.Value != nil {
		rb := op.RequestBody.Value
		props["body"] = schemaJSON(bodySchema(rb), rb.Description)
		if rb.Required {
			required = append(required, "body")
		}
	}

	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// bodySchema picks the schema of a request body's application/json media
// type, or its first declared media type when json is not one of them — Call
// makes the same choice when it decides what Content-Type to send.
func bodySchema(rb *openapi3.RequestBody) *openapi3.SchemaRef {
	if mt, ok := rb.Content["application/json"]; ok && mt != nil {
		return mt.Schema
	}
	for _, mt := range rb.Content {
		if mt != nil {
			return mt.Schema
		}
	}
	return nil
}

// schemaJSON renders one parameter or request body's schema as plain JSON
// Schema, keyed off ref.Value rather than the SchemaRef itself: a SchemaRef
// that names a $ref serializes as that bare {"$ref": "..."} otherwise, which
// tells a caller nothing about the shape actually expected. A schema nested
// inside this one that is itself a $ref is relayed as one — this adapter
// resolves one level, not the whole document recursively.
func schemaJSON(ref *openapi3.SchemaRef, fallbackDescription string) map[string]any {
	if ref == nil || ref.Value == nil {
		m := map[string]any{}
		if fallbackDescription != "" {
			m["description"] = fallbackDescription
		}
		return m
	}
	raw, err := json.Marshal(ref.Value)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		m = map[string]any{}
	}
	if _, ok := m["description"]; !ok && fallbackDescription != "" {
		m["description"] = fallbackDescription
	}
	return m
}

// Call runs one operation: input's keys fill the parameters and, under the
// reserved "body" key, the request body the operation's own schema described
// — the shape ListTools just published back to whoever is about to fill it
// in. It relays the response verbatim when the response is valid JSON, which
// covers virtually every real target, and wraps anything else so the return
// value is always valid JSON — the type CallOutput.Result promises.
func (a *Adapter) Call(ctx context.Context, name string, input json.RawMessage) (json.RawMessage, error) {
	if a.ops == nil {
		return nil, errNotConnected
	}
	e, ok := a.ops[name]
	if !ok {
		return nil, fmt.Errorf("no operation named %q in this toolset's OpenAPI document", name)
	}

	args := map[string]any{}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return nil, fmt.Errorf("tool %s: input is not a JSON object: %w", name, err)
		}
	}

	path := e.path
	query := url.Values{}
	headers := map[string]string{}
	for _, ref := range e.op.Parameters {
		p := ref.Value
		if p == nil || p.Name == "" {
			continue
		}
		v, present := args[p.Name]
		if !present {
			if p.Required {
				return nil, fmt.Errorf("tool %s: missing required parameter %q", name, p.Name)
			}
			continue
		}
		s := stringify(v)
		switch p.In {
		case openapi3.ParameterInPath:
			path = strings.ReplaceAll(path, "{"+p.Name+"}", url.PathEscape(s))
		case openapi3.ParameterInQuery:
			query.Set(p.Name, s)
		case openapi3.ParameterInHeader:
			headers[p.Name] = s
		case openapi3.ParameterInCookie:
			headers["Cookie"] = p.Name + "=" + s
		}
	}

	var bodyReader io.Reader
	contentType := ""
	if e.op.RequestBody != nil && e.op.RequestBody.Value != nil {
		rb := e.op.RequestBody.Value
		if v, present := args["body"]; present {
			raw, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("tool %s: encoding \"body\": %w", name, err)
			}
			bodyReader = bytes.NewReader(raw)
			contentType = "application/json"
			if _, ok := rb.Content["application/json"]; !ok {
				for ct := range rb.Content {
					contentType = ct
					break
				}
			}
		} else if rb.Required {
			return nil, fmt.Errorf("tool %s: missing required \"body\"", name)
		}
	}

	target := *a.server
	target.Path = joinPath(a.server.Path, path)
	target.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, e.method, target.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("tool %s: building the request: %w", name, err)
	}
	for k, v := range a.headers {
		req.Header.Set(k, v)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling %s: %w", name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tool %s: reading the response: %w", name, err)
	}

	result := asJSON(respBody)
	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("tool %s reported HTTP %d", name, resp.StatusCode)
	}
	return result, nil
}

// joinPath concatenates a server's own path prefix with an operation's path,
// leaving exactly one slash between them regardless of whether either side
// already carries one.
func joinPath(prefix, suffix string) string {
	return strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(suffix, "/")
}

// stringify renders one argument value the way it belongs in a URL or header:
// a JSON string unquoted, anything else in its plain JSON form.
func stringify(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(raw)
}

// asJSON returns b unchanged when it already is a JSON value, and a small
// JSON envelope around it otherwise — CallOutput.Result is documented as
// opaque JSON, and a REST target is under no obligation to answer with any.
func asJSON(b []byte) json.RawMessage {
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) > 0 && json.Valid(trimmed) {
		return json.RawMessage(trimmed)
	}
	wrapped, err := json.Marshal(map[string]string{"raw": string(b)})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return wrapped
}

// Close releases nothing — a document fetch does not hold a connection open
// — and is always safe to call more than once, same as every other Adapter.
func (a *Adapter) Close() error {
	a.ops = nil
	a.client = nil
	return nil
}
