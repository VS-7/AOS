package model

import "context"

// Catalog is the one port: which providers this installation can authenticate
// as, and what each of them serves.
//
// Two methods rather than one because they answer to different things.
// Connected is a question about local configuration and is instant; Models is a
// network call to somebody else's API and is not. Collapsing them would make
// the cheap question inherit the expensive one's failure modes.
type Catalog interface {
	// Connected lists the provider ids this installation holds a credential
	// for, sorted. A provider absent from this list is not asked: it would
	// answer 401, and "you have not connected this yet" is not a provider
	// error to report as one.
	Connected(ctx context.Context) ([]string, error)

	// Models asks one provider for its catalogue.
	Models(ctx context.Context, provider string) ([]Model, error)
}
