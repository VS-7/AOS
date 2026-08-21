package tunnel

import (
	"context"
	"errors"
	"time"
)

// The two ways Runner.Spawn can fail that Start's error path tells apart from
// a plain spawn failure — see errors.go. os/exec is forbidden under
// internal/domain (see internal/architecture/rules.go), so the real spawning
// lives in an adapter (e.g. internal/adapters/cloudflaredproc) behind Runner,
// and reports failure through these sentinels rather than a concrete type
// this package would have to import.
var (
	ErrBinaryMissing    = errors.New("cloudflared binary not found on PATH")
	ErrReadinessTimeout = errors.New("cloudflared did not report a connected tunnel within the wait")
)

// Service exposes the local daemon on the public internet. There is no MCP
// surface: publishing this machine to the internet is a human decision, not
// an agent capability — see commands.go.
type Service interface {
	// Start publishes the local daemon. It refuses when the API is not
	// authenticated (errInsecureExposure) or when hostname/token are not
	// configured (errConfigIncomplete) — see the guard in service.go.
	Start(ctx context.Context) (State, error)

	// Stop tears the tunnel down. Hostname and token are left in the config,
	// so Start can bring it back up without reconfiguring.
	Stop(ctx context.Context) (State, error)

	// Status reads the current state without changing it.
	Status(ctx context.Context) (State, error)
}

// Config is the narrow slice of config.Config this domain reads: whether the
// API is authenticated, and the tunnel's own hostname/token. A separate port
// rather than importing config.Config's full surface, for the same reason
// every domain here narrows its dependencies to what it actually uses. The
// integrator's adapter reads config.Config and fills this in.
type Config interface {
	Raw(ctx context.Context) (RawConfig, error)
}

// RawConfig is the two substructures Start's guard reads.
type RawConfig struct {
	SecurityEnabled bool
	APIToken        string
	Hostname        string
	Token           string
}

// Runner spawns and supervises the cloudflared process, kept behind a port
// because the domain layer may not import os/exec directly. The real
// implementation lives in an adapter (e.g. internal/adapters/cloudflaredproc);
// service_test.go fakes it in-process, without a real binary.
type Runner interface {
	// Spawn starts cloudflared for hostname/token and waits for it to report
	// a registered connection, up to timeout. On failure it has already
	// cleaned up whatever process it started. On ErrBinaryMissing or
	// ErrReadinessTimeout specifically, the returned error wraps one of the
	// sentinels above (errors.Is), so Start can tell them apart from any
	// other spawn failure.
	Spawn(ctx context.Context, hostname, token string, timeout time.Duration) (Process, error)
}

// Process is a running cloudflared instance.
type Process interface {
	PID() int
	// Wait blocks until the process exits and reports why.
	Wait() error
	// Stop signals the process to end gracefully.
	Stop() error
}
