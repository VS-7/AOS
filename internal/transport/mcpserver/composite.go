package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OWNER/aos/internal/core/command"
)

// CompositeInput is the payload of a composite tool.
type CompositeInput struct {
	Action string          `json:"action"`
	Input  json.RawMessage `json:"input,omitempty"`
	Schema bool            `json:"schema,omitempty"`

	Reasoning string `json:"_reasoning"`
}

// ActionDetail is what `schema: true` returns.
//
// The type lives in the Command Layer because `schema: true` is answered by
// every surface, not only by this one — the composite shape was the only place
// that honoured it while the flat HTTP and terminal surfaces validated the
// question as if it were a call (defect #2). These aliases keep this package's
// published names.
type ActionDetail = command.Detail

// TokenEstimate breaks the cost of a detail down by section, so the agent knows
// what it is about to spend.
type TokenEstimate = command.TokenEstimate

// RegisterComposite publishes one tool per group, with an action field.
func RegisterComposite(s *mcp.Server, reg *command.Registry) {
	for _, group := range reg.Groups() {
		if len(group.Commands) == 0 {
			continue
		}
		g := group
		name := compositeName(g)
		s.AddTool(&mcp.Tool{
			Name:        name,
			Description: compositeDescription(name, g),
			InputSchema: compositeSchema(g),
			Annotations: mergeAnnotations(g),
		}, compositeHandler(name, g))
	}
}

// compositeName is the group's tool name: "Memory" for the "memories" group,
// following the original's casing.
func compositeName(g command.Group) string {
	if g.Tool != "" {
		return g.Tool
	}
	name := strings.TrimSuffix(g.Name, "s")
	if name == "" {
		name = g.Name
	}
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}

// DescriptionOf renders the composite description of a group. It is exported
// because the documentation generator and the golden files must show the exact
// text an MCP client receives, not a second rendering of it.
func DescriptionOf(g command.Group) string { return compositeDescription(compositeName(g), g) }

// compositeDescription assembles the tool description the way the original
// does: the group documentation, the action list, an optional hint, and the
// usage block — in that order.
func compositeDescription(tool string, g command.Group) string {
	actions := actionNames(g)

	var parts []string
	if g.Doc != "" {
		parts = append(parts, g.Doc)
	}
	parts = append(parts, fmt.Sprintf("Composite tool `%s` with %d actions: %s.",
		tool, len(actions), strings.Join(actions, ", ")))
	if g.Hint != "" {
		parts = append(parts, g.Hint)
	}
	parts = append(parts, strings.Join([]string{
		"## Usage",
		fmt.Sprintf("Call as `%s({ action: \"<action>\", input: { ... }, _reasoning: \"...\" })`.", tool),
		"Set `schema: true` on the same level as `action` to receive the full action detail " +
			"(description, examples, input schema, token estimate) instead of executing.",
	}, "\n"))

	return strings.Join(parts, "\n\n")
}

func actionNames(g command.Group) []string {
	out := make([]string, 0, len(g.Commands))
	for _, d := range g.Commands {
		out = append(out, d.Name())
	}
	return out
}

// compositeSchema describes the composite payload. The per-action input schemas
// are offered through `schema: true` rather than inlined as a union: inlining
// thirteen action schemas in one tool description is exactly the context cost
// the composite shape exists to avoid.
func compositeSchema(g command.Group) *jsonschema.Schema {
	actions := actionNames(g)
	enum := make([]any, 0, len(actions))
	for _, a := range actions {
		enum = append(enum, a)
	}

	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"action": {
				Type:        "string",
				Enum:        enum,
				Description: "Action to execute. One of: " + strings.Join(actions, ", "),
			},
			"input": {
				Type: "object",
				Description: "Payload for the chosen action. Call with schema:true to see " +
					"the exact shape the action expects.",
			},
			"schema": {
				Type: "boolean",
				Description: "When true, skip execution and return the full action detail " +
					"(description, examples, input schema, token estimate). Use this to " +
					"inspect a composite tool's surface before calling it.",
			},
			command.ReasoningField: {
				Type:        "string",
				MinLength:   jsonschema.Ptr(1),
				Description: command.ReasoningDescription,
			},
		},
		Required: []string{"action", command.ReasoningField},
	}
}

// mergeAnnotations folds the actions' annotations the way the original does:
// read-only only if every action is, destructive if any action is.
func mergeAnnotations(g command.Group) *mcp.ToolAnnotations {
	merged := command.Annotations{Title: compositeName(g), ReadOnlyHint: true, IdempotentHint: true}
	for _, d := range g.Commands {
		a := d.Annotations()
		merged.ReadOnlyHint = merged.ReadOnlyHint && a.ReadOnlyHint
		merged.IdempotentHint = merged.IdempotentHint && a.IdempotentHint
		merged.DestructiveHint = merged.DestructiveHint || a.DestructiveHint
		merged.OpenWorldHint = merged.OpenWorldHint || a.OpenWorldHint
	}
	return annotationsOf(merged)
}

func compositeHandler(tool string, g command.Group) mcp.ToolHandler {
	byAction := make(map[string]command.Descriptor, len(g.Commands))
	for _, d := range g.Commands {
		byAction[d.Name()] = d
	}

	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in CompositeInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return errorResult(errInvalidComposite(tool, err)), nil
		}
		if strings.TrimSpace(in.Reasoning) == "" {
			return errorResult(errReasoningRequired(tool)), nil
		}
		d, ok := byAction[in.Action]
		if !ok {
			return errorResult(errUnknownAction(tool, in.Action, actionNames(g))), nil
		}

		// schema:true inspects without executing. A tool that ran anyway would
		// make introspection unusable for anything destructive.
		if in.Schema {
			return successResult(detailOf(tool, d), nil), nil
		}

		// `_reasoning` travels on the outer payload, not inside `input`: the
		// original builds the per-action input schema without it and carries it
		// next to `action`. The handler still expects it in its own input, so
		// the two are spliced back together here — which also keeps a call
		// through the composite shape byte-identical to the flat one.
		payload, err := withReasoning(in.Input, in.Reasoning)
		if err != nil {
			return errorResult(errInvalidComposite(tool, err)), nil
		}
		out, err := d.Invoke(ctx, command.SurfaceMCP, payload)
		if err != nil {
			return errorResult(err), nil
		}
		return successResult(out, nil), nil
	}
}

// withReasoning splices the outer reasoning into the action payload.
func withReasoning(input json.RawMessage, reasoning string) (json.RawMessage, error) {
	fields := map[string]json.RawMessage{}
	if len(input) > 0 && string(input) != "null" {
		if err := json.Unmarshal(input, &fields); err != nil {
			return nil, err
		}
	}
	raw, err := json.Marshal(reasoning)
	if err != nil {
		return nil, err
	}
	fields[command.ReasoningField] = raw
	return json.Marshal(fields)
}

// detailOf renders one action's contract. The schema is the command's own
// without `_reasoning`, because on this shape the field belongs to the
// composite payload rather than to the action.
func detailOf(tool string, d command.Descriptor) ActionDetail {
	return command.DetailOf(tool, d, command.SchemaWithoutReasoning(d))
}
