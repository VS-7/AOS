package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/domain/activity"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/chat"
	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/domain/goal"
	"github.com/OWNER/aos/internal/domain/job"
	"github.com/OWNER/aos/internal/domain/project"
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

	// root is this workspace's own directory of checkouts (worktreeRootFor),
	// and legacyRoot the installation-wide one every workspace shared before.
	root       string
	legacyRoot string
}

// worktreeRootFor is the directory the checkouts of the workspace at dir go
// in: one per workspace, under the installation's data directory.
//
// They all went into one directory, and that stopped being safe once a
// workspace that is a folder of a project cuts its checkouts from the project:
// two folders of one monorepo then share a repository, `git worktree list`
// shows each the other's checkouts under the shared directory, and the prune
// took the other's for leftovers of its own and removed them with --force.
//
// Named after the directory rather than the registry id, because the same
// directory is the primary scope of a daemon started in it — with no id — and
// a secondary scope of another, and its tasks' checkouts are the same in
// both. Resolved first, so a link to it names the same directory. The folder's
// own name is kept in front of the digest for whoever looks inside.
func worktreeRootFor(data, dir string) string {
	resolved := filepath.Clean(dir)
	if abs, err := filepath.Abs(resolved); err == nil {
		resolved = abs
	}
	if real, err := filepath.EvalSymlinks(resolved); err == nil {
		resolved = real
	}
	sum := sha256.Sum256([]byte(resolved))
	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return '-'
		}
	}, filepath.Base(resolved))
	name = strings.Trim(name, "-")
	if len(name) > 32 {
		name = name[:32]
	}
	if name == "" {
		name = "workspace"
	}
	return filepath.Join(data, "worktrees", name+"-"+hex.EncodeToString(sum[:6]))
}

