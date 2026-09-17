package agentloop

// ThresholdChars is where the history gets pruned. The original's number.
const ThresholdChars = 100_000

// KeepToolCalls is how many recent messages keep their tool calls and results.
// Beyond that the model has the conclusion it drew from them, which is what it
// reasons with; the raw output is on disk if it needs to go back.
const KeepToolCalls = 15

// Policy is what a prune removes.
//
// The three settings are the original's, and they are the reason compaction is
// safe here at all: nothing that matters lives only in the context. What the
// subconscious wrote is in the memory graph, a task's progress is in its
// comments, and a tool's full output is in the spillover directory for a day.
// Compaction throws away the copy, not the thing.
type Policy struct {
	// Reasoning removes the model's own thinking from older turns. It is the
	// largest single saving and the least costly: reasoning is how a
	// conclusion was reached, and the conclusion is in the message.
	Reasoning bool

	// ToolCalls removes calls and results older than KeepToolCalls messages.
	ToolCalls bool

	// EmptyMessages removes what pruning emptied, so the model does not read
	// a conversation full of blanks.
	EmptyMessages bool
}

// DefaultPolicy is the original's: reasoning all, tool calls before the last
// fifteen messages, empty messages removed.
func DefaultPolicy() Policy {
	return Policy{Reasoning: true, ToolCalls: true, EmptyMessages: true}
}

// Compactor decides when to prune and how.
type Compactor struct {
	Threshold int
	Policy    Policy
}

// NewCompactor builds one with the inherited defaults.
func NewCompactor() *Compactor {
	return &Compactor{Threshold: ThresholdChars, Policy: DefaultPolicy()}
}

// ShouldCompact reports whether the history has grown past the threshold.
func (c *Compactor) ShouldCompact(chars int) bool {
	threshold := c.Threshold
	if threshold <= 0 {
		threshold = ThresholdChars
	}
	return chars > threshold
}

// Prune applies the policy.
//
// The last user message is never removed, whatever the policy says. An agent
// that pruned away the thing it was asked to do would answer a question nobody
// asked, and no saving is worth that.
func Prune(messages []Message, p Policy) []Message {
	if len(messages) == 0 {
		return messages
	}
	cutoff := pairedCutoff(messages, len(messages)-KeepToolCalls)
	lastUser := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == RoleUser {
			lastUser = i
			break
		}
	}

	out := make([]Message, 0, len(messages))
	for i, m := range messages {
		recent := i >= cutoff
		protected := i == lastUser

		if p.Reasoning && !protected {
			m.Reasoning = ""
			m.Encrypted = ""
		}
		if p.ToolCalls && !recent && !protected {
			m.ToolCalls = nil
			if m.Role == RoleTool {
				// A tool message with its result removed carries nothing at
				// all, so it is dropped rather than kept as an empty turn.
				continue
			}
		}
		if p.EmptyMessages && !protected && isEmpty(m) {
			continue
		}
		out = append(out, m)
	}
	return out
}

// pairedCutoff moves the window back until it no longer splits an exchange.
//
// A turn is two messages: the assistant message that asks for a tool and the
// tool message that answers it. A flat index cut falls between them roughly
// half the time — the assistant message loses its calls and is dropped as
// empty, while the results one index later sit inside the window and survive.
//
// What reaches the provider is then a function_call_output whose function_call
// is gone, and the Responses API refuses the entire request:
//
//	No tool call found for function call output with call_id call_…
//
// Which is to say: every session long enough to compact stopped answering at
// all. The saving is not worth a conversation the model cannot be sent.
//
// So the window is widened, never narrowed, to the start of the step the cut
// falls in: the assistant message whose results follow it. It used to widen
// to the earliest message offering any id a kept result answers, and on
// Google, which names every step's first Read "Read-1", that was the first
// step of the turn — nothing was ever pruned, and a long turn there grew until
// the context ran out.
func pairedCutoff(messages []Message, cutoff int) int {
	if cutoff <= 0 {
		return 0
	}
	if cutoff >= len(messages) {
		return len(messages)
	}
	for cutoff > 0 && messages[cutoff].Role == RoleTool {
		cutoff--
	}
	return cutoff
}

// PairToolMessages returns the history with every tool call and tool result
// that lacks its counterpart removed.
//
// Every provider refuses the other shape outright — the Responses API with "No
// tool call found for function call output", Anthropic with a tool_result that
// names no tool_use, Gemini with a functionResponse turn that does not follow
// its functionCall turn part for part — and refuses the whole request, so one
// unpaired item makes a conversation unanswerable for good. pairedCutoff keeps
// a prune from making one; this is what makes a history that already holds
// one sendable, whatever wrote it. Conversations stored before answers
// recorded their calls beside their results are the case that exists.
//
// A call and its result pair within one step: the assistant message that asks,
// and the run of tool messages right after it. Inside the step each result
// answers the first call with its id that nothing has answered yet. Pairing by
// id across the whole history is what this used to do, and ids are not unique
// across it — Google names a call after its position in the answer, and
// Antigravity does when its API names none — so the first step's "Read-1"
// claimed every later "Read-1" result and every later step was thrown away.
// Nor is a result read as an answer to a call made before somebody else spoke:
// no provider reads it that way.
//
// The input is not modified. A history with nothing to repair comes back as it
// was given.
func PairToolMessages(messages []Message) []Message {
	// Which calls of each asking message are answered, and which results
	// answer something. Decided before anything is copied, so an intact history
	// costs two passes and no copy.
	answered := map[int][]bool{}
	kept := make([]bool, len(messages))
	for i := 0; i < len(messages); i++ {
		m := messages[i]
		if m.Role == RoleTool || len(m.ToolCalls) == 0 {
			continue
		}
		asking := i
		open := map[string][]int{}
		for k, c := range m.ToolCalls {
			open[c.ID] = append(open[c.ID], k)
		}
		calls := make([]bool, len(m.ToolCalls))
		for i+1 < len(messages) && messages[i+1].Role == RoleTool {
			i++
			waiting := open[messages[i].CallID]
			if len(waiting) == 0 {
				continue
			}
			calls[waiting[0]] = true
			open[messages[i].CallID] = waiting[1:]
			kept[i] = true
		}
		answered[asking] = calls
	}

	var out []Message
	for i, m := range messages {
		next, drop := m, false
		switch {
		case m.Role == RoleTool:
			drop = !kept[i]
		case len(m.ToolCalls) > 0:
			calls := answered[i]
			if n := countTrue(calls); n != len(m.ToolCalls) {
				next.ToolCalls = make([]ToolCall, 0, n)
				for k, c := range m.ToolCalls {
					if calls[k] {
						next.ToolCalls = append(next.ToolCalls, c)
					}
				}
				if n == 0 {
					next.ToolCalls = nil
					// An assistant message that only asked for what was
					// removed carries nothing the model could read.
					drop = isEmpty(next)
				}
			}
		}

		changed := drop || len(next.ToolCalls) != len(m.ToolCalls)
		if changed && out == nil {
			out = make([]Message, i, len(messages))
			copy(out, messages[:i])
		}
		if out != nil && !drop {
			out = append(out, next)
		}
	}
	if out == nil {
		return messages
	}
	return out
}

func countTrue(v []bool) int {
	n := 0
	for _, b := range v {
		if b {
			n++
		}
	}
	return n
}

func isEmpty(m Message) bool {
	return m.Text == "" && m.Reasoning == "" && m.Encrypted == "" &&
		len(m.ToolCalls) == 0 && len(m.Result) == 0
}
