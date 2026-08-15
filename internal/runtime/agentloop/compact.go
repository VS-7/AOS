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
	cutoff := len(messages) - KeepToolCalls
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

func isEmpty(m Message) bool {
	return m.Text == "" && m.Reasoning == "" && m.Encrypted == "" &&
		len(m.ToolCalls) == 0 && len(m.Result) == 0
}
