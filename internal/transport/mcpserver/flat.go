package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OWNER/aos/internal/core/command"
)

// RegisterFlat publishes one tool per command: memories_store,
// tasks_todos_set_status. The name is the command path joined by underscores,
// and the list is alphabetical so that two runs present it identically.
func RegisterFlat(s *mcp.Server, reg *command.Registry) {
	for _, d := range reg.Sorted() {
		s.AddTool(&mcp.Tool{
			Name:        d.Key(),
			Description: d.Doc(),
			InputSchema: d.InputSchema(),
			Annotations: annotationsOf(d.Annotations()),
		}, invoke(d, nil))
	}

	// A published tool name is a contract. An alias keeps working after a
	// rename and tells the model, in the result, which name to use next time —
	// so the model relearns by itself instead of breaking (ADR-0011).
	for _, alias := range reg.Aliases() {
		target, notice, ok := reg.Lookup(alias.From)
		if !ok {
			continue
		}
		s.AddTool(&mcp.Tool{
			Name:        alias.From,
			Description: deprecatedDoc(alias, target),
			InputSchema: target.InputSchema(),
			Annotations: annotationsOf(target.Annotations()),
		}, invoke(target, notice))
	}
}

func invoke(d command.Descriptor, notice *command.DeprecationNotice) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		out, err := d.Invoke(ctx, command.SurfaceMCP, req.Params.Arguments)
		if err != nil {
			return errorResult(err), nil
		}
		return successResult(out, notice), nil
	}
}

func deprecatedDoc(a command.Alias, target command.Descriptor) string {
	notice := "DEPRECATED — use `" + a.To + "` instead."
	if a.RemoveAt != "" {
		notice += " This name stops working in " + a.RemoveAt + "."
	}
	return notice + "\n\n" + target.Doc()
}
