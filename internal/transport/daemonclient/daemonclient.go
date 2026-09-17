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
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync/atomic"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/transport/wailsvc"
)

// Client calls the daemon.
type Client struct {
	base      string
	token     atomic.Pointer[string]
	workspace atomic.Pointer[string]
	agent     atomic.Pointer[string]
	workdir   atomic.Pointer[string]
	http      *http.Client
	timeout   time.Duration
	liveness  time.Duration
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

	// Agent is who is calling, when the caller is one of the workspace's own
	// agents rather than the person. An external coding agent operating this
	// installation through `aos` or `aos --mcp` sets AOS_AGENT_ID and is that
	// agent: without it every call arrives anonymous, `agents_me` has nobody
	// to answer with, and a memory — which belongs to an agent — cannot be
	// stored at all. The daemon reads it from this header (httpapi's
	// ambientIdentity).
	Agent string

	// WorkingDir is where this process is standing, sent so the daemon can
	// resolve "the directory you are in" as the caller's rather than its own.
	// The terminal sets it; the window does not — it opens a workspace it
	// names, and the daemon's own directory is the right fallback there.
	WorkingDir string

	// Timeout bounds a question about the daemon itself — its health, the
	// session, signing in, the command listing — and how long a stream waits
	// for its answer to start. It does not bound a command: see
	// LivenessInterval.
	Timeout time.Duration

	// LivenessInterval is how often a command still waiting for its answer
	// checks that the daemon is still answering at all.
	//
	// A command used to be bounded by Timeout, as http.Client.Timeout, and a
	// download, an install or a large write outlasted it: the call was cut
	// while the daemon was still doing the work, and reported as a daemon that
	// was not there — which the window asks again for. A command now waits for
	// as long as the daemon is alive (a turn is not taken here — the daemon
	// runs it and the answer arrives over the realtime channel), and a daemon
	// that stops answering its health check ends the wait. A deadline on the
	// caller's own context still applies.
	LivenessInterval time.Duration
}

const (
	defaultTimeout  = 30 * time.Second
	defaultLiveness = 10 * time.Second

	// livenessMisses is how many health checks in a row a daemon may miss
	// before a command waiting on it is given up: one is a busy machine.
	livenessMisses = 3

	// connectTimeout bounds reaching the daemon's port, which on this machine
	// is immediate or refused. A request that never connected is the one
	// failure that is safe to send again, and waiting longer to learn that
	// buys nothing.
	connectTimeout = 5 * time.Second
)

// New builds a client.
func New(opts Options) *Client {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	liveness := opts.LivenessInterval
	if liveness <= 0 {
		liveness = defaultLiveness
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: min(timeout, connectTimeout), KeepAlive: 30 * time.Second}).DialContext
	c := &Client{
		base: strings.TrimSuffix(opts.BaseURL, "/"),
		// No client-wide Timeout: it also covers reading the body, which cut
		// a video in the Files panel after thirty seconds, and it cannot tell
		// a command that is taking long from a daemon that is gone. Each call
		// bounds itself below.
		http:     &http.Client{Transport: transport},
		timeout:  timeout,
		liveness: liveness,
	}
	c.SetToken(opts.Token)
	c.SetWorkspace(opts.Workspace)
	c.SetAgent(opts.Agent)
	c.SetWorkingDir(opts.WorkingDir)
	return c
}

// BaseURL is where this client reaches the daemon. The window needs it to
// open the event channel: that connection is a plain WebSocket the webview
// makes itself, not a call through the Wails bridge, so it cannot be
// relative to the page — the page is served by the application, not by the
// daemon.
func (c *Client) BaseURL() string { return c.base }

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

// Workspace is the workspace id later calls are scoped to, or "".
func (c *Client) Workspace() string { return c.currentWorkspace() }

func (c *Client) currentWorkspace() string {
	if v := c.workspace.Load(); v != nil {
		return *v
	}
	return ""
}

// SetAgent replaces the agent identity later calls are made as.
func (c *Client) SetAgent(id string) { c.agent.Store(&id) }

func (c *Client) currentAgent() string {
	if v := c.agent.Load(); v != nil {
		return *v
	}
	return ""
}

// SetWorkingDir replaces the directory later calls report themselves as
// standing in.
func (c *Client) SetWorkingDir(dir string) { c.workdir.Store(&dir) }

func (c *Client) currentWorkingDir() string {
	if v := c.workdir.Load(); v != nil {
		return *v
	}
	return ""
}

