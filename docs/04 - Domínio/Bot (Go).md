---
tags: [dominio, bot, telegram, canais]
aliases: [Bot Go, Canais]
fase: 8
status: pronto
origem: "[[Bot]]"
---

# Bot (Go)

> Pai: [[Agent (Go)]] · Origem no original: [[Bot]] · [[Bots Registry]] · Ver: [[Tunnel (Go)]] · Fase: 8

## Objetivo

Canais de mensageria externos. Um agente ganha presença fora da UI — hoje, Telegram.

## Estado atual

A entrega estava só de ida: webhook recebido, mensagem vira um `chat.Send`,
o agente responde — mas nada levava a resposta de volta ao Telegram.
`Registry.Deliver` já existia, com rate limit, mas ninguém o chamava (ver
`internal/domain/bot/INTEGRATION.md`, seção 3). `session.Runner.Run` agora
chama `Bots.Deliver` assim que `persist` grava a resposta, quando a conversa
tem `Channel` e o turno produziu texto — a mesma conversa do Telegram e da UI
volta a ser, de fato, o mesmo objeto nas duas direções.

## Comportamento do original

Um bot por agente, declarado no frontmatter do [[Agent (Go)]] ([[Bots Registry]]):

```yaml
channels:
  - provider: telegram
    data: { token: "..." }
```

A propriedade mais interessante é o mapeamento de identidade:

```
chatId = `${provider}-${channel.id}`
```

Uma conversa do Telegram vira um [[Chat (Go)]] com ID determinístico — **a conversa no Telegram e a conversa na UI são o mesmo objeto**, com o mesmo histórico e a mesma memória.

Dependência estrutural: a URL de webhook é construída a partir do [[Tunnel (Go)]]. Sem tunnel ativo, o Telegram não tem para onde entregar. Por isso a ordem de boot é `tunnel → bots`.

A feature não tem CLI nem MCP.

## Design em Go

```go
// internal/domain/bot/entity.go

type Registration struct {
	Provider      string    `json:"provider"`
	WorkspaceID   string    `json:"workspaceId"`
	AgentID       string    `json:"agentId"`
	WebhookURL    string    `json:"webhookUrl"`
	WebhookSecret string    `json:"-" secret:"true"`
	Status        Status    `json:"status"`
	RegisteredAt  time.Time `json:"registeredAt"`
}
```

```go
// internal/domain/bot/port.go

// Provider is the strategy for one messaging platform. Telegram is the only
// implementation today; the interface exists because the original's schema
// already anticipates others and because it makes the registry testable.
type Provider interface {
	Name() string
	RegisterWebhook(ctx context.Context, url, secret string, cfg map[string]any) error
	UnregisterWebhook(ctx context.Context, cfg map[string]any) error

	// Parse converts an inbound update into a normalized inbound message,
	// verifying the webhook secret first.
	Parse(ctx context.Context, r *http.Request, secret string) (Inbound, error)

	Send(ctx context.Context, out Outbound) error
	SetTyping(ctx context.Context, chatID string, on bool) error
}
```

```go
// internal/domain/bot/registry.go

// Registry holds one registration per agent with channels. It is rebuilt at
// boot, after the tunnel is up, and updated when an agent's channels change.
type Registry struct{ /* ... */ }

// resolveChat maps an external conversation to a Chat with a deterministic id,
// so the Telegram thread and the in-app thread are literally the same object.
func resolveChat(provider, channelID string) string {
	return provider + "-" + channelID
}
```

### Formatação de saída

O [[Prompt Assembly]] injeta a seção de formatação Telegram quando o chat está vinculado, com os limites reais: **32.768 caracteres e 500 blocos por mensagem**. O `Send` divide mensagens longas nesses limites, quebrando em fronteira de bloco, nunca no meio de um code fence.

## Decisões e divergências

> [!decision] Token do bot fora do arquivo versionado
> No original, o token do Telegram fica em `channels[].data.token` no `agent.md` — que é versionado em Git. Aqui, o frontmatter aceita `${env.TELEGRAM_TOKEN}` e a interpolação acontece no registro, como nos [[Toolset (Go)]]. Um token literal gera aviso no boot.

> [!decision] Verificação de segredo do webhook obrigatória
> O original gera um `webhookSecret` por registro. Nós **verificamos** em toda requisição de entrada, com comparação em tempo constante — sem isso, o endpoint aceita qualquer POST.

> [!decision] Continua sem CLI nem MCP
> Configurar canal externo é ação de configuração humana, não capacidade do agente.

> [!decision] Rate limit de saída
> Adição. O Telegram impõe limites; estourá-los bloqueia o bot. Um limitador por chat evita que um agente verboso derrube o próprio canal.

## Testes

- Registro depois do tunnel: URL contém o hostname público
- Sem tunnel, o registro é adiado com motivo explícito, não falha silenciosa
- Webhook com segredo errado é rejeitado
- Mensagem de entrada resolve para o chat determinístico
- Mensagem de 40.000 caracteres é dividida em fronteira de bloco
- Token literal no frontmatter gera aviso; `${env.*}` interpola
- Rate limit por chat aplicado

## Critério de pronto

- [x] Conversa de Telegram e conversa na UI são o mesmo chat — `TestHandleWebhookResolvesToTheDeterministicChat` para a ida; `TestDeliverToChannelPushesTheAnswerOut` (`internal/runtime/session`) para a volta
- [x] Webhook verificado por segredo — `TestParseRejectsAWrongWebhookSecret`, `TestParseAcceptsTheRightSecretAndDecodesTheMessage`
- [x] Token via variável de ambiente — `TestRegisterAllInterpolatesAnEnvToken`, `TestRegisterAllWarnsOnALiteralToken`
- [x] Divisão de mensagens longas respeitando os limites da plataforma — `TestSplitOfA40000CharacterMessage`, `TestSendSplitsALongMessageAcrossMultipleCalls`
