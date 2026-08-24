package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
)

// DefaultTimeout bounds one call to a provider. It is generous because a
// reasoning model on a hard question legitimately takes minutes, and the loop
// has its own per-step ceiling above it.
const DefaultTimeout = 10 * time.Minute

// Client is the plumbing every adapter shares: one JSON POST, one error
// translation, one reader for server-sent events.
//
// It exists so that "the provider returned 429" reads the same whichever
// provider it was, and so that a new adapter is a request shape and a response
// shape rather than a fourth copy of this.
type Client struct {
	HTTP    *http.Client
	BaseURL string
	Headers map[string]string

	// Auth is called per request so that a token refreshed between calls is
	// the one that gets used.
	Auth func(ctx context.Context, r *http.Request) error

	// Provider names the adapter in errors.
	Provider string
}

// NewClient builds a client with a bounded timeout.
func NewClient(provider, baseURL string, headers map[string]string) *Client {
	return &Client{
		HTTP:     &http.Client{Timeout: DefaultTimeout},
		BaseURL:  strings.TrimRight(baseURL, "/"),
		Headers:  headers,
		Provider: provider,
	}
}

// PostJSON sends a request and decodes the answer.
func (c *Client) PostJSON(ctx context.Context, path string, body, out any) error {
	res, err := c.post(ctx, path, body, false)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return errTransport(c.Provider, err)
	}
	if res.StatusCode >= 300 {
		return errStatus(c.Provider, res.StatusCode, raw)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return errUnreadable(c.Provider, err, raw)
	}
	return nil
}

// GetJSON reads a resource and decodes the answer.
//
// The one call shape that is not a POST. Every provider publishes its
// catalogue on a plain GET, and routing that through post with a nil body
// would send `null` as a payload to an endpoint that takes none.
func (c *Client) GetJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return errTransport(c.Provider, err)
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	if c.Auth != nil {
		if err := c.Auth(ctx, req); err != nil {
			return err
		}
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return errTransport(c.Provider, err)
	}
	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return errTransport(c.Provider, err)
	}
	if res.StatusCode >= 300 {
		return errStatus(c.Provider, res.StatusCode, raw)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return errUnreadable(c.Provider, err, raw)
	}
	return nil
}

// PostSSE sends a request and returns a reader over the event stream.
func (c *Client) PostSSE(ctx context.Context, path string, body any) (*EventReader, error) {
	res, err := c.post(ctx, path, body, true)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		raw, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		return nil, errStatus(c.Provider, res.StatusCode, raw)
	}
	return &EventReader{body: res.Body, scanner: bufio.NewScanner(res.Body)}, nil
}

func (c *Client) post(ctx context.Context, path string, body any, stream bool) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, errTransport(c.Provider, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, errTransport(c.Provider, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	if c.Auth != nil {
		if err := c.Auth(ctx, req); err != nil {
			return nil, err
		}
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, errTransport(c.Provider, err)
	}
	return res, nil
}

// EventReader reads server-sent events.
//
// The format is three lines and a blank one, and every provider that streams
// uses it. What differs is what is inside the data field, which is the
// adapter's business rather than this one's.
type EventReader struct {
	body    io.ReadCloser
	scanner *bufio.Scanner
	buf     []byte
}

// Event is one message off the wire.
type Event struct {
	Name string
	Data []byte
}

// Next returns the next event, or io.EOF at the end of the stream.
func (r *EventReader) Next() (Event, error) {
	var e Event
	if r.buf == nil {
		r.buf = make([]byte, 0, 64*1024)
		r.scanner.Buffer(r.buf, 4*1024*1024)
	}
	var data []string
	for r.scanner.Scan() {
		line := r.scanner.Text()
		switch {
		case line == "":
			if len(data) == 0 {
				continue // a keep-alive, or the space between two events
			}
			e.Data = []byte(strings.Join(data, "\n"))
			return e, nil
		case strings.HasPrefix(line, ":"):
			// a comment, which is how servers keep a connection warm
		case strings.HasPrefix(line, "event:"):
			e.Name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := r.scanner.Err(); err != nil {
		return Event{}, err
	}
	if len(data) > 0 {
		e.Data = []byte(strings.Join(data, "\n"))
		return e, nil
	}
	return Event{}, io.EOF
}

// Close releases the connection.
func (r *EventReader) Close() error { return r.body.Close() }

func errTransport(provider string, cause error) error {
	return apperr.New("PROVIDER_UNREACHABLE").
		Causer("providers."+provider).
		Msgf("could not reach the %s provider", provider).
		Issue("provider", provider).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check the network, then the provider's status page"})
}

// errStatus translates the provider's own status into ours, keeping the two
// cases a person can act on distinct: a credential problem and a rate limit.
func errStatus(provider string, status int, body []byte) error {
	e := apperr.New("PROVIDER_REFUSED").
		Causer("providers."+provider).
		Msgf("the %s provider answered %d: %s", provider, status, snippet(body)).
		Issue("provider", provider).
		Issue("status", status).
		Status(apperr.StatusBadGateway)

	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		e = e.CTA(apperr.CallToAction{
			Label: "the credential for this provider was refused; check the key in the configuration",
			Tool:  "config_update",
		})
	case http.StatusTooManyRequests:
		e = e.CTA(apperr.CallToAction{
			Label: "the provider is rate limiting; wait, or configure a different provider for this agent",
		})
	default:
		e = e.CTA(apperr.CallToAction{
			Label: "the provider's own message is in the error; it usually names the field it did not like",
		})
	}
	return e
}

func errUnreadable(provider string, cause error, body []byte) error {
	return apperr.New("PROVIDER_UNREADABLE").
		Causer("providers."+provider).
		Msgf("the %s provider answered with something this build cannot read", provider).
		Issue("provider", provider).
		Issue("body", snippet(body)).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label: "the provider may have changed its API; the first part of what it sent is in the issue",
		})
}

func snippet(body []byte) string {
	const max = 400
	s := strings.TrimSpace(string(body))
	if len(s) > max {
		return s[:max] + "…"
	}
	if s == "" {
		return "(an empty body)"
	}
	return s
}

// JSONString renders a value the way a provider that wants arguments as a
// string expects them.
func JSONString(v json.RawMessage) string {
	if len(v) == 0 {
		return "{}"
	}
	return string(v)
}

// ToolArguments parses what a provider sent back as a JSON string, tolerating
// the empty case a model produces for a tool with no required fields.
func ToolArguments(s string) json.RawMessage {
	s = strings.TrimSpace(s)
	if s == "" {
		return json.RawMessage("{}")
	}
	if !json.Valid([]byte(s)) {
		// A malformed payload is passed through rather than repaired: repair
		// is deliberately disabled here as in the original, and the validation
		// error the tool produces is what teaches the model the schema.
		return json.RawMessage(fmt.Sprintf("%q", s))
	}
	return json.RawMessage(s)
}
