package memory

import (
	"context"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
//
// The text is close to the original's, and deliberately so: it is the piece of
// this system that most changes how a model behaves. The warning about
// globality and the instruction to recall before storing are not decoration —
// they are the difference between a graph that compounds and one that fills
// with duplicates.
var GroupDoc = command.GroupDoc{
	Name:    "memories",
	Tool:    "Memory",
	Summary: "Record and recall what you have learned, across sessions.",
	Doc: `Your persistent cognitive layer. Memories survive sessions and parallel
selves. They are not logs or journal entries — they are first-person traces that
compound your understanding over time.

A memory carries a category, a confidence score, scopes that anchor it to files,
and links to related memories. Together those form an associative graph that
gets more useful the more honestly it is maintained.

## Commands
- **recall** — scan with filters: status, category, agent, scopes, full text
- **graph** — map the whole graph, with its hubs, its silos and its health
- **reflect** — read one memory in full
- **store** — record new durable knowledge
- **forget** — deprecate what no longer holds, preserving the lineage

## When to use
- **At the start of a session:** recall or graph, to reorient
- **After meaningful work:** store what you learned
- **Before deciding:** check whether you already decided this once
- **When knowledge goes stale:** store the replacement, then forget the old one

## When NOT to use
- Not for knowledge that binds everyone — that is an instruction, not a memory
- Not for ephemeral session chatter
- Never for secrets or credentials`,
	Hint: `Memories are GLOBAL across all your parallel selves. What you store here, every
one of them sees. There is no private flag.

Before storing: recall first. If a trace already exists, link to it or supersede
it. Duplicates dilute the graph and confuse retrieval later.

Confidence, honestly: 0.9-1.0 verified, 0.6-0.8 strong inference, below 0.6 a
guess. Inflated confidence is the main way future-you gets misled.

Scopes are the anchor. Without them a recall returns everything you know, and
you pay for all of it.`,
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[RecallInput, RecallOutput]{
		Group:   "memories",
		Name:    "recall",
		Aliases: []string{"list"},
		Summary: "Scan your memories.",
		Doc: `Search and filter what you know.

A text query matches when every one of its words appears somewhere in the title,
the description, the tags or the body. That is stricter than a relevance search
on purpose: a result that contains one word out of three is worse than no result,
because you will act on what comes back.

Deprecated memories are excluded unless you ask for them by status. They are
never deleted — read one with reflect to see why it was retired.`,
		Examples: []command.Example{
			{Description: "everything you know that is still current", Input: RecallInput{}},
			{Description: "what you decided about identifiers", Input: RecallInput{Query: "uuid identifiers", Category: CatDecision}},
			{Description: "what applies to the files you are about to change", Input: RecallInput{
				Scopes: []string{"internal/domain/memory/**"}, ScopesMode: ScopesStrict,
			}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Recall memories", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Recall,
	})

	command.MustRegister(reg, command.Command[GraphInput, Graph]{
		Group:   "memories",
		Name:    "graph",
		Summary: "Map your cognitive graph.",
		Doc: `Map what you know and how it connects.

Beyond the nodes and edges this reports the shape of the graph: the hubs that
other knowledge hangs off, the silos with no links at all, the average
confidence, and how much of it has been superseded. Those four numbers say
whether your knowledge is organised or merely large.

Ask for the isolated nodes when you want the list of memories most likely to be
lost — an unlinked trace is one nobody will find again.`,
		Examples: []command.Example{
			{Description: "the whole map", Input: GraphInput{}},
			{Description: "the memories at risk of being lost", Input: GraphInput{Isolated: true}},
			{Description: "only what you are confident about", Input: GraphInput{MinConfidence: 0.8}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Map the memory graph", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Graph,
	})

	command.MustRegister(reg, command.Command[ReflectInput, *Memory]{
		Group:   "memories",
		Name:    "reflect",
		Aliases: []string{"get"},
		Summary: "Read one memory in full.",
		Doc: `Read a single memory: its body, its links, its lineage.

Treat what you read as a reference, not as fact. Check it against the code and
the situation in front of you before acting on it. If it turns out to be stale,
store the correction and then forget this one.`,
		Examples: []command.Example{
			{Description: "read a memory a recall turned up", Input: ReflectInput{Memory: "550e8400-e29b-41d4-a716-446655440000"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read one memory", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Reflect,
	})

	command.MustRegister(reg, command.Command[StoreInput, StoreOutput]{
		Group:   "memories",
		Name:    "store",
		Aliases: []string{"create"},
		Summary: "Record durable knowledge.",
		Doc: `Store a memory when something leaves a durable trace on how you should
think, decide or act.

Without this, every session starts from zero: the same mistakes, the same lost
context, the same rediscovery. Storing is the only mechanism you have to get
better over time rather than merely faster.

Recall before you store. If a trace already exists, link to it or supersede it.
Superseding writes the replacement first and then deprecates the old one with
your reason, so the lineage can be walked later — that is how future-you finds
out not just what you believe but why you changed your mind.`,
		Examples: []command.Example{
			{
				Description: "a preference learned from a review",
				Input: StoreInput{
					Title:       "Commit messages here explain why, not what",
					Description: "A review pointed out that the message described the change and not the reason for it. The diff already shows what changed.",
					Category:    CatPreference,
					Tags:        []string{"git", "code-review"},
					Scopes:      []string{"**/*"},
				},
			},
			{
				Description: "a decision that replaces an earlier one",
				Input: StoreInput{
					Title:       "Identifiers moved from random to time-sortable",
					Description: "Cursor pagination and event ordering both need sortable identifiers, which the previous choice could not give.",
					Category:    CatDecision,
					Supersedes: []Super{{
						ID:     "660e8400-e29b-41d4-a716-446655440000",
						Reason: "The earlier choice predates the pagination requirement and cannot satisfy it.",
					}},
				},
			},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Store a memory"},
		Handler:     svc.Store,
	})

	command.MustRegister(reg, command.Command[ForgetInput, *Memory]{
		Group:   "memories",
		Name:    "forget",
		Summary: "Deprecate knowledge that no longer holds.",
		Doc: `Mark a memory as no longer trustworthy, with your reason attached.

Nothing is deleted. The memory stays readable so that the record of what you
believed, and why you stopped, survives. That is the difference between changing
your mind and losing the thread.

Store the replacement first, pointing at this memory with supersedes, and then
forget it. And if you are unsure whether something is still true, lower its
confidence instead of forgetting it — uncertainty is knowledge too.`,
		Examples: []command.Example{
			{
				Description: "retire a decision whose premise changed",
				Input: ForgetInput{
					Memory: "660e8400-e29b-41d4-a716-446655440000",
					Reason: "The requirement that made this the right call was dropped in the redesign, and the replacement memory records the new one.",
				},
			},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Deprecate a memory", IdempotentHint: true},
		Handler:     svc.Forget,
	})
}

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, RecallInput) (RecallOutput, error) = (*Service)(nil).Recall
	_ func(context.Context, GraphInput) (Graph, error)         = (*Service)(nil).Graph
	_ func(context.Context, StoreInput) (StoreOutput, error)   = (*Service)(nil).Store
)
