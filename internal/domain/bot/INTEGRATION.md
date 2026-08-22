# Integrating `bot`

**Done**, all four sections below — `wire.go` builds `botRegistry` before
`session.New`, `httpapi.Config.Bot` mounts the webhook route, `config.Service`
needed nothing extra, and §3's outbound loop is closed: `session.Runner`
calls `Bots.Deliver` once a channel-bound conversation's turn is persisted
(`internal/runtime/session.deliverToChannel`) — the "Narrower" option this
note itself offered, not the generic-event one; see §3's own note for why.

Not wired into `internal/app/wire.go`, `internal/transport/httpapi`, or
`frontend/src/lib/command-map.ts` by this branch — every domain this Phase 8
round left those to a single central pass, to avoid an N-way merge conflict.
This is that pass's checklist for `bot`.

`bot` has no CLI/MCP surface at all (see `docs/04 - Domínio/Bot (Go).md`'s
own "Continua sem CLI nem MCP" — configuring an external channel is a human
action, not an agent capability), so there is genuinely nothing to add to
`command-map.ts`. Confirmed, not just assumed: `bot`'s package has no
`commands.go` and registers nothing on `*command.Registry`.

## 1. `internal/app/wire.go`

Import:

```go
"github.com/OWNER/aos/internal/adapters/telegramapi"
"github.com/OWNER/aos/internal/domain/bot"
```

Must be constructed **after** `configSvc`, `chatSvc`, `agentSvc` and
`tunnelSvc` all exist — it depends on all four. Near where `tunnelSvc` is
built:

```go
botRegistry := bot.NewRegistry(bot.Deps{
	Providers: map[string]bot.Provider{"telegram": telegramapi.New()},
	Chats:     chatsForBot{svc: chatSvc},
	PublicURL: tunnelPublicURL{svc: tunnelSvc},
	Env:       resolver, // env.Resolver already satisfies bot.EnvResolver — both are { String(key, def string) string }
	Clock:     clock,
	Log:       logger,
})
```

Two small adapters, in `ecosystem.go` next to `tunnelConfig`/`goalTasksAdapter`:

```go
// chatsForBot adapts chat.Service to bot.Chats.
type chatsForBot struct{ svc *chat.Service }

func (c chatsForBot) GetByChannel(ctx context.Context, provider, chatID string) (bot.ChatRef, error) {
	got, err := c.svc.GetByChannel(ctx, provider, chatID)
	if err != nil {
		return bot.ChatRef{}, err
	}
	return bot.ChatRef{ID: got.ID}, nil
}

func (c chatsForBot) CreateForChannel(ctx context.Context, provider, chatID, agentID, title string) (bot.ChatRef, error) {
	got, err := c.svc.Create(ctx, chat.CreateInput{
		Title: title, Kind: chat.KindExternal, Agent: agentID,
		Channel: &chat.ChannelMeta{Provider: provider, ChatID: chatID},
	})
	if err != nil {
		return bot.ChatRef{}, err
	}
	return bot.ChatRef{ID: got.ID}, nil
}

func (c chatsForBot) Send(ctx context.Context, chatID, text, agentID string) error {
	_, err := c.svc.Send(ctx, chat.SendInput{Chat: chatID, Text: text, Agent: agentID})
	return err
}

// tunnelPublicURL adapts tunnel.Service to bot.PublicURL.
type tunnelPublicURL struct{ svc tunnel.Service }

func (t tunnelPublicURL) URL(ctx context.Context) (string, bool) {
	state, err := t.svc.Status(ctx)
	if err != nil || state.Status != tunnel.Running || state.URL == "" {
		return "", false
	}
	return state.URL, true
}
```

**Boot-time registration**, after `botRegistry` is built and after every
agent with channels can be read (i.e. after `agentSvc` is usable) — collect
every agent's `Channels` into `[]bot.AgentChannel` and call `RegisterAll`
once. `agent.Agent.Channels` is `[]agent.Channel{Provider, Data any}`;
`Data` needs a type assertion to `map[string]any` (it decodes off YAML
frontmatter as one already, the same shape `toolset`'s own frontmatter
fields do):

