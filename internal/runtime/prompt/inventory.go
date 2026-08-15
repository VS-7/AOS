package prompt

// Inventory is what the workspace holds, as identifiers.
//
// Names and never bodies. Two properties follow, and both have tests. The
// prompt stays the same size whether the workspace holds ten records or ten
// thousand, and retrieval becomes a deliberate act the agent performs with a
// tool rather than something that happened to it before it started thinking.
type Inventory struct {
	Workspace WorkspaceRef
	Skills    []string
	Templates []string
	Views     []string
	Goals     []string
	Routines  []string
	Projects  []string
	Artifacts []string
	Agents    []string

	// Instructions and Collections are separated from the rest because the
	// first is workspace policy — it carries its own trust attributes inside
	// the block — and the second is the operational source of truth.
	Instructions []string
	Collections  []string
}

// WorkspaceRef identifies the active workspace.
type WorkspaceRef struct {
	ID   string
	Name string
	Path string
}

// categoryGuide is the pedagogical text per category: what the resource is, and
// when to reach for it. It is the part of the inventory that costs tokens and
// earns them back — a list of names with no explanation produces an agent that
// knows a skill exists and never uses it.
var categoryGuide = []struct {
	tag         string
	description string
	pick        func(Inventory) []string
	policy      bool
}{
	{
		tag: "skills",
		description: "Modular capability packages extending your behavior with domain-specific knowledge and tools.\n" +
			"Treat installed skills as first-class citizens — their instructions, templates, and toolsets are immediately available to you. Before building a capability from scratch, check whether a skill already covers it; retrieve the skill's records when you need details.",
		pick: func(i Inventory) []string { return i.Skills },
	},
	{
		tag: "instructions",
		description: "Workspace-wide behavioral rules that shape ALL agents in the workspace.\n" +
			"Follow all active instructions during every task and conversation — they apply to you. When the user establishes a policy that should apply workspace-wide, create or update an instruction, not a memory. Retrieve instruction details when a task enters a domain that requires additional rules.",
		pick:   func(i Inventory) []string { return i.Instructions },
		policy: true,
	},
	{
		tag: "templates",
		description: "Reusable Liquid-based scaffolds for generating boilerplate code, docs, and structured files.\n" +
			"Use a template when work has a recognizable, repeatable shape — task briefs, plans, reports, emails. Inspect a template's schema before rendering to know the variables it expects.",
		pick: func(i Inventory) []string { return i.Templates },
	},
	{
		tag: "views",
		description: "Adaptive interface definitions presenting workspace data as tables, dashboards, pipelines, or kanban boards.\n" +
			"Create views when data needs a visible, actionable surface — they let users operate on records directly without chat. Open the relevant view to see and act on its data.",
		pick: func(i Inventory) []string { return i.Views },
	},
	{
		tag: "goals",
		description: "Strategic outcomes the workspace is pursuing — the destination your tasks serve.\n" +
			"Before planning or executing significant work, check active goals to align your efforts and avoid strategically useless results. Create goals when a request implies an objective that outlasts individual tasks.",
		pick: func(i Inventory) []string { return i.Goals },
	},
	{
		tag: "collections",
		description: "Structured domain data serving as the workspace's source of truth for operational records.\n" +
			"Query the relevant collection before decisions that depend on operational state. Create collections when the workspace needs structured domain data — contacts, deals, articles, inventory.",
		pick: func(i Inventory) []string { return i.Collections },
	},
	{
		tag: "routines",
		description: "Durable automation entry points triggering agent work on schedules, webhooks, or manual fire.\n" +
			"Create routines for recurring or event-driven work that should persist beyond a conversation. Each activation becomes a run with its own context, input, status, and history.",
		pick: func(i Inventory) []string { return i.Routines },
	},
	{
		tag: "projects",
		description: "Top-level workspace boundaries organizing related goals, tasks, and files into coherent bodies of work.\n" +
			"Use projects to structure long-running efforts that span multiple tasks; associate tasks and goals with the right project to keep work traceable.",
		pick: func(i Inventory) []string { return i.Projects },
	},
	{
		tag: "artifacts",
		description: "Shareable, workspace-owned applications or presentation surfaces served directly by the system — dashboards, reports, static web apps.\n" +
			"Create artifacts for external-facing deliverables with their own audience and visibility.",
		pick: func(i Inventory) []string { return i.Artifacts },
	},
	{
		tag: "agents",
		description: "Your teammates in the workspace — each with its own identity, role, memory, and autonomy.\n" +
			"Delegate to specialists when work needs focused expertise or parallel execution; coordinate through mentions in shared chats or task assignment.",
		pick: func(i Inventory) []string { return i.Agents },
	},
}

// node renders the workspace block.
func (i Inventory) node() Object {
	out := Object{
		Field{Key: "description", Value: Text("The active workspace and its available resources.\n" +
			"Each entry tells you what the resource is, how to use it, and when to retrieve full records via tools. Names are the identifiers you can reference, retrieve, or act on.")},
		Field{Key: "config", Value: Object{
			Field{Key: "id", Value: Text(i.Workspace.ID)},
			Field{Key: "name", Value: Text(i.Workspace.Name)},
			Field{Key: "path", Value: Text(i.Workspace.Path)},
		}},
	}

	for _, c := range categoryGuide {
		block := Object{}
		if c.policy {
			block = append(block,
				Attr("kind", string(KindPolicy)),
				Attr("source", string(SourceWorkspace)),
				Attr("trust", string(TrustTrusted)),
			)
		}
		block = append(block,
			Field{Key: "description", Value: Text(c.description)},
			Field{Key: "names", Value: Strings(c.pick(i))},
		)
		out = append(out, Field{Key: c.tag, Value: block})
	}
	return out
}

// MemoryCounts is the memory block: how many of each category the agent holds,
// and what each category is for.
//
// Counts and never records. The agent reads "I have twelve decision memories"
// and decides whether retrieving them is worth the tokens — which is the same
// judgment a person makes about opening a notebook.
type MemoryCounts struct {
	Total      int
	ByCategory map[string]int
}

func (m MemoryCounts) node() Object {
	categories := Object{}
	for _, g := range MemoryCategoryGuide {
		categories = append(categories, Field{Key: g.Category, Value: Object{
			Attr("count", itoa(m.ByCategory[g.Category])),
			Body(g.Summary),
		}})
	}
	return Object{
		Field{Key: "total_count", Value: Text(itoa(m.Total))},
		Field{Key: "general_rules", Value: GeneralMemoryRules},
		Field{Key: "categories", Value: categories},
	}
}
