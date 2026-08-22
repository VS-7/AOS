// Package netguard enforces the host allowlist toolset.Service.Call attaches
// to a call's context (see toolset.WithAllowedHosts) — the "cliente HTTP com
// allowlist de host" the toolset design doc calls for.
//
// It cannot live in internal/domain/toolset itself: that package cannot
// import net/http (internal/architecture forbids it), the same reason
// internal/runtime/execguard exists as its own package rather than living in
// internal/domain/toolset too. internal/adapters/openapiclient and
// internal/adapters/mcpclient, the two toolset.Adapter implementations that
// speak HTTP, both depend on this package rather than checking hosts
// themselves, so the one place either request-level check like this exists.
package netguard

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/OWNER/aos/internal/domain/toolset"
)

// Transport wraps next so every request it carries is checked against the
// hosts toolset.WithAllowedHosts attached to ctx, before the request ever
// leaves this process. A ctx with no allowlist attached — a toolset with no
// Skill — runs next unmodified: the restriction only exists for a toolset a
// skill's manifest declared, matching UpdateConfig's own "no Skill, no
// promise to keep faith with" rule.
//
// Callers build the *http.Client Connect uses with this once, so every
// request that client ever sends is checked — not only the toolset's own
// declared BaseURL. That is what closes the gap a BaseURL-only check leaves
// open: an OpenAPI document can declare its own servers[] pointing
// somewhere else entirely, and a check that only ever looked at BaseURL
// would never see it.
func Transport(ctx context.Context, next http.RoundTripper) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	hosts, restricted := toolset.AllowedHostsFrom(ctx)
	if !restricted {
		return next
	}
	return &hostAllowlistTransport{allowed: hosts, next: next}
}

type hostAllowlistTransport struct {
	allowed []string
	next    http.RoundTripper
}

func (t *hostAllowlistTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()
	for _, a := range t.allowed {
		if strings.EqualFold(host, a) {
			return t.next.RoundTrip(req)
		}
	}
	return nil, fmt.Errorf(
		"host %q is not declared under this skill's permissions.network (allowed: %s)",
		host, strings.Join(t.allowed, ", "),
	)
}
