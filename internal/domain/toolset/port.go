package toolset

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/activity"
)

// Adapter is the one boundary where this system executes something outside
// its own process. Every connection type — MCP over stdio, MCP over HTTP, a
// REST API and a CLI today; a custom kind tomorrow — implements exactly
// this, and Service.Call never distinguishes between them: it connects, lists
// or calls, and closes, without knowing which wire protocol is underneath.
//
// It lives in this package, not the packages that implement each Type,
// precisely so those can be written against it without touching Service —
// internal/domain/testsuite.RunAdapterContract is what
// proves a new implementation actually satisfies it.
type Adapter interface {
	// Connect establishes the connection ts describes. ts arrives with every
	// ${env.VAR} placeholder already resolved — an Adapter never sees the
	// literal placeholder, only the secret or setting it named.
	//
	// It must return promptly with an error rather than block forever when
	// the target is unreachable: RunAdapterContract asserts this against a
	// deliberately invalid target, and a caller of Service.Call has no other
	// way to bound how long one call may hang.
	Connect(ctx context.Context, ts Toolset) error

	// ListTools discovers what the connected target currently publishes. It
	// runs on demand, never cached across calls — the reason the agent's own
	// registry does not grow with the number of toolsets configured is that
	// nothing here is loaded until something asks.
	ListTools(ctx context.Context) ([]ToolSpec, error)

	// Call runs one tool. input and the returned payload are opaque JSON to
	// this interface and to Service — the adapter relays them, it does not
	// interpret them.
	Call(ctx context.Context, tool string, input json.RawMessage) (json.RawMessage, error)

	// Close releases the connection. It must be safe to call more than once:
	// Service.Call defers it unconditionally, including on the path where
	// Connect itself failed partway through.
	Close() error
}

// Factory builds one fresh Adapter. A new instance per call, never a shared
// one, because what it wraps — a spawned process, an open HTTP connection — is
// scoped to a single call; a shared instance would let one hung external
// server wedge every subsequent call behind it.
type Factory func() Adapter

// Adapters maps a connection Type to the Factory that implements it. Only
// MCPStdio, MCPHTTP, RESTAPI and CLI have an entry in this slice's wiring;
// Service.Call reports the remaining one, Custom, as errTypeNotAvailable
// rather than as a decode error — the configuration is fine, the binary is
// only missing the connector.
type Adapters map[Type]Factory

// EnvResolver is what ${env.VAR} interpolation reads a value from — the slice
// of internal/core/env.Resolver this package needs, kept as an interface so
// this package never imports internal/core/env's os.LookupEnv-backed default,
// and a test supplies a resolver over a plain map instead.
type EnvResolver interface {
	String(key, def string) string
}

// Repository is what this package needs to persist a Toolset. It omits
// Create on purpose: nothing in this slice creates one — a toolset reaches
// disk the way every native collection record does, through the collection
// engine, and this port only reads, reconfigures and removes it.
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*Toolset, error)
	List(ctx context.Context, q collections.Query) ([]Toolset, error)
	Update(ctx context.Context, v *Toolset, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// Activities is the audit sink every Call is recorded to — the toolset, the
// tool, the duration and the outcome, never the payload. It is the exact
// slice of activity.Service's own signature, so *activity.Service satisfies it
// with no adapter of its own.
type Activities interface {
	Publish(ctx context.Context, in activity.PublishInput) (*activity.Activity, error)
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }
