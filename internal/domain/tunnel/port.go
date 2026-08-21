package tunnel

import "context"

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
