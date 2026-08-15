// Package logging builds the structured logger of the system.
//
// log/slog from the standard library, not pino or zerolog: structured, no
// dependency, and with a customisable Handler — which is what makes redaction
// possible at the handler level rather than at every call site.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/OWNER/aos/internal/core/identity"
)

// Config selects level, format and destination.
type Config struct {
	Level  string    // debug | info | warn | error
	Format string    // text | json | auto
	Output io.Writer // defaults to os.Stderr
	TTY    bool      // used when Format is auto
}

// New builds the structured logger. Text on a TTY, JSON otherwise — the same
// "who is consuming this" heuristic the CLI uses to pick an output format.
func New(cfg Config) *slog.Logger {
	out := cfg.Output
	if out == nil {
		out = os.Stderr
	}
	opts := &slog.HandlerOptions{Level: parseLevel(cfg.Level)}

	var h slog.Handler
	if useJSON(cfg) {
		h = slog.NewJSONHandler(out, opts)
	} else {
		h = slog.NewTextHandler(out, opts)
	}
	return slog.New(redactHandler(h))
}

func useJSON(cfg Config) bool {
	switch strings.ToLower(cfg.Format) {
	case "json":
		return true
	case "text":
		return false
	default:
		return !cfg.TTY
	}
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type ctxKey struct{}

// Into attaches a logger to ctx so that downstream code shares its fields.
func Into(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns a logger already carrying the ambient identity, so every
// line in a request is correlatable without the caller repeating fields.
func FromContext(ctx context.Context) *slog.Logger {
	l, ok := ctx.Value(ctxKey{}).(*slog.Logger)
	if !ok || l == nil {
		l = slog.Default()
	}
	id := identity.From(ctx)
	attrs := make([]any, 0, 6)
	if id.RequestID != "" {
		attrs = append(attrs, "request_id", id.RequestID)
	}
	if id.WorkspaceID != "" {
		attrs = append(attrs, "workspace", id.WorkspaceID)
	}
	if id.AgentID != "" {
		attrs = append(attrs, "agent", id.AgentID)
	}
	if len(attrs) == 0 {
		return l
	}
	return l.With(attrs...)
}

// Component returns a logger tagged with the component that writes it.
func Component(ctx context.Context, name string) *slog.Logger {
	return FromContext(ctx).With("component", name)
}
