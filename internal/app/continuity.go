package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/domain/activity"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/chat"
	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/domain/job"
	"github.com/OWNER/aos/internal/domain/routine"
	"github.com/OWNER/aos/internal/domain/task"
	"github.com/OWNER/aos/internal/domain/todo"
	"github.com/OWNER/aos/internal/domain/workspace"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/sandbox"
	"github.com/OWNER/aos/internal/runtime/session"
	"github.com/OWNER/aos/internal/transport/realtime"
)

// planner adapts the todo aggregate to the review guard's port.
//
// It exists because task.Progress and todo.Progress are the same three numbers
// in two packages, and neither may import the other: a subcollection that could
// reach its parent could move it, and a parent that could reach the plan could
// rewrite what it is being judged against.
type planner struct{ todos *todo.Service }

func (p planner) CountPending(ctx context.Context, taskID string) (int, error) {
	return p.todos.CountPending(ctx, taskID)
}

func (p planner) PendingIDs(ctx context.Context, taskID string) ([]string, error) {
	return p.todos.PendingIDs(ctx, taskID)
}

func (p planner) Progress(ctx context.Context, taskID string) (task.Progress, error) {
	got, err := p.todos.Progress(ctx, taskID)
	if err != nil {
		return task.Progress{}, err
	}
	return task.Progress{Completed: got.Completed, Total: got.Total}, nil
}

// assignees resolves a task's owner to what it actually is.
//
// The answer decides execution policy: only an agent is dispatched. It is asked
// afresh each time rather than stored, so a task whose agent was deleted stops
// being dispatchable instead of keeping a label that says it still is.
type assignees struct{ agents *agent.Service }

func (a assignees) Resolve(ctx context.Context, id string) (task.ResolvedAssignee, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return task.ResolvedAssignee{Type: task.AssigneeUnknown}, nil
	}
	found, err := a.agents.Get(ctx, agent.GetInput{ID: trimmed})
	if err == nil && found != nil {
		return task.ResolvedAssignee{
			ID: found.ID, Type: task.AssigneeAgent,
			Name: found.DisplayName(), Role: found.Role,
		}, nil
	}
	// Not an agent. It may be a user identifier or a name nobody recognises,
	// and the two are treated the same here: neither receives dispatch.
	return task.ResolvedAssignee{ID: trimmed, Type: task.AssigneeUnknown}, nil
}

// taskPolicy reads the workspace's isolation policy and task taxonomy.
type taskPolicy struct {
	workspaces *workspace.Service
	active     string
	root       string
}

func (p taskPolicy) Worktrees(ctx context.Context) (task.WorktreePolicy, error) {
	out := task.WorktreePolicy{
		BranchPrefix: task.DefaultBranchPrefix,
		Limit:        workspace.DefaultWorktrees().WorktreeLimit,
		DeleteOld:    workspace.DefaultWorktrees().DeleteOldWorktrees,
		Root:         p.root,
	}
	current, err := p.workspaces.Get(ctx, workspace.GetInput{Workspace: p.active})
	if err != nil || current == nil {
		// A workspace that is not registered yet still gets the defaults, so a
		// task can be branched in a repository nobody has introspected.
		return out, nil //nolint:nilerr // the defaults are the answer, not a failure
	}
	if current.Git.BranchPrefix != "" {
		out.BranchPrefix = current.Git.BranchPrefix
	}
	if current.Worktrees.WorktreeLimit > 0 {
		out.Limit = current.Worktrees.WorktreeLimit
	}
	out.DeleteOld = current.Worktrees.DeleteOldWorktrees
	out.OnCreateScript = current.Worktrees.OnCreateScript
	return out, nil
}

func (p taskPolicy) TaskTypes(ctx context.Context) ([]string, error) {
	current, err := p.workspaces.Get(ctx, workspace.GetInput{Workspace: p.active})
	if err != nil || current == nil {
		return nil, nil //nolint:nilerr // an unregistered workspace constrains nothing
	}
	out := make([]string, 0, len(current.Tasks))
	for _, t := range current.Tasks {
		out = append(out, t.ID)
	}
	return out, nil
}

// setupScript runs a workspace's onCreateScript inside a fresh checkout.
//
// It runs under the assigned agent's sandbox policy rather than with free rein,
// which is the divergence recorded in the Task note: a setup script is
// third-party code in most workspaces, and the original executes it unguarded.
type setupScript struct {
	agents *agent.Service
	tmp    string
	log    *slog.Logger
}

