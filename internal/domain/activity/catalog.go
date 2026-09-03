package activity

// The catalogue of what this workspace publishes.
//
// A routine's activity trigger names a (namespace, event) pair and fires when
// one arrives — `Key.Matches` is the whole mechanism, and it has worked since
// the routine domain was written. What did not exist was any way to *find out*
// which pairs are real: the coordinates were literals scattered across the
// publishers, and the routine editor's trigger picker read them from a call
// that answered nothing. So the one trigger type that reacts to the workspace
// was unreachable from the interface, and the only way to declare one was to
// guess a namespace and an event and hope.
//
// The catalogue is declared here rather than assembled from the publishers
// because it is a promise, not an observation: "task.status_changed exists and
// carries these fields" has to be answerable on a workspace where no task has
// ever moved. `TestEveryPublishedEventIsInTheCatalogue` keeps the promise
// honest from the other side — it exercises the real mutations and fails if
// anything reaches the log that is not declared here.

// EventKind is one coordinate a routine can react to.
type EventKind struct {
	Namespace string `json:"namespace" jsonschema:"What kind of thing this happens to. Example: \"task\"."`
	Event     string `json:"event" jsonschema:"What happens to it. Example: \"status_changed\"."`

	Title       string `json:"title" jsonschema:"One line naming the event, for a person choosing one."`
	Description string `json:"description" jsonschema:"What causes it, in a sentence."`

	// Data names the payload keys a filter can match on. It is a list rather
	// than a JSON Schema because that is the whole of what the picker needs —
	// which fields exist — and a schema would promise types the publishers do
	// not enforce.
	Data []string `json:"data,omitempty" jsonschema:"Keys the event's payload carries, which a trigger filter can match on."`
}

// Kinds is every event a routine can trigger on.
//
// Ordered by namespace and then by the shape of a lifecycle — created, then
// updated, then whatever is specific to it, then deleted — because that is the
// order somebody scanning a picker expects to read them in.
var Kinds = []EventKind{
	{
		Namespace: "task", Event: "created",
		Title:       "Task created",
		Description: "A task was added to the workspace.",
		Data:        []string{"task", "name", "type", "status"},
	},
	{
		Namespace: "task", Event: "updated",
		Title:       "Task changed",
		Description: "A task's name, description or fields were edited.",
		Data:        []string{"task", "name", "type", "status"},
	},
	{
		Namespace: "task", Event: "status_changed",
		Title:       "Task moved",
		Description: "A task moved from one status to another. The status it left is in \"from\".",
		Data:        []string{"task", "name", "type", "status", "from", "to"},
	},
	{
		Namespace: "task", Event: "branched",
		Title:       "Task branched",
		Description: "A worktree and branch were created for a task.",
		Data:        []string{"task", "name", "type", "status", "branch", "path"},
	},
	{
		Namespace: "task", Event: "deleted",
		Title:       "Task deleted",
		Description: "A task was removed from the workspace.",
		Data:        []string{"task", "name", "type", "status"},
	},

	{
		Namespace: "project", Event: "created",
		Title:       "Project created",
		Description: "A project was added to the workspace.",
		Data:        []string{"project", "name", "status"},
	},
	{
		Namespace: "project", Event: "updated",
		Title:       "Project changed",
		Description: "A project's details or status were edited.",
		Data:        []string{"project", "name", "status"},
	},
	{
		Namespace: "project", Event: "deleted",
		Title:       "Project deleted",
		Description: "A project was removed from the workspace.",
		Data:        []string{"project", "name", "status"},
	},

	{
		Namespace: "goal", Event: "created",
		Title:       "Goal created",
		Description: "A goal was added to the workspace.",
		Data:        []string{"goal", "title", "status"},
	},
	{
		Namespace: "goal", Event: "updated",
		Title:       "Goal changed",
		Description: "A goal's details, status or progress were edited.",
		Data:        []string{"goal", "title", "status"},
	},
	{
		Namespace: "goal", Event: "deleted",
		Title:       "Goal deleted",
		Description: "A goal was removed from the workspace.",
		Data:        []string{"goal", "title", "status"},
	},

	{
		Namespace: "agent", Event: "created",
		Title:       "Agent created",
		Description: "An agent was defined in this workspace.",
		Data:        []string{"agent", "name"},
	},
	{
		Namespace: "agent", Event: "updated",
		Title:       "Agent changed",
		Description: "An agent's instructions, model or tools were edited.",
		Data:        []string{"agent", "name"},
	},
	{
		Namespace: "agent", Event: "deleted",
		Title:       "Agent deleted",
		Description: "An agent was removed from this workspace.",
		Data:        []string{"agent", "name"},
	},

	{
		Namespace: "routine", Event: "fired",
		Title:       "Routine fired",
		Description: "A routine ran, by schedule, by webhook or by hand.",
		Data:        []string{"routine", "agent", "run", "status", "trigger"},
	},

	{
		Namespace: "toolset", Event: "call",
		Title:       "Toolset called",
		Description: "A tool on a connected toolset was called, and either answered or did not.",
		Data:        []string{"toolset", "tool", "outcome", "durationMs"},
	},
}

// Namespaces is every namespace in the catalogue, in the order Kinds declares
// them. The picker groups by it.
func Namespaces() []string {
	seen := make(map[string]bool, len(Kinds))
	out := make([]string, 0, 8)
	for _, kind := range Kinds {
		if seen[kind.Namespace] {
			continue
		}
		seen[kind.Namespace] = true
		out = append(out, kind.Namespace)
	}
	return out
}

// Declared reports whether a published event is one the catalogue promises.
func Declared(namespace, event string) bool {
	for _, kind := range Kinds {
		if kind.Namespace == namespace && kind.Event == event {
			return true
		}
	}
	return false
}
