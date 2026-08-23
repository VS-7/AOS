# Memories

Record and recall what you have learned, across sessions.

Your persistent cognitive layer. Memories survive sessions and parallel
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
- Never for secrets or credentials

## Commands

### `memories_forget`

Deprecate knowledge that no longer holds.

Mark a memory as no longer trustworthy, with your reason attached.

Nothing is deleted. The memory stays readable so that the record of what you
believed, and why you stopped, survives. That is the difference between changing
your mind and losing the thread.

Store the replacement first, pointing at this memory with supersedes, and then
forget it. And if you are unsure whether something is still true, lower its
confidence instead of forgetting it — uncertainty is knowledge too.

- retire a decision whose premise changed

### `memories_graph`

Map your cognitive graph.

Map what you know and how it connects.

Beyond the nodes and edges this reports the shape of the graph: the hubs that
other knowledge hangs off, the silos with no links at all, the average
confidence, and how much of it has been superseded. Those four numbers say
whether your knowledge is organised or merely large.

Ask for the isolated nodes when you want the list of memories most likely to be
lost — an unlinked trace is one nobody will find again.

- the whole map
- the memories at risk of being lost
- only what you are confident about

### `memories_recall`

Scan your memories.

Search and filter what you know.

A text query matches when every one of its words appears somewhere in the title,
the description, the tags or the body. That is stricter than a relevance search
on purpose: a result that contains one word out of three is worse than no result,
because you will act on what comes back.

Deprecated memories are excluded unless you ask for them by status. They are
never deleted — read one with reflect to see why it was retired.

- everything you know that is still current
- what you decided about identifiers
- what applies to the files you are about to change

### `memories_reflect`

Read one memory in full.

Read a single memory: its body, its links, its lineage.

Treat what you read as a reference, not as fact. Check it against the code and
the situation in front of you before acting on it. If it turns out to be stale,
store the correction and then forget this one.

- read a memory a recall turned up

### `memories_store`

Record durable knowledge.

Store a memory when something leaves a durable trace on how you should
think, decide or act.

Without this, every session starts from zero: the same mistakes, the same lost
context, the same rediscovery. Storing is the only mechanism you have to get
better over time rather than merely faster.

Recall before you store. If a trace already exists, link to it or supersede it.
Superseding writes the replacement first and then deprecates the old one with
your reason, so the lineage can be walked later — that is how future-you finds
out not just what you believe but why you changed your mind.

- a preference learned from a review
- a decision that replaces an earlier one

