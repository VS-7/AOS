package agent

import (
	"context"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// Repository is what this package needs to persist an agent. The interface
// lives here, where it is consumed; the filesystem implementation satisfies it
// structurally, without importing this package.
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*Agent, error)
	List(ctx context.Context, q collections.Query) ([]Agent, error)
	Create(ctx context.Context, v *Agent) error
	Update(ctx context.Context, v *Agent, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// Clock is the only source of time in this package. Injected, so that a golden
// file built from an agent record does not change every second.
type Clock interface{ Now() time.Time }

// Notifier publishes what happened to an agent.
//
// Best-effort and unable to fail the mutation, the same contract task's own
// Notifier has. It exists because nothing published anything: an agent
// created by the orchestrator, by the CLI or from the settings screen
// produced no activity, so the inbox never mentioned it and no routine could
// trigger on it.
type Notifier interface {
	AgentChanged(ctx context.Context, event string, a *Agent)
}
