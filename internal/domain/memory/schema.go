package memory

import "github.com/OWNER/aos/internal/core/command"

// RecallInput scans the memory graph.
type RecallInput struct {
	Query      string     `json:"query,omitempty" jsonschema:"Full-text search across title, description, tags and body. Every word must appear somewhere."`
	Category   Category   `json:"category,omitempty" jsonschema:"Narrow to one category."`
	Status     Status     `json:"status,omitempty" jsonschema:"Lifecycle status to include. Defaults to active."`
	Agent      string     `json:"agent,omitempty" jsonschema:"Whose memories to read. Defaults to your own."`
	Scopes     []string   `json:"scopes,omitempty" jsonschema:"Glob patterns anchoring the search to files. Example: [\"internal/domain/**\"]."`
	ScopesMode ScopesMode `json:"scopesMode,omitempty" jsonschema:"strict excludes memories with no scopes; lax includes them. Defaults to lax."`
	OrderBy    string     `json:"orderBy,omitempty" jsonschema:"One of: relevance, confidence, createdAt, updatedAt. Defaults to relevance when searching, and to createdAt otherwise."`
	Desc       bool       `json:"desc,omitempty" jsonschema:"Reverse the ordering."`
	Limit      int        `json:"limit,omitempty" jsonschema:"Maximum number of memories to return. Defaults to 10."`
	Offset     int        `json:"offset,omitempty" jsonschema:"How many matches to skip."`

	command.Reasoning
}

// RecallOutput is what a scan returns.
type RecallOutput struct {
	Memories []Memory `json:"memories" jsonschema:"The memories found."`
	Total    int      `json:"total" jsonschema:"How many matched before the limit was applied."`
	Indexed  bool     `json:"indexed" jsonschema:"True when a search index served the query; false when it was answered by scanning."`
}

// ReflectInput reads one memory in full.
type ReflectInput struct {
	Memory string `json:"memory" cli:"arg" jsonschema:"Identifier of the memory to read." validate:"required,notblank"`
	Agent  string `json:"agent,omitempty" jsonschema:"Whose memory it is. Defaults to your own."`

	command.Reasoning
}

// StoreInput records new durable knowledge.
type StoreInput struct {
	// Agent is whose memory this is. It is the one field the other three
	// memory commands had and this one did not, and its absence made the
	// application's own "Record a memory" impossible: Store resolved the
	// owner from the ambient identity only, the desktop window is a *user*
	// identity, and so every write from the agent's Memories tab came back
	// AOS_MEMORY_AGENT_REQUIRED with a call to action telling the person to
	// pass `--agent` on a command line they were not using.
	Agent       string   `json:"agent,omitempty" jsonschema:"Whose memory this is. Defaults to your own."`
	Title       string   `json:"title" jsonschema:"Sharp, specific headline. It is what you will scan later." validate:"required,notblank"`
	Description string   `json:"description" jsonschema:"Dense, keyword-rich summary written to be found by search." validate:"required,notblank"`
	Category    Category `json:"category" jsonschema:"One of: decision, intent, commitment, relationship, event, observation, error, learning, fact, reference, instruction, preference, context." validate:"required"`
	Content     string   `json:"content,omitempty" jsonschema:"The Markdown body of the memory."`
	Tags        []string `json:"tags,omitempty" jsonschema:"Labels grouping related knowledge."`
	Scopes      []string `json:"scopes,omitempty" jsonschema:"Glob patterns anchoring this memory to the files that gave rise to it."`
	Links       []string `json:"links,omitempty" jsonschema:"Identifiers of related memories. Aim for one or two whenever related traces exist."`
	Supersedes  []Super  `json:"supersedes,omitempty" jsonschema:"Memories this one replaces, each with the reason it no longer applies."`
	Confidence  *float64 `json:"confidence,omitempty" jsonschema:"From 0 to 1. Defaults to 1. Be honest — an inflated number is how future-you gets misled."`
	ExpiresAt   string   `json:"expiresAt,omitempty" jsonschema:"RFC 3339 timestamp after which this memory stops applying."`

	command.Reasoning
}

