---
tags: [dominio, event, hooks, auditoria]
aliases: [Event Hooks Go, Hooks, Eventos]
fase: 5
status: pronto
origem: "[[Eventos e Hooks]]"
---

# Event Hooks (Go)

> Pai: [[Agent Loop]] · Origem no original: [[Eventos e Hooks]] · [[Event (Hooks)]] · Fase: 5

## Objetivo

Sistema de hooks **compatível com o contrato do Claude Code**, permitindo interceptar e modificar o comportamento do agente em nove pontos definidos.

## Comportamento do original

Nove eventos, com capacidades distintas ([[Eventos e Hooks]]):

| Evento | Pode bloquear? | Retorna |
|---|---|---|
| `SessionStart` | não | `additionalContext` |
| `UserPromptSubmit` | **sim** | `additionalContext`, `decision`, `reason` |
| `PreToolUse` | **sim** | `permissionDecision`, `updatedInput`, `additionalContext` |
| `PostToolUse` | **sim** | `decision`, `additionalContext` |
| `PostToolUseFailure` | — | `additionalContext` |
| `SubagentStart` | não | `additionalContext` |
| `SubagentStop` | **sim** | `decision` |
| `Stop` | **sim** | `decision` |
| `PreCompact` | não | `additionalContext` |

Os três superpoderes: injetar contexto, bloquear execução e **reescrever a entrada de uma tool**. A engenharia reversa é explícita sobre o terceiro:

> Um hook pode **reescrever silenciosamente** o que uma tool vai fazer. Serve para sanitizar caminhos, injetar flags obrigatórias ou redirecionar operações — mas também significa que quem controla os hooks controla o comportamento efetivo do agente, independentemente do que o modelo decidiu.

Persistência: log **append-only** em `.fractal/agents/{agent}/events/`, somente leitura via CLI, sem tools MCP. Nada além da execução real produz registros — isso preserva a integridade da auditoria.

Três adaptadores: `claude-code`, `vscode`, nativo. O primeiro permite que hooks escritos para o Claude Code rodem sem modificação.

## Design em Go

```go
// internal/domain/event/entity.go

type Type string

const (
	SessionStart       Type = "SessionStart"
	UserPromptSubmit   Type = "UserPromptSubmit"
	PreToolUse         Type = "PreToolUse"
	PostToolUse        Type = "PostToolUse"
	PostToolUseFailure Type = "PostToolUseFailure"
	SubagentStart      Type = "SubagentStart"
	SubagentStop       Type = "SubagentStop"
	Stop               Type = "Stop"
	PreCompact         Type = "PreCompact"
	Generic            Type = "GenericHook"
)

// Record is the append-only audit entry. It is written exclusively by the
// runtime; there is no create command and no MCP tool, which is what makes
// the log trustworthy.
type Record struct {
	ID        string          `json:"id"`
	Agent     string          `json:"agent"`
	SessionID string          `json:"sessionId"`
	Type      Type            `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Outcome   Outcome         `json:"outcome"`
	Duration  time.Duration   `json:"durationMs"`
	CreatedAt time.Time       `json:"createdAt"`
}

type Outcome struct {
	Decision           string          `json:"decision,omitempty"`           // block | continue
	PermissionDecision string          `json:"permissionDecision,omitempty"` // allow | deny | ask
	Reason             string          `json:"reason,omitempty"`
	AdditionalContext  string          `json:"additionalContext,omitempty"`
	UpdatedInput       json.RawMessage `json:"updatedInput,omitempty"`
	HookID             string          `json:"hookId,omitempty"` // which hook decided
}
```

### Adição da Fase 8: `Deregister`

`Service.Register` já podia ser chamado a qualquer momento (o mutex já
existia), mas nada desfazia um registro. `Service.Deregister(ids ...string)`
fecha isso — remove um handler de todo `event.Type` em que estava registrado,
idempotente para um id nunca registrado. Existe para os hooks de uma skill
([[Skill (Go)]]): `internal/adapters/skillhooks` rastreia quais ids
pertencem a qual skill e chama isto no Uninstall, namespacing cada id como
`{skillID}/{hookID}` para que duas skills com um hook de mesmo nome não
colidam no espaço global de ids do bus.

### O barramento

```go
// internal/core/eventbus/hooks.go

type Handler interface {
	ID() string
	Handles() []event.Type
	Handle(ctx context.Context, e event.Event) (event.Outcome, error)
}

// Emit runs the registered handlers in order, accumulating additional context
// and stopping at the first block. Every invocation is recorded, including
// handlers that did nothing — an audit log with gaps is not an audit log.
func (b *Bus) Emit(ctx context.Context, e event.Event) (event.Outcome, error) {
	var acc event.Outcome
	for _, h := range b.handlersFor(e.Type()) {
		hctx, cancel := context.WithTimeout(ctx, b.hookTimeout)
		out, err := h.Handle(hctx, e)
		cancel()

		b.record(ctx, e, h.ID(), out, err) // always
		if err != nil {
			if b.strict {
				return acc, err
			}
			b.log.Warn("hook failed, continuing", "hook", h.ID(), "err", err)
			continue
		}
		acc.Merge(out)
		if out.Decision == "block" {
			acc.HookID = h.ID()
			return acc, nil
		}
	}
	return acc, nil
}
```

### Log append-only

```go
// internal/adapters/eventlog/log.go

// Writer appends JSONL records under .aos/agents/{agent}/events/{date}.jsonl.
// Append-only by construction: the file is opened with O_APPEND and there is
// no update or delete path anywhere in the codebase.
type Writer struct{ /* ... */ }