// Invoke runs one command.
//
// The command key maps to the path the HTTP surface publishes: memories_store
// becomes /api/memories/store. There is one rule and it is the same one the
// frontend's browser transport uses, so a path that works in one works in both.
//
// It waits for as long as the daemon is alive — see Options.LivenessInterval —
// and a failure says whether the daemon can have received the command: see
// send.
func (c *Client) Invoke(ctx context.Context, key string, input json.RawMessage) (json.RawMessage, error) {
	path := "/api/" + strings.ReplaceAll(key, "_", "/")
	body := bytes.NewReader(input)

	ctx, done := c.whileAlive(ctx)
	defer done()
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
	if agent := c.currentAgent(); agent != "" {
		req.Header.Set("x-agent-id", agent)
	}
	if dir := c.currentWorkingDir(); dir != "" {
		req.Header.Set("x-working-dir", dir)
	}

	res, err := c.send(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, c.failed(req, true, err)
	}
	// The envelope carries the error, so a non-2xx status is passed through
	// rather than replaced: the domain already said what went wrong and in
	// which terms, and restating it here would lose the code and the call to
	// action the interface reads.
	return raw, nil
}

// Status reports what this window should show — Onboarding, Login, or the
// application — without requiring a credential; unlike the others it works
// whether or not this client currently holds a token.
func (c *Client) Status(ctx context.Context) (wailsvc.AuthStatus, error) {
	ctx, cancel := c.brief(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/auth/status", nil)
	if err != nil {
		return wailsvc.AuthStatus{}, errUnreachable(c.base, err)
	}
	if token := c.currentToken(); token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}
	res, err := c.send(req)
	if err != nil {
		return wailsvc.AuthStatus{}, err
	}
	defer func() { _ = res.Body.Close() }()

	var envelope struct {
		Data  wailsvc.AuthStatus `json:"data"`
		Error *apiError          `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		return wailsvc.AuthStatus{}, errUnreadable(c.base, err)
	}
	if envelope.Error != nil {
		return wailsvc.AuthStatus{}, envelope.Error.asError(res.StatusCode)
	}
	return envelope.Data, nil
}

// Login authenticates against the daemon's own /api/auth surface — not
// Invoke, since auth has no command group to route through (see
// internal/domain/auth's package doc) — and, on success, stores the token
// this client authenticates every later call with. The token itself never
// leaves this method: the caller gets back who is now signed in, not the
// credential, which is the desktop-side equivalent of the browser's HttpOnly
// cookie never being readable from JS either.
func (c *Client) Login(ctx context.Context, identifier, password string) (wailsvc.AuthResult, error) {
	return c.authRequest(ctx, "/api/auth/login", map[string]string{
		"identifier": identifier, "password": password,
	})
}

// Onboarding creates the installation's first account, the same way.
func (c *Client) Onboarding(ctx context.Context, name, email, password string) (wailsvc.AuthResult, error) {
	return c.authRequest(ctx, "/api/auth/onboarding", map[string]string{
		"name": name, "email": email, "password": password,
	})
}

// Logout revokes the token this client currently holds and forgets it.
func (c *Client) Logout(ctx context.Context) error {
	token := c.currentToken()
	ctx, cancel := c.brief(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/api/auth/logout", bytes.NewReader([]byte("{}")))
	if err != nil {
		return errUnreachable(c.base, err)
	}
	req.Header.Set("content-type", "application/json")
	if token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}
	res, err := c.send(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	c.SetToken("")
	return nil
}

// Session reads the account the token this client holds belongs to, or
// reports that there is none.
func (c *Client) Session(ctx context.Context) (wailsvc.PublicUser, error) {
	ctx, cancel := c.brief(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/auth/session", nil)
	if err != nil {
		return wailsvc.PublicUser{}, errUnreachable(c.base, err)
	}
	if token := c.currentToken(); token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}
	res, err := c.send(req)
	if err != nil {
		return wailsvc.PublicUser{}, err
	}
	defer func() { _ = res.Body.Close() }()

	var envelope struct {
		Data struct {
			User wailsvc.PublicUser `json:"user"`
		} `json:"data"`
		Error *apiError `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		return wailsvc.PublicUser{}, errUnreadable(c.base, err)
	}
	if envelope.Error != nil {
		return wailsvc.PublicUser{}, envelope.Error.asError(res.StatusCode)
	}
	return envelope.Data.User, nil
}

// apiError is the daemon's error envelope, as the auth routes answer it.
type apiError struct {
	Code    string                `json:"code"`
	Message string                `json:"message"`
	Issues  map[string]any        `json:"issue,omitempty"`
	Actions []apperr.CallToAction `json:"cta,omitempty"`
}

