package toolexec

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/safe"
)

// Recorder is told what a call cost. It is a port so the runtime can publish
// metrics without this package knowing what metrics are.
type Recorder interface {
	ToolCall(ctx context.Context, name string, elapsed time.Duration, err error)
}

// Option configures the wrapper.
type Option func(*wrapped)

// WithSpill installs the truncation and spillover behaviour.
func WithSpill(s *Spiller) Option { return func(w *wrapped) { w.spill = s } }

// WithRecorder installs metrics.
func WithRecorder(r Recorder) Option { return func(w *wrapped) { w.recorder = r } }

// WithClock injects time, so a test can assert on a duration.
func WithClock(now func() time.Time) Option { return func(w *wrapped) { w.now = now } }

// Wrap composes the cross-cutting concerns around a bare tool, in a fixed
// order: metrics on the outside, then the panic boundary, then truncation and
// spillover, then the tool.
//
// Approval is not in this list, and its absence is the design. A denied call
// must never reach the tool at all, so the decision happens in the loop before
// anything here runs.
func Wrap(t Tool, opts ...Option) Tool {
	w := &wrapped{Tool: t, now: time.Now} //nolint:forbidigo // overridden by WithClock in tests; the composition root injects one
	for _, o := range opts {
		o(w)
	}
	return w
}

type wrapped struct {
	Tool
	spill    *Spiller
	recorder Recorder
	now      func() time.Time
}

func (w *wrapped) Invoke(ctx context.Context, in json.RawMessage) (any, error) {
	start := w.now()

	var out any
	// A tool is the third of the three places this system recovers from a
	// panic. A bug in one tool costs that call, not the turn and not the
	// daemon (defect #16).
	err := safe.Do(ctx, "tool "+w.Name(), func(c context.Context) error {
		var ierr error
		out, ierr = w.Tool.Invoke(c, in)
		return ierr
	})

	if w.recorder != nil {
		w.recorder.ToolCall(ctx, w.Name(), w.now().Sub(start), err)
	}
	if err != nil {
		return nil, err
	}
	if w.spill == nil {
		return out, nil
	}
	return w.spill.Process(ctx, CallIDFrom(ctx), out), nil
}

type callIDKey struct{}

// WithCallID puts the model's tool call id in the context, so the spilled file
// is named after the call the model can refer back to.
func WithCallID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, callIDKey{}, id)
}

// CallIDFrom reads the tool call id.
func CallIDFrom(ctx context.Context) string {
	if id, ok := ctx.Value(callIDKey{}).(string); ok {
		return id
	}
	return "output"
}

// Registry is the set of tools one agent may call.
//
// It is per turn rather than global because two agents in the same workspace
// have different sandbox policies, and a tool bound to one agent's sandbox must
// not be reachable by another.
type Registry struct {
	mu     sync.RWMutex
	byName map[string]Tool
	order  []string
}

// NewRegistry builds an empty registry.
func NewRegistry() *Registry {
	return &Registry{byName: map[string]Tool{}}
}

// Add registers a tool. A duplicate name replaces the earlier one, which is
// what lets a workspace override a native tool with a skill's version.
func (r *Registry) Add(tools ...Tool) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range tools {
		if t == nil {
			continue
		}
		if _, exists := r.byName[t.Name()]; !exists {
			r.order = append(r.order, t.Name())
		}
		r.byName[t.Name()] = t
	}
	return r
}

// Get finds a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.byName[name]
	return t, ok
}

// Specs lists what the model is told it can call, in a stable order.
func (r *Registry) Specs() []Spec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := append([]string(nil), r.order...)
	sort.Strings(names)
	out := make([]Spec, 0, len(names))
	for _, n := range names {
		out = append(out, r.byName[n].Spec())
	}
	return out
}

// Len reports how many tools are registered.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byName)
}

// Invoke runs a tool by name.
func (r *Registry) Invoke(ctx context.Context, name string, in json.RawMessage) (any, error) {
	t, ok := r.Get(name)
	if !ok {
		return nil, errUnknownTool(name, r.names())
	}
	return t.Invoke(ctx, in)
}

func (r *Registry) names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := append([]string(nil), r.order...)
	sort.Strings(out)
	return out
}

func errUnknownTool(name string, known []string) error {
	return apperr.New("TOOL_UNKNOWN").
		Causer("toolexec.Registry.Invoke").
		Msgf("there is no tool called %q", name).
		Issue("tool", name).
		Issue("available", known).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "the tools you can call are listed in the issue; pick one of those rather than retrying this name",
		})
}
