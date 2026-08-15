// Package toolexec is what every tool call goes through.
//
// It exists for one practical problem that dominates all the others: tool
// output burns the context window. A grep over a large repository returns half
// a megabyte, and injecting that costs the whole window in one call. The answer
// here is the original's: the model gets a bounded prefix, the full output goes
// to a file, and the model is told where it is and how to read a slice of it.
package toolexec

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/OWNER/aos/internal/core/command"
)

// The limits, all four verified in the original's source.
const (
	// MaxToolOutputChars is what reaches the model.
	MaxToolOutputChars = 12_000
	// SpillThreshold is where the file starts being written. It is the same
	// number: anything the model cannot see whole is worth keeping whole.
	SpillThreshold = MaxToolOutputChars
	// OutputTTL is how long a spilled output stays readable.
	OutputTTL = 24 * time.Hour
	// MaxBase64Len bounds an inline blob that is not recognisable multimodal
	// content, so an accidental image does not eat the window.
	MaxBase64Len = 1_024
	// SafeJSONLimit bounds serialisation of a structure before it is given up
	// on, which is also the cycle guard's backstop.
	SafeJSONLimit = 2_000_000
)

// Spec is what the model reads before choosing a tool.
type Spec struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	InputSchema *jsonschema.Schema  `json:"inputSchema,omitempty"`
	Annotations command.Annotations `json:"-"`
}

// Tool is one capability the agent can call.
type Tool interface {
	Name() string
	Spec() Spec
	Invoke(ctx context.Context, in json.RawMessage) (any, error)
}

// Output is what a wrapped tool returns.
//
// A small output keeps its structure: a list of five tasks arrives as an array,
// not as JSON inside a string. Only truncation turns a result into text, which
// is a divergence from the original — it serializes in order to truncate, and
// pays that cost on every call including the ninety per cent that are small.
type Output struct {
	Data      any        `json:"data"`
	Output    string     `json:"output,omitempty"`
	Truncated *Truncated `json:"truncated,omitempty"`
}

// Truncated says what was cut and where the rest is.
type Truncated struct {
	Original    int    `json:"original"`
	Visible     int    `json:"visible"`
	Output      string `json:"output"`
	Instruction string `json:"instruction"`
}

// Func adapts a function to the Tool interface.
type Func struct {
	Definition Spec
	Fn         func(ctx context.Context, in json.RawMessage) (any, error)
}

func (f Func) Name() string { return f.Definition.Name }
func (f Func) Spec() Spec   { return f.Definition }
func (f Func) Invoke(ctx context.Context, in json.RawMessage) (any, error) {
	return f.Fn(ctx, in)
}

// FromCommand publishes a domain command as a tool.
//
// The surface is the agent registry, which is what makes `_reasoning` required:
// a model that calls a tool must say why, and the same call from a human at a
// terminal must not have to. That distinction lives in the Command Layer and is
// not re-decided here.
func FromCommand(d command.Descriptor) Tool {
	return Func{
		Definition: Spec{
			Name:        d.Key(),
			Description: d.Doc(),
			InputSchema: d.InputSchema(),
			Annotations: d.Annotations(),
		},
		Fn: func(ctx context.Context, in json.RawMessage) (any, error) {
			return d.Invoke(ctx, command.SurfaceAgent, in)
		},
	}
}