func (w *Writer) Append(ctx context.Context, r event.Record) error
```

Rotação diária por arquivo, retenção configurável, e um job de poda que remove arquivos antigos — não registros individuais.

### Adaptadores

```go
// internal/domain/event/adapter/claudecode/adapter.go

// Adapter translates between the Claude Code hook contract and ours. Because
// we adopted the same event names and payload shape (ADR-0016), the translation
// is thin — which is the point: hooks written for one tool run on the other.
func Translate(in ClaudeCodeHookInput) (event.Event, error)
func Render(out event.Outcome) ClaudeCodeHookOutput
```

## Decisões e divergências

> [!decision] `ask` chega ao humano
> A mudança mais relevante. Ver [[ADR-0007 Canal real de aprovação de tool]] e [[Agent Loop]].

> [!decision] Timeout por hook
> Adição. Um hook lento ou travado não pode congelar o turno. Estouro de timeout é registrado e tratado como "continuar" — exceto em modo `strict`, configurável por workspace.

> [!decision] Toda invocação é registrada, inclusive as que nada fizeram
> O original registra o evento. Registramos também qual handler decidiu o quê. Sem isso, "por que essa tool foi bloqueada?" não tem resposta quando há várias skills instaladas.

> [!decision] Reescrita de payload é auditada em detalhe
> `updatedInput` fica no registro, com o `hookId`. Dado o poder desse mecanismo, invisibilidade seria inaceitável — especialmente com skills de terceiros ([[ADR-0015 Skills com permissões declaradas]]).

> [!decision] Continua sem escrita via CLI ou MCP
> Herdado. A integridade da auditoria depende de nada além da execução real produzir registros.

## Testes

- Os nove tipos disparam nos pontos corretos do [[Agent Loop]]
- `UserPromptSubmit` com `block` aborta o turno; nenhum passo seguinte roda
- `PreToolUse` com `updatedInput` altera o payload; registro guarda o antes e o depois
- Dois hooks: o primeiro injeta contexto, o segundo bloqueia; o contexto do primeiro é preservado
- Hook que estoura timeout é registrado e o turno continua
- Modo `strict` propaga a falha
- Log é append-only: nenhuma API de update ou delete existe (teste sobre o registry de comandos)
- Adaptador Claude Code: hook real do formato deles roda sem modificação
- Nenhuma tool MCP de events registrada

## Critério de pronto

- [x] Nove eventos implementados com as capacidades corretas — `TestTheNineEventsDeclareWhatTheyCanDo`
- [x] `ask` abrindo aprovação real — `TestAskReachesAHumanAndTheAnswerComesBack`
- [x] Log append-only com rotação e retenção — `TestASecondWriteAppendsRatherThanReplaces`
- [x] Adaptador Claude Code rodando um hook real — `TestAHookWrittenForClaudeCodeRunsUnchanged`
- [x] Auditoria identificando qual hook decidiu — `TestTheRewrittenPayloadIsInTheRecord`

## Saída dos testes — Fase 5

```
$ go test -race ./internal/domain/event/ ./internal/adapters/eventlog/ ./internal/adapters/hookexec/
ok  	github.com/OWNER/aos/internal/domain/event
ok  	github.com/OWNER/aos/internal/adapters/eventlog
ok  	github.com/OWNER/aos/internal/adapters/hookexec
```

| Caso da nota | Teste |
|---|---|
| Os nove tipos nos pontos corretos do loop | `TestTheNineEventsFireWhereTheySay` |
| `UserPromptSubmit` com `block` aborta o turno | `TestABlockingPromptHookEndsTheTurnBeforeATokenIsSpent` |
| `updatedInput` altera o payload; o registro guarda antes e depois | `TestTheRewrittenPayloadIsInTheRecord` |
| Dois hooks: o primeiro injeta contexto, o segundo bloqueia | `TestContextAccumulatesAndTheFirstBlockWins` |
| Hook que estoura timeout é registrado e o turno continua | `TestASlowHookIsBoundedByTheTimeout` |
| Modo `strict` propaga a falha | `TestAFailingHookIsRecordedAndTheTurnContinues/strict` |
| Log append-only, sem API de update | `TestASecondWriteAppendsRatherThanReplaces` |
| Hook real do formato Claude Code | `TestAHookWrittenForClaudeCodeRunsUnchanged` |
| Nenhuma tool de events registrada | Nenhum comando `events_*` existe; a superfície de leitura está pendente |

**Divergência estrutural:** o barramento vive em `internal/domain/event`, não em `internal/core/eventbus`. Escrito em termos de `Event` e `Outcome`, em core ele faria core importar o domínio — a seta que a regra de dependência existe para manter apontando ao contrário.

**Adições, cada uma com teste.** Um hook roda dentro da fronteira de pânico: código de terceiro no caminho mais quente do sistema não derruba o turno (`TestAPanickingHookCostsOnlyItsOwnOpinion`). Uma decisão que o evento não pode carregar é descartada e registrada, em vez de obedecida — `SessionStart` não ganha o poder de abortar um turno porque um handler pediu (`TestABlockOnAnEventThatCannotBlockIsDropped`). E `exit 2` bloqueia com o stderr como motivo, que é a forma que a maioria dos hooks escritos à mão usa.

**Pendente:** leitura do log por CLI. O original lê por CLI e não expõe MCP; aqui não existe comando de events de nenhum tipo, o que satisfaz literalmente "nenhuma tool MCP" e deixa a leitura de fora. Publicar um `events_list` pelo Command Layer o publicaria também em MCP, então a superfície de leitura entra junto com o manifesto de superfície já pendente da Fase 4.
