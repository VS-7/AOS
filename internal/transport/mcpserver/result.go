package mcpserver

import (
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/command"
)

// successResult renders a command result for a model.
//
// The envelope travels intact, including the call to action on success: after
// creating a task, the result suggests starting it. That is how the original's
// agent chains operations without being told to.
func successResult(out any, notice *command.DeprecationNotice) *mcp.CallToolResult {
	raw, err := json.MarshalIndent(command.Wrap(out, notice), "", "  ")
	if err != nil {
		return errorResult(err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}},
	}
}

// errorResult renders a failure as a tool error rather than a protocol error.
//
// A protocol error ends the call; a tool error reaches the model, which reads
// the code, the issues and the call to action and can act on them. The whole
// point of the structured error is lost if it is delivered as a transport
// failure.
func errorResult(err error) *mcp.CallToolResult {
	e, ok := apperr.As(err)
	if !ok {
		e = apperr.New("INTERNAL").
			Causer("mcpserver").
			Msgf("%v", err)
	}
	raw, merr := json.MarshalIndent(e, "", "  ")
	if merr != nil {
		raw = []byte(`{"code":"` + e.Code + `"}`)
	}
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}},
	}
}
