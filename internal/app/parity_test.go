package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OWNER/aos/internal/adapters/fscollections"
	"github.com/OWNER/aos/internal/app"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/domain/activity"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/artifact"
	"github.com/OWNER/aos/internal/domain/chat"
	"github.com/OWNER/aos/internal/domain/collection"
	"github.com/OWNER/aos/internal/domain/comment"
	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/domain/gateway"
	"github.com/OWNER/aos/internal/domain/goal"
	"github.com/OWNER/aos/internal/domain/instruction"
	"github.com/OWNER/aos/internal/domain/job"
	"github.com/OWNER/aos/internal/domain/memory"
	"github.com/OWNER/aos/internal/domain/model"
	"github.com/OWNER/aos/internal/domain/project"
	"github.com/OWNER/aos/internal/domain/routine"
	"github.com/OWNER/aos/internal/domain/skill"
	"github.com/OWNER/aos/internal/domain/task"
	"github.com/OWNER/aos/internal/domain/template"
	"github.com/OWNER/aos/internal/domain/theme"
	"github.com/OWNER/aos/internal/domain/todo"
	"github.com/OWNER/aos/internal/domain/toolset"
	"github.com/OWNER/aos/internal/domain/tunnel"
	"github.com/OWNER/aos/internal/domain/update"
	"github.com/OWNER/aos/internal/domain/view"
	"github.com/OWNER/aos/internal/domain/workspace"
	"github.com/OWNER/aos/internal/transport/clix"
	"github.com/OWNER/aos/internal/transport/httpapi"
	"github.com/OWNER/aos/internal/transport/mcpserver"
)

var refTime = time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

// emptyOutside clears what the previous surface left in the one directory a
// scenario writes to outside its installation, so each of the five starts
// from the same state the first one found: nothing there.
func emptyOutside(t *testing.T, sc scenario) {
	t.Helper()
	if sc.Outside == "" {
		return
	}
	if err := os.RemoveAll(sc.Outside); err != nil {
		t.Fatal(err)
	}
}

