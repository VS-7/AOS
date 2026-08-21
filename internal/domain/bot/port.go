package bot

import (
	"context"
	"net/http"
	"time"
)

// Provider is the strategy for one messaging platform. Telegram
// (internal/adapters/telegramapi) is the only implementation today; the
// interface exists because the original's schema already anticipates others
// and because it makes the registry testable without a real network.
type Provider interface {
	Name() string

	// RegisterWebhook points the provider at url, so the daemon starts
	// receiving updates. secret is verified on every inbound request Parse
	// handles — see its own doc.
	RegisterWebhook(ctx context.Context, url, secret string, cfg map[string]any) error

	// UnregisterWebhook tells the provider to stop delivering updates,
	// e.g. when an agent's channel is removed or the tunnel goes down.
	UnregisterWebhook(ctx context.Context, cfg map[string]any) error

	// Parse converts an inbound HTTP request into a normalized message,
	// verifying the webhook secret first — in constant time, so a caller
	// timing the comparison learns nothing about the real secret. A request
	// whose secret does not match is refused before its body is even read.
	Parse(ctx context.Context, r *http.Request, secret string) (Inbound, error)

	// Send delivers text to chatID, splitting at the provider's own limits
	// on a block boundary when the message is too long for one call. Token
	// is the registration's own — a shared *telegramapi.Provider serves
	// every agent's channel, so it cannot hold just one.
	Send(ctx context.Context, token string, out Outbound) error

	SetTyping(ctx context.Context, token, chatID string, on bool) error
}

// Inbound is one normalized message a Provider parsed from a webhook.
type Inbound struct {
	// ChatID is the provider's own conversation identifier — resolveChat
	// combines it with Provider to address the bound chat.Chat.
	ChatID string
	UserID string
	Text   string
}

// Outbound is what Send delivers.
type Outbound struct {
	ChatID string
	Text   string
}

// Chats is the narrow slice of chat.Service this package needs: finding the
// conversation an inbound message belongs to, opening one when there is
// none yet, and writing to it. Kept a separate port rather than an import of
// internal/domain/chat directly, the same reason every domain here narrows
// its dependencies to what it actually uses — see e.g.
// internal/domain/goal/port.go's Tasks.
type Chats interface {
	GetByChannel(ctx context.Context, provider, chatID string) (ChatRef, error)
	CreateForChannel(ctx context.Context, provider, chatID, agentID, title string) (ChatRef, error)
	Send(ctx context.Context, chatID, text, agentID string) error
}

// ChatRef is the one field this package reads off a chat.Chat: its id, to
// address it in a later Send.
type ChatRef struct{ ID string }

// EnvResolver is what Interpolate reads a ${env.VAR} placeholder against —
// the same shape internal/domain/toolset's own EnvResolver declares. Not
// imported from there: a domain package does not import another domain
// package (see internal/architecture's dependency rule; toolset's own
// Interpolate is duplicated in interpolate.go for the identical reason
// artifact's password hashing duplicates auth's argon2 parameters rather
// than importing that package).
type EnvResolver interface {
	String(key, def string) string
}

// PublicURL is the narrow slice of tunnel.Service this package needs: the
// public hostname a webhook can be reached at, and whether one currently
// exists. Boot order is tunnel -> bots (see the design doc), enforced by the
// integrator calling RegisterAll only after the tunnel is up, not by this
// package polling for one.
type PublicURL interface {
	// URL returns the tunnel's public base URL and true when one is active.
	// false means "no tunnel right now" — Pending, not an error.
	URL(ctx context.Context) (string, bool)
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }

// Logger is the narrow slice of *slog.Logger this package needs, kept as an
// interface so a test can assert on what was logged without a real logger.
type Logger interface {
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}
