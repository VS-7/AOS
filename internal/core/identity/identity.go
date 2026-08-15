// Package identity carries the ambient identity of an inbound request.
//
// The original propagates this implicitly with AsyncLocalStorage. Go propagates
// it explicitly through context.Context, and the explicitness is the gain: the
// signature of a function says whether it can see who is acting.
package identity

import "context"

// ActorType distinguishes the two kinds of actor the system attributes work to.
type ActorType string

const (
	ActorAgent ActorType = "agent"
	ActorUser  ActorType = "user"
	ActorNone  ActorType = ""
)

// Identity is the server-side ambient context of one request: which workspace
// it addresses, who is acting, and the id that correlates every log line.
type Identity struct {
	WorkspaceID string
	AgentID     string
	UserID      string
	RequestID   string
}

type ctxKey struct{}

// With attaches the ambient identity to ctx. It is called once per inbound
// request by the transport layer, never inside domain code.
func With(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// From reads the ambient identity. It returns the zero value when absent, so
// callers that require an actor must check explicitly.
func From(ctx context.Context) Identity {
	id, _ := ctx.Value(ctxKey{}).(Identity)
	return id
}

// Actor returns the agent when present, otherwise the user — matching the
// original's getActor(), which prefers the agent over the user.
func Actor(ctx context.Context) (string, ActorType) {
	id := From(ctx)
	switch {
	case id.AgentID != "":
		return id.AgentID, ActorAgent
	case id.UserID != "":
		return id.UserID, ActorUser
	default:
		return "", ActorNone
	}
}

// IsAgent reports whether the current actor is an agent. Redaction and the
// config write allowlist branch on this (ADR-0010).
func IsAgent(ctx context.Context) bool {
	_, kind := Actor(ctx)
	return kind == ActorAgent
}

// WithWorkspace returns a context whose ambient identity addresses ws, keeping
// the rest of the identity intact.
func WithWorkspace(ctx context.Context, ws string) context.Context {
	id := From(ctx)
	id.WorkspaceID = ws
	return With(ctx, id)
}
