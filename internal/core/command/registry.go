package command

import (
	"fmt"
	"reflect"
	"sort"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
)

// Registry holds every declared capability.
//
// It knows nothing about the domain: commands are handed to it already built,
// by internal/domain/{feature}/commands.go. The arrow points inwards — the
// domain knows the Command Layer, the Command Layer does not know the domain —
// which is why adding a capability never means touching this package.
type Registry struct {
	mu       sync.RWMutex
	byKey    map[string]Descriptor
	order    []string
	groups   map[string]GroupDoc
	aliases  map[string]Alias
	frozen   bool
	registry []Descriptor
}

// NewRegistry builds an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		byKey:   map[string]Descriptor{},
		groups:  map[string]GroupDoc{},
		aliases: map[string]Alias{},
	}
}

// Register erases the type parameters and stores the descriptor.
//
// The JSON schema is computed here, once, at registration: ~140 structs
// reflected in milliseconds at boot, so that a call is a plain json.Unmarshal.
func Register[In, Out any](r *Registry, c Command[In, Out]) error {
	if c.Group == "" || c.Name == "" {
		return fmt.Errorf("command needs a group and a name, got %q/%q", c.Group, c.Name)
	}
	if c.Handler == nil {
		return fmt.Errorf("%s_%s has no handler", c.Group, c.Name)
	}
	inType := reflect.TypeFor[In]()

	// No command may forget `_reasoning`. Catching it at registration turns a
	// class of runtime surprise into a boot failure.
	reason := reasoningFieldPath(inType)
	if reason == "" {
		return fmt.Errorf("%s_%s: input type %s has no %q field — embed command.Reasoning",
			c.Group, c.Name, inType, ReasoningField)
	}
	if err := checkTags(inType); err != nil {
		return fmt.Errorf("%s_%s: %w", c.Group, c.Name, err)
	}

	schema, err := jsonschema.For[In](nil)
	if err != nil {
		return fmt.Errorf("%s_%s: cannot infer the input schema: %w", c.Group, c.Name, err)
	}
	if err := completeSchema(inType, schema); err != nil {
		return fmt.Errorf("%s_%s: %w", c.Group, c.Name, err)
	}

	d := &descriptor[In, Out]{cmd: c, schema: schema, inType: inType, reason: reason}
	return r.add(d)
}

// MustRegister is Register for the static wiring in cmd/, where a failure is a
// programming error that must surface at start-up rather than at first use.
func MustRegister[In, Out any](r *Registry, c Command[In, Out]) {
	if err := Register(r, c); err != nil {
		panic(err)
	}
}

func (r *Registry) add(d Descriptor) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return fmt.Errorf("registry is frozen: %s cannot be added after boot", d.Key())
	}
	if _, dup := r.byKey[d.Key()]; dup {
		return fmt.Errorf("%s is registered twice", d.Key())
	}
	r.byKey[d.Key()] = d
	r.order = append(r.order, d.Key())
	if d.InRegistry() {
		r.registry = append(r.registry, d)
	}
	return nil
}

// DescribeGroup attaches documentation to a group.
func (r *Registry) DescribeGroup(g GroupDoc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.groups[g.Name] = g
}

// Freeze closes the registry. Everything is declared at boot; a surface that
// could add a command later would make the published tool list a moving target.
func (r *Registry) Freeze() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
}

// Lookup finds a command by its key, following aliases.
func (r *Registry) Lookup(key string) (Descriptor, *DeprecationNotice, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if d, ok := r.byKey[key]; ok {
		return d, nil, true
	}
	if a, ok := r.aliases[key]; ok {
		if d, ok := r.byKey[a.To]; ok {
			return d, a.Notice(), true
		}
	}
	return nil, nil, false
}

// Sorted returns every descriptor in alphabetical order of key.
//
// The order is the published tool list. It is alphabetical so that two runs of
// the same binary present tools identically — a model that re-reads the list
// should not see it shuffled.
func (r *Registry) Sorted() []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Descriptor, 0, len(r.byKey))
	for _, d := range r.byKey {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}

// All returns every descriptor in registration order.
func (r *Registry) All() []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Descriptor, 0, len(r.order))
	for _, k := range r.order {
		out = append(out, r.byKey[k])
	}
	return out
}

// AgentTools returns the descriptors the agent's internal registry exposes:
// everything except the privilege boundary.
func (r *Registry) AgentTools() []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Descriptor, len(r.registry))
	copy(out, r.registry)
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}

// Group is one group with its commands, in alphabetical order.
type Group struct {
	GroupDoc
	Commands []Descriptor
}

// Groups returns the groups in alphabetical order.
func (r *Registry) Groups() []Group {
	r.mu.RLock()
	defer r.mu.RUnlock()

	byGroup := map[string][]Descriptor{}
	for _, k := range r.order {
		d := r.byKey[k]
		byGroup[d.Group()] = append(byGroup[d.Group()], d)
	}
	names := make([]string, 0, len(byGroup))
	for name := range byGroup {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]Group, 0, len(names))
	for _, name := range names {
		cmds := byGroup[name]
		sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name() < cmds[j].Name() })
		doc, ok := r.groups[name]
		if !ok {
			doc = GroupDoc{Name: name}
		}
		out = append(out, Group{GroupDoc: doc, Commands: cmds})
	}
	return out
}

// GroupOf returns the documentation of one group.
func (r *Registry) GroupOf(name string) (GroupDoc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.groups[name]
	return g, ok
}

// Len reports how many commands are registered.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byKey)
}
