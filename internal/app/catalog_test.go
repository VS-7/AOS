package app_test

import (
	"testing"

	"github.com/OWNER/aos/internal/domain/activity"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/goal"
	"github.com/OWNER/aos/internal/domain/project"
	"github.com/OWNER/aos/internal/domain/task"
)

// The catalogue is a promise the workspace makes to a routine: "these are the
// events you may react to". A promise nothing checks drifts — a publisher gets
// renamed, an event is added, and the trigger picker keeps offering coordinates
// that will never arrive while the real ones stay invisible.
//
// So this exercises the actual mutations through the real application and holds
// everything that reaches the log against what the catalogue declares. It is
// deliberately the weaker direction of the two: an entry in the catalogue that
// nothing publishes yet is a promise about a surface that exists (a toolset
// call needs a connected toolset; a routine firing needs a scheduler), while an
// event that arrives undeclared is a trigger nobody can write.
func TestEveryPublishedEventIsInTheCatalogue(t *testing.T) {
	a := newApp(t)
	ctx := agentCtx()

	created, err := a.Tasks.Create(ctx, task.CreateInput{Name: "Ship it", Type: "feature"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Tasks.Update(ctx, task.UpdateInput{ID: created.ID, Name: ptr("Ship it well")}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Tasks.SetStatus(ctx, task.SetStatusInput{ID: created.ID, Status: task.Todo}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Tasks.Delete(ctx, task.DeleteInput{ID: created.ID}); err != nil {
		t.Fatal(err)
	}

	proj, err := a.Projects.Create(ctx, project.CreateInput{Name: "Atlas"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Projects.Update(ctx, project.UpdateInput{ID: proj.ID, Name: ptr("Atlas II")}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Projects.Delete(ctx, project.DeleteInput{ID: proj.ID}); err != nil {
		t.Fatal(err)
	}

	g, err := a.Goals.Create(ctx, goal.CreateInput{Title: "Reach parity"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Goals.Update(ctx, goal.UpdateInput{ID: g.ID, Title: ptr("Reach parity everywhere")}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Goals.Delete(ctx, goal.DeleteInput{ID: g.ID}); err != nil {
		t.Fatal(err)
	}

	worker, err := a.Agents.Create(ctx, agent.CreateInput{Name: "Scout"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Agents.Update(ctx, agent.UpdateInput{ID: worker.ID, Description: ptr("Reads things")}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Agents.Delete(ctx, agent.DeleteInput{ID: worker.ID}); err != nil {
		t.Fatal(err)
	}

	logged, err := a.Activities.List(ctx, activity.ListInput{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(logged.Activities) == 0 {
		t.Fatal("nothing was published; this test would pass on an empty log")
	}

	for _, entry := range logged.Activities {
		if !activity.Declared(entry.Namespace, entry.Event) {
			t.Errorf("%s.%s reached the log and no routine can trigger on it: "+
				"declare it in internal/domain/activity/catalog.go",
				entry.Namespace, entry.Event)
		}
	}
}

// Every declared event has to carry the keys it says it carries, or a filter
// written against them silently never matches.
func TestTheCatalogueNamesTheKeysTheEventsCarry(t *testing.T) {
	a := newApp(t)
	ctx := agentCtx()

	created, err := a.Tasks.Create(ctx, task.CreateInput{Name: "Ship it", Type: "feature"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Tasks.SetStatus(ctx, task.SetStatusInput{ID: created.ID, Status: task.Todo}); err != nil {
		t.Fatal(err)
	}

	logged, err := a.Activities.List(ctx, activity.ListInput{Namespace: "task", Event: "status_changed"})
	if err != nil {
		t.Fatal(err)
	}
	if len(logged.Activities) != 1 {
		t.Fatalf("the log holds %d status changes", len(logged.Activities))
	}

	var declared activity.EventKind
	for _, kind := range activity.Kinds {
		if kind.Namespace == "task" && kind.Event == "status_changed" {
			declared = kind
		}
	}
	for _, key := range declared.Data {
		if _, ok := logged.Activities[0].Data[key]; !ok {
			t.Errorf("the catalogue promises %q on task.status_changed and the event does not carry it", key)
		}
	}
}
