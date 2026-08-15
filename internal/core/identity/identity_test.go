package identity_test

import (
	"context"
	"testing"

	"github.com/OWNER/aos/internal/core/identity"
)

func TestFromReturnsZeroValueWhenAbsent(t *testing.T) {
	if got := identity.From(context.Background()); got != (identity.Identity{}) {
		t.Fatalf("expected the zero identity, got %+v", got)
	}
}

// TestActorPrefersAgent mirrors the original's getActor(), which resolves the
// agent before the user.
func TestActorPrefersAgent(t *testing.T) {
	ctx := identity.With(context.Background(), identity.Identity{
		AgentID: "orchestrator", UserID: "vitor",
	})
	id, kind := identity.Actor(ctx)
	if id != "orchestrator" || kind != identity.ActorAgent {
		t.Fatalf("actor = %q/%v", id, kind)
	}
	if !identity.IsAgent(ctx) {
		t.Error("IsAgent should be true")
	}
}

func TestActorFallsBackToUser(t *testing.T) {
	ctx := identity.With(context.Background(), identity.Identity{UserID: "vitor"})
	id, kind := identity.Actor(ctx)
	if id != "vitor" || kind != identity.ActorUser {
		t.Fatalf("actor = %q/%v", id, kind)
	}
	if identity.IsAgent(ctx) {
		t.Error("a user is not an agent")
	}
}

func TestWithWorkspaceKeepsTheRestOfTheIdentity(t *testing.T) {
	ctx := identity.With(context.Background(), identity.Identity{
		AgentID: "a", UserID: "u", RequestID: "r",
	})
	got := identity.From(identity.WithWorkspace(ctx, "ws-1"))
	if got.WorkspaceID != "ws-1" || got.AgentID != "a" || got.UserID != "u" || got.RequestID != "r" {
		t.Fatalf("identity = %+v", got)
	}
}
