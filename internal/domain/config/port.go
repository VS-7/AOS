package config

import "context"

// Store is what this package needs from the outside world: a file it can read
// and replace. The implementation lives in internal/adapters/fsconfig; the
// interface lives here because this is where it is consumed.
type Store interface {
	// Load returns the stored configuration. A missing file is not an error:
	// it yields the default configuration, so a fresh installation works
	// before onboarding writes anything.
	Load(ctx context.Context) (Config, error)

	// Save replaces the stored configuration atomically, with the permissions
	// a secret-bearing file requires.
	Save(ctx context.Context, c Config) error
}
