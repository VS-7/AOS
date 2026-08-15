---
tags: [dominio, chat, conversa]
aliases: [Chat Go, Conversa]
fase: 3
status: especificado
origem: "[[Chat]]"
---

# Chat (Go)

> Pai: [[Workspace (Go)]] · Origem no original: [[Chat]] · Fase: 3

## Objetivo

Conversas persistentes. O prompt-mestre as define como *"the front door to execution, not a dead-end text box"*.

## Comportamento do original

Única entidade principal em **JSON**, não Markdown ([[Chat]]). O comentário do fonte explica: payloads de mensagem do SDK de IA usam tipos que não serializam para JSON Schema, então o schema persistido relaxa para `messages: z.array(z.any())` enquanto o schema de domínio mantém tipagem forte.

Dois padrões de caminho:
```
.fractal/chats/{id}.chat.json
.fractal/tasks/{task}/runs/{id}.run.json
```

O segundo revela algo importante: **execuções de task são chats** — mesma estrutura, local diferente.

Pontos notáveis: `participants` com papéis (chats são multi-agente por desenho), `runs` por mensagem guardando status, uso de tokens e erro (auditoria de custo no nível da mensagem), `reactions`, e `channel` ligando o chat a um canal externo.

Chats sem menção explícita caem no agente com `orchestrator: true`. Em chats compartilhados, o prompt instrui a usar `@[agent-id]`.

## Design em Go

```go
// internal/domain/chat/entity.go

type Chat struct {
	ID    string `json:"id" collection:"path"`
	Title string `json:"title"`

	Participants []Participant `json:"participants"`
	Messages     []Message     `json:"messages"`

	Provider    string          `json:"provider,omitempty"`
	Channel     *ChannelMeta    `json:"channel,omitempty"` // link to Telegram etc.
	Attachments []Attachment    `json:"attachments,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Participant struct {
	Type string `json:"type"` // "agent" | "user"
	ID   string `json:"id"`
	Role string `json:"role"`
}

type Message struct {
	ID        string     `json:"id"`
	Role      string     `json:"role"` // system | user | assistant | tool
	Parts     []Part     `json:"parts"`
	Reactions []Reaction `json:"reactions,omitempty"`

	// Runs records every execution attempt for this message: status, token
	// usage and error. It is cost and failure auditing at message granularity.
	Runs []Run `json:"runs,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

// Part is a tagged union over the message content kinds. Unlike the original,
// which relaxes the persisted schema to `any`, we keep it typed on both sides:
// Go's encoding/json handles discriminated unions with a custom Unmarshaler,
// so there is no reason to lose the type on disk.
type Part struct {
	Type      string          `json:"type"` // text | reasoning | tool-call | tool-result | file
	Text      string          `json:"text,omitempty"`
	ToolName  string          `json:"toolName,omitempty"`
	ToolCallID string         `json:"toolCallId,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Output    json.RawMessage `json:"output,omitempty"`
	MediaType string          `json:"mediaType,omitempty"`
	URI       string          `json:"uri,omitempty"`
}

type Run struct {
	Status string     `json:"status"`
	Usage  TokenUsage `json:"usage"`
	Error  *RunError  `json:"error,omitempty"`
}

type TokenUsage struct {
	Input, Output, Reasoning, Cached, Total int
	CostUSD float64 `json:"costUsd,omitempty"`
}
```

```go
// internal/domain/chat/service.go

type Service interface {
	List(ctx context.Context, q Query) ([]Chat, error)
	Get(ctx context.Context, id string) (*Chat, error)
	Create(ctx context.Context, in CreateInput) (*Chat, error)

	// Send appends a user message and dispatches the agent turn. It returns as
	// soon as the message is persisted; the turn runs on the chat queue and
	// streams over the realtime channel.
	Send(ctx context.Context, in SendInput) (*Message, error)
}
```

### Resolução de destinatário

```go
// resolveTarget mirrors the original's routing: an explicit @[agent-id] mention
// wins; otherwise the workspace orchestrator answers. In a direct message with
// exactly one agent participant, that agent answers.
func resolveTarget(c *Chat, text string, orchestrator string) string
```

### Execução assíncrona

`Send` enfileira na fila `chat` ([[ADR-0008 SQLite puro Go para filas]]). O progresso vai por [[Realtime WebSocket]]. O `Run` da mensagem registra status, tokens e erro — o que dá auditoria de custo por mensagem, como no original.

## Decisões e divergências

> [!decision] Mensagens tipadas também no disco
> O original relaxa o schema persistido para `any` por limitação do ecossistema TypeScript/Zod. Em Go não há essa limitação: um `UnmarshalJSON` customizado no `Part` mantém o tipo dos dois lados. Ganha-se validação na leitura e migração possível.

> [!decision] JSON, não Markdown
> Mantido. Uma conversa não tem "corpo Markdown com frontmatter" — a estrutura é a mensagem. Ver [[ADR-0004 Collections em Markdown]].

> [!decision] Custo em USD no `Run`
> Adição. O original registra tokens; sem custo, o usuário não vê o que está gastando. O cálculo usa uma tabela de preços por modelo, versionada e atualizável.

## Testes

- Round-trip de chat com todos os tipos de `Part`, inclusive tool-call com input/output brutos
- Padrão de run: chat sob `.aos/tasks/{t}/runs/{id}.run.json` é lido pela mesma coleção
- Roteamento: menção explícita ganha do orquestrador; DM com um agente vai para ele
- `Send` persiste antes de enfileirar (mensagem sobrevive a crash do worker)
- `Run` acumula uso de tokens de todas as tentativas
- Mensagem com anexo de imagem preserva `mediaType` e `uri`

## Critério de pronto

- [ ] Conversa real persistida, com histórico recuperável
- [ ] Execução assíncrona com streaming por WebSocket
- [ ] Auditoria de tokens e custo por mensagem
- [ ] Roteamento multi-agente com `@[agent-id]`
