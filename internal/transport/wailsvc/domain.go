package wailsvc

import (
	"context"
	"encoding/json"
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
type DomainService struct{ caller Caller }

// NewDomain builds the domain service over a caller.
func NewDomain(caller Caller) *DomainService { return &DomainService{caller: caller} }

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
