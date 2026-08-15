---
tags: [arquitetura, erros, cta, llm]
aliases: [Erros, AppError, CTA]
fase: 0
status: especificado
origem: "[[Códigos de Erro]]"
---

# Estratégia de Erros

> Pai: [[AOS]] · Origem no original: [[Códigos de Erro]] · Fase: 0

## Objetivo

Erros aqui não são só diagnóstico: são **instrução para o próximo passo de um agente**. Um erro que só falha desperdiça um turno de LLM; um erro que orienta economiza vários. Esta nota fixa a forma que garante isso.

## Comportamento do original

O original tem **201 códigos** no formato `FRACTAL_{DOMÍNIO}_{CONDIÇÃO}`, com quatro campos além da mensagem ([[Códigos de Erro]]):

```ts
new FractalAppError({
  code:   "FRACTAL_AGENT_SANDBOX_PERMISSION_DENIED",
  causer: "FractalAgentSandboxService.verifyFsAccess",
  issue:  { operation: "write", path: "...", reason: "..." },
  status: 403,
  cta:    { ... },
})
```

Dois acertos que copiamos sem alteração:

- **`causer`** identifica o método exato que lançou — rastreabilidade sem stack trace, o que importa quando o consumidor é um LLM que não lê stack.
- **`cta`** transforma o erro em orientação acionável, entregue ao agente como próximo passo sugerido.

Há também o envelope `ResponseWithCTA<T> = T & { _cta?: CallToAction }` no caminho de **sucesso** — a resposta de criar uma task já sugere o que fazer em seguida ([[Igniter.js Framework]]).

## Design em Go

```go
// internal/core/apperr/apperr.go
package apperr

type CallToAction struct {
	Label   string `json:"label"`             // "recupere memórias relacionadas"
	Command string `json:"command,omitempty"` // "aos memories recall --query 'auth'"
	Tool    string `json:"tool,omitempty"`    // "memories_recall"
	Input   any    `json:"input,omitempty"`   // payload pronto para a tool
}

type Error struct {
	Code   string         `json:"code"`             // AOS_MEMORY_NOT_FOUND
	Causer string         `json:"causer"`           // memory.Service.Get
	Msg    string         `json:"message"`
	Issue  map[string]any `json:"issue,omitempty"`  // dados estruturados do problema
	CTA    []CallToAction `json:"cta,omitempty"`
	Status int            `json:"-"`                // HTTP status
	Err    error          `json:"-"`                // wrapped cause
}

func (e *Error) Error() string { return e.Code + ": " + e.Msg }
func (e *Error) Unwrap() error { return e.Err }
```

### Construção fluente, sem verbosidade

```go
// internal/domain/memory/errors.go
package memory

func errNotFound(id string) error {
	return apperr.New("MEMORY_NOT_FOUND").
		Causer("memory.Service.Get").
		Msgf("memory %q does not exist", id).
		Issue("id", id).
		Status(http.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list active memories for this agent",
			Tool:    "memories_recall",
			Command: "aos memories recall --limit 20",
		})
}
```

O prefixo `AOS_` é aplicado por `apperr.New` a partir de `build.ErrorPrefix` ([[ADR-0000 Nome provisório do projeto]]) — nunca escrito à mão.

### O CTA no caminho de sucesso

Herdado do original e igualmente importante:

```go
// internal/core/command/envelope.go

// Envelope wraps every command result. CTA on success is guidance, not error
// handling: after creating a task, suggest starting it.
type Envelope[T any] struct {
	Data       T                     `json:"data"`
	CTA        []apperr.CallToAction `json:"_cta,omitempty"`
	Deprecated *DeprecationNotice    `json:"_deprecated,omitempty"` // ADR-0011
}
```

### Tradução por superfície

O mesmo `*apperr.Error` é apresentado de forma diferente em cada superfície — e essa é a razão de o erro ser estruturado em vez de string:

| Superfície | Apresentação |
|---|---|
| CLI (TTY) | Mensagem colorida + `Issue` em tabela + CTAs como comandos copiáveis |
| CLI (não-TTY / agente) | JSON completo no formato de saída escolhido |
| MCP | `isError: true` com o JSON no conteúdo — o modelo lê `cta` e age |
| HTTP | `Status` + corpo JSON + header `X-Request-ID` |
| Wails | Erro tipado, mapeado para toast + ação sugerida na UI |

