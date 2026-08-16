package theme

import "context"

// Store persists the presets a user installs.
//
// The built-in themes are not here: they are embedded in the binary, so they
// cannot be missing at runtime and cannot be edited into an unreadable state by
// something that is not this program.
type Store interface {
	List(ctx context.Context) ([]Theme, error)
	Get(ctx context.Context, id string) (*Theme, error)
	Save(ctx context.Context, t Theme) error
	Delete(ctx context.Context, id string) error
}
