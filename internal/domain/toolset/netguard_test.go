package toolset_test

import (
	"context"
	"testing"

	"github.com/OWNER/aos/internal/domain/toolset"
)

// The enforcement itself — an HTTP RoundTripper checking a request against
// what WithAllowedHosts attached — is proved in internal/adapters/netguard's
// own tests; this package cannot reach that far up the stack (see
// internal/architecture's dependency rule). What belongs here is the context
// plumbing alone.

func TestAllowedHostsFromReportsNoRestrictionWhenNeverAttached(t *testing.T) {
	hosts, ok := toolset.AllowedHostsFrom(context.Background())
	if ok {
		t.Fatalf("ok = true, hosts = %v — a plain context should carry no restriction", hosts)
	}
}

func TestAllowedHostsFromRoundTripsWhatWasAttached(t *testing.T) {
	ctx := toolset.WithAllowedHosts(context.Background(), []string{"api.example.com"})
	hosts, ok := toolset.AllowedHostsFrom(ctx)
	if !ok || len(hosts) != 1 || hosts[0] != "api.example.com" {
		t.Fatalf("hosts = %v, ok = %v", hosts, ok)
	}
}

// TestAnEmptyAllowlistIsStillReportedAsRestricted: a skill whose manifest
// declares zero network hosts is held to zero hosts, not to "no
// restriction" — the two must not collapse into each other, or a lookup
// that legitimately returns nothing would silently open the toolset up
// instead of closing it. internal/adapters/netguard's own tests prove what
// that restriction actually does to a request.
func TestAnEmptyAllowlistIsStillReportedAsRestricted(t *testing.T) {
	ctx := toolset.WithAllowedHosts(context.Background(), nil)
	_, ok := toolset.AllowedHostsFrom(ctx)
	if !ok {
		t.Fatal("an explicitly empty allowlist must still report ok = true")
	}
}
