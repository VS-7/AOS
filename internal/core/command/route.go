package command

import (
	"context"
	"encoding/json"
)

// Route publishes one command surface over several backing registries, and
// picks which one a call lands in from the call's own context.
//
// A daemon serves an installation, and an installation holds more than one
// workspace. Every workspace has its own agents, tasks, chats and collections,
// on its own directory — so the services behind agents_list are not the same
// objects for two workspaces, and neither are the registries those services
// were registered into.
//
// What must stay the same is the surface. The CLI's help, MCP's tool list and
// the HTTP router are all built once, at boot, from the descriptors a registry
// publishes; they cannot be rebuilt per request, and they must not differ
// between workspaces. So the shape comes from a template registry — the one
// built for the workspace the process opened — and only the handler is
// resolved per call.
//
// The template's descriptors are never invoked through the returned registry.
// The resolver answers with the registry that owns the workspace this call is
// addressed to, which for the template's own workspace is the template itself.
//
// The result is frozen: a published tool list that could still grow is a tool
// list a model cannot rely on.
func Route(template *Registry, resolve func(context.Context) (*Registry, error)) *Registry {
	routed := NewRegistry()

	for _, d := range template.All() {
		// The error is impossible here and would be a programming mistake if
		// it happened: the template has already refused duplicates, and this
		// registry is empty until this loop fills it.
		_ = routed.add(&routedDescriptor{Descriptor: d, resolve: resolve})
	}

	for name, group := range template.groupsSnapshot() {
		routed.DescribeGroup(group)
		_ = name
	}
	for old, alias := range template.Aliases() {
		routed.aliases[old] = alias
	}

	routed.Freeze()
	return routed
}

// routedDescriptor is one command, described by the template and executed by
// whichever registry the context resolves to.
//
// It embeds Descriptor so every published detail — key, schemas, docs,
// annotations, examples — is the template's, unchanged. Only Invoke is
// overridden.
type routedDescriptor struct {
	Descriptor
	resolve func(context.Context) (*Registry, error)
}

func (r *routedDescriptor) Invoke(ctx context.Context, surface Surface, raw json.RawMessage) (any, error) {
	// Introspection describes the published surface, and the published surface
	// is the template's — identical for every workspace, by construction. So it
	// is answered here rather than routed: asking what a command takes must not
	// depend on being able to resolve a workspace to run it in.
	if asksForSchema(raw) {
		return FlatDetail(r.Descriptor), nil
	}

	registry, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	if registry == nil {
		// A resolver that answers with neither a registry nor an error has a
		// bug; running the template's own handler would silently write to the
		// wrong workspace, which is the failure this whole mechanism exists to
		// prevent.
		return nil, errUnroutable(r.Key())
	}

	target, _, ok := registry.Lookup(r.Key())
	if !ok {
		// Every backing registry is built by the same wiring, so a key present
		// in the template is present in all of them. Reaching here means two
		// registries were built by different code paths.
		return nil, errUnroutable(r.Key())
	}
	return target.Invoke(ctx, surface, raw)
}

// groupsSnapshot copies the group documentation, under the lock.
func (r *Registry) groupsSnapshot() map[string]GroupDoc {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]GroupDoc, len(r.groups))
	for k, v := range r.groups {
		out[k] = v
	}
	return out
}
