package bot_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/domain/bot"
)

// fakeProvider is an in-memory bot.Provider a test drives directly, without
// a real messaging platform.
type fakeProvider struct {
	mu          sync.Mutex
	registered  []string // urls RegisterWebhook was called with
	sent        []bot.Outbound
	parseResult bot.Inbound
	parseErr    error
	registerErr error
}

func (f *fakeProvider) Name() string { return "telegram" }

func (f *fakeProvider) RegisterWebhook(_ context.Context, url, _ string, _ map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.registerErr != nil {
		return f.registerErr
	}
	f.registered = append(f.registered, url)
	return nil
}

func (f *fakeProvider) UnregisterWebhook(context.Context, map[string]any) error { return nil }

func (f *fakeProvider) Parse(context.Context, string, string, []byte) (bot.Inbound, error) {
	return f.parseResult, f.parseErr
}

func (f *fakeProvider) Send(_ context.Context, _ string, out bot.Outbound) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, out)
	return nil
}

func (f *fakeProvider) SetTyping(context.Context, string, string, bool) error { return nil }

// fakeChats is an in-memory bot.Chats.
type fakeChats struct {
	mu     sync.Mutex
	byKey  map[string]string // provider+chatID -> chat id
	sent   []sentMessage
	nextID int
}

type sentMessage struct{ chatID, text, agentID string }

func newFakeChats() *fakeChats { return &fakeChats{byKey: map[string]string{}} }

func (f *fakeChats) key(provider, chatID string) string { return provider + "|" + chatID }

func (f *fakeChats) GetByChannel(_ context.Context, provider, chatID string) (bot.ChatRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.byKey[f.key(provider, chatID)]
	if !ok {
		return bot.ChatRef{}, apperr.New("CHAT_CHANNEL_NOT_FOUND")
	}
	return bot.ChatRef{ID: id}, nil
}

func (f *fakeChats) CreateForChannel(_ context.Context, provider, chatID, _, _ string) (bot.ChatRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	id := "c-" + string(rune('0'+f.nextID))
	f.byKey[f.key(provider, chatID)] = id
	return bot.ChatRef{ID: id}, nil
}

func (f *fakeChats) Send(_ context.Context, chatID, text, agentID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentMessage{chatID: chatID, text: text, agentID: agentID})
	return nil
}

// fakePublicURL controls whether the tunnel is "up" for a test.
type fakePublicURL struct {
	url string
	up  bool
}

func (f fakePublicURL) URL(context.Context) (string, bool) { return f.url, f.up }

// fakeEnv resolves ${env.*} placeholders from a map, the same shape
// EnvResolver's real implementation reads the process environment through.
type fakeEnv map[string]string

func (f fakeEnv) String(key, def string) string {
	if v, ok := f[key]; ok {
		return v
	}
	return def
}

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

type fakeLogger struct {
	mu    sync.Mutex
	warns []string
}

func (l *fakeLogger) Warn(msg string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warns = append(l.warns, msg)
}
func (l *fakeLogger) Error(string, ...any) {}

var refTime = time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

func newRegistry(provider *fakeProvider, chats *fakeChats, up fakePublicURL, env fakeEnv, log *fakeLogger) *bot.Registry {
	return bot.NewRegistry(bot.Deps{
		Providers: map[string]bot.Provider{"telegram": provider},
		Chats:     chats,
		PublicURL: up,
		Env:       env,
		Clock:     fixedClock{at: refTime},
		Log:       log,
	})
}

func TestRegisterAllProducesAURLContainingThePublicHostnameOnceTheTunnelIsUp(t *testing.T) {
	provider := &fakeProvider{}
	reg := newRegistry(provider, newFakeChats(), fakePublicURL{url: "https://my-tunnel.example.com", up: true}, fakeEnv{}, &fakeLogger{})

	out := reg.RegisterAll(context.Background(), []bot.AgentChannel{
		{AgentID: "atlas", WorkspaceID: "ws", Provider: "telegram", Data: map[string]any{"token": "tok"}},
	})
	if len(out) != 1 {
		t.Fatalf("got %d registrations, want 1", len(out))
	}
	if out[0].Status != bot.Registered {
		t.Fatalf("status = %q, want registered: %+v", out[0].Status, out[0])
	}
	if got := out[0].WebhookURL; got == "" || !strings.Contains(got, "my-tunnel.example.com") {
		t.Fatalf("WebhookURL = %q, want it to contain the public hostname", got)
	}
}