// StoreOutput reports the memory that was written and what the supersede
// protocol did alongside it.
type StoreOutput struct {
	Memory Memory `json:"memory" jsonschema:"The memory that was stored."`

	// Deprecated lists the memories this one replaced.
	Deprecated []string `json:"deprecated,omitempty" jsonschema:"Memories deprecated because this one supersedes them."`

	// Incomplete names the supersede targets that could not be deprecated.
	//
	// There is no transaction across files, so a failure halfway leaves the new
	// memory stored and an old one still active. Reporting that is the whole
	// point: the alternative is pretending it did not happen and letting two
	// contradictory memories both read as current.
	Incomplete []string `json:"incomplete,omitempty" jsonschema:"Memories that should have been deprecated by this one and were not. Deprecate them by hand."`
}

// ForgetInput deprecates a memory. There is no delete.
type ForgetInput struct {
	Memory string `json:"memory" cli:"arg" jsonschema:"Identifier of the memory to deprecate." validate:"required,notblank"`
	Reason string `json:"reason" jsonschema:"Why this knowledge no longer holds. Be specific: a vague reason confuses your future self." validate:"required,min=5"`
	Agent  string `json:"agent,omitempty" jsonschema:"Whose memory it is. Defaults to your own."`

	command.Reasoning
}

// GraphInput maps the cognitive graph.
type GraphInput struct {
	Agent         string     `json:"agent,omitempty" jsonschema:"Whose graph to map. Defaults to your own."`
	Category      Category   `json:"category,omitempty" jsonschema:"Narrow the graph to one category."`
	Scopes        []string   `json:"scopes,omitempty" jsonschema:"Glob patterns narrowing the graph to memories anchored to these files."`
	ScopesMode    ScopesMode `json:"scopesMode,omitempty" jsonschema:"strict excludes memories with no scopes; lax includes them. Defaults to lax."`
	MinConfidence float64    `json:"minConfidence,omitempty" jsonschema:"Exclude memories below this confidence."`
	Isolated      bool       `json:"isolated,omitempty" jsonschema:"Return only the memories with no links at all — the ones most likely to be lost."`
	Limit         int        `json:"limit,omitempty" jsonschema:"Maximum number of nodes. Defaults to 1000."`

	command.Reasoning
}

// Graph is the map of what an agent knows and how it connects.
type Graph struct {
	Nodes  []Node  `json:"nodes" jsonschema:"One node per memory."`
	Edges  []Edge  `json:"edges" jsonschema:"Links between memories."`
	Health Health  `json:"health" jsonschema:"The shape of the graph, not its contents."`
	Counts []Count `json:"counts" jsonschema:"How many memories per category."`
}

// Node is one memory as it appears on the map: enough to decide whether to open
// it, and no more.
type Node struct {
	ID          string   `json:"id" jsonschema:"Memory identifier."`
	Title       string   `json:"title" jsonschema:"Its headline."`
	Description string   `json:"description" jsonschema:"Its summary, truncated for the map."`
	Category    Category `json:"category" jsonschema:"Its category."`
	Status      Status   `json:"status" jsonschema:"Its lifecycle status."`
	Confidence  float64  `json:"confidence" jsonschema:"Its confidence."`
	Degree      int      `json:"degree" jsonschema:"How many edges touch it."`
}

// Edge is a link between two memories.
type Edge struct {
	From string `json:"from" jsonschema:"Source memory."`
	To   string `json:"to" jsonschema:"Target memory."`
	Type string `json:"type" jsonschema:"reference for a soft association, supersedes for a replacement."`
}

// Health describes the shape of the graph rather than its contents, so that an
// agent can see how its own knowledge is organised and not only what is in it.
type Health struct {
	Hubs          []string `json:"hubs,omitempty" jsonschema:"The most connected memories: what the rest of the knowledge hangs off."`
	Silos         []string `json:"silos,omitempty" jsonschema:"Memories with no links. An isolated trace is the one most likely to be lost."`
	AvgConfidence float64  `json:"avgConfidence" jsonschema:"Mean confidence across the graph."`
	DeprecatedPct float64  `json:"deprecatedPct" jsonschema:"Share of the graph that is deprecated, from 0 to 1."`
}

// Count is one category and how many memories carry it.
type Count struct {
	Category Category `json:"category" jsonschema:"The category."`
	Count    int      `json:"count" jsonschema:"How many memories have it."`
}
