package command

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

// Descriptor is the type-erased view of a Command, used by every surface.
//
// Wide port: the surfaces need all of it — the CLI needs flags and help, MCP
// needs schema and annotations, the agent registry needs the invoker, the
// documentation generator needs examples. Splitting it would hand every surface
// four interfaces to do one job.
type Descriptor interface {
	Key() string    // "memories_store"
	Path() []string // ["memories", "store"]
	Group() string
	Name() string
	Aliases() []string
	Summary() string
	Doc() string
	Examples() []Example
	Local() bool
	InRegistry() bool
	Annotations() Annotations
	InputSchema() *jsonschema.Schema
	InputType() reflect.Type

	// Invoke decodes, validates and runs. The surface decides whether
	// `_reasoning` is required.
	Invoke(ctx context.Context, surface Surface, raw json.RawMessage) (any, error)
}

type descriptor[In, Out any] struct {
	cmd    Command[In, Out]
	schema *jsonschema.Schema
	inType reflect.Type
	reason string // validator path of the reasoning field
}

func (d *descriptor[In, Out]) Key() string         { return strings.Join(d.Path(), "_") }
func (d *descriptor[In, Out]) Path() []string      { return []string{d.cmd.Group, d.cmd.Name} }
func (d *descriptor[In, Out]) Group() string       { return d.cmd.Group }
func (d *descriptor[In, Out]) Name() string        { return d.cmd.Name }
func (d *descriptor[In, Out]) Aliases() []string   { return d.cmd.Aliases }
func (d *descriptor[In, Out]) Summary() string     { return d.cmd.Summary }
func (d *descriptor[In, Out]) Doc() string         { return d.cmd.Doc }
func (d *descriptor[In, Out]) Examples() []Example { return d.cmd.Examples }
func (d *descriptor[In, Out]) Local() bool         { return d.cmd.Local }
func (d *descriptor[In, Out]) InRegistry() bool    { return d.cmd.Registry }
func (d *descriptor[In, Out]) Annotations() Annotations {
	return d.cmd.Annotations
}
func (d *descriptor[In, Out]) InputSchema() *jsonschema.Schema { return d.schema }
func (d *descriptor[In, Out]) InputType() reflect.Type         { return d.inType }

// Invoke is the one place where a payload becomes a call. Decoding, validation
// and the handler run here, so the five surfaces cannot validate differently.
func (d *descriptor[In, Out]) Invoke(ctx context.Context, surface Surface, raw json.RawMessage) (any, error) {
	var in In
	if len(raw) > 0 && string(raw) != "null" {
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		if err := dec.Decode(&in); err != nil {
			return nil, errInvalidInput(d.Key(), err)
		}
	}
	if err := d.validate(surface, in); err != nil {
		return nil, err
	}
	return d.cmd.Handler(ctx, in)
}

func (d *descriptor[In, Out]) validate(surface Surface, in In) error {
	if surface.RequiresReasoning() || d.reason == "" {
		if err := validateStruct(in); err != nil {
			return translateValidation(d.Key(), err)
		}
		return nil
	}
	// A human at a terminal does not justify their own command.
	if err := validateStructExcept(in, d.reason); err != nil {
		return translateValidation(d.Key(), err)
	}
	return nil
}