// newApp builds an isolated installation: its own state directory, its own
// workspace, and a frozen clock so two runs of the same command are byte-equal.
func newApp(t *testing.T) *app.App {
	t.Helper()
	home := t.TempDir()
	root := t.TempDir()
	a, err := app.New(app.Options{
		Env: env.New(env.Map(map[string]string{
			env.KeyHome: home,
			// Every surface addresses the same workspace, so a command that
			// takes no identifier resolves to one rather than to nothing.
			env.KeyWorkspaceID: activeWorkspace,
		})),
		WorkspaceRoot: root,
		Clock:         clockx.Fixed{At: refTime},
		// Predictable identifiers: a memory created on one surface must be
		// byte-comparable with the one created on the next.
		IDs: &ids.Sequence{Prefix: "m"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	return a
}

// excluded lists the commands the parity suite deliberately does not run, with
// the reason. It is a map rather than a silence: a command that is not covered
// has to be visible as a decision, and the reason has to survive review.
var excluded = map[string]string{
	"gateway_start": "spawns an operating-system process; running it on five surfaces " +
		"would start five daemons. Covered end to end by TestTheDeliveryOfPhaseFour.",
	"gateway_restart": "stops and spawns; same reason as gateway_start.",
	"tasks_branch": "creates a real Git worktree on disk, which needs a repository " +
		"the parity installation does not have. Covered by the task suite, which " +
		"drives the same code path over a fake worktree driver.",
	"routines_rotate": "returns a freshly minted secret, so no two runs can be " +
		"byte-equal by construction. That is the point of the command, not a defect " +
		"in it; the rotation itself is covered by the routine suite.",
	"routines_fire": "executes a real turn against a model. Running it on four surfaces " +
		"would be four turns. Covered by the routine suite and end to end by " +
		"TestTheDeliveryOfPhaseSix.",
	"toolsets_call": "the one boundary where this system executes something outside its " +
		"own process — spawning an MCP server five times over would leave five " +
		"processes behind. Covered by the toolset suite over a fake Adapter.",
	"skills_install": "asks a person for approval before writing; the parity harness has " +
		"no way to answer that channel, and answering it automatically would stop " +
		"testing the one thing ADR-0007 exists to guarantee. Covered by the skill " +
		"suite, which drives the same Install with AcceptedAll set.",
	"skills_create": "the same operation as skills_install, under the name a script or an " +
		"agent assembling a package reaches for — same reason, same exclusion.",
	"instructions_create": "asks a person for approval before writing, the same as " +
		"skills_install — parityCtx sets an agent identity on every surface here, so " +
		"every one of them would hit the same unanswered channel. Covered by the " +
		"instruction suite's own approval tests.",
	"instructions_update": "the same operation reaches the same approval gate — " +
		"instructions_create's own exclusion besides.",
	"marketplace_discovery": "the parity installation has no registry configured, so this " +
		"could only exercise the refusal path, not a real search. Covered by the " +
		"marketplace suite's contract tests over a fake Registry.",
	"marketplace_get": "same reason as marketplace_discovery — no registry configured here " +
		"to read a real listing from.",
	"marketplace_install": "same reason as marketplace_discovery, and it shares " +
		"skills_install's own exclusion besides: consent has nobody to answer it in this " +
		"harness. Covered by the marketplace suite.",
	"tunnel_start": "spawns cloudflared, an operating-system process — running it on four " +
		"surfaces would start four. The parity installation also has no API token, so " +
		"even a single run here would only exercise the exposure guard, not a real " +
		"tunnel. Both the guard and the supervised run are covered by the tunnel suite " +
		"over a fake Runner.",
	"update_check": "the parity installation has no release feed configured (BaseURL " +
		"empty), so this could only exercise the up-to-date refusal path, never a real " +
		"channel read. Covered by the update suite's own fakeSource.",
	"update_download": "same reason as update_check, and it needs a real signed release " +
		"to verify against, which the parity harness has no channel to fetch one from. " +
		"Covered by the update suite over a fakeSource and a real relsig keypair.",
	"update_apply": "restarts the daemon out from under whichever surface is running it — " +
		"running it on five surfaces would restart the daemon five times, mid-suite. " +
		"Covered end to end by the update suite's own Apply tests over a fake " +
		"DaemonSupervisor.",
}

// scenario describes one command well enough to run it on every surface.
type scenario struct {
	// Payload is the input, identical on all four surfaces.
	Payload any

	// Seed prepares the state the command needs. Every surface gets a fresh
	// installation, so a mutating command does not disturb the next surface.
	Seed func(t *testing.T, a *app.App)

	// PayloadIn builds the payload around a directory the command writes to
	// *outside* the installation, and replaces Payload when it is set.
	//
	// The isolation Seed's comment describes stops at the state directory.
	// workspace_create makes a directory wherever it is told, and all five
	// surfaces are told the same place — so the first one scaffolds it and
	// the four after it are answering a different question, about a
	// directory that already exists. This is what that path is threaded
	// through, so it can be a place this subtest owns and empties.
	PayloadIn func(dir string) any

	// Outside is that directory, filled in per subtest. Empty for the
	// scenarios that stay inside their installation, which is most of them.
	Outside string
}

const activeWorkspace = "parity"

// parityCtx is the ambient identity every surface runs under. Memories belong
// to an agent, so a surface with no identity would be answering a different
// question than the others.
func parityCtx() context.Context {
	return identity.With(context.Background(), identity.Identity{
		AgentID: "atlas", WorkspaceID: activeWorkspace,
	})
}

func seedAgent(t *testing.T, a *app.App) {
	t.Helper()
	_, err := a.Agents.Create(context.Background(), agent.CreateInput{
		ID: "atlas", Name: "Atlas", Role: "Orchestrator",
		Content: "# Instructions\nCoordinate the team.\n",
	})
	if err != nil {
		t.Fatal(err)
	}
}

// seedWorkspace registers the workspace the commands that take no identifier
// resolve to.
func seedWorkspace(t *testing.T, a *app.App) {
	t.Helper()
	if _, err := a.Workspaces.Create(parityCtx(), workspace.CreateInput{
		Name: "Parity", Path: a.Workspace,
	}); err != nil {
		t.Fatal(err)
	}
}

func seedChat(t *testing.T, a *app.App) {
	t.Helper()
	seedAgent(t, a)
	if _, err := a.Chats.Create(parityCtx(), chat.CreateInput{Title: "Planning"}); err != nil {
		t.Fatal(err)
	}
}

func seedMemory(t *testing.T, a *app.App) {
	t.Helper()
	if _, err := a.Memories.Store(parityCtx(), memory.StoreInput{
		Title:       "Gateway restart protocol",
		Description: "Ask before restarting the gateway after a code change.",
		Category:    memory.CatInstruction,
		Content:     "# Rule\nAsk first.\n",
	}); err != nil {
		t.Fatal(err)
	}
}

// scenarios covers the whole registry. A command without one fails the suite,
// which is what keeps a new capability from being published untested.
var scenarios = map[string]scenario{
	"agents_list": {
		Payload: agent.ListInput{Reasoning: reason()},
		Seed:    seedAgent,
	},
	"agents_get": {
		Payload: agent.GetInput{ID: "atlas", Reasoning: reason()},
		Seed:    seedAgent,
	},
	"agents_create": {
		Payload: agent.CreateInput{
			ID: "reviewer", Name: "Reviewer", Role: "Quality Assurance Specialist",
			Provider: "openai", Model: "gpt-5.5", Reasoning: reason(),
		},
	},
	"agents_update": {
		Payload: agent.UpdateInput{ID: "atlas", Role: ptr("Lead"), Reasoning: reason()},
		Seed:    seedAgent,
	},
	"agents_delete": {
		Payload: agent.DeleteInput{ID: "atlas", Reasoning: reason()},
		Seed:    seedAgent,
	},
	"agents_me": {
		Payload: agent.MeInput{Reasoning: reason()},
		Seed:    seedAgent,
	},
	"memories_recall": {
		Payload: memory.RecallInput{Reasoning: reason()},
		Seed:    seedMemory,
	},
	"memories_graph": {
		Payload: memory.GraphInput{Reasoning: reason()},
		Seed:    seedMemory,
	},
	"memories_reflect": {
		Payload: memory.ReflectInput{Memory: "m-1", Reasoning: reason()},
		Seed:    seedMemory,
	},
	"memories_store": {
		Payload: memory.StoreInput{
			Title:       "Parity is checked by running every surface",
			Description: "One definition, four surfaces, one normalised result.",
			Category:    memory.CatFact,
			Reasoning:   reason(),
		},
	},
	"memories_forget": {
		Payload: memory.ForgetInput{
			Memory:    "m-1",
			Reason:    "The protocol changed when the gateway learned to reload itself.",
			Reasoning: reason(),
		},
		Seed: seedMemory,
	},
	"workspace_list": {
		Payload: workspace.ListInput{Reasoning: reason()},
		Seed:    seedWorkspace,
	},
	"workspace_get": {
		Payload: workspace.GetInput{Reasoning: reason()},
		Seed:    seedWorkspace,
	},
	"workspace_create": {
		// Not a fixed /tmp path: that one was shared by all five surfaces
		// *and* by every other run on the machine, so this passed only once
		// the directory already existed and failed on any clean checkout.
		PayloadIn: func(dir string) any {
			return workspace.CreateInput{Name: "Another", Path: dir, Reasoning: reason()}
		},
	},
	"workspace_update": {
		Payload: workspace.UpdateInput{
			Set:       map[string]any{"git.branchPrefix": "feat"},
			Reasoning: reason(),
		},
		Seed: seedWorkspace,
	},
	"workspace_delete": {
		Payload: workspace.DeleteInput{Workspace: activeWorkspace, Reasoning: reason()},
		Seed:    seedWorkspace,
	},
	"workspace_inventory": {
		Payload: workspace.InventoryInput{Reasoning: reason()},
		Seed:    seedWorkspace,
	},
	"workspace_introspect": {
		Payload: workspace.IntrospectInput{Reasoning: reason()},
		Seed:    seedWorkspace,
	},
	"chats_list": {
		Payload: chat.ListInput{Reasoning: reason()},
		Seed:    seedChat,
	},
	"chats_get": {
		Payload: chat.GetInput{Chat: "m-1", Reasoning: reason()},
		Seed:    seedChat,
	},
	"chats_create": {
		Payload: chat.CreateInput{Title: "Another room", Reasoning: reason()},
		Seed:    seedAgent,
	},
	"chats_send": {
		Payload: chat.SendInput{Chat: "m-1", Text: "@atlas what changed?", Reasoning: reason()},
		Seed:    seedChat,
	},
	"chats_update": {
		Payload: chat.UpdateInput{Chat: "m-1", Title: "Renamed", Reasoning: reason()},
		Seed:    seedChat,
	},
	"chats_clear": {
		Payload: chat.ClearInput{Chat: "m-1", Reasoning: reason()},
		Seed:    seedChat,
	},
	"chats_delete": {
		Payload: chat.DeleteInput{Chat: "m-1", Reasoning: reason()},
		Seed:    seedChat,
	},
	// The two read-only halves of supervision are safe to run five times over:
	// neither spawns anything. The two that do are in `excluded`.
	"gateway_status": {
		Payload: gateway.StatusInput{Reasoning: reason()},
	},
	"gateway_stop": {
		Payload: gateway.StopInput{Reasoning: reason()},
	},
	"config_get": {
		Payload: config.GetInput{Reasoning: reason()},
	},
	// Nothing is ever waiting in a parity run, which is the point worth
	// checking on every surface: an answer to a request that is not there
	// reports that it did not land rather than reporting success.
	"approvals_list": {
		Payload: app.PendingInput{Reasoning: reason()},
	},
	"approvals_decide": {
		Payload: app.DecideInput{ID: "nothing-is-waiting", Approved: true, Reasoning: reason()},
	},
	"config_update": {
		Payload: config.UpdateInput{
			Set:       map[string]any{"region.timezone": "America/Sao_Paulo"},
			Reasoning: reason(),
		},
	},

	"tasks_list": {
		Payload: task.ListInput{Reasoning: reason()},
		Seed:    seedTask,
	},
	"tasks_get": {
		Payload: task.GetInput{ID: "m-1", Reasoning: reason()},
		Seed:    seedTask,
	},
	"tasks_create": {
		Payload: task.CreateInput{
			Name: "Fix the denial pattern", Type: "bug",
			Priority: task.High, Reasoning: reason(),
		},
		Seed: seedWorkspace,
	},
	"tasks_update": {
		Payload: task.UpdateInput{ID: "m-1", Summary: ptr("Reproduced."), Reasoning: reason()},
		Seed:    seedTask,
	},
	"tasks_set-status": {
		Payload: task.SetStatusInput{ID: "m-1", Status: task.InProgress, Reasoning: reason()},
		Seed:    seedTask,
	},
	"tasks_delete": {
		Payload: task.DeleteInput{ID: "m-1", Reasoning: reason()},
		Seed:    seedTask,
	},

	"todos_list": {
		Payload: todo.ListInput{Task: "m-1", Reasoning: reason()},
		Seed:    seedTodo,
	},
	"todos_get": {
		Payload: todo.GetInput{Task: "m-1", ID: "m-3", Reasoning: reason()},
		Seed:    seedTodo,
	},
	"todos_create": {
		Payload: todo.CreateInput{Task: "m-1", Title: "Write the failing test", Reasoning: reason()},
		Seed:    seedTask,
	},
	"todos_update": {
		Payload: todo.UpdateInput{
			Task: "m-1", ID: "m-3", Evidence: ptr("go test passes"), Reasoning: reason(),
		},
		Seed: seedTodo,
	},
	"todos_set-status": {
		Payload: todo.SetStatusInput{
			Task: "m-1", ID: "m-3", Status: todo.Finished,
			Evidence: "the new test fails before the fix", Reasoning: reason(),
		},
		Seed: seedTodo,
	},
	"todos_delete": {
		Payload: todo.DeleteInput{Task: "m-1", ID: "m-3", Reasoning: reason()},
		Seed:    seedTodo,
	},

	"comments_list": {
		Payload: comment.ListInput{Task: "m-1", Reasoning: reason()},
		Seed:    seedComment,
	},
	"comments_get": {
		Payload: comment.GetInput{Task: "m-1", ID: "m-3", Reasoning: reason()},
		Seed:    seedComment,
	},
	"comments_create": {
		Payload: comment.CreateInput{Task: "m-1", Body: "Reproduced it.", Reasoning: reason()},
		Seed:    seedTask,
	},
	"comments_update": {
		Payload: comment.UpdateInput{Task: "m-1", ID: "m-3", Body: "Corrected.", Reasoning: reason()},
		Seed:    seedComment,
	},
	"comments_delete": {
		Payload: comment.DeleteInput{Task: "m-1", ID: "m-3", Reasoning: reason()},
		Seed:    seedComment,
	},

	"routines_list": {
		Payload: routine.ListInput{Reasoning: reason()},
		Seed:    seedRoutine,
	},
	"routines_get": {
		Payload: routine.GetInput{ID: "m-1", Reasoning: reason()},
		Seed:    seedRoutine,
	},
	"routines_create": {
		Payload: routine.CreateInput{
			Name:      "Triage new bugs",
			Triggers:  []routine.TriggerInput{{Type: routine.Scheduled, Cron: "0 9 * * 1-5"}},
			Content:   "List the bugs in the backlog and set a priority on each.",
			Reasoning: reason(),
		},
		Seed: seedAgent,
	},
	"routines_update": {
		Payload: routine.UpdateInput{ID: "m-1", Name: ptr("Renamed"), Reasoning: reason()},
		Seed:    seedRoutine,
	},
	"routines_runs": {
		Payload: routine.RunsInput{ID: "m-1", Reasoning: reason()},
		Seed:    seedRoutine,
	},
	"routines_delete": {
		Payload: routine.DeleteInput{ID: "m-1", Reasoning: reason()},
		Seed:    seedRoutine,
	},

	"activity_list": {
		Payload: activity.ListInput{Reasoning: reason()},
		Seed:    seedTask,
	},
	// Creating the task publishes the entry these three address: m-1 is the
	// task, m-2 the activity that announced it, and m-3 whatever a seed adds
	// after both.
	"activity_get": {
		Payload: activity.GetInput{ID: "m-2", Reasoning: reason()},
		Seed:    seedTask,
	},
	"activity_read": {
		Payload: activity.MarkInput{ID: "m-2", Reasoning: reason()},
		Seed:    seedTask,
	},
	"activity_delete": {
		Payload: activity.DeleteInput{ID: "m-2", Reasoning: reason()},
		Seed:    seedTask,
	},
	"activity_read-all": {
		Payload: activity.MarkAllInput{Reasoning: reason()},
		Seed:    seedTask,
	},
	"activity_purge": {
		Payload: activity.PurgeInput{OlderThanDays: 1, Reasoning: reason()},
		Seed:    seedTask,
	},

	"jobs_list": {
		Payload: job.ListInput{Reasoning: reason()},
	},
	"jobs_get": {
		Payload: job.GetInput{ID: "j-1", Reasoning: reason()},
		Seed:    seedJob,
	},
	"jobs_stats": {
		Payload: job.StatsInput{Reasoning: reason()},
	},
	"jobs_recover": {
		Payload: job.RecoverInput{Reasoning: reason()},
	},
	"jobs_purge": {
		Payload: job.PurgeInput{OlderThanDays: 1, Reasoning: reason()},
	},

	"themes_list": {
		Payload: theme.ListInput{Reasoning: reason()},
	},

	// A parity installation has no provider connected, so this asks nobody
	// and reaches no network — which is exactly the case worth checking on
	// every surface. "Nothing is connected" has to be an empty answer, not an
	// error and not a surface answering it differently from the next.
	"models_list": {
		Payload: model.ListInput{Reasoning: reason()},
	},
	"themes_get": {
		Payload: theme.GetInput{ID: "nord", Reasoning: reason()},
	},
	"themes_install": {
		Payload: theme.InstallInput{
			ID: "midnight", Name: "Midnight",
			Variants: map[theme.Appearance]theme.Palette{
				theme.Dark: {Surface: "#0b0d12", Ink: "#e6e9ef", Accent: "#7aa2f7", Contrast: 70},
			},
			Reasoning: reason(),
		},
	},
	"themes_delete": {
		Payload: theme.DeleteInput{ID: "midnight", Reasoning: reason()},
		Seed:    seedTheme,
	},

	// The ecosystem slice (Task 10).
	"collections_list": {
		Payload: collection.ListInput{Reasoning: reason()},
		Seed:    seedCollection,
	},
	"collections_get": {
		Payload: collection.GetInput{ID: "contacts", Reasoning: reason()},
		Seed:    seedCollection,
	},
	"collections_create": {
		Payload: collection.CreateInput{
			ID: "deals", Name: "Deals", Format: collection.FormatJSON,
			Fields:    []collection.Field{{Name: "title", Type: collection.TypeString, Required: true}},
			Reasoning: reason(),
		},
	},
	"collections_delete": {
		Payload: collection.DeleteInput{ID: "contacts", Reasoning: reason()},
		Seed:    seedCollection,
	},
	"collections_records-list": {
		Payload: collection.RecordsListInput{Collection: "contacts", Reasoning: reason()},
		Seed:    seedCollectionRecord,
	},
	"collections_records-get": {
		Payload: collection.RecordsGetInput{Collection: "contacts", ID: "m-1", Reasoning: reason()},
		Seed:    seedCollectionRecord,
	},
	"collections_records-create": {
		Payload: collection.RecordsCreateInput{
			Collection: "contacts", Data: map[string]any{"name": "Grace Hopper"}, Reasoning: reason(),
		},
		Seed: seedCollection,
	},
	"collections_records-update": {
		Payload: collection.RecordsUpdateInput{
			Collection: "contacts", ID: "m-1", Data: map[string]any{"name": "Ada, Countess of Lovelace"}, Reasoning: reason(),
		},
		Seed: seedCollectionRecord,
	},
	"collections_records-delete": {
		Payload: collection.RecordsDeleteInput{Collection: "contacts", ID: "m-1", Reasoning: reason()},
		Seed:    seedCollectionRecord,
	},

	"views_list": {
		Payload: view.ListInput{Reasoning: reason()},
		Seed:    seedView,
	},
	"views_get": {
		Payload: view.GetInput{ID: "contacts-table", Reasoning: reason()},
		Seed:    seedView,
	},
	"views_create": {
		Payload: view.CreateRequest{
			ID: "deals-table", Title: "Deals", Source: view.Source{Collection: "contacts"},
			Tree:      json.RawMessage(`{"component":"Table","props":{"columns":["name"],"rows":[]}}`),
			Reasoning: reason(),
		},
		Seed: seedCollection,
	},
	"views_delete": {
		Payload: view.DeleteInput{ID: "contacts-table", Reasoning: reason()},
		Seed:    seedView,
	},
	"views_render": {
		Payload: view.RenderInput{ID: "contacts-table", Reasoning: reason()},
		Seed:    seedView,
	},
	"views_execute-action": {
		Payload: view.ExecuteActionInput{ID: "contacts-table", Label: "Refresh", Reasoning: reason()},
		Seed:    seedView,
	},
	"views_components": {
		Payload: view.ComponentsInput{Reasoning: reason()},
	},
	"views_scaffold": {
		Payload: view.ScaffoldInput{Collection: "contacts", Kind: view.KindTable, Reasoning: reason()},
		Seed:    seedCollection,
	},

	"toolsets_list": {
		Payload: toolset.ListInput{Reasoning: reason()},
		Seed:    seedToolset,
	},
	"toolsets_get": {
		Payload: toolset.GetInput{ID: "gh", Reasoning: reason()},
		Seed:    seedToolset,
	},
	"toolsets_get-config": {
		Payload: toolset.GetInput{ID: "gh", Reasoning: reason()},
		Seed:    seedToolset,
	},
	"toolsets_update-config": {
		Payload: toolset.UpdateConfigInput{ID: "gh", Description: ptr("Reconfigured"), Reasoning: reason()},
		Seed:    seedToolset,
	},
	"toolsets_delete": {
		Payload: toolset.DeleteInput{ID: "gh", Reasoning: reason()},
		Seed:    seedToolset,
	},

	"skills_list": {
		Payload: skill.ListInput{Reasoning: reason()},
		Seed:    seedSkill,
	},
	"skills_update": {
		Payload: skill.UpdateInput{ID: "crm", Active: ptr(false), Reasoning: reason()},
		Seed:    seedSkill,
	},
	"skills_delete": {
		Payload: skill.DeleteInput{ID: "crm", Reasoning: reason()},
		Seed:    seedSkill,
	},

	// The seven Phase 8 domains declared alongside the ecosystem core — see
	// docs/08 - Entrega/Roteiro de Fases.md's "Fora do núcleo, declarado".
	"artifacts_list": {
		Payload: artifact.ListInput{Reasoning: reason()},
		Seed:    seedArtifact,
	},
	"artifacts_get": {
		Payload: artifact.GetInput{ID: "dashboard", Reasoning: reason()},
		Seed:    seedArtifact,
	},
	"artifacts_create": {
		Payload: artifact.CreateInput{ID: "report", Name: "Report", Reasoning: reason()},
	},
	"artifacts_update": {
		Payload: artifact.UpdateInput{ID: "dashboard", Name: ptr("Dashboard v2"), Reasoning: reason()},
		Seed:    seedArtifact,
	},
	"artifacts_set-password": {
		Payload: artifact.SetPasswordInput{ID: "dashboard", Password: "correct-horse-battery-staple", Reasoning: reason()},
		Seed:    seedArtifact,
	},
	"artifacts_delete": {
		Payload: artifact.DeleteInput{ID: "dashboard", Reasoning: reason()},
		Seed:    seedArtifact,
	},

	"goals_list": {
		Payload: goal.ListInput{Reasoning: reason()},
		Seed:    seedGoal,
	},
	"goals_get": {
		Payload: goal.GetInput{ID: "grow-revenue", Reasoning: reason()},
		Seed:    seedGoal,
	},
	"goals_create": {
		Payload: goal.CreateInput{Title: "Ship the Migration", Reasoning: reason()},
	},
	"goals_update": {
		Payload: goal.UpdateInput{ID: "grow-revenue", Title: ptr("Grow Revenue 20%"), Reasoning: reason()},
		Seed:    seedGoal,
	},
	"goals_delete": {
		Payload: goal.DeleteInput{ID: "grow-revenue", Reasoning: reason()},
		Seed:    seedGoal,
	},

	"instructions_list": {
		Payload: instruction.ListInput{Reasoning: reason()},
		Seed:    seedInstruction,
	},
	"instructions_get": {
		Payload: instruction.GetInput{ID: "protocol", Reasoning: reason()},
		Seed:    seedInstruction,
	},
	"instructions_delete": {
		Payload: instruction.DeleteInput{ID: "protocol", Reasoning: reason()},
		Seed:    seedInstruction,
	},

	"projects_list": {
		Payload: project.ListInput{Reasoning: reason()},
		Seed:    seedProject,
	},
	"projects_get": {
		Payload: project.GetInput{ID: "launch", Reasoning: reason()},
		Seed:    seedProject,
	},
	"projects_create": {
		Payload: project.CreateInput{ID: "onboarding", Name: "Onboarding", Reasoning: reason()},
	},
	"projects_update": {
		Payload: project.UpdateInput{ID: "launch", Name: ptr("Launch v2"), Reasoning: reason()},
		Seed:    seedProject,
	},
	"projects_delete": {
		Payload: project.DeleteInput{ID: "launch", Reasoning: reason()},
		Seed:    seedProject,
	},

	"templates_list": {
		Payload: template.ListInput{Reasoning: reason()},
		Seed:    seedTemplate,
	},
	"templates_get": {
		Payload: template.GetInput{ID: "welcome-email", Reasoning: reason()},
		Seed:    seedTemplate,
	},
	"templates_create": {
		Payload: template.CreateInput{
			ID: "farewell-email", Name: "Farewell Email", Content: "Bye, {{ name }}!", Reasoning: reason(),
		},
	},
	"templates_update": {
		Payload: template.UpdateInput{ID: "welcome-email", Name: ptr("Welcome Email v2"), Reasoning: reason()},
		Seed:    seedTemplate,
	},
	"templates_render": {
		Payload: template.RenderInput{ID: "welcome-email", Variables: map[string]any{"name": "Ada"}, Reasoning: reason()},
		Seed:    seedTemplate,
	},
	"templates_delete": {
		Payload: template.DeleteInput{ID: "welcome-email", Reasoning: reason()},
		Seed:    seedTemplate,
	},

	"tunnel_status": {
		Payload: tunnel.StatusInput{Reasoning: reason()},
	},
	"tunnel_stop": {
		Payload: tunnel.StopInput{Reasoning: reason()},
	},

	"update_status": {
		Payload: update.StatusInput{Reasoning: reason()},
	},
}

func seedTheme(t *testing.T, a *app.App) {
	t.Helper()
	if _, err := a.Themes.Install(parityCtx(), theme.InstallInput{
		ID: "midnight",
		Variants: map[theme.Appearance]theme.Palette{
			theme.Dark: {Surface: "#0b0d12", Ink: "#e6e9ef", Accent: "#7aa2f7", Contrast: 70},
		},
	}); err != nil {
		t.Fatal(err)
	}
}

// seedTask creates the task the subcollections hang off. Its identifier is m-1
// because the identifier sequence is per installation and this is the first
// record these scenarios create.
func seedTask(t *testing.T, a *app.App) {
	t.Helper()
	seedWorkspace(t, a)
	if _, err := a.Tasks.Create(parityCtx(), task.CreateInput{
		Name: "Fix the denial pattern", Type: "bug", Status: task.Todo,
	}); err != nil {
		t.Fatal(err)
	}
}

func seedTodo(t *testing.T, a *app.App) {
	t.Helper()
	seedTask(t, a)
	if _, err := a.Todos.Create(parityCtx(), todo.CreateInput{
		Task: "m-1", Title: "Write the failing test",
	}); err != nil {
		t.Fatal(err)
	}
}

func seedComment(t *testing.T, a *app.App) {
	t.Helper()
	seedTask(t, a)
	if _, err := a.Comments.Create(parityCtx(), comment.CreateInput{
		Task: "m-1", Body: "Reproduced it.",
	}); err != nil {
		t.Fatal(err)
	}
}

func seedRoutine(t *testing.T, a *app.App) {
	t.Helper()
	seedAgent(t, a)
	if _, err := a.Routines.Create(parityCtx(), routine.CreateInput{
		Name:    "Triage new bugs",
		Content: "List the bugs in the backlog.",
	}); err != nil {
		t.Fatal(err)
	}
}

func seedJob(t *testing.T, a *app.App) {
	t.Helper()
	if a.Queue == nil {
		t.Skip("this installation has no queue")
	}
	if _, err := a.Queue.Enqueue(parityCtx(), job.Job{
		ID: "j-1", Queue: job.QueueChat, Kind: "turn",
	}); err != nil {
		t.Fatal(err)
	}
}

// seedCollection declares one workspace collection every collections_* and
// views_* scenario below builds on.
func seedCollection(t *testing.T, a *app.App) {
	t.Helper()
	if _, err := a.Collections.Create(parityCtx(), collection.CreateInput{
		ID: "contacts", Name: "Contacts", Format: collection.FormatJSON,
		Fields: []collection.Field{{Name: "name", Type: collection.TypeString, Required: true}},
	}); err != nil {
		t.Fatal(err)
	}
}

// seedCollectionRecord files the one row every records-get/update/delete
// scenario names by id. ids.Sequence hands out "m-1" on the first call a
// fresh app makes to it, and nothing between a fresh app and this call asks
// it for one — collection declarations carry their own id, not a generated
// one — so the id is deterministic without this test naming it twice.
func seedCollectionRecord(t *testing.T, a *app.App) {
	t.Helper()
	seedCollection(t, a)
	if _, err := a.Collections.Records().Create(parityCtx(), "contacts", map[string]any{"name": "Ada Lovelace"}); err != nil {
		t.Fatal(err)
	}
}

// scaffoldedTableTree is the exact shape view.Scaffold produces for a Table
// over seedCollection's "contacts" — proven to pass Validate by the view
// domain's own scaffold tests, reused here rather than invented so a parity
// fixture is never the first place this shape is tried.
func scaffoldedTableTree() view.Node {
	return view.Node{
		Component: "Table",
		Props:     map[string]any{"columns": []any{"name"}, "rows": []any{}},
		Actions:   []view.Action{{Label: "Refresh", Command: "collections_list"}},
	}
}

// seedView composes one view over seedCollection's "contacts" — a Table
// whose one action dispatches collections_list, so views_execute-action has
// something real and deterministic to invoke.
func seedView(t *testing.T, a *app.App) {
	t.Helper()
	seedCollection(t, a)
	if _, err := a.Views.Create(parityCtx(), view.CreateInput{
		ID: "contacts-table", Title: "Contacts",
		Source: view.Source{Collection: "contacts"},
		Tree:   scaffoldedTableTree(),
	}); err != nil {
		t.Fatal(err)
	}
}

// seedToolset writes one toolset's declaration directly through the same
// filesystem repository the daemon's own wiring builds (collections.ModelOf
// plus fscollections.New), bypassing toolset.Service — which deliberately
// has no Create of its own; a toolset reaches disk the way every native
// record does, never through this domain (see toolset.Repository's own
// doc). Command is "true": get, get-config, update-config, list and delete
// never connect to it, and toolsets_call — the one command that would — is
// excluded above.
func seedToolset(t *testing.T, a *app.App) {
	t.Helper()
	model, err := collections.ModelOf[toolset.Toolset]("toolsets")
	if err != nil {
		t.Fatal(err)
	}
	repo := fscollections.New(a.Workspace, model)
	ts := toolset.Toolset{
		ID: "gh", Type: toolset.MCPStdio, Status: toolset.StatusEnabled,
		Command: "true", CreatedAt: refTime, UpdatedAt: refTime,
	}
	if err := repo.Create(context.Background(), &ts); err != nil {
		t.Fatal(err)
	}
}

// seedSkill installs the committed crm-skill fixture (internal/app/testdata/
// crm-skill) through the Go API directly, with consent already given —
// AcceptedAll, not the approval channel skills_install itself always goes
// through (see the "skills_install" exclusion above). Every other Seed
// helper in this file does the same thing for the same reason: it drives
// the domain, not a surface, to set up state a surface is then tested
// against.
func seedSkill(t *testing.T, a *app.App) {
	t.Helper()
	if _, err := a.Skills.Install(parityCtx(), skill.InstallInput{
		Source:      "testdata/crm-skill",
		AcceptedAll: func(skill.Permissions) bool { return true },
	}); err != nil {
		t.Fatal(err)
	}
}

func seedArtifact(t *testing.T, a *app.App) {
	t.Helper()
	if _, err := a.Artifacts.Create(parityCtx(), artifact.CreateInput{
		ID: "dashboard", Name: "Dashboard",
	}); err != nil {
		t.Fatal(err)
	}
}

func seedGoal(t *testing.T, a *app.App) {
	t.Helper()
	if _, err := a.Goals.Create(parityCtx(), goal.CreateInput{
		Title: "Grow Revenue",
	}); err != nil {
		t.Fatal(err)
	}
}

// seedInstruction creates the record a scenario expects already there before
// it runs — not context.Background() as an oversight, but why the seed
// scenarios are still worth running at all: parityCtx() (Create's own
// approval gate) is what the excluded "instructions_create" and
// "instructions_update" scenarios cannot get past in this harness, and
// seeding here would hit exactly the same unanswered channel for the
// commands that do not even need it — instructions_list, instructions_get
// and instructions_delete undergo no approval themselves. This is the
// state, not the behaviour under test.
func seedInstruction(t *testing.T, a *app.App) {
	t.Helper()
	if _, err := a.Instructions.Create(context.Background(), instruction.CreateInput{
		ID: "protocol", Name: "Protocol",
	}); err != nil {
		t.Fatal(err)
	}
}

func seedProject(t *testing.T, a *app.App) {
	t.Helper()
	if _, err := a.Projects.Create(parityCtx(), project.CreateInput{
		ID: "launch", Name: "Launch",
	}); err != nil {
		t.Fatal(err)
	}
}

func seedTemplate(t *testing.T, a *app.App) {
	t.Helper()
	if _, err := a.Templates.Create(parityCtx(), template.CreateInput{
		ID: "welcome-email", Name: "Welcome Email", Content: "Hello, {{ name }}!",
	}); err != nil {
		t.Fatal(err)
	}
}

func reason() command.Reasoning {
	return command.Reasoning{Reasoning: "surface parity check"}
}

func ptr[T any](v T) *T { return &v }

// TestEveryCommandHasAParityScenario is the gate that keeps the suite honest:
// publishing a capability without covering it here fails the build.
func TestEveryCommandHasAParityScenario(t *testing.T) {
	a := newApp(t)
	for _, d := range a.Registry.Sorted() {
		if _, ok := scenarios[d.Key()]; ok {
			continue
		}
		if reason, ok := excluded[d.Key()]; ok {
			t.Logf("%s is not covered: %s", d.Key(), reason)
			continue
		}
		t.Errorf("%s is registered but has no parity scenario", d.Key())
	}
	for key := range scenarios {
		if _, _, ok := a.Registry.Lookup(key); !ok {
			t.Errorf("the parity scenario %q covers a command that no longer exists", key)
		}
		if _, both := excluded[key]; both {
			t.Errorf("%q is both covered and excluded", key)
		}
	}
	for key := range excluded {
		if _, _, ok := a.Registry.Lookup(key); !ok {
			t.Errorf("the exclusion %q names a command that no longer exists", key)
		}
	}
}

// TestSurfaceParity is the claim of the whole phase: one definition, four
// surfaces, the same effect and the same normalised result.
//
// Without it, ~140 capabilities times five surfaces are 700 points of manual
// synchronisation, and they diverge in weeks.
func TestSurfaceParity(t *testing.T) {
	for key, sc := range scenarios {
		t.Run(key, func(t *testing.T) {
			if sc.PayloadIn != nil {
				// Owned by this subtest and removed with it, so the five
				// surfaces neither inherit each other's work nor outlive the
				// run the way the old /tmp path did.
				sc.Outside = filepath.Join(t.TempDir(), "outside")
				sc.Payload = sc.PayloadIn(sc.Outside)
			}
			payload, err := json.Marshal(sc.Payload)
			if err != nil {
				t.Fatal(err)
			}

			viaAgent := runInternal(t, sc, key, payload)
			viaCLI := runCLI(t, sc, key, payload)
			viaHTTP := runHTTP(t, sc, key, payload)
			viaFlat := runMCP(t, sc, key, payload, mcpserver.ShapeFlat)
			viaComposite := runMCP(t, sc, key, payload, mcpserver.ShapeComposite)

			for _, other := range []struct {
				name string
				got  string
			}{
				{"cli", viaCLI},
				{"http", viaHTTP},
				{"mcp flat", viaFlat},
				{"mcp composite", viaComposite},
			} {
				if other.got != viaAgent {
					t.Errorf("%s differs from the agent registry.\n--- agent ---\n%s\n--- %s ---\n%s",
						other.name, viaAgent, other.name, other.got)
				}
			}
		})
	}
}

// runInternal is the agent's own registry: the descriptor invoked in process,
// with no transport at all.
func runInternal(t *testing.T, sc scenario, key string, payload json.RawMessage) string {
	t.Helper()
	emptyOutside(t, sc)
	a := newApp(t)
	if sc.Seed != nil {
		sc.Seed(t, a)
	}
	d, _, ok := a.Registry.Lookup(key)
	if !ok {
		t.Fatalf("%s is not registered", key)
	}
	out, err := d.Invoke(parityCtx(), command.SurfaceAgent, payload)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	return stabilise(a, canonical(t, out))
}

func runCLI(t *testing.T, sc scenario, key string, payload json.RawMessage) string {
	t.Helper()
	emptyOutside(t, sc)
	a := newApp(t)
	if sc.Seed != nil {
		sc.Seed(t, a)
	}
	d, _, ok := a.Registry.Lookup(key)
	if !ok {
		t.Fatalf("%s is not registered", key)
	}
	argv, err := clix.CommandLineFor(d, payload)
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	root := clix.NewRoot(clix.Config{
		Registry: a.Registry,
		Out:      &stdout,
		Err:      &stderr,
		IsTTY:    func() bool { return false }, // a program is watching: JSON
	})
	root.SetArgs(append(argv, "--format", "json"))
	if err := root.ExecuteContext(parityCtx()); err != nil {
		t.Fatalf("cli %v: %v\nstderr: %s", argv, err, stderr.String())
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("cli output is not an envelope: %v\n%s", err, stdout.String())
	}
	return stabilise(a, canonicalRaw(t, envelope.Data))
}

// runHTTP is the fifth surface: the same payload posted to the route the
// registry generated, through a real HTTP server and a real client.
//
// Authentication is off here for the same reason the other runners carry no
// credential — this suite asks whether the surfaces agree about what a command
// does, and the answer must not depend on who is asking. Whether the door is
// locked is the HTTP transport's own suite.
func runHTTP(t *testing.T, sc scenario, key string, payload json.RawMessage) string {
	t.Helper()
	emptyOutside(t, sc)
	a := newApp(t)
	if sc.Seed != nil {
		sc.Seed(t, a)
	}
	d, _, ok := a.Registry.Lookup(key)
	if !ok {
		t.Fatalf("%s is not registered", key)
	}

	server := httptest.NewServer(httpapi.New(httpapi.Config{
		Registry:        a.Registry,
		SecurityEnabled: func() bool { return false },
		Log:             slog.New(slog.NewTextHandler(io.Discard, nil)),
	}).Handler())
	defer server.Close()

	req, err := http.NewRequestWithContext(parityCtx(), http.MethodPost,
		server.URL+httpapi.RouteOf(d), bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	// The ambient identity travels in headers here, where the other surfaces
	// take it from the context. That is the transport's job, and getting it
	// wrong is exactly what this suite would catch.
	req.Header.Set(httpapi.HeaderAgent, "atlas")
	req.Header.Set(httpapi.HeaderWorkspace, activeWorkspace)

	res, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("http %s returned %d: %s", key, res.StatusCode, body)
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("http output is not an envelope: %v\n%s", err, body)
	}
	return stabilise(a, canonicalRaw(t, envelope.Data))
}

func runMCP(t *testing.T, sc scenario, key string, payload json.RawMessage, shape mcpserver.Shape) string {
	t.Helper()
	emptyOutside(t, sc)
	a := newApp(t)
	if sc.Seed != nil {
		sc.Seed(t, a)
	}
	ctx := parityCtx()

	server := mcpserver.New(mcpserver.Config{Registry: a.Registry, Shape: shape})
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = serverSession.Wait() }()

	client := mcp.NewClient(&mcp.Implementation{Name: "parity", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()

	name, args := key, payload
	if shape == mcpserver.ShapeComposite {
		d, _, _ := a.Registry.Lookup(key)
		group, _ := a.Registry.GroupOf(d.Group())
		name = group.Tool
		// A real client puts `_reasoning` next to `action`, not inside
		// `input`: the per-action schema does not contain it.
		args, err = json.Marshal(mcpserver.CompositeInput{
			Action:    d.Name(),
			Input:     withoutReasoning(t, payload),
			Reasoning: "surface parity check",
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("call %s returned an error: %s", name, textOf(res))
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(textOf(res)), &envelope); err != nil {
		t.Fatalf("mcp output is not an envelope: %v\n%s", err, textOf(res))
	}
	return stabilise(a, canonicalRaw(t, envelope.Data))
}

// stabilise replaces the paths that differ between surfaces with placeholders.
//
// Each surface gets its own installation and its own repository, so the state
// directory and the workspace root are necessarily different strings. Those are
// the environment, not the answer: comparing them would make the suite assert
// that four temporary directories have the same name, which is both false and
// beside the point.
func stabilise(a *app.App, out string) string {
	out = strings.ReplaceAll(out, a.Workspace, "<workspace>")
	return strings.ReplaceAll(out, a.Paths.Root, "<state>")
}

// withoutReasoning strips the field that belongs to the composite payload
// rather than to the action.
func withoutReasoning(t *testing.T, payload json.RawMessage) json.RawMessage {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatal(err)
	}
	delete(fields, command.ReasoningField)
	out, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func textOf(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// canonical renders a result the same way regardless of the Go type it came
// back as, so two surfaces are compared on what they say, not on how they say it.
func canonical(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return canonicalRaw(t, raw)
}

func canonicalRaw(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("result is not JSON: %v\n%s", err, raw)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
