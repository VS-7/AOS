package app_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/routine"
)

// TestAWebhookFiresARoutineThroughTheDaemon is the trigger end to end: the
// token routines_create hands out, posted to the fire URL with nothing else,
// records a run on the routine.
func TestAWebhookFiresARoutineThroughTheDaemon(t *testing.T) {
	base, a := serving(t, nil)
	ctx := context.Background()

	owner, err := a.Agents.Create(ctx, agent.CreateInput{Name: "Deployer"})
	if err != nil {
		t.Fatal(err)
	}
	made, err := a.Routines.Create(ctx, routine.CreateInput{
		Agent: owner.ID, Name: "Deploy hook",
		Triggers: []routine.TriggerInput{{Type: routine.Webhook}},
		Content:  "Check the deploy.",
	})
	if err != nil {
		t.Fatal(err)
	}

	fire := func(token string) int {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			base+"/api/hooks/routines/"+made.Routine.ID, strings.NewReader(`{"ref":"main"}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return res.StatusCode
	}

	if got := fire("guessed"); got != http.StatusUnauthorized {
		t.Fatalf("a wrong token: status = %d", got)
	}
	if got := fire(made.Token); got != http.StatusAccepted {
		t.Fatalf("the routine's token: status = %d", got)
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		history, err := a.Routines.Runs(ctx, routine.RunsInput{ID: made.Routine.ID})
		if err != nil {
			t.Fatal(err)
		}
		if len(history.Runs) == 1 && history.Runs[0].Status != routine.RunRunning {
			run := history.Runs[0]
			if run.Trigger != routine.Webhook || run.Payload["ref"] != "main" {
				t.Fatalf("the run does not say a webhook fired it with its payload: %+v", run)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no finished run after the webhook: %+v", history.Runs)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
