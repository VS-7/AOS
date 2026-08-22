// Package mcpserver publishes the command registry as an MCP server.
//
// The server is not a separate program: it is the CLI in another mode, exactly
// as in the original, where `fractal --mcp` and `fractal <command>` collect the
// same tools. That is what makes divergence between the two impossible.
package mcpserver

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/command"
)

// Shape selects the projection of the tool surface (ADR-0011).
type Shape string

const (
	// ShapeFlat publishes one tool per command: memories_store,
	// tasks_todos_set_status. It matches the mental model of any MCP client.
	ShapeFlat Shape = "flat"

	// ShapeComposite publishes one tool per group, with an action field:
	// Memory({action:"store", …}). It cuts the context spent listing tools
	// dramatically, and it is where the ecosystem went.
	ShapeComposite Shape = "composite"

	// ShapeBoth publishes the two. Useful during a migration, at the cost of
	// listing every capability twice.
	ShapeBoth Shape = "both"
)

// Config wires a server.
type Config struct {
	Registry *command.Registry
	Shape    Shape

	// Instructions is the server-level hint an MCP client shows to the model.
	Instructions string
}

func (c Config) shape() Shape {
	switch c.Shape {
	case ShapeFlat, ShapeComposite, ShapeBoth:
		return c.Shape
	default:
		return ShapeComposite
	}
}

// New builds the server and registers the tool surface.
func New(cfg Config) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    build.Name,
		Version: build.Current().Version,
	}, &mcp.ServerOptions{Instructions: cfg.Instructions})

	switch cfg.shape() {
	case ShapeFlat:
		RegisterFlat(s, cfg.Registry)
	case ShapeBoth:
		RegisterFlat(s, cfg.Registry)
		RegisterComposite(s, cfg.Registry)
	default:
		RegisterComposite(s, cfg.Registry)
	}
	return s
}

// ServeStdio runs the MCP server over stdio. This is what `aos --mcp` does and
// what a client registers in its MCP configuration.
func ServeStdio(ctx context.Context, cfg Config) error {
	return New(cfg).Run(ctx, &mcp.StdioTransport{})
}

// NewHTTPHandler returns the same tool surface ServeStdio serves over stdio,
// as an http.Handler speaking the Streamable HTTP transport — for a client
// that reaches this daemon over the network instead of spawning it as a
// subprocess, mounted at /mcp (internal/transport/httpapi's own Config.MCP).
//
// One *mcp.Server answers every request: New's own registration work — one
// tool per command, per cfg.Shape — happens once here, not per connection,
// the same way the composite/flat shapes are chosen once for the whole
// process rather than per call. The SDK's own example does the same for
// exactly this reason.
//
// It carries no authentication of its own — httpapi.New wraps this mount in
// the same bearer-token middleware every other command surface answers
// behind, so this stays the same handler whether or not the caller requires
// a credential to reach it.
func NewHTTPHandler(cfg Config) http.Handler {
	srv := New(cfg)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
}

// annotationsOf maps our annotations onto the protocol's.
func annotationsOf(a command.Annotations) *mcp.ToolAnnotations {
	destructive := a.DestructiveHint
	openWorld := a.OpenWorldHint
	return &mcp.ToolAnnotations{
		Title:           a.Title,
		ReadOnlyHint:    a.ReadOnlyHint,
		IdempotentHint:  a.IdempotentHint,
		DestructiveHint: &destructive,
		OpenWorldHint:   &openWorld,
	}
}