func TestRegisterAllDefersWithoutATunnel(t *testing.T) {
	provider := &fakeProvider{}
	reg := newRegistry(provider, newFakeChats(), fakePublicURL{up: false}, fakeEnv{}, &fakeLogger{})

	out := reg.RegisterAll(context.Background(), []bot.AgentChannel{
		{AgentID: "atlas", Provider: "telegram", Data: map[string]any{"token": "tok"}},
	})
	if len(out) != 1 {
		t.Fatalf("got %d registrations, want 1", len(out))
	}
	if out[0].Status != bot.Pending {
		t.Fatalf("status = %q, want pending", out[0].Status)
	}
	if out[0].Error == "" {
		t.Fatal("Pending with no reason given — the design doc asks for an explicit deferral, not a silent one")
	}
	if len(provider.registered) != 0 {
		t.Fatal("RegisterWebhook was called despite no tunnel being up")
	}
}

func TestRegisterAllInterpolatesAnEnvToken(t *testing.T) {
	provider := &fakeProvider{}
	env := fakeEnv{"TELEGRAM_TOKEN": "real-token-value"}
	log := &fakeLogger{}
	reg := newRegistry(provider, newFakeChats(), fakePublicURL{url: "https://h.example.com", up: true}, env, log)

	out := reg.RegisterAll(context.Background(), []bot.AgentChannel{
		{AgentID: "atlas", Provider: "telegram", Data: map[string]any{"token": "${env.TELEGRAM_TOKEN}"}},
	})
	if out[0].Status != bot.Registered {
		t.Fatalf("status = %q, want registered: %+v", out[0].Status, out[0])
	}
	if len(log.warns) != 0 {
		t.Fatalf("a ${env.*} token warned, want silence: %v", log.warns)
	}
}

func TestRegisterAllWarnsOnALiteralToken(t *testing.T) {
	provider := &fakeProvider{}
	log := &fakeLogger{}
	reg := newRegistry(provider, newFakeChats(), fakePublicURL{url: "https://h.example.com", up: true}, fakeEnv{}, log)

	out := reg.RegisterAll(context.Background(), []bot.AgentChannel{
		{AgentID: "atlas", Provider: "telegram", Data: map[string]any{"token": "literal-secret-in-frontmatter"}},
	})
	if out[0].Status != bot.Registered {
		t.Fatalf("status = %q, want registered: %+v", out[0].Status, out[0])
	}
	if len(log.warns) != 1 {
		t.Fatalf("got %d warnings, want 1 for a literal token", len(log.warns))
	}
}

func TestHandleWebhookResolvesToTheDeterministicChat(t *testing.T) {
	provider := &fakeProvider{parseResult: bot.Inbound{ChatID: "12345", UserID: "u1", Text: "hi"}}
	chats := newFakeChats()
	reg := newRegistry(provider, chats, fakePublicURL{url: "https://h.example.com", up: true}, fakeEnv{}, &fakeLogger{})
	reg.RegisterAll(context.Background(), []bot.AgentChannel{
		{AgentID: "atlas", Provider: "telegram", Data: map[string]any{"token": "tok"}},
	})

	if err := reg.HandleWebhook(context.Background(), "telegram", "atlas", "", nil); err != nil {
		t.Fatal(err)
	}
	if len(chats.sent) != 1 || chats.sent[0].text != "hi" || chats.sent[0].agentID != "atlas" {
		t.Fatalf("chats.sent = %+v", chats.sent)
	}

	// A second inbound message from the same Telegram chat must land in the
	// same conversation, not open a second one.
	if err := reg.HandleWebhook(context.Background(), "telegram", "atlas", "", nil); err != nil {
		t.Fatal(err)
	}
	if len(chats.sent) != 2 || chats.sent[0].chatID != chats.sent[1].chatID {
		t.Fatalf("two inbound messages from the same Telegram chat resolved to different conversations: %+v", chats.sent)
	}
}

