// Package subconscious is the second model: a cheap, frequent observer that
// watches a session and decides what becomes memory.
//
// It exists because of a gap between what the master prompt asks for and what
// agents actually do. The prompt tells the agent to reflect and maintain
// memories before answering; under cost pressure that is the step that gets
// skipped. The subconscious takes the responsibility off the critical path, so
// the memory forms whether or not the main agent thought about it. That is the
// difference between "the agent should remember" and "the system remembers".
package subconscious

import "time"

// The operating limits, from the original and deliberately aggressive. This is
// a cheap and frequent observer, never a deep one.
const (
	// MaxRecentMessages is the window of conversation it reads.
	MaxRecentMessages = 8

	// MaxRecentEvents is the window of session events it reads.
	MaxRecentEvents = 12

	// InputCharLimit caps the whole formatted context.
	InputCharLimit = 12_000
)

// Defaults for the parts the original leaves fixed.
const (
	// DefaultTimeout bounds one observation.
	DefaultTimeout = 30 * time.Second

	// DefaultMaxDrafts is how many memories one observation may propose. The
	// cap matters: a model asked for durable learnings will find some if it
	// looks hard enough, and an unbounded pass fills the graph with noise.
	DefaultMaxDrafts = 3

	// DefaultMinInterval is the floor between observations of one session.
	// Ten short turns in a minute should produce one observation, not ten.
	// The original does not coalesce at all.
	DefaultMinInterval = 60 * time.Second

	// DefaultSignatureTTL is how long a signature suppresses a repeat draft.
	// Long enough that a session cannot recreate the same memory; short enough
	// that a genuinely recurring lesson can be recorded again months later.
	DefaultSignatureTTL = 30 * 24 * time.Hour
)

// Config is what one observer runs with.
type Config struct {
	// Model names the slot. Empty falls back through the cascade in the
	// composition root: the subconscious slot, then the agent's own model, then
	// the default slot.
	Provider  string
	Model     string
	Reasoning string

	Timeout     time.Duration
	MaxDrafts   int
	MinInterval time.Duration

	// SignatureTTL is how long a stored signature suppresses a repeat.
	SignatureTTL time.Duration
}

// withDefaults fills the zero values.
func (c Config) withDefaults() Config {
	if c.Timeout <= 0 {
		c.Timeout = DefaultTimeout
	}
	if c.MaxDrafts <= 0 {
		c.MaxDrafts = DefaultMaxDrafts
	}
	if c.MinInterval <= 0 {
		c.MinInterval = DefaultMinInterval
	}
	if c.SignatureTTL <= 0 {
		c.SignatureTTL = DefaultSignatureTTL
	}
	return c
}