// asError rebuilds the daemon's refusal, with the status it was answered with.
//
// The window reads this on the other side of the Wails bridge — the error is
// marshalled into the call's rejection — so what it carries is what the page
// can act on. It used to be a 401 with no issue and no call to action whatever
// the daemon had said, which made a wrong password (422) read as a missing
// credential to anything that looked at the status.
func (e *apiError) asError(status int) error {
	if status < 400 {
		status = apperr.StatusInternalServerError
	}
	err := apperr.New(e.Code).
		Causer("daemonclient").
		Msgf("%s", e.Message).
		Status(status).
		CTA(e.Actions...)
	for k, v := range e.Issues {
		err = err.Issue(k, v)
	}
	return err
}

// authRequest is Login and Onboarding's shared body: POST JSON, decode the
// envelope, and — on success — remember the token so every subsequent call
// this client makes is already authenticated.
func (c *Client) authRequest(ctx context.Context, path string, body map[string]string) (wailsvc.AuthResult, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return wailsvc.AuthResult{}, err
	}
	ctx, cancel := c.brief(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(raw))
	if err != nil {
		return wailsvc.AuthResult{}, errUnreachable(c.base, err)
	}
	req.Header.Set("content-type", "application/json")

	res, err := c.send(req)
	if err != nil {
		return wailsvc.AuthResult{}, err
	}
	defer func() { _ = res.Body.Close() }()

	var envelope struct {
		Data struct {
			User      wailsvc.PublicUser `json:"user"`
			Token     string             `json:"token"`
			ExpiresAt time.Time          `json:"expiresAt"`
		} `json:"data"`
		Error *apiError `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		return wailsvc.AuthResult{}, errUnreadable(c.base, err)
	}
	if envelope.Error != nil {
		return wailsvc.AuthResult{}, envelope.Error.asError(res.StatusCode)
	}
	c.SetToken(envelope.Data.Token)
	return wailsvc.AuthResult{User: envelope.Data.User, ExpiresAt: envelope.Data.ExpiresAt.Format(time.RFC3339)}, nil
}

// Commands lists what the daemon publishes.
func (c *Client) Commands(ctx context.Context) ([]wailsvc.CommandInfo, error) {
	ctx, cancel := c.brief(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/_commands", nil)
	if err != nil {
		return nil, errUnreachable(c.base, err)
	}
	if token := c.currentToken(); token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}

	res, err := c.send(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()

	var out []wailsvc.CommandInfo
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, errUnreadable(c.base, err)
	}
	return out, nil
}

// Manifest reads the whole published surface: every group, every command, every
// input schema.
//
// Commands() is what the terminal fetches before every command it runs, so it
// stays lean. This is the deliberate, heavier question — what `self tools` and
// `self llms` answer with, and what they used to answer with four commands
// because the only registry they could see was the terminal's own.
func (c *Client) Manifest(ctx context.Context) (command.Manifest, error) {
	ctx, cancel := c.brief(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/_manifest", nil)
	if err != nil {
		return command.Manifest{}, errUnreachable(c.base, err)
	}
	if token := c.currentToken(); token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}
	res, err := c.send(req)
	if err != nil {
		return command.Manifest{}, err
	}
	defer func() { _ = res.Body.Close() }()

	var envelope struct {
		Data  command.Manifest `json:"data"`
		Error *apiError        `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		return command.Manifest{}, errUnreadable(c.base, err)
	}
	if envelope.Error != nil {
		return command.Manifest{}, envelope.Error.asError(res.StatusCode)
	}
	return envelope.Data, nil
}

// SetBaseURL points this client at a different daemon.
//
// The terminal offers --base-url on every command it publishes, and a flag
// that is declared and ignored is worse than one that is absent.
func (c *Client) SetBaseURL(address string) { c.base = strings.TrimSuffix(address, "/") }

// Ready reports whether the daemon is answering its health check.
func (c *Client) Ready(ctx context.Context) (bool, error) {
	// /api/health, which is where the daemon answers it (see httpapi.New): the
	// bare /health this used to ask for reached the not-found handler, so the
	// answer was always "not ready" no matter what the daemon was doing.
	ctx, cancel := c.brief(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/health", nil)
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

// HealthInfo is what the daemon says about itself on /api/health.
type HealthInfo struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Version string `json:"version"`
}

// Health reads the daemon's own account of itself — above all its version,
// which is the running binary's and not whatever started it.
//
// Ready answers whether something is serving; this answers what. The window
// needs the second: an install replaces the application bundle while the
// daemon it started keeps running, and a new window that only asked "is it
// up" adopted the old daemon, with every defect the update had fixed.
func (c *Client) Health(ctx context.Context) (HealthInfo, error) {
	ctx, cancel := c.brief(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/health", nil)
	if err != nil {
		return HealthInfo{}, errUnreachable(c.base, err)
	}
	res, err := c.send(req)
	if err != nil {
		return HealthInfo{}, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return HealthInfo{}, errUnreadable(c.base, fmt.Errorf("health answered %d", res.StatusCode))
	}
	var info HealthInfo
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return HealthInfo{}, errUnreadable(c.base, err)
	}
	return info, nil
}

// brief bounds a question about the daemon itself by Timeout, or by the
// caller's own deadline when that is sooner.
func (c *Client) brief(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, c.timeout)
}