func (s setupScript) Run(ctx context.Context, agentID, dir, script string) error {
	perms := sandbox.Permissions{Read: true, Write: true, Execute: true}
	exec := sandbox.ExecPolicy{Policy: sandbox.PolicyAllowlist}

	if agentID != "" {
		found, err := s.agents.Get(ctx, agent.GetInput{ID: agentID})
		if err == nil && found != nil && found.Sandbox != nil {
			perms = sandbox.PermissionsFrom(found.Sandbox.Permissions)
			perms.Execute = true // a setup script that cannot run is not a setup script
			if found.Sandbox.Exec != nil {
				exec = sandbox.ExecPolicy{
					Policy:     found.Sandbox.Exec.Policy,
					Allow:      found.Sandbox.Exec.Allow,
					DenyArgs:   found.Sandbox.Exec.DenyArgs,
					AllowShell: found.Sandbox.Exec.AllowShell,
				}
			}
		}
	}

	box, err := sandbox.New(sandbox.Options{
		WorktreePath: dir, TmpDir: s.tmp, Permissions: perms, Exec: exec,
	})
	if err != nil {
		return err
	}

	// A script is a command line, so it runs through a shell — which the
	// allowlist only permits when the agent's own policy opted into one. An
	// agent that did not is refused here rather than having a setup script run
	// behind its back at a privilege it never asked for.
	name, args := shellFor(script)
	out, err := box.Run(ctx, sandbox.Command{
		Name: name, Args: args, Dir: dir, Timeout: 5 * time.Minute,
	})
	if err != nil {
		return err
	}
	if out.ExitCode != 0 {
		return fmt.Errorf("the setup script exited %d: %s",
			out.ExitCode, strings.TrimSpace(out.Stderr.Content))
	}
	s.log.Info("the workspace setup script ran in a new worktree", "path", dir)
	return nil
}

// shellFor builds the invocation of a command line. The shell is named
// explicitly so the sandbox's allowlist sees it and can refuse it: a script run
// through an unnamed shell would be a hole in the policy rather than a use of
// it.
func shellFor(script string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", script}
	}
	return "sh", []string{"-c", script}
}

// tokens mints and verifies webhook secrets.
//
// The domain's rule is only that the file holds a hash and the token is shown
// once; this is where the hashing lives. SHA-256 is enough: the token is 256
// bits of randomness, so there is no dictionary to attack, and the comparison
// is constant-time because the hash is what an attacker gets to guess against.
type tokens struct{}

func (tokens) New() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}

func (tokens) Verify(token, hash string) bool {
	if token == "" || hash == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(hashToken(token)), []byte(hash)) == 1
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// agentDirectory answers whether an identifier names an agent.
type agentDirectory struct{ agents *agent.Service }

func (d agentDirectory) IsAgent(ctx context.Context, id string) bool {
	found, err := d.agents.Get(ctx, agent.GetInput{ID: strings.TrimSpace(id)})
	return err == nil && found != nil
}

// taskActivity turns a task mutation into an entry in the activity log, which
// is also what a routine with an activity trigger reacts to.
type taskActivity struct {
	activities *activity.Service
	log        *slog.Logger
}

func (n taskActivity) TaskChanged(ctx context.Context, event string, t *task.Task, data map[string]any) {
	if n.activities == nil {
		return
	}
	payload := map[string]any{"task": t.ID, "name": t.Name, "type": t.Type, "status": string(t.Status)}
	for k, v := range data {
		payload[k] = v
	}
	if _, err := n.activities.Publish(ctx, activity.PublishInput{
		Namespace: "task", Event: event,
		Title: titleFor(event, t), Icon: "CheckSquare", Data: payload,
	}); err != nil {
		// The activity log is a consequence of the mutation, not a condition of
		// it. Losing an entry is worth a line; failing the move is not.
		n.log.Warn("a task change was not published to the activity log",
			"task", t.ID, "event", event, "err", err)
	}
}

func titleFor(event string, t *task.Task) string {
	switch event {
	case "status_changed":
		return t.Name + " moved to " + string(t.Status)
	case "created":
		return "New task: " + t.Name
	case "deleted":
		return "Deleted: " + t.Name
	case "branched":
		return t.Name + " was branched"
	default:
		return t.Name + " " + event
	}
}

