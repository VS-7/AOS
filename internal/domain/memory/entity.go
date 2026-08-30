// Package memory is the cognitive layer: what an agent knows, and why.
//
// Memories are the agent's, not the workspace's. That distinction is the whole
// design — a memory is first-person and personal, an instruction is shared and
// binding. Two properties follow from it and shape everything here: nothing is
// ever deleted, only deprecated with a reason; and confidence is a number the
// agent is asked to be honest about, because an inflated one is how future
// selves get misled.
package memory

import (
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/search"
)

// Category is the function of a piece of knowledge, not its writing style.
//
// The thirteen cognitive categories replaced the nine technical ones of an
// earlier version of the original, and the instruction that came with the
// change is worth keeping: pick what the knowledge does, not how it reads.
type Category string

const (
	CatDecision     Category = "decision"     // design choices, trade-offs, rationale
	CatIntent       Category = "intent"       // goals, desires, expected outcomes
	CatCommitment   Category = "commitment"   // agreements, promises, action items
	CatRelationship Category = "relationship" // people, roles, teams, dynamics
	CatEvent        Category = "event"        // occurrences and milestones
	CatObservation  Category = "observation"  // patterns and signals
	CatError        Category = "error"        // bugs, incidents, missteps
	CatLearning     Category = "learning"     // root causes, fixes, insights
	CatFact         Category = "fact"         // verified objective knowledge
	CatReference    Category = "reference"    // external links and docs
	CatInstruction  Category = "instruction"  // repeatable procedures
	CatPreference   Category = "preference"   // tastes and stylistic choices
	CatContext      Category = "context"      // session and environment awareness
)

// Categories lists them in the original's order, which groups them by kind
// rather than alphabetically.
var Categories = []Category{
	CatDecision, CatIntent, CatCommitment, CatRelationship, CatEvent,
	CatObservation, CatError, CatLearning, CatFact, CatReference,
	CatInstruction, CatPreference, CatContext,
}

// EnumValues publishes the accepted categories in the schema every surface
// reads, so the way to learn them is not to store one with the wrong category
// first (defect #8).
func (c Category) EnumValues() []string {
	out := make([]string, 0, len(Categories))
	for _, v := range Categories {
		out = append(out, string(v))
	}
	return out
}

// Valid reports whether c is one of the thirteen.
func (c Category) Valid() bool {
	for _, known := range Categories {
		if c == known {
			return true
		}
	}
	return false
}

// Status is where a memory sits in its lifecycle. There is no "deleted".
type Status string

const (
	StatusActive     Status = "active"
	StatusDeprecated Status = "deprecated"
	StatusArchived   Status = "archived"
	StatusTTLExpired Status = "ttl_expired"
)

// Statuses lists every lifecycle state.
var Statuses = []Status{StatusActive, StatusDeprecated, StatusArchived, StatusTTLExpired}

// EnumValues publishes the lifecycle a memory can be filtered by.
func (s Status) EnumValues() []string {
	out := make([]string, 0, len(Statuses))
	for _, v := range Statuses {
		out = append(out, string(v))
	}
	return out
}

// Valid reports whether s is one of the four.
func (s Status) Valid() bool {
	for _, known := range Statuses {
		if s == known {
			return true
		}
	}
	return false
}

// Super points at a memory this one replaces, and says why.
//
// The reason is the load-bearing half. A lineage of replacements without
// reasons records that the agent changed its mind and loses what it learned.
type Super struct {
	ID     string `yaml:"id" json:"id" jsonschema:"Identifier of the memory being superseded."`
	Reason string `yaml:"reason" json:"reason" jsonschema:"Why the older trace no longer applies. Be specific; a vague reason confuses your future self." validate:"required,min=5"`
}

// Memory is one durable trace.
type Memory struct {
	// ID and Agent come from the path: a memory lives at
	// .aos/agents/{agent}/memories/{id}.memory.md.
	ID    string `yaml:"-" json:"id" collection:"path" jsonschema:"Identifier of this memory."`
	Agent string `yaml:"-" json:"agent" collection:"path" jsonschema:"Slug of the agent that owns this memory."`

	Title       string   `yaml:"title" json:"title" jsonschema:"Sharp, specific headline. Example: \"UUID v4 chosen for memory ids after a collision scare\"."`
	Description string   `yaml:"description" json:"description" jsonschema:"Dense, keyword-rich summary written to be found by search later."`
	Category    Category `yaml:"category" json:"category" jsonschema:"One of: decision, intent, commitment, relationship, event, observation, error, learning, fact, reference, instruction, preference, context. Pick the function of the knowledge, not the writing style."`
	Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty" jsonschema:"Labels grouping related knowledge. Example: [\"database\", \"scalability\"]."`

	// Confidence is asked for honestly: 0.9–1.0 verified, 0.6–0.8 strong
	// inference, below 0.6 a guess.
	Confidence float64  `yaml:"confidence" json:"confidence" jsonschema:"Strength of this trace, from 0 to 1. Be honest: 0.9-1.0 verified, 0.6-0.8 strong inference, below 0.6 a guess."`
	Links      []string `yaml:"links,omitempty" json:"links,omitempty" jsonschema:"Identifiers of related memories. A memory with no links is an island."`
	Supersedes []Super  `yaml:"supersedes,omitempty" json:"supersedes,omitempty" jsonschema:"Memories this trace makes obsolete, each with the reason it no longer applies."`

	Status           Status     `yaml:"status" json:"status" jsonschema:"Lifecycle status. Defaults to active."`
	DeprecatedBy     string     `yaml:"deprecatedBy,omitempty" json:"deprecatedBy,omitempty" jsonschema:"Identifier of the memory that replaced this one."`
	DeprecatedAt     *time.Time `yaml:"deprecatedAt,omitempty" json:"deprecatedAt,omitempty" jsonschema:"When this memory was deprecated."`
	DeprecatedReason string     `yaml:"deprecatedReason,omitempty" json:"deprecatedReason,omitempty" jsonschema:"Why this memory is no longer trustworthy."`

	// Scopes anchor the memory to the files that gave rise to it. They are the
	// difference between recalling everything and recalling what applies.
	Scopes    []string   `yaml:"scopes,omitempty" json:"scopes,omitempty" jsonschema:"Glob patterns tying this memory to files. Example: [\"internal/domain/memory/**/*.go\"]."`
	ExpiresAt *time.Time `yaml:"expiresAt,omitempty" json:"expiresAt,omitempty" jsonschema:"When this memory stops applying, for knowledge that is true only for a while."`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt" jsonschema:"When the memory was formed."`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt" jsonschema:"When it was last changed."`

	Content string `yaml:"-" json:"content,omitempty" collection:"content" jsonschema:"The Markdown body: the memory itself, shaped by its category."`
}

// Document projects a memory into the shape the search port indexes.
func (m Memory) Document() search.Document {
	return search.Document{
		ID:         m.ID,
		Collection: "memories",
		Text: map[string]string{
			"title":       m.Title,
			"description": m.Description,
			"content":     m.Content,
			"tags":        strings.Join(m.Tags, " "),
		},
		Filters: map[string]string{
			"agent":    m.Agent,
			"category": string(m.Category),
			"status":   string(m.Status),
		},
	}
}

// IsActive reports whether this memory still counts as current knowledge.
func (m Memory) IsActive() bool { return m.Status == StatusActive }
