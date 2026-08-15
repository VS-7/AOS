---
tags: [dominio, chat, conversa]
aliases: [Chat Go, Conversa]
fase: 3
status: em-construcao
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
> Adição. O original registra tokens; sem custo, o usuário não vê o que está gastando. O campo existe desde a Fase 3; a tabela de preços por modelo que o preenche entra com os providers, na **Fase 5**.

> [!decision] União discriminada plana, sem `UnmarshalJSON` customizado
> A nota previa um unmarshaler próprio no `Part`. Uma struct plana com discriminador `type` e campos opcionais mantém o tipo dos dois lados e faz round-trip sem código de decodificação — o mesmo resultado com menos superfície. Travado por `TestEveryPartTypeRoundTrips`.

> [!decision] `dispatched: false` é estado, não erro
> `Send` persiste antes de despachar. Se o despacho falha, ou se não há agente para responder, a mensagem continua gravada e o resultado diz que nada foi enfileirado. Um chamador que assumisse o contrário esperaria por uma resposta que não vem.

> [!decision] O roteamento reporta o porquê
> `Target` traz `reason`: `mention`, `participant` ou `orchestrator`. Uma resposta inesperada precisa ser rastreável até a regra que a escolheu, e as três são tipos diferentes de evidência — instrução, inferência e default.

## Testes

- Round-trip de chat com todos os tipos de `Part`, inclusive tool-call com input/output brutos
- Padrão de run: chat sob `.aos/tasks/{t}/runs/{id}.run.json` é lido pela mesma coleção
- Roteamento: menção explícita ganha do orquestrador; DM com um agente vai para ele
- `Send` persiste antes de enfileirar (mensagem sobrevive a crash do worker)
- `Run` acumula uso de tokens de todas as tentativas
- Mensagem com anexo de imagem preserva `mediaType` e `uri`

## Critério de pronto

- [x] Conversa real persistida, com histórico recuperável — `TestSendPersistsBeforeItDispatches`, `TestListLeavesTheTranscriptsOut`
- [ ] Execução assíncrona com streaming por WebSocket — **Fases 4 e 5**
- [x] Estrutura de auditoria por mensagem — `TestTotalUsageSumsEveryAttempt`. Os valores vêm dos providers, na Fase 5
- [x] Roteamento multi-agente com `@[agent-id]` — `TestRoutingPrefersAnExplicitMention`

## Saída dos testes — Fase 3

```
$ go test -race ./internal/domain/chat/
ok  	github.com/OWNER/aos/internal/domain/chat
```

| Caso da nota | Teste |
|---|---|
| Round-trip com todos os tipos de `Part` | `TestEveryPartTypeRoundTrips` |
| Roteamento: menção ganha; DM com um agente vai para ele | `TestRoutingPrefersAnExplicitMention` |
| `Send` persiste antes de enfileirar | `TestSendPersistsBeforeItDispatches` |
| `Run` acumula uso de tokens de todas as tentativas | `TestTotalUsageSumsEveryAttempt` |
| Anexo preserva `mediaType` e `uri` | `TestEveryPartTypeRoundTrips` |

**Não verificado nesta fase:** o padrão de run sob `.aos/tasks/{t}/runs/{id}.run.json` — a coleção `runs` existe no registry desde a Fase 1, mas nada a escreve até a Fase 6. O `Dispatcher` é um porto sem implementação: a raiz de composição passa `nil`, e o resultado já reporta `dispatched: false`. A fila e o streaming chegam nas Fases 4 e 5.

## Adições da Fase 5

O chat ganhou o `Dispatcher` de verdade e o caminho de volta.

`Send` persiste e despacha; o turno roda destacado do request que o pediu, porque uma pessoa fechando a aba não é uma pessoa cancelando o agente. `Reply` é como o runtime escreve a resposta: a mensagem do agente, cada chamada de tool com o que devolveu, e o `Run` registrado na mensagem que pediu — com status, uso e a falha quando houve uma.

`Reply` **não é comando e não está no registry**, deliberadamente. A resposta de um agente é escrita pelo runtime que rodou o turno; uma superfície capaz de forjar uma faria do transcript um registro sem valor.

Um turno que falha aparece na conversa, não só no log: `TestATurnThatFailsIsVisibleInTheConversation`.
