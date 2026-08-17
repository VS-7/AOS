// Package daemonclient reaches the daemon's HTTP surface.
//
// It exists so the desktop can be what the dependency rule says it is: a client
// of the daemon rather than a second copy of it. Nothing here knows about the
// domain — it moves a command key and a JSON payload, and the answer comes back
// having been validated by the one registry that exists.
package daemonclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/transport/wailsvc"
)

// Client calls the daemon.
type Client struct {
	base      string
	token     atomic.Pointer[string]
	workspace atomic.Pointer[string]
	http      *http.Client
}

// Options configure the client.
type Options struct {
	// BaseURL is where the daemon answers, without a trailing slash.
	BaseURL string

	// Token authenticates the call. The desktop reads it from the same state
	// directory the daemon wrote it to.
	Token string

	// Workspace is sent as a header rather than a cookie. That is defect #5 of
	// the original: a cookie is attached by the browser to a WebSocket upgrade
	// whether or not the page meant it to.
	Workspace string

	// Timeout bounds one call. A turn is not taken here — the daemon runs it
	// and the answer arrives over the realtime channel — so this is short.
	Timeout time.Duration
}

// New builds a client.
func New(opts Options) *Client {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	c := &Client{
		base: strings.TrimSuffix(opts.BaseURL, "/"),
		http: &http.Client{Timeout: timeout},
	}
	c.SetToken(opts.Token)
	c.SetWorkspace(opts.Workspace)
	return c
}

// SetToken replaces the token later calls authenticate with.
//
// It exists because the desktop constructs this client before the daemon it
// talks to is necessarily running yet — ensureDaemon starts it in the
// background so a slow or failing daemon never blocks the window from
// opening. On a first run, the token this client was built with may be empty
// because nothing had written it yet; once the daemon is confirmed healthy,
// the caller reads it again and sets it here, without reconstructing the
// client or losing anything in flight.
func (c *Client) SetToken(token string) { c.token.Store(&token) }

func (c *Client) currentToken() string {
	if v := c.token.Load(); v != nil {
		return *v
	}
	return ""
}

// SetWorkspace replaces the workspace id later calls are scoped to. Set once
// the desktop learns it from workspace_introspect — the directory it was
// launched against, resolved to a registered workspace — for the same reason
// SetToken exists: that answer is not known at construction time.
func (c *Client) SetWorkspace(id string) { c.workspace.Store(&id) }

func (c *Client) currentWorkspace() string {
	if v := c.workspace.Load(); v != nil {
		return *v
	}
	return ""
}

// Invoke runs one command.
//
// The command key maps to the path the HTTP surface publishes: memories_store
// becomes /api/memories/store. There is one rule and it is the same one the
// frontend's browser transport uses, so a path that works in one works in both.
func (c *Client) Invoke(ctx context.Context, key string, input json.RawMessage) (json.RawMessage, error) {
	path := "/api/" + strings.ReplaceAll(key, "_", "/")
	body := bytes.NewReader(input)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, body)
	if err != nil {
		return nil, errUnreachable(c.base, err)
	}
	req.Header.Set("content-type", "application/json")
	if token := c.currentToken(); token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}
	if workspace := c.currentWorkspace(); workspace != "" {
		req.Header.Set("x-workspace-id", workspace)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, errUnreachable(c.base, err)
	}
	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errUnreachable(c.base, err)
	}
	// The envelope carries the error, so a non-2xx status is passed through
	// rather than replaced: the domain already said what went wrong and in
	// which terms, and restating it here would lose the code and the call to
	// action the interface reads.
	return raw, nil
}

// Commands lists what the daemon publishes.
func (c *Client) Commands(ctx context.Context) ([]wailsvc.CommandInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/_commands", nil)
	if err != nil {
		return nil, errUnreachable(c.base, err)
	}
	if token := c.currentToken(); token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, errUnreachable(c.base, err)
	}
	defer func() { _ = res.Body.Close() }()

	var out []wailsvc.CommandInfo
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, errUnreadable(c.base, err)
	}
	return out, nil
}

// Ready reports whether the daemon is answering its health check.
func (c *Client) Ready(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/health", nil)
	if err != nil {
		return false, nil //nolint:nilerr // a malformed address is "not ready", not an incident
	}
	res, err := c.http.Do(req)
	if err != nil {
		return false, nil //nolint:nilerr // the daemon is not up yet, which is what the splash is waiting to stop being true
	}
	defer func() { _ = res.Body.Close() }()
	return res.StatusCode == http.StatusOK, nil
}

func errUnreachable(base string, cause error) error {
	return apperr.New("DAEMON_UNREACHABLE").
		Causer("daemonclient.Client").
		Msgf("the daemon at %s did not answer", base).
		Issue("address", base).
		Status(apperr.StatusServiceUnavailable).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label:   "start the daemon, or check that nothing else is holding its port",
			Command: "aos gateway status",
			Tool:    "gateway_status",
		})
}

func errUnreadable(base string, cause error) error {
	return apperr.New("DAEMON_UNREADABLE").
		Causer("daemonclient.Client").
		Msgf("the daemon at %s answered with something this build cannot read", base).
		Issue("address", base).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label: "the window and the daemon are usually different versions when this happens",
		})
}
