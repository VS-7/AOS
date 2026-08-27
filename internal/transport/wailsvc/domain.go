package wailsvc

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
)

// Caller is how the desktop reaches the domain.
//
// It is a port rather than the command registry itself, and the reason is the
// dependency rule: the desktop is a client of the daemon, and a client binary
// may not link domain code. Handing it a registry would put a second copy of
// every aggregate in the window's process, writing to the same files as the one
// in the daemon — which is not "the same registry", it is two of them.
//
// What the note asks for — no parallel implementation, no second set of
// validation rules — is better served this way: there is literally one
// registry, in one process, and every surface reaches it.
type Caller interface {
	Invoke(ctx context.Context, key string, input json.RawMessage) (json.RawMessage, error)
	Commands(ctx context.Context) ([]CommandInfo, error)
}

// CommandInfo is one entry of the registry, as the frontend sees it.
type CommandInfo struct {
	Key      string `json:"key"`
	Group    string `json:"group"`
	Name     string `json:"name"`
	Summary  string `json:"summary"`
	ReadOnly bool   `json:"readOnly"`
}

// DomainService is the desktop's door to the domain.
//
// One generic Invoke rather than a method per command. A hundred and fifty
// generated methods would be a hundred and fifty places to keep in step, and
// the typing they would buy is bought instead by the TypeScript types generated
// from the same registry — see React 19 e Bindings.
type DomainService struct {
	caller Caller
	// fetcher is the same daemon client, when it can also reach the surfaces
	// that are not commands. Asked for by assertion rather than required, so
	// a test double stays a Caller and nothing more.
	fetcher Fetcher
	// scoped is the same client again, when its workspace can be re-pointed.
	scoped WorkspaceScoped
	// onWorkspace is what else has to follow the change — the realtime relay,
	// which holds its own connection per workspace. Set by cmd/aos-desktop.
	onWorkspace func(id string)
}

// NewDomain builds the domain service over a caller.
func NewDomain(caller Caller) *DomainService {
	svc := &DomainService{caller: caller}
	if f, ok := caller.(Fetcher); ok {
		svc.fetcher = f
	}
	if w, ok := caller.(WorkspaceScoped); ok {
		svc.scoped = w
	}
	return svc
}

// OnWorkspaceChange registers what to run after SetWorkspace has re-pointed
// the client — the realtime relay, in the desktop. Called once, at wiring.
func (s *DomainService) OnWorkspaceChange(fn func(id string)) { s.onWorkspace = fn }

// SetWorkspace points every subsequent call from this window at another
// workspace. The interface calls it whenever its own active workspace
// changes, so the bridge and the page never disagree about which one they
// are addressing.
func (s *DomainService) SetWorkspace(_ context.Context, id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return errNoWorkspaceNamed()
	}
	if s.scoped == nil {
		// A window with no re-pointable client is a test double or a build
		// without a daemon behind it. Nothing to do, and not a failure the
		// interface should surface.
		return nil
	}
	s.scoped.SetWorkspace(trimmed)
	if s.onWorkspace != nil {
		s.onWorkspace(trimmed)
	}
	return nil
}

func errNoWorkspaceNamed() error {
	return apperr.New("DESKTOP_NO_WORKSPACE_NAMED").
		Causer("wailsvc.DomainService.SetWorkspace").
		Msgf("a workspace change arrived with no workspace id").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "this is a defect in the interface"})
}

// ServiceName is what Wails calls this service in the generated bindings.
func (s *DomainService) ServiceName() string { return "DomainService" }

// Commands lists what can be invoked, so the frontend can check at boot that
// the daemon it is talking to has the commands it was built against.
func (s *DomainService) Commands(ctx context.Context) ([]CommandInfo, error) {
	if s.caller == nil {
		return nil, errNoDaemon()
	}
	out, err := s.caller.Commands(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// Invoke runs one command against the daemon.
func (s *DomainService) Invoke(ctx context.Context, key string, input json.RawMessage) (json.RawMessage, error) {
	if s.caller == nil {
		return nil, errNoDaemon()
	}
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return nil, errNoCommandNamed()
	}
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}
	return s.caller.Invoke(ctx, trimmed, input)
}