```go
var channels []bot.AgentChannel
if agents, err := agentSvc.List(context.Background(), agent.ListInput{}); err == nil {
	for _, a := range agents {
		for _, ch := range a.Channels {
			data, _ := ch.Data.(map[string]any)
			channels = append(channels, bot.AgentChannel{
				AgentID: a.ID, WorkspaceID: active, Provider: ch.Provider, Data: data,
			})
		}
	}
}
botRegistry.RegisterAll(context.Background(), channels)
```

This has to run **after the tunnel is up**, per the design doc's `tunnel ->
bots` boot order — `PublicURL.URL` reports `false` until it is, so calling
`RegisterAll` too early is safe (everything lands `Pending`) but pointless.
The cleanest place is wherever `Serve` already starts the daemon's
background pieces (the watcher, the worker) — call it there, not in `New`,
for the same one-shot-CLI-must-not-start-background-work reason the watcher
and the worker are already split that way.

**`App` struct field:**

```go
Bots *bot.Registry
```

...and in `return &App{...}`: `Bots: botRegistry,`.

## 2. `internal/transport/httpapi` — the webhook route

Add a `Bot http.Handler` field to `httpapi.Config` (same shape as the
existing `Files`/`AuthRoutes` fields), mounted **outside** the authenticated
`guarded` group — a Telegram server has no session cookie or API token for
this system; it proves itself with its own webhook secret, exactly like
`AuthRoutes` is mounted outside auth because a request has to reach login
before it can hold a credential:

```go
if cfg.Bot != nil {
	api.Mount("/bot", cfg.Bot)
}
```

The handler itself (new file, e.g. `internal/transport/httpapi/botapi.go`,
mirroring `fileapi`'s or `authapi`'s own small router-in-a-package shape)
needs one route:

```
POST /api/bot/{provider}/webhook/{agentID}
```

reading `chi.URLParam(r, "provider")` / `"agentID"`, the
`telegramapi.SecretHeader` header, and the raw body, then calling:

```go
body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
err := app.Bots.HandleWebhook(r.Context(), provider, agentID, r.Header.Get(telegramapi.SecretHeader), body)
```

Respond `200` regardless of `err` being non-nil for anything other than a
malformed request — Telegram retries (and eventually gives up on) a webhook
that doesn't answer 200, and a registration-not-found or a downstream
dispatch failure is not something re-delivery fixes. Log the error; don't
surface it to Telegram as a reason to keep retrying.

## 3. Outbound delivery — closed

`Registry.Deliver(ctx, provider, agentID, chatID, text)` sends a reply back
out, rate-limited. Closed the **narrower** of the two ways this note used to
offer: `session.Runner` — which already holds the conversation it fetched at
the top of `Run` and the turn's own final text once the loop returns — calls
`Bots.Deliver` itself, right after `persist` writes the answer to the chat
record and before the (also fire-and-forget) subconscious observation:

```go
// internal/runtime/session/session.go

type Bots interface {
	Deliver(ctx context.Context, provider, agentID, chatID, text string) error
}

func (r *Runner) deliverToChannel(ctx context.Context, conversation *chat.Chat, agentID string, result *agentloop.Result) {
	if r.deps.Bots == nil || conversation.Channel == nil || result.Text == "" {
		return
	}
	if err := r.deps.Bots.Deliver(ctx, conversation.Channel.Provider, agentID, conversation.Channel.ChatID, result.Text); err != nil {
		r.log.Warn("could not deliver the turn's answer to its external channel", ...)
	}
}
```

Chosen over the "cheapest" (generic `Publisher` on `chat.Service`) option
because `session.Runner` already had every piece this needs — the
conversation, its `Channel`, and the finished turn's text — at exactly the
point the answer becomes durable, with nothing new to plumb through
`chat.Service`. A delivery failure is logged, not propagated: `persist`
already succeeded, so the turn itself did not fail, only reaching Telegram
did. `wire.go` passes `Bots: botRegistry` into `session.Deps` — `botRegistry`
is already built earlier in `New`, so no reordering was needed. Tested at
`internal/runtime/session/session_test.go`'s `TestDeliverToChannel*` battery
against a fake `Bots`, the same level the file's own doc comment says this
kind of logic (not wiring) belongs at.

## 4. `config.Service` — nothing needed

`internal/domain/config`'s `Security`/`Tunnel` sections already exist
(confirmed by `tunnel`'s own INTEGRATION.md, landed earlier this round). Bot
reads no config section of its own — every setting it needs comes from
`agent.Agent.Channels`, which already exists.
