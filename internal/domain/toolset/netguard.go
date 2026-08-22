package toolset

import "context"

// ctxKey is the context key WithAllowedHosts/AllowedHostsFrom share — an
// unexported type so nothing outside this package can collide with it, the
// same idiom internal/runtime/execguard uses for the sandbox it carries.
type ctxKey struct{}

// WithAllowedHosts attaches the hosts a rest-api or mcp-server::http
// toolset's owning skill declared under permissions.network — Service.Call's
// enforcement of the restriction the design doc describes ("um cliente HTTP
// com allowlist de host aplica isso"). hosts may be empty: that still means
// "restricted", just to nothing, which is what a skill whose manifest
// declares no network hosts at all is held to — not the same as never
// calling this at all, which AllowedHostsFrom reports as unrestricted.
//
// The actual enforcement — an http.RoundTripper checking every request
// against what this attaches — lives in internal/adapters/netguard, not
// here: this package cannot import net/http (internal/architecture forbids
// it), the same reason internal/runtime/execguard exists as its own package
// rather than living in this one.
func WithAllowedHosts(ctx context.Context, hosts []string) context.Context {
	return context.WithValue(ctx, ctxKey{}, hosts)
}

// AllowedHostsFrom reads the hosts WithAllowedHosts attached to ctx. ok is
// false when it was never called at all — a toolset with no Skill, which
// carries no manifest and so no restriction to enforce — and true otherwise,
// even when hosts is empty.
func AllowedHostsFrom(ctx context.Context) (hosts []string, ok bool) {
	hosts, ok = ctx.Value(ctxKey{}).([]string)
	return hosts, ok
}