// routineActivity records a routine firing.
type routineActivity struct {
	activities *activity.Service
	log        *slog.Logger
}

func (n routineActivity) RoutineFired(ctx context.Context, r *routine.Routine, run *routine.Run) {
	if n.activities == nil {
		return
	}
	if _, err := n.activities.Publish(ctx, activity.PublishInput{
		Namespace: "routine", Event: "fired",
		Title: r.Name + " " + string(run.Status),
		Icon:  "Repeat",
		Data: map[string]any{
			"routine": r.ID, "agent": r.Agent, "run": run.ID,
			"status": string(run.Status), "trigger": string(run.Trigger),
		},
	}); err != nil {
		n.log.Warn("a routine run was not published to the activity log",
			"routine", r.ID, "err", err)
	}
}

// routineTriggers is the sink that closes the reactive loop: an activity is
// published, and every routine whose trigger matches it fires.
type routineTriggers struct{ routines *routine.Service }

func (t routineTriggers) OnActivity(ctx context.Context, a activity.Activity) {
	if t.routines == nil {
		return
	}
	t.routines.OnActivity(ctx, a.Namespace, a.Event, a.Data)
}

// realtimeSink pushes an activity to whoever is watching.
type realtimeSink struct {
	hub   *realtime.Hub
	scope *eventScope
}

func (s realtimeSink) OnActivity(ctx context.Context, a activity.Activity) {
	if s.hub == nil {
		return
	}
	// Resolved per publish, not captured at wiring: see eventScope.
	workspaceID := s.scope.ID(ctx)
	s.hub.Publish(ctx, realtime.ChannelFor(workspaceID), realtime.Event{
		Type: realtime.EventActivity, Workspace: workspaceID, Data: a,
	})
}

// routineExecutor runs a routine's prompt as a real turn.
//
// It opens a conversation with the routine's own agent, sends the routine body
// as the message, and returns the conversation so the run record points at the
// transcript. That is what makes a run auditable rather than a status word.
type routineExecutor struct {
	chats   *chat.Service
	runtime *session.Runner
	log     *slog.Logger
}

func (e routineExecutor) Execute(ctx context.Context, req routine.Execution) (routine.Outcome, error) {
	if e.chats == nil || e.runtime == nil {
		return routine.Outcome{}, fmt.Errorf("this installation has no runtime to execute a routine")
	}

	created, err := e.chats.Create(ctx, chat.CreateInput{
		Title: "Routine: " + req.Routine,
		Agent: req.Agent,
	})
	if err != nil {
		return routine.Outcome{}, err
	}

	// Post rather than Send: Send dispatches, and a routine that both dispatched
	// and ran the turn itself would take two turns for one message. The run has
	// to finish before the run record can say how it went, so the turn is taken
	// here, synchronously.
	sent, err := e.chats.Post(ctx, created.ID, routinePrompt(req))
	if err != nil {
		return routine.Outcome{ChatID: created.ID}, err
	}

	result, err := e.runtime.Run(ctx, chat.Turn{
		ChatID: created.ID, MessageID: sent.ID,
		AgentID: req.Agent, Routine: req.Routine,
	})
	out := routine.Outcome{ChatID: created.ID}
	if result != nil {
		out.Usage = routine.Usage{
			Input:   result.Usage.Input,
			Output:  result.Usage.Output,
			Total:   result.Usage.Total,
			CostUSD: result.Usage.CostUSD,
		}
	}
	return out, err
}

// routinePrompt frames the routine body with the mode's rules.
//
// The framing is the original's routine mode: nobody may be present, no input
// will arrive, the run has to complete or say why it could not, and the scope
// is a boundary rather than advice.
func routinePrompt(req routine.Execution) string {
	var b strings.Builder
	b.WriteString("You are running as a routine. Nobody is watching this conversation.\n\n")
	b.WriteString("- Do not ask a question and wait: no answer will come.\n")
	b.WriteString("- Complete the work or record why you could not.\n")
	b.WriteString("- Act only within what this routine is allowed to do.\n")
	if !req.Scope.AllowCreateTasks {
		b.WriteString("- You may not create tasks from this routine.\n")
	}
	if !req.Scope.AllowExternalCalls {
		b.WriteString("- You may not reach outside this machine from this routine.\n")
	}
	if len(req.Payload) > 0 {
		if raw, err := json.Marshal(req.Payload); err == nil {
			b.WriteString("\n## What triggered this run\n\n```json\n")
			b.Write(raw)
			b.WriteString("\n```\n")
		}
	}
	b.WriteString("\n## What to do\n\n")
	b.WriteString(req.Prompt)
	return b.String()
}