### Sentinelas para controle de fluxo

Códigos são strings para o consumidor; internamente, comparação é por `errors.Is`:

```go
var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrForbidden    = errors.New("forbidden")
	ErrUnavailable  = errors.New("unavailable")
)

// Every apperr.Error wraps one sentinel, so callers branch on behaviour
// instead of parsing codes.
if errors.Is(err, apperr.ErrNotFound) { /* ... */ }
```

### Catálogo de códigos, verificado

Todos os códigos vivem em um catálogo gerado, e um teste garante três coisas:

```go
// internal/core/apperr/catalog_test.go
// 1. Every code used in the tree exists in the catalog.
// 2. Every catalog entry is reachable from at least one call site.
// 3. Every code carries a Causer and, when actionable, at least one CTA.
```

Isso responde a um problema real do original: dos 201 códigos listados, alguns são nomes de variável de ambiente capturados por engano pelo extrator (`FRACTAL_BASE_URL`, `FRACTAL_TOKEN`, `FRACTAL_VERSION`) — o catálogo não é curado.

### Correspondência de códigos com o original

Mantemos a estrutura `{PREFIXO}_{DOMÍNIO}_{CONDIÇÃO}` e os nomes de condição onde fazem sentido, para que quem migra reconheça:

| Original | Aqui |
|---|---|
| `FRACTAL_MEMORY_NOT_FOUND` | `AOS_MEMORY_NOT_FOUND` |
| `FRACTAL_TASK_INVALID_TRANSITION` | `AOS_TASK_INVALID_TRANSITION` |
| `FRACTAL_AGENT_SANDBOX_PERMISSION_DENIED` | `AOS_SANDBOX_PERMISSION_DENIED` |
| — | `AOS_SANDBOX_EXEC_NOT_ALLOWED` (novo, [[ADR-0006 Allowlist no sandbox]]) |
| — | `AOS_TOOL_APPROVAL_UNAVAILABLE` (novo, [[ADR-0007 Canal real de aprovação de tool]]) |
| — | `AOS_COLLECTION_CONFLICT` (novo, [[ADR-0012 Escrita atômica e lock por arquivo]]) |
| — | `AOS_CONFIG_FIELD_FORBIDDEN` (novo, [[ADR-0010 Segredos com permissão restrita]]) |

## Decisões e divergências

> [!decision] CTA é obrigatório em erro acionável
> Um teste falha se um erro com status 4xx não tiver CTA. Erros de 5xx são isentos — não há ação útil para o agente diante de falha interna, e sugerir uma seria mentir.

> [!decision] `causer` continua manual
> Poderia vir de `runtime.Caller`. Mantido explícito porque o valor é semântico ("qual operação de domínio falhou"), não posicional — e porque `runtime.Caller` reporta o wrapper, não a intenção.

> [!decision] Nenhum segredo em `Issue`
> `Issue` é serializado para o agente e para logs. Um teste percorre o catálogo e falha se algum construtor colocar valor de campo marcado `secret:"true"` no issue. Ver [[ADR-0010 Segredos com permissão restrita]].

## Testes

- **Catálogo:** os três invariantes acima.
- **CTA em 4xx:** teste tabular sobre o catálogo.
- **Golden por superfície:** o mesmo erro renderizado em CLI, MCP, HTTP e Wails, comparado com `testdata/errors/*.golden` ([[Fixtures e Golden Files]]).
- **`errors.Is`:** todo `*apperr.Error` desembrulha para exatamente uma sentinela.
- **Sem segredo em `Issue`:** varredura do catálogo.

## Critério de pronto

- [ ] Catálogo de códigos completo e verificado por teste
- [ ] Todo erro 4xx com pelo menos um CTA
- [ ] Golden files das quatro superfícies
- [ ] Prefixo derivado de `build.ErrorPrefix`, nunca literal
- [ ] Nenhum campo secreto alcançável via `Issue`