func errNoDaemon() error {
	return apperr.New("DESKTOP_NO_DAEMON").
		Causer("wailsvc.DomainService").
		Msgf("this window has no daemon behind it").
		Status(apperr.StatusServiceUnavailable).
		CTA(apperr.CallToAction{
			Label: "the window is a client; the daemon is what holds the workspace, and it is not answering",
		})
}

func errNoCommandNamed() error {
	return apperr.New("DESKTOP_NO_COMMAND_NAMED").
		Causer("wailsvc.DomainService.Invoke").
		Msgf("a call arrived with no command name").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "this is a defect in the interface, not in your configuration",
		})
}

// WorkspaceScoped is a caller whose workspace can be re-pointed after
// construction. *daemonclient.Client satisfies it.
//
// The window addresses one workspace at a time, and which one is a decision
// the person makes in the interface long after this service was built. Before
// this, the interface published its choice only to its own HTTP transport —
// which the desktop does not use for commands — so switching workspace in the
// window changed nothing: every call kept the id the client was constructed
// with, and the header it sends beats the cookie the page had just set.
type WorkspaceScoped interface {
	SetWorkspace(id string)
}

// Fetcher reaches the daemon's HTTP surfaces that are not command routes.
//
// A separate, narrower port than Caller for the reason SystemService's
// Addressable is separate from Health: not every caller of NewDomain has one,
// and a test double should not have to grow a method it never exercises.
// *daemonclient.Client satisfies it.
type Fetcher interface {
	Fetch(ctx context.Context, method, path, contentType string, body []byte) (int, []byte, error)
}

// FetchResult is one answer from a non-command surface, as the interface
// reads it: the status, and the body verbatim. The body is not decoded here —
// these surfaces answer in the same {data|error} envelope the interface
// already unwraps, and re-encoding it would only be a second place for the
// shape to drift.
type FetchResult struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// Fetch calls one of the daemon's non-registry surfaces with this window's
// credential — the file explorer and the account endpoints AuthService does
// not bind.
//
// Restricted to /api/file and /api/auth, and deliberately: this is a bridge
// for two known surfaces, not a general-purpose authenticated proxy for
// anything the page can name. The command surface has its own door (Invoke),
// which validates; opening every path here would route around that.
func (s *DomainService) Fetch(ctx context.Context, method, path, contentType, body string) (FetchResult, error) {
	if s.fetcher == nil {
		return FetchResult{}, errNoDaemon()
	}
	if !allowedFetchPath(path) {
		return FetchResult{}, errPathNotBridged(path)
	}
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return FetchResult{}, errMethodNotBridged(method)
	}

	status, raw, err := s.fetcher.Fetch(ctx, strings.ToUpper(method), path, contentType, []byte(body))
	if err != nil {
		return FetchResult{}, err
	}
	return FetchResult{Status: status, Body: string(raw)}, nil
}

// allowedFetchPath is the allowlist Fetch enforces. The check is on the path
// before any query string, and it refuses a traversal outright rather than
// cleaning it: "/api/file/../../whatever" is not a request this bridge has
// any reason to make.
func allowedFetchPath(path string) bool {
	if strings.Contains(path, "..") {
		return false
	}
	base, _, _ := strings.Cut(path, "?")
	for _, prefix := range []string{"/api/file/", "/api/auth/"} {
		if strings.HasPrefix(base, prefix) {
			return true
		}
	}
	return false
}

func errPathNotBridged(path string) error {
	return apperr.New("DESKTOP_PATH_NOT_BRIDGED").
		Causer("wailsvc.DomainService.Fetch").
		Msgf("%q is not one of the surfaces this window may reach directly", path).
		Issue("path", path).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "commands go through Invoke, which validates them; only the file and account surfaces are bridged this way",
		})
}

func errMethodNotBridged(method string) error {
	return apperr.New("DESKTOP_METHOD_NOT_BRIDGED").
		Causer("wailsvc.DomainService.Fetch").
		Msgf("%q is not a method this bridge forwards", method).
		Issue("method", method).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "this is a defect in the interface"})
}