// errStoppedAnswering and errNoAnswerInTime are why this client itself gave up
// waiting for an answer, as the cause of the context it cancelled.
var (
	errStoppedAnswering = errors.New("the daemon stopped answering its health check")
	errNoAnswerInTime   = errors.New("the daemon did not start answering in time")
)

// whileAlive is the context a command waits under: the caller's, cancelled
// once the daemon has missed livenessMisses health checks in a row. The
// returned function ends the watch, and has to be called.
func (c *Client) whileAlive(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(ctx)
	go func() {
		ticker := time.NewTicker(c.liveness)
		defer ticker.Stop()
		misses := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			probe, stop := context.WithTimeout(ctx, c.liveness)
			ready, _ := c.Ready(probe)
			stop()
			switch {
			case ctx.Err() != nil:
				return
			case ready:
				misses = 0
			default:
				misses++
				if misses >= livenessMisses {
					cancel(errStoppedAnswering)
					return
				}
			}
		}
	}()
	return ctx, func() { cancel(nil) }
}

// send performs req, and a failure says whether the daemon can have received
// it — which decides whether anything may send it again.
//
// The line is the request having been written in full. Before that the daemon
// has not got a request it could act on: nothing listening, a refused
// connection, a dial that timed out. That is AOS_DAEMON_UNREACHABLE, and the
// window waits it out and asks again, since a daemon it started a moment ago
// may not be listening yet. After it, the daemon may be doing the work, and
// asking again could do it twice — unless it was only a question: see failed. The mark is reset for every
// attempt, since the transport retries on a fresh connection by itself when an
// idle one turns out to have been closed before anything reached it.
func (c *Client) send(req *http.Request) (*http.Response, error) {
	var written atomic.Bool
	trace := &httptrace.ClientTrace{
		GetConn: func(string) { written.Store(false) },
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err == nil {
				written.Store(true)
			}
		},
	}
	res, err := c.http.Do(req.WithContext(httptrace.WithClientTrace(req.Context(), trace)))
	if err != nil {
		return nil, c.failed(req, written.Load(), err)
	}
	return res, nil
}

// failed names a request that did not get its whole answer.
//
// Only a request that can change something is left in doubt. A GET is a
// question — the session, the published surface, the daemon's health, a
// file's bytes — and one the daemon took and never answered is the daemon not
// answering, which asking again cannot make worse: it is AOS_DAEMON_UNREACHABLE
// like a request that never left, the code the sign-in screen and the layout
// recognise, rather than a warning that it may run twice.
func (c *Client) failed(req *http.Request, written bool, err error) error {
	path := req.URL.Path
	cause := context.Cause(req.Context())
	switch {
	case !written, req.Method == http.MethodGet, req.Method == http.MethodHead:
		return errUnreachable(c.base, err)
	case errors.Is(cause, errStoppedAnswering), errors.Is(cause, errNoAnswerInTime):
		return errTimedOut(c.base, path, cause)
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, errNoAnswerInTime), isTimeout(err):
		return errTimedOut(c.base, path, err)
	default:
		return errAnswerLost(c.base, path, err)
	}
}

func isTimeout(err error) bool {
	var timeout interface{ Timeout() bool }
	return errors.As(err, &timeout) && timeout.Timeout()
}

// errUnreachable is a request that never reached the daemon.
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

// errTimedOut is a request the daemon received and has not answered.
func errTimedOut(base, path string, cause error) error {
	return apperr.New("DAEMON_TIMEOUT").
		Causer("daemonclient.Client").
		Msgf("the daemon at %s received %s and has not answered it; it may still be doing it", base, path).
		Issue("address", base).
		Issue("path", path).
		Status(apperr.StatusGatewayTimeout).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label: "check whether it was done before trying again — sending it again could do it twice",
		})
}

