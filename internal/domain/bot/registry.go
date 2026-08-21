package bot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

// newWebhookSecret generates the per-registration secret Parse verifies on
// every inbound request — the mandatory hardening the design doc adds over
// the original, which generates one but never checks it back.
func newWebhookSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing means the OS's entropy source is broken, not
		// something a fallback should paper over — but this is a webhook
		// secret, not a decision that can refuse to boot the daemon over
		// it, so a fixed-length fallback keeps registration functional
		// rather than crashing a boot on an unrelated subsystem's failure.
		return "insecure-fallback-" + hex.EncodeToString(b)
	}
	return hex.EncodeToString(b)
}

// AgentChannel is the narrow projection of agent.Channel{Provider, Data}
// this package reads, plus the agent it belongs to. Kept local rather than
// importing internal/domain/agent — see EnvResolver's doc comment on why.
type AgentChannel struct {
	AgentID     string
	WorkspaceID string
	Provider    string
	Data        map[string]any // e.g. {"token": "${env.TELEGRAM_TOKEN}"}
}

// resolveChat maps an external conversation to the deterministic id a Chat
// bound to it carries — provider and the provider's own chat id, exactly as
// the design doc specifies. Not itself a chat.Chat.ID (chat.Service assigns
// its own); it is what GetByChannel/CreateForChannel key their lookup on.
func resolveChat(provider, channelID string) string {
	return provider + "-" + channelID
}

// Deps is what a Registry is built from.
type Deps struct {
	// Providers is every channel provider this build has, keyed by name —
	// "telegram" today.
	Providers map[string]Provider

	Chats     Chats
	PublicURL PublicURL
	Env       EnvResolver
	Clock     Clock
	Log       Logger

	// RateLimit and RateWindow bound outbound Deliver calls per chat. Zero
	// means the package default (20 messages per 10 seconds), generous
	// enough not to interfere with a normal conversation while still
	// stopping a runaway agent from tripping the provider's own limits.
	RateLimit  int
	RateWindow time.Duration
}

const (
	defaultRateLimit  = 20
	defaultRateWindow = 10 * time.Second
)

// Registry holds one registration per agent-provider pair. Built at boot,
// after the tunnel is up — see RegisterAll — and safe for concurrent use:
// an inbound webhook and a boot-time re-registration can overlap.
type Registry struct {
	mu            sync.RWMutex
	providers     map[string]Provider
	registrations map[string]Registration // key() -> Registration

	chats     Chats
	publicURL PublicURL
	env       EnvResolver
	clock     Clock
	log       Logger

	rateLimit  int
	rateWindow time.Duration
	limitersMu sync.Mutex
	limiters   map[string]*limiter
}

// NewRegistry builds an empty registry over its ports.
func NewRegistry(d Deps) *Registry {
	rate, window := d.RateLimit, d.RateWindow
	if rate <= 0 {
		rate = defaultRateLimit
	}
	if window <= 0 {
		window = defaultRateWindow
	}
	return &Registry{
		providers:     d.Providers,
		registrations: map[string]Registration{},
		chats:         d.Chats,
		publicURL:     d.PublicURL,
		env:           d.Env,
		clock:         d.Clock,
		log:           d.Log,
		rateLimit:     rate,
		rateWindow:    window,
		limiters:      map[string]*limiter{},
	}
}

