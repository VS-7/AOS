package agent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/domain/agent"
)

func agentCtx(id string) context.Context {
	return identity.With(context.Background(), identity.Identity{AgentID: id})
}

func humanCtx() context.Context {
	return identity.With(context.Background(), identity.Identity{UserID: "vitor"})
}

func mustCreate(t *testing.T, svc *agent.Service, in agent.CreateInput) *agent.Agent {
	t.Helper()
	got, err := svc.Create(ctx(), in)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestMeResolvesTheCallingAgent is how an agent running in another tool finds
// out who it is here, without having been told its own slug.
func TestMeResolvesTheCallingAgent(t *testing.T) {
	svc, _ := newService(t)
	mustCreate(t, svc, agent.CreateInput{ID: "atlas", Orchestrator: true})
	mustCreate(t, svc, agent.CreateInput{ID: "reviewer", Role: "QA"})

	got, err := svc.Me(agentCtx("reviewer"), agent.MeInput{})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "reviewer" {
		t.Fatalf("me = %q, want the calling agent", got.ID)
	}
}

func TestMeFromATerminalResolvesTheOrchestrator(t *testing.T) {
	svc, _ := newService(t)
	mustCreate(t, svc, agent.CreateInput{ID: "reviewer"})
	mustCreate(t, svc, agent.CreateInput{ID: "atlas", Orchestrator: true})

	for _, c := range []struct {
		name string
		ctx  context.Context
	}{
		{"a human", humanCtx()},
		{"no identity at all", context.Background()},
	} {
		got, err := svc.Me(c.ctx, agent.MeInput{})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got.ID != "atlas" {
			t.Errorf("%s: me = %q, want the orchestrator", c.name, got.ID)
		}
	}
}

func TestMeWithoutAnOrchestratorSaysSoWithAWayOut(t *testing.T) {
	svc, _ := newService(t)
	mustCreate(t, svc, agent.CreateInput{ID: "reviewer"})

	_, err := svc.Me(humanCtx(), agent.MeInput{})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
	e, _ := apperr.As(err)
	if len(e.Actions) < 2 {
		t.Errorf("the caller should be told how to fix this, got %d actions", len(e.Actions))
	}
}

// TestPromotingASecondOrchestratorDemotesTheFirst is the invariant the original
// documents and does not impose. Without it, routing for every chat with no
// explicit mention depends on which file the directory listing returns first.
func TestPromotingASecondOrchestratorDemotesTheFirst(t *testing.T) {
	svc, _ := newService(t)
	mustCreate(t, svc, agent.CreateInput{ID: "atlas", Orchestrator: true})

	yes := true
	mustCreate(t, svc, agent.CreateInput{ID: "luara"})
	if _, err := svc.Update(ctx(), agent.UpdateInput{ID: "luara", Orchestrator: &yes}); err != nil {
		t.Fatal(err)
	}

	all, err := svc.List(ctx(), agent.ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	var orchestrators []string
	for _, a := range all.Agents {
		if a.Orchestrator {
			orchestrators = append(orchestrators, a.ID)
		}
	}
	if len(orchestrators) != 1 || orchestrators[0] != "luara" {
		t.Fatalf("orchestrators = %v, want exactly [luara]", orchestrators)
	}
}

func TestCreatingASecondOrchestratorAlsoDemotes(t *testing.T) {
	svc, _ := newService(t)
	mustCreate(t, svc, agent.CreateInput{ID: "atlas", Orchestrator: true})
	mustCreate(t, svc, agent.CreateInput{ID: "luara", Orchestrator: true})

	got, err := svc.Get(ctx(), agent.GetInput{ID: "atlas"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Orchestrator {
		t.Fatal("the first orchestrator was not demoted")
	}
}

// TestUpdatingTheOrchestratorDoesNotDemoteItself: an unrelated edit to the
// orchestrator must not leave the workspace with none.
func TestUpdatingTheOrchestratorDoesNotDemoteItself(t *testing.T) {
	svc, _ := newService(t)
	mustCreate(t, svc, agent.CreateInput{ID: "atlas", Orchestrator: true})

	yes := true
	role := "Lead"
	if _, err := svc.Update(ctx(), agent.UpdateInput{ID: "atlas", Role: &role, Orchestrator: &yes}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Me(humanCtx(), agent.MeInput{})
	if err != nil || got.ID != "atlas" {
		t.Fatalf("me = %+v, err = %v", got, err)
	}
}

func TestAnAgentCannotLeadItself(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.Create(ctx(), agent.CreateInput{ID: "atlas", Leader: "Atlas"})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("error = %v", err)
	}
}

// TestALeaderCycleIsRejectedOnWrite: prompt assembly walks this chain to say who
// an agent reports to, and delegation walks it to decide escalation. A loop
// would run until something ran out.
func TestALeaderCycleIsRejectedOnWrite(t *testing.T) {
	svc, _ := newService(t)
	mustCreate(t, svc, agent.CreateInput{ID: "a"})
	mustCreate(t, svc, agent.CreateInput{ID: "b", Leader: "a"})
	mustCreate(t, svc, agent.CreateInput{ID: "c", Leader: "b"})

	// Closing the loop: a would report to c, which reports to b, which reports
	// to a.
	leader := "c"
	_, err := svc.Update(ctx(), agent.UpdateInput{ID: "a", Leader: &leader})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("error = %v", err)
	}
	e, _ := apperr.As(err)
	if e.Issues["chain"] == nil {
		t.Error("the error should show the loop it found")
	}

	// And the record was not written.
	got, err := svc.Get(ctx(), agent.GetInput{ID: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Leader != "" {
		t.Fatalf("leader = %q, want the rejected write not to have landed", got.Leader)
	}
}

// TestALeaderThatDoesNotExistYetIsAllowed: teams are assembled in whatever
// order the person thinks of them, and a dangling reference closes no loop.
func TestALeaderThatDoesNotExistYetIsAllowed(t *testing.T) {
	svc, _ := newService(t)
	got, err := svc.Create(ctx(), agent.CreateInput{ID: "b", Leader: "not-yet"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Leader != "not-yet" {
		t.Fatalf("leader = %q", got.Leader)
	}
}

func TestALongButValidChainIsAccepted(t *testing.T) {
	svc, _ := newService(t)
	previous := ""
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		mustCreate(t, svc, agent.CreateInput{ID: id, Leader: previous})
		previous = id
	}
	got, err := svc.Get(ctx(), agent.GetInput{ID: "e"})
	if err != nil || got.Leader != "d" {
		t.Fatalf("got = %+v, err = %v", got, err)
	}
}