// errAnswerLost is a request the daemon received whose answer never arrived
// whole: the connection closed under it, a daemon that went away mid-request
// above all.
func errAnswerLost(base, path string, cause error) error {
	return apperr.New("DAEMON_ANSWER_LOST").
		Causer("daemonclient.Client").
		Msgf("the connection to the daemon at %s closed before its answer to %s arrived; it may have been done", base, path).
		Issue("address", base).
		Issue("path", path).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label: "check whether it was done before trying again — sending it again could do it twice",
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

// Fetch performs one authenticated request against a daemon path that is not
// a command route.
//
// Two surfaces are not in the command registry by backend decision — the file
// explorer (/api/file) and the account endpoints /api/auth — and the window
// used to call them with a bare fetch from the page. That could not work:
// the page's origin is the application's own scheme, so those requests were
// cross-origin, carried no cookie and no bearer, and were refused with 401
// (or blocked before they left, by CORS). The file tree, the editor, the
// diffs and the account roster were all empty in the desktop for that reason.
//
// This is the same credential every other call from the window already uses,
// applied to the two surfaces that had no bridge of their own. The token
// never reaches the page: it is held here, in the process that owns it.
//
// A write can be as slow as any command, so it waits the way Invoke does.
func (c *Client) Fetch(ctx context.Context, method, path, contentType string, body []byte) (int, []byte, error) {
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	ctx, done := c.whileAlive(ctx)
	defer done()
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return 0, nil, errUnreachable(c.base, err)
	}
	if contentType != "" {
		req.Header.Set("content-type", contentType)
	}
	if token := c.currentToken(); token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}
	if workspace := c.currentWorkspace(); workspace != "" {
		req.Header.Set("x-workspace-id", workspace)
	}
	if agent := c.currentAgent(); agent != "" {
		req.Header.Set("x-agent-id", agent)
	}

	res, err := c.send(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return res.StatusCode, nil, c.failed(req, true, err)
	}
	return res.StatusCode, raw, nil
}

// Token is the credential this client currently authenticates with.
//
// It exists for the one caller that has to make its own connection rather
// than go through Invoke: the desktop's realtime relay, which dials the
// daemon's WebSocket directly (cmd/aos-desktop) and has to present the same
// credential every other call does. It stays inside this process — nothing
// exposes it to the page.
func (c *Client) Token() string { return c.currentToken() }

// Stream performs a GET against the daemon and hands back the live response,
// credentials attached.
//
// Fetch reads the whole body into memory, which is right for a JSON envelope
// and wrong for the one surface that answers bytes: `/api/file/content`, the
// URL the Files panel's image, PDF and video viewers put in a `src`. A video
// is not a string, and a player asking for a byte range needs the range to
// reach the daemon and the daemon's own 206 to come back — which is why the
// caller's headers are forwarded and the response is returned unread.
//
// The daemon has Timeout to start answering; the body then arrives for as long
// as the caller reads it, since what it reads is a video as often as not.
//
// The caller closes the body.
func (c *Client) Stream(ctx context.Context, path string, header http.Header) (*http.Response, error) {
	return c.StreamIn(ctx, c.currentWorkspace(), path, header)
}

// StreamIn is Stream scoped to workspace rather than to the one this client
// holds now — for an answer that is only right in the workspace it was asked
// for, whatever the client has been re-pointed at since.
func (c *Client) StreamIn(ctx context.Context, workspace, path string, header http.Header) (*http.Response, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	waiting := time.AfterFunc(c.timeout, func() { cancel(errNoAnswerInTime) })
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		waiting.Stop()
		cancel(nil)
		return nil, errUnreachable(c.base, err)
	}
	// Range and the conditional headers, so seeking and caching work through
	// the proxy exactly as they would against the daemon.
	for _, name := range []string{"range", "if-range", "if-modified-since", "if-none-match", "accept"} {
		if v := header.Get(name); v != "" {
			req.Header.Set(name, v)
		}
	}
	if token := c.currentToken(); token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}
	if workspace != "" {
		req.Header.Set("x-workspace-id", workspace)
	}
	if agent := c.currentAgent(); agent != "" {
		req.Header.Set("x-agent-id", agent)
	}

	res, err := c.send(req)
	if !waiting.Stop() && err == nil {
		// The answer started just as the wait ran out, and the context it
		// would be read under is already cancelled.
		_ = res.Body.Close()
		err = c.failed(req, true, errNoAnswerInTime)
	}
	if err != nil {
		cancel(nil)
		return nil, err
	}
	res.Body = &cancelOnClose{ReadCloser: res.Body, cancel: func() { cancel(nil) }}
	return res, nil
}

// cancelOnClose releases a stream's context when its body is closed.
type cancelOnClose struct {
	io.ReadCloser
	cancel func()
}

func (b *cancelOnClose) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}