// subconsciousModels resolves the observer's own slot.
//
// The cascade is the original's: the subconscious slot, then the agent's own
// model, then the default slot. The point of the first level is to run a cheap,
// frequent observer beside an expensive, deep main agent — on the machine the
// reverse engineering looked at, the user had not separated them, and both
// pointed at the same large model.
type subconsciousModels struct {
	config config.Service
	agents *agent.Service
	home   string
	build  func(provider, key string) (agentloop.LLMProvider, error)
}

func (m subconsciousModels) Subconscious(ctx context.Context, agentID string) (agentloop.LLMProvider, agentloop.ModelRef, error) {
	current, err := m.config.Get(ctx, config.GetInput{})
	if err != nil {
		return nil, agentloop.ModelRef{}, err
	}

	var own agentloop.AgentModel
	if found, err := m.agents.Get(ctx, agent.GetInput{ID: agentID}); err == nil && found != nil {
		own = agentloop.AgentModel{Provider: found.Provider, Model: found.Model, Reasoning: found.Reasoning}
	}

	slot := current.Agents.Models[config.SlotSubconscious]
	fallback := current.Agents.Models[config.SlotDefault]

	// Resolve takes two levels; the third is folded in by preferring the
	// subconscious slot and falling back to the default one as the config level.
	configured := agentloop.ConfigModel{
		Provider: slot.Provider, Model: slot.Model, Reasoning: slot.Reasoning,
	}
	if configured.Model == "" {
		configured = agentloop.ConfigModel{
			Provider: fallback.Provider, Model: fallback.Model, Reasoning: fallback.Reasoning,
		}
	}

	ref, err := agentloop.Resolve(own, configured)
	if err != nil {
		return nil, agentloop.ModelRef{}, err
	}
	provider, err := m.build(ref.Provider, keyFor(current, ref.Provider))
	if err != nil {
		return nil, agentloop.ModelRef{}, err
	}
	return provider, ref, nil
}

// turnHandler runs a queued conversation turn.
type turnHandler struct{ runtime *session.Runner }

func (h turnHandler) Handle(ctx context.Context, j job.Job) (json.RawMessage, error) {
	var turn chat.Turn
	if err := json.Unmarshal(j.Payload, &turn); err != nil {
		return nil, err
	}
	result, err := h.runtime.Run(ctx, turn)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"chat": turn.ChatID, "steps": result.Steps, "usage": result.Usage,
	})
}

// routineTick evaluates the cron triggers once per window.
func routineTick(routines *routine.Service, workspaceID string) func(context.Context, time.Time) error {
	return func(ctx context.Context, now time.Time) error {
		// The tick acts as the system, not as any agent: a routine that fires
		// on a schedule was not asked for by whoever last used the terminal.
		ctx = identity.With(ctx, identity.Identity{WorkspaceID: workspaceID})
		out, err := routines.ProcessScheduled(ctx, now)
		if err != nil {
			return err
		}
		if len(out.Fired) > 0 || len(out.Broken) > 0 {
			slog.Default().Info("the scheduler evaluated the routines",
				"fired", len(out.Fired), "failed", len(out.Failed), "broken", len(out.Broken))
		}
		return nil
	}
}

// activityRetention purges the activity log on the tick.
func activityRetention(activities *activity.Service) func(context.Context, time.Time) error {
	return func(ctx context.Context, _ time.Time) error {
		_, err := activities.Purge(ctx, activity.PurgeInput{})
		return err
	}
}

// jobRetention drops finished jobs on the tick.
func jobRetention(jobs job.Queue) func(context.Context, time.Time) error {
	return func(ctx context.Context, _ time.Time) error {
		_, err := jobs.Purge(ctx, 7*24*time.Hour)
		return err
	}
}

// The kinds of queued work this build knows.
const (
	kindTurn = build.Name + ".turn"
)