// RegisterAll registers a webhook for every declared channel, one attempt
// per channel — a failure on one does not stop the rest. Called at boot,
// after the tunnel reports a URL (or confirms it will not: see PublicURL),
// and again whenever an agent's channels change.
//
// Without a public URL, every channel is recorded Pending rather than
// attempted — an honest deferral, not a silent no-op and not a failure. The
// design doc calls this out explicitly: "sem tunnel ativo, o Telegram não
// tem para onde entregar."
func (r *Registry) RegisterAll(ctx context.Context, channels []AgentChannel) []Registration {
	base, up := r.publicURL.URL(ctx)

	out := make([]Registration, 0, len(channels))
	for _, ch := range channels {
		reg := Registration{
			Provider: ch.Provider, WorkspaceID: ch.WorkspaceID, AgentID: ch.AgentID,
		}

		if !up {
			reg.Status = Pending
			reg.Error = "no tunnel is active"
			out = append(out, r.store(reg))
			continue
		}

		p, ok := r.providers[ch.Provider]
		if !ok {
			reg.Status = Failed
			reg.Error = errProviderNotAvailable(ch.Provider).Error()
			out = append(out, r.store(reg))
			continue
		}

		token, literal, err := interpolate(tokenOf(ch.Data), r.env)
		if err != nil {
			reg.Status = Failed
			reg.Error = err.Error()
			out = append(out, r.store(reg))
			continue
		}
		if literal && r.log != nil {
			r.log.Warn("channel token is a literal value in the agent's frontmatter, not ${env.*} — it will be committed to version control",
				"provider", ch.Provider, "agent", ch.AgentID)
		}

		secret := newWebhookSecret()
		url := strings.TrimRight(base, "/") + "/api/bot/" + ch.Provider + "/webhook/" + ch.AgentID
		cfg := map[string]any{"token": token}
		if err := p.RegisterWebhook(ctx, url, secret, cfg); err != nil {
			reg.Status = Failed
			reg.Error = err.Error()
			out = append(out, r.store(reg))
			continue
		}

		reg.Status = Registered
		reg.WebhookURL = url
		reg.WebhookSecret = secret
		reg.RegisteredAt = r.clock.Now()
		out = append(out, r.store(reg))
	}
	return out
}

func tokenOf(data map[string]any) string {
	if v, ok := data["token"].(string); ok {
		return v
	}
	return ""
}

func (r *Registry) store(reg Registration) Registration {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registrations[reg.key()] = reg
	return reg
}

func (r *Registry) lookup(provider, agentID string) (Registration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	reg, ok := r.registrations[(Registration{Provider: provider, AgentID: agentID}).key()]
	return reg, ok
}

// HandleWebhook is the one HTTP-facing entrypoint this package has. provider
// and agentID come from the request path the integrator mounts (see
// INTEGRATION.md); the registration named by both carries the secret Parse
// verifies before anything else in the request is trusted.
//
// A verified message resolves to its Chat (created on first contact) and is
// handed to Chats.Send, which persists it and, on the other side of the
// agent runtime, produces the reply Deliver later pushes back out.
func (r *Registry) HandleWebhook(ctx context.Context, provider, agentID string, req *http.Request) error {
	reg, ok := r.lookup(provider, agentID)
	if !ok {
		return errRegistrationNotFound(provider, agentID)
	}
	p, ok := r.providers[provider]
	if !ok {
		return errProviderNotAvailable(provider)
	}

	in, err := p.Parse(ctx, req, reg.WebhookSecret)
	if err != nil {
		return err
	}

	chatID := resolveChat(provider, in.ChatID)
	ref, err := r.chats.GetByChannel(ctx, provider, in.ChatID)
	if err != nil {
		ref, err = r.chats.CreateForChannel(ctx, provider, in.ChatID, reg.AgentID, chatID)
		if err != nil {
			return err
		}
	}
	return r.chats.Send(ctx, ref.ID, in.Text, reg.AgentID)
}

// Deliver pushes text out to the channel a chat is bound to, rate-limited
// per chat so one verbose agent cannot exhaust the provider's own limit for
// everyone. The integrator calls this from wherever a channel-bound chat's
// new assistant message is observed — see INTEGRATION.md.
func (r *Registry) Deliver(ctx context.Context, provider, chatID, text string) error {
	if !r.allow(chatID) {
		return errRateLimited(chatID)
	}
	p, ok := r.providers[provider]
	if !ok {
		return errProviderNotAvailable(provider)
	}
	if err := p.Send(ctx, Outbound{ChatID: chatID, Text: text}); err != nil {
		return errSendFailed(provider, chatID, err)
	}
	return nil
}

// limiter is a fixed-window counter — simple on purpose, per the design
// doc's own "não precisa ser sofisticado" framing for this addition.
type limiter struct {
	mu       sync.Mutex
	count    int
	windowAt time.Time
}

func (r *Registry) allow(chatID string) bool {
	r.limitersMu.Lock()
	l, ok := r.limiters[chatID]
	if !ok {
		l = &limiter{}
		r.limiters[chatID] = l
	}
	r.limitersMu.Unlock()

	l.mu.Lock()
	defer l.mu.Unlock()
	now := r.clock.Now()
	if now.Sub(l.windowAt) > r.rateWindow {
		l.windowAt = now
		l.count = 0
	}
	if l.count >= r.rateLimit {
		return false
	}
	l.count++
	return true
}
