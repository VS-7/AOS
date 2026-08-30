package command

import (
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/OWNER/aos/internal/core/tokens"
)

// SchemaField is the reserved key that turns a call into a question about the
// call.
//
// It is reserved on every surface rather than declared per command, because it
// is what every validation error tells the caller to send: "read the issues,
// inspect the contract with schema:true, then fix the payload". A caller that
// does not know the contract cannot be asked to satisfy it before being allowed
// to read it, which is what happened while this was honoured only by the
// composite MCP tool — validation ran first, so the answer to "what does this
// command take?" was the same refusal that prompted the question.
//
// No command input may declare a field of this name; registration refuses one.
const SchemaField = "schema"

// Detail is what `schema: true` answers with.
//
// It carries the token estimate as well as the contract, so the caller can
// budget its own context before deciding to read further. The system tells the
// agent to inspect before executing; this is what it inspects.
type Detail struct {
	// Tool is what the caller addressed: the flat key on HTTP and the CLI
	// ("memories_store"), the composite tool's name over MCP ("Memory").
	Tool        string             `json:"tool"`
	Action      string             `json:"action,omitempty"`
	Description string             `json:"description"`
	Examples    []Example          `json:"examples,omitempty"`
	InputSchema *jsonschema.Schema `json:"inputSchema"`
	Tokens      TokenEstimate      `json:"_tokens"`
}

// TokenEstimate breaks the cost of a detail down by section, so the agent knows
// what it is about to spend.
type TokenEstimate struct {
	Description int `json:"description"`
	Examples    int `json:"examples"`
	InputSchema int `json:"inputSchema"`
	Total       int `json:"total"`
}

// DetailOf renders the introspection answer for one command.
//
// schema is passed in rather than read from d because the two shapes publish
// different ones: on the flat surface `_reasoning` is part of the payload, and
// on the composite tool it belongs next to `action` instead.
func DetailOf(tool string, d Descriptor, schema *jsonschema.Schema) Detail {
	detail := Detail{
		Tool:        tool,
		Action:      d.Name(),
		Description: d.Doc(),
		Examples:    d.Examples(),
		InputSchema: schema,
	}
	detail.Tokens = estimateOf(detail)
	return detail
}

// FlatDetail is DetailOf for the surfaces that take the command's own payload
// whole: HTTP, the terminal and the flat MCP tools.
func FlatDetail(d Descriptor) Detail { return DetailOf(d.Key(), d, d.InputSchema()) }

// SchemaWithoutReasoning copies a command's input schema with `_reasoning`
// removed, for the shape that carries the field on the outer payload.
func SchemaWithoutReasoning(d Descriptor) *jsonschema.Schema {
	original := d.InputSchema()
	if original == nil {
		return nil
	}
	trimmed := *original
	trimmed.Properties = make(map[string]*jsonschema.Schema, len(original.Properties))
	for name, prop := range original.Properties {
		if name == ReasoningField {
			continue
		}
		trimmed.Properties[name] = prop
	}
	trimmed.Required = nil
	for _, name := range original.Required {
		if name == ReasoningField {
			continue
		}
		trimmed.Required = append(trimmed.Required, name)
	}
	return &trimmed
}

func estimateOf(d Detail) TokenEstimate {
	est := TokenEstimate{Description: tokens.Estimate(d.Description)}
	if raw, err := json.Marshal(d.Examples); err == nil {
		est.Examples = tokens.Estimate(string(raw))
	}
	if raw, err := json.Marshal(d.InputSchema); err == nil {
		est.InputSchema = tokens.Estimate(string(raw))
	}
	est.Total = est.Description + est.Examples + est.InputSchema
	return est
}

// asksForSchema reports whether a payload is a question about the contract
// rather than a call.
//
// A payload that does not parse is not one: it has to reach the decoder, which
// reports what is wrong with it. Only the literal `true` counts, so
// `schema:false` runs the command as written.
func asksForSchema(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return false
	}
	value, present := fields[SchemaField]
	if !present {
		return false
	}
	var asked bool
	if err := json.Unmarshal(value, &asked); err != nil {
		return false
	}
	return asked
}