func (p taskPolicy) Worktrees(ctx context.Context) (task.WorktreePolicy, error) {
	out := task.WorktreePolicy{
		BranchPrefix: task.DefaultBranchPrefix,
		Limit:        workspace.DefaultWorktrees().WorktreeLimit,
		DeleteOld:    workspace.DefaultWorktrees().DeleteOldWorktrees,
		Root:         p.root,
		LegacyRoot:   p.legacyRoot,
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

// recordActivity is what every one of the small notifiers below does: publish,
// and warn rather than fail when the log is down.
//
// The activity log is a consequence of a mutation, not a condition of it —
// losing an entry is worth a line, refusing the write is not.
func recordActivity(ctx context.Context, activities *activity.Service, log *slog.Logger, in activity.PublishInput) {
	if activities == nil {
		return
	}
	if _, err := activities.Publish(ctx, in); err != nil {
		log.Warn("a change was not published to the activity log",
			"namespace", in.Namespace, "event", in.Event, "err", err)
	}
}

// projectActivity, goalActivity and agentActivity are what the interface's
// inbox and the routine triggers read.
//
// None of the three published anything. `collection.changed` tells a cache
// what to refetch and nothing more: it carries no title, so the inbox cannot
// show it, and routines only ever react to activities
// (routineTriggers.OnActivity). The original app emitted created / updated /
// deleted for all three, which is what these restore.
//
// The namespace is the singular feature name on purpose: `lib/realtime.ts`
// invalidates the react-query key `[namespace]`, and those keys are named
// after the interface's features — `project`, not `projects`.
type projectActivity struct {
	activities *activity.Service
	log        *slog.Logger
}

func (n projectActivity) ProjectChanged(ctx context.Context, event string, p *project.Project) {
	recordActivity(ctx, n.activities, n.log, activity.PublishInput{
		Namespace: "project", Event: event,
		Title: titleForRecord(event, "project", p.Name, p.ID),
		Icon:  "FolderKanban",
		Data:  map[string]any{"project": p.ID, "name": p.Name, "status": string(p.Status)},
	})
}

type goalActivity struct {
	activities *activity.Service
	log        *slog.Logger
}

func (n goalActivity) GoalChanged(ctx context.Context, event string, g *goal.Goal) {
	recordActivity(ctx, n.activities, n.log, activity.PublishInput{
		Namespace: "goal", Event: event,
		Title: titleForRecord(event, "goal", g.Title, g.ID),
		Icon:  "Target",
		Data:  map[string]any{"goal": g.ID, "title": g.Title, "status": string(g.Status)},
	})
}

type agentActivity struct {
	activities *activity.Service
	log        *slog.Logger
}

func (n agentActivity) AgentChanged(ctx context.Context, event string, a *agent.Agent) {
	recordActivity(ctx, n.activities, n.log, activity.PublishInput{
		Namespace: "agent", Event: event,
		Title: titleForRecord(event, "agent", a.DisplayName(), a.ID),
		Icon:  "Bot",
		Data:  map[string]any{"agent": a.ID, "name": a.DisplayName()},
	})
}

// titleForRecord is the line the inbox shows. A deleted record has only its
// id left to name it, which is why the id is the fallback rather than an
// empty string.
func titleForRecord(event, kind, name, id string) string {
	if strings.TrimSpace(name) == "" {
		name = id
	}
	switch event {
	case "created":
		return "New " + kind + ": " + name
	case "deleted":
		return "Deleted " + kind + ": " + name
	default:
		return name + " " + event
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

	// A run's transcript, not a channel: the sidebar files a conversation by
	// its kind, and an untyped one with an agent in it read as a DM titled
	// with the routine's UUID, while the Runs tab stayed empty.
	title := req.Name
	if strings.TrimSpace(title) == "" {
		title = req.Routine
	}
	created, err := e.chats.Create(ctx, chat.CreateInput{
		Title:   "Routine: " + title,
		Kind:    chat.KindRun,
		Routine: req.Routine,
		Agent:   req.Agent,
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
	// Raw, for the reason models.For gives: Get redacts every key to a
	// fingerprint, and keyFor below would then hand the adapter "***…1234" to
	// authenticate with. Every observation on an API-key provider was refused
	// that way, silently, because a failed observation is only a warning.
	current, err := m.config.Raw(ctx)
	if err != nil {
		return nil, agentloop.ModelRef{}, err
	}

	slot := current.Agents.Models[config.SlotSubconscious]
	fallback := current.Agents.Models[config.SlotDefault]
	configured := agentloop.ConfigModel{
		Provider: fallback.Provider, Model: fallback.Model, Reasoning: fallback.Reasoning,
	}

	// The first level decides alone when it is set. It went through Resolve
	// as the configuration level, and Resolve ranks an agent's own model above
	// the configuration — right for the agent's turn, backwards here: an
	// agent that named an expensive model was observed with it, and the cheap
	// slot somebody set for exactly that case was ignored. Resolve still reads
	// it, as the agent level, so "gpt-5 (openai)" and a model with no provider
	// resolve the way they do everywhere else.
	want := agentloop.AgentModel{Provider: slot.Provider, Model: slot.Model, Reasoning: slot.Reasoning}
	if strings.TrimSpace(slot.Model) == "" {
		want = agentloop.AgentModel{}
		if found, err := m.agents.Get(ctx, agent.GetInput{ID: agentID}); err == nil && found != nil {
			want = agentloop.AgentModel{Provider: found.Provider, Model: found.Model, Reasoning: found.Reasoning}
		}
	}

	ref, err := agentloop.Resolve(want, configured)
	if err != nil {
		return nil, agentloop.ModelRef{}, err
	}
	provider, err := m.build(ref.Provider, keyFor(current, ref.Provider))
	if err != nil {
		return nil, agentloop.ModelRef{}, err
	}
	return provider, ref, nil
}

// turnHandler runs a queued conversation turn, in the workspace the job names.
type turnHandler struct {
	runtimeFor func(ctx context.Context, workspaceID string) (*session.Runner, error)
}

func (h turnHandler) Handle(ctx context.Context, j job.Job) (json.RawMessage, error) {
	var turn chat.Turn
	if err := json.Unmarshal(j.Payload, &turn); err != nil {
		return nil, err
	}
	runner, err := h.runtimeFor(ctx, j.Workspace)
	if err != nil {
		return nil, err
	}
	result, err := runner.Run(ctx, turn)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"chat": turn.ChatID, "steps": result.Steps, "usage": result.Usage,
	})
}

// runtimeFor is the runtime of the workspace a queued job names.
func (a *App) runtimeFor(ctx context.Context, workspaceID string) (*session.Runner, error) {
	target, err := a.jobScope(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return target.Runtime, nil
}

// jobScope is the services of the workspace a queued job names.
//
// A job that names no workspace is the primary's. One that names a workspace
// this installation cannot open fails rather than running in the primary —
// commands fall back to it (scopeFor), but a turn run against another
// workspace's conversations would read a chat that is not there, or worse,
// one of the same id that belongs to somebody else.
func (a *App) jobScope(ctx context.Context, workspaceID string) (*App, error) {
	if workspaceID == "" || a.scopes == nil {
		return a, nil
	}
	return a.scopes.forID(ctx, a.Workspaces, workspaceID)
}

// routineHandler runs a scheduled firing the worker's tick queued, in the
// workspace the job names, as the scheduled run it is.
type routineHandler struct {
	scopeFor func(ctx context.Context, workspaceID string) (*App, error)
}

func (h routineHandler) Handle(ctx context.Context, j job.Job) (json.RawMessage, error) {
	var firing routine.Firing
	if err := json.Unmarshal(j.Payload, &firing); err != nil {
		return nil, err
	}
	target, err := h.scopeFor(ctx, j.Workspace)
	if err != nil {
		return nil, err
	}
	// As the system, the way the tick that found it due acted: the run itself
	// is taken as the routine's own agent (routine.Service.fire).
	ctx = identity.With(ctx, identity.Identity{WorkspaceID: j.Workspace})
	run, err := target.Routines.Fire(ctx, firing.Input())
	if err != nil {
		// The run record says how it went, and the job fails with the same
		// reason, once: it is queued with a single try, because a routine's
		// turn is not taken again behind its owner's back.
		return nil, err
	}
	return json.Marshal(map[string]any{
		"agent": firing.Agent, "routine": firing.Routine,
		"run": run.ID, "status": run.Status, "chat": run.ChatID,
	})
}

// routineQueue hands one workspace's due firings to the job queue, where the
// pool's slots run them.
type routineQueue struct {
	queue     job.Queue
	ids       ids.Generator
	workspace string
}

func (q routineQueue) Dispatch(ctx context.Context, f routine.Firing) (bool, error) {
	// A firing of this routine still waiting or running is the one the new
	// firing would repeat. A worker that died holding one leaves it claimed
	// until its lease lapses and it is handed back and run, so this never
	// waits on work nothing will do.
	for _, status := range []job.Status{job.Pending, job.Claimed} {
		waiting, err := q.queue.List(ctx, job.Filter{Kind: kindRoutine, Status: status, Workspace: q.workspace})
		if err != nil {
			return false, err
		}
		for _, j := range waiting {
			var queued routine.Firing
			// An empty workspace filters nothing, so the primary's own is
			// compared here too.
			if j.Workspace == q.workspace && json.Unmarshal(j.Payload, &queued) == nil &&
				queued.Agent == f.Agent && queued.Routine == f.Routine {
				return false, nil
			}
		}
	}
	payload, err := json.Marshal(f)
	if err != nil {
		return false, err
	}
	if _, err := q.queue.Enqueue(ctx, job.Job{
		ID: q.ids.New(), Queue: job.QueueRoutine, Kind: kindRoutine,
		Workspace: q.workspace, Payload: payload, MaxTries: 1,
	}); err != nil {
		return false, err
	}
	return true, nil
}

// servedScope is one workspace the periodic work runs in: its services, and
// the id they act as.
type servedScope struct {
	app *App
	id  string
}

// servedScopes lists the workspaces this daemon's periodic work runs in: the
// directory it opened, and every registered workspace whose directory is
// there, each once.
//
// The list is taken afresh on every tick. A workspace registered or adopted
// while the daemon runs — the desktop registers the person's own after the
// daemon is up — is served from the next tick on, and one archived or deleted
// stops being served, without anything having to tell the worker.
//
// A registered workspace whose directory is gone is skipped rather than
// opened: building its services would scaffold state for a directory nobody
// has, every tick, to find no routines in it.
func (a *App) servedScopes(ctx context.Context) ([]servedScope, error) {
	out := []servedScope{{app: a, id: a.workspaceID}}
	if a.scopes == nil {
		return out, nil
	}
	listed, err := a.Workspaces.List(ctx, workspace.ListInput{})
	if err != nil {
		return out, err
	}
	var errs []error
	seen := map[*App]bool{a: true}
	for _, w := range listed.Workspaces {
		if info, statErr := os.Stat(w.Path); statErr != nil || !info.IsDir() {
			continue
		}
		target, openErr := a.scopes.forID(ctx, a.Workspaces, w.ID)
		if openErr != nil {
			errs = append(errs, fmt.Errorf("workspace %q: %w", w.ID, openErr))
			continue
		}
		if target == a && out[0].id == "" {
			// The primary is a registered workspace nothing pinned: the
			// registry is what names it.
			out[0].id = w.ID
		}
		if seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, servedScope{app: target, id: w.ID})
	}
	return out, errors.Join(errs...)
}

// scopeTick is periodic work done once in each served workspace.
type scopeTick func(ctx context.Context, scope *App, workspaceID string, now time.Time) error

// everyScope runs a tick in every workspace this daemon serves. One workspace
// failing is reported with its id and does not keep the tick from the others.
func (a *App) everyScope(tick scopeTick) func(context.Context, time.Time) error {
	return func(ctx context.Context, now time.Time) error {
		served, err := a.servedScopes(ctx)
		errs := []error{err}
		for _, scope := range served {
			if tickErr := tick(ctx, scope.app, scope.id, now); tickErr != nil {
				errs = append(errs, fmt.Errorf("workspace %q: %w", scope.id, tickErr))
			}
		}
		return errors.Join(errs...)
	}
}

// routineTick evaluates one workspace's cron triggers once per window, and
// queues the firings that are due.
//
// Queued, not run: a run is a whole turn, and taken here it held up every
// other workspace's due routines and both retention passes behind it, while
// the ticks that passed meanwhile were dropped. The pool's slots run the
// queued firings side by side, stop with the daemon inside its shutdown
// timeout, and a firing a dead worker held is handed back and run.
func routineTick(queue job.Queue, idgen ids.Generator) scopeTick {
	return func(ctx context.Context, scope *App, workspaceID string, now time.Time) error {
		// The tick acts as the system, not as any agent: a routine that fires
		// on a schedule was not asked for by whoever last used the terminal.
		ctx = identity.With(ctx, identity.Identity{WorkspaceID: workspaceID})
		out, err := scope.Routines.DispatchScheduled(ctx, now,
			routineQueue{queue: queue, ids: idgen, workspace: workspaceID})
		if err != nil {
			return err
		}
		if len(out.Fired) > 0 || len(out.Failed) > 0 || len(out.Running) > 0 || len(out.Broken) > 0 {
			slog.Default().Info("the scheduler evaluated the routines",
				"workspace", workspaceID, "queued", len(out.Fired), "failed", len(out.Failed),
				"stillRunning", len(out.Running), "broken", len(out.Broken))
		}
		return nil
	}
}

// activityRetention purges one workspace's activity log on the tick.
func activityRetention(ctx context.Context, scope *App, workspaceID string, _ time.Time) error {
	ctx = identity.With(ctx, identity.Identity{WorkspaceID: workspaceID})
	_, err := scope.Activities.Purge(ctx, activity.PurgeInput{})
	return err
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
	kindTurn    = build.Name + ".turn"
	kindRoutine = build.Name + ".routine"
)
