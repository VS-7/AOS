---
tags: [arquitetura, concorrencia, context, goroutines]
aliases: [Concorrência, Context, Goroutines]
fase: 0
status: especificado
origem: "[[Jobs e Queues]]"
---

# Concorrência e Context

> Pai: [[AOS]] · Origem no original: [[Jobs e Queues]] · [[Camada @server]] · Fase: 0

## Objetivo

Fixar as regras de concorrência do sistema e corrigir o modelo de falha do original, em que uma promise rejeitada derruba o daemon inteiro.

## Comportamento do original

Três fatos relevantes ([[Camada @server]], [[Jobs e Queues]], [[Fluxo de Requisição]]):

1. **Falha dura em exceção não tratada.** `uncaughtException` e `unhandledRejection` fazem `process.exit(1)`. Uma promise rejeitada em qualquer feature encerra a sessão de todos os workspaces. Defeito #16.
2. **Concorrência global de 20** no worker de jobs, valor fixo com intenção declarada de virar configurável.
3. **Dois `AsyncLocalStorage`** coexistindo: `FractalCliTransportContext` (lado cliente: `baseUrl`, `token`) e `RequestContext` (lado servidor: `workspaceId`, `agentId`, `userId`).

O terceiro ponto é o mais interessante para a tradução: ALS é o mecanismo do JavaScript para propagar contexto implicitamente. Em Go, `context.Context` faz isso **explicitamente**, e a explicitude é ganho.

## Design em Go

### Regra 1 — `context.Context` é o primeiro parâmetro, sempre

Toda função que faz I/O, chama serviço ou pode demorar recebe `ctx context.Context` como primeiro parâmetro. Sem exceção, sem `context.TODO()` em código de produção (o linter rejeita).

O contexto carrega, além de cancelamento e deadline, a identidade ambiente — o equivalente ao `RequestContext` do original:

```go
// internal/core/identity/identity.go

type Identity struct {
	WorkspaceID string
	AgentID     string
	UserID      string
	RequestID   string
}

type ctxKey struct{}

// With attaches the ambient identity to ctx. Called once per inbound request
// by the transport layer, never inside domain code.
func With(ctx context.Context, id Identity) context.Context

// From reads the ambient identity. It returns the zero value when absent, so
// callers that require an actor must check explicitly.
func From(ctx context.Context) Identity

// Actor returns the agent when present, otherwise the user — matching the
// original's getActor(), which prefers the agent over the user.
func Actor(ctx context.Context) (string, ActorType)
```

O equivalente ao `FractalCliTransportContext` é um campo do cliente HTTP, não um contexto implícito — o CLI constrói o cliente com `baseURL` e `token` e o passa adiante.

### Regra 2 — nenhuma goroutine sem dono

Toda goroutine tem um `errgroup.Group` ou um supervisor que a aguarda. Proibido `go f()` solto em código de produção.

```go
// internal/domain/workspace/tick.go

// Tick fans out recovery and processing jobs for every registered workspace.
// It mirrors the original's 15-minute global tick, including recovery-first
// ordering, but with bounded concurrency and full cancellation.
func (s *Service) Tick(ctx context.Context, runID string) (TickReport, error) {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(s.cfg.TickConcurrency)

	var mu sync.Mutex
	report := TickReport{RunID: runID, Now: s.clock.Now()}

	for _, ws := range s.list() {
		ws := ws
		g.Go(func() error {
			res := s.dispatchFor(ctx, ws) // recovery jobs first, then due work
			mu.Lock()
			defer mu.Unlock()
			report.Merge(ws.ID, res)
			return nil // a failing workspace is recorded, never fatal to the tick
		})
	}
	return report, g.Wait()
}
```

Note o `return nil`: um workspace com problema entra no relatório como `failed`, e os outros continuam. A saída totalmente observável (`scanned`, `dispatched`, `failed`, `workspaces`) é herdada do original.

### Regra 3 — `recover()` na fronteira, nunca no meio

Panic é bug. Mas um bug em uma feature não pode derrubar o daemon. Recuperação acontece em três fronteiras, e só nelas:

```go
// internal/core/safe/safe.go

// Go runs fn in a goroutine that recovers from panics, logs the stack with the
// ambient request id, and reports the incident. It never swallows the failure
// silently: the caller's error channel receives a wrapped panic error.
func Go(ctx context.Context, name string, fn func(context.Context) error) <-chan error
```

| Fronteira | Efeito de um panic |
|---|---|
| Handler HTTP | 500 com `X-Request-ID`, stack no log, daemon vivo |
| Worker de job | Job marcado `failed` com a stack, worker continua |
| Chamada de tool no loop | `ToolResult` de erro devolvido ao modelo, loop continua sob a Two-Strike Rule |

**Corrige o defeito #16.** Um panic degrada uma operação, não o processo.

### Regra 4 — cancelamento propaga até o fim

`ctx` cancelado interrompe: a requisição HTTP em voo, a chamada ao provider de LLM, o comando em execução no sandbox e a escrita em disco em curso (que, sendo atômica, deixa o arquivo anterior intacto — ver [[ADR-0012 Escrita atômica e lock por arquivo]]).

```go
// internal/runtime/sandbox/exec.go
cmd := exec.CommandContext(ctx, bin, args...)
cmd.WaitDelay = 5 * time.Second // SIGKILL after SIGTERM grace
```

Encerrar o daemon com `SIGTERM` cancela o contexto raiz; tudo desmonta em ordem, com `shutdownTimeout` de 15 s antes de forçar.

### Regra 5 — proteja o estado, não a operação

Mutex protege dados, e o escopo é o menor possível. Onde couber, canal em vez de mutex. Onde couber, valor imutável em vez dos dois.

```go
// internal/domain/workspace/service.go
type Service struct {
	mu       sync.RWMutex
	runtimes map[string]*Runtime // guarded by mu
	sf       singleflight.Group  // dedupes concurrent builds
}
```

### Regra 6 — `-race` é obrigatório

`go test -race ./...` é portão de CI ([[Estratégia de Testes]]). Um teste que só passa sem `-race` está errado.

### Concorrência configurável

| Parâmetro | Default | Env | Origem |
|---|---|---|---|
| Workers de job | 20 | `AOS_JOBS_CONCURRENCY` | worker global do original |
| Fan-out do tick | 8 | `AOS_TICK_CONCURRENCY` | — (o original não limita) |
| Tools em paralelo por turno | 4 | `AOS_TOOL_CONCURRENCY` | prompt manda paralelizar tools independentes |
| Timeout de shutdown | 15 s | `AOS_SHUTDOWN_TIMEOUT` | — |

## Decisões e divergências

> [!decision] Panic degrada, não mata
> Divergência deliberada do original. O daemon serve N workspaces; derrubá-lo por um bug em uma feature penaliza todos os usuários por um erro localizado.

> [!decision] Identidade no `context`, transporte no cliente
> O original usa duas ALS. Aqui, identidade ambiente (servidor) vai no `context`; configuração de transporte (cliente) é campo do cliente HTTP. A separação fica visível na assinatura das funções.

> [!decision] Tools independentes em paralelo, com limite
> O [[System Prompt (BASE)]] instrui: *"Call multiple independent tools in parallel. Call dependent tools sequentially."* O loop executa tool calls do mesmo passo em `errgroup` com limite de 4. Chamadas dependentes vêm em passos distintos por construção do modelo.

## Testes

- **`goleak`** no `TestMain` de todo pacote com goroutine — nenhuma sobrevive ao teste.
- **Panic em handler:** requisição responde 500, servidor continua atendendo a seguinte.
- **Panic em worker:** job vira `failed` com stack, worker processa o próximo.
- **Cancelamento:** `ctx` cancelado durante comando de sandbox mata o processo filho em até 5 s (verificado por `ps`).
- **Tick sob falha:** um workspace com repositório quebrado não impede os demais; o relatório o lista em `failed`.
- **`-race` em todo o pacote de coleções** com 50 escritores concorrentes no mesmo caminho.

## Critério de pronto

- [ ] Nenhum `go f()` solto fora de `safe.Go` ou `errgroup`
- [ ] `context.TODO()` ausente de código de produção (linter)
- [ ] `go test -race ./...` verde no CI
- [ ] `goleak` sem vazamentos
- [ ] `SIGTERM` desmonta em menos de 15 s com jobs em voo