func TestRegisterAllFailsCleanlyOnAnUnconfiguredProvider(t *testing.T) {
	reg := newRegistry(&fakeProvider{}, newFakeChats(), fakePublicURL{url: "https://h.example.com", up: true}, fakeEnv{}, &fakeLogger{})

	out := reg.RegisterAll(context.Background(), []bot.AgentChannel{
		{AgentID: "atlas", Provider: "whatsapp", Data: map[string]any{"token": "tok"}},
	})
	if out[0].Status != bot.Failed {
		t.Fatalf("status = %q, want failed for an unconfigured provider", out[0].Status)
	}
}

func TestRegisterAllFailsCleanlyOnAMissingEnvVariable(t *testing.T) {
	reg := newRegistry(&fakeProvider{}, newFakeChats(), fakePublicURL{url: "https://h.example.com", up: true}, fakeEnv{}, &fakeLogger{})

	out := reg.RegisterAll(context.Background(), []bot.AgentChannel{
		{AgentID: "atlas", Provider: "telegram", Data: map[string]any{"token": "${env.NOT_SET}"}},
	})
	if out[0].Status != bot.Failed || out[0].Error == "" {
		t.Fatalf("got %+v, want failed with a reason naming the missing variable", out[0])
	}
}

func TestDeliverToAnUnregisteredAgentIsRefused(t *testing.T) {
	reg := newRegistry(&fakeProvider{}, newFakeChats(), fakePublicURL{up: false}, fakeEnv{}, &fakeLogger{})
	err := reg.Deliver(context.Background(), "telegram", "nobody", "chat-1", "hi")
	got, ok := apperr.As(err)
	if !ok || got.Code != apperr.New("BOT_REGISTRATION_NOT_FOUND").Code {
		t.Fatalf("want BOT_REGISTRATION_NOT_FOUND, got %v", err)
	}
}

func TestHandleWebhookOfAnUnregisteredAgentIsRefused(t *testing.T) {
	provider := &fakeProvider{}
	reg := newRegistry(provider, newFakeChats(), fakePublicURL{up: false}, fakeEnv{}, &fakeLogger{})

	err := reg.HandleWebhook(context.Background(), "telegram", "nobody", "", nil)
	got, ok := apperr.As(err)
	if !ok || got.Code != apperr.New("BOT_REGISTRATION_NOT_FOUND").Code {
		t.Fatalf("want BOT_REGISTRATION_NOT_FOUND, got %v", err)
	}
}

func TestDeliverAppliesAPerChatRateLimit(t *testing.T) {
	provider := &fakeProvider{}
	reg := bot.NewRegistry(bot.Deps{
		Providers: map[string]bot.Provider{"telegram": provider},
		Chats:     newFakeChats(),
		PublicURL: fakePublicURL{url: "https://h.example.com", up: true},
		Env:       fakeEnv{},
		Clock:     fixedClock{at: refTime},
		Log:       &fakeLogger{},
		RateLimit: 2, RateWindow: time.Minute,
	})
	reg.RegisterAll(context.Background(), []bot.AgentChannel{
		{AgentID: "atlas", Provider: "telegram", Data: map[string]any{"token": "tok"}},
	})

	ctx := context.Background()
	if err := reg.Deliver(ctx, "telegram", "atlas", "chat-1", "one"); err != nil {
		t.Fatal(err)
	}
	if err := reg.Deliver(ctx, "telegram", "atlas", "chat-1", "two"); err != nil {
		t.Fatal(err)
	}
	err := reg.Deliver(ctx, "telegram", "atlas", "chat-1", "three")
	got, ok := apperr.As(err)
	if !ok || got.Code != apperr.New("BOT_RATE_LIMITED").Code {
		t.Fatalf("a third Deliver within the window should have been rate-limited, got %v", err)
	}
	if len(provider.sent) != 2 {
		t.Fatalf("provider.sent = %d, want exactly 2 to have gone through", len(provider.sent))
	}
}
