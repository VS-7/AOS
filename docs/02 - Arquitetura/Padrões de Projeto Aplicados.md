---
tags: [arquitetura, padroes, design-patterns]
aliases: [Padrões, Design Patterns]
fase: 0
status: especificado
origem: "[[Padrão Feature-Slice]]"
---

# Padrões de Projeto Aplicados

> Pai: [[AOS]] · Ver: [[SOLID no Go]] · [[Hexagonal e Regra de Dependência]] · Fase: 0

## Objetivo

Nomear os padrões que o sistema usa, dizer **onde** cada um vive e **o que substitui** no original. Serve para que uma peça nova encontre seu lugar sem inventar estrutura.

## Comportamento do original

O original aplica os mesmos padrões, ainda que sem nomeá-los: `FractalCommand` é Builder + Command, `@igniter-js/collections` é Repository, o pipeline de hooks é Chain of Responsibility, `WorkspaceService.resolve()` é Abstract Factory, o gateway é State.

## Design em Go

| Padrão | Onde | Substitui no original |
|---|---|---|
| **Command** | `core/command` | `FractalCommand` — a fonte única das superfícies |
| **Registry** | `core/command`, `runtime/agentloop`, `runtime/providers` | `collectTools` do `incur` |
| **Repository** | `core/collections` + `adapters/fscollections` | `@igniter-js/collections` |
| **Builder** | `runtime/prompt`, `core/command` | `AgentPromptBuilder`, `FractalCommand` |
| **Strategy** | `runtime/providers`, `domain/toolset` | adaptadores de modelo e de toolset |
| **Adapter** | `adapters/*` | integrações externas |
| **Chain of Responsibility** | `core/eventbus` + hooks | pipeline dos 9 eventos |
| **Decorator** | `runtime/toolexec` | wrapper de truncagem/spillover sobre cada tool |
| **Observer / Pub-Sub** | `core/eventbus` → `realtime` + `activity` | `NotificationBuilder` + `RealtimeBuilder` |
| **Abstract Factory** | `domain/workspace` | `WorkspaceService.resolve()` |
| **State** | `domain/gateway`, `domain/task` | `stopped/stale/running`; ciclo de 8 estados |
| **Template Method** | esqueleto de feature | o Padrão Feature-Slice |
| **Functional Options** | construtores | idioma Go para configuração opcional |
| **Singleflight** | `domain/workspace` | — (adicionado; o original não tem) |

### Os que merecem código

**Decorator — o mais importante depois do Command.** Toda tool é embrulhada, e é por isso que spillover, hooks e métricas funcionam sem que cada tool saiba disso:

```go
// internal/runtime/toolexec/decorator.go

type Tool interface {
	Name() string
	Spec() ToolSpec
	Invoke(ctx context.Context, in json.RawMessage) (any, error)
}

// Wrap composes the cross-cutting concerns around a bare tool, in order:
// approval → hooks → metrics → truncation+spillover.
func Wrap(t Tool, opts ...Option) Tool

// Each concern is itself a Tool that delegates, so the chain is testable
// one layer at a time.
type truncating struct{ next Tool; max int; dir string }

func (d truncating) Invoke(ctx context.Context, in json.RawMessage) (any, error) {
	out, err := d.next.Invoke(ctx, in)
	if err != nil {
		return nil, err
	}
	return d.spill(ctx, out)
}
```

**Chain of Responsibility — hooks.** Cada handler pode injetar contexto, bloquear ou reescrever payload:

```go
// internal/core/eventbus/chain.go

type Handler interface {
	Handle(ctx context.Context, e Event) (Outcome, error)
}

type Outcome struct {
	AdditionalContext string
	Decision          Decision        // Continue | Block
	Reason            string
	UpdatedInput      json.RawMessage // PreToolUse only
}

// Run executes handlers in registration order. It stops at the first Block and
// accumulates AdditionalContext from every handler that ran.
func Run(ctx context.Context, hs []Handler, e Event) (Outcome, error)
```

**Functional Options — o idioma Go para configuração.** Substitui os objetos de opção do original:

```go
// internal/adapters/fscollections/fs.go

type Option func(*Repo)

func WithLock() Option                      { return func(r *Repo) { r.lock = newPathLock() } }
func WithAtomicWrite() Option               { return func(r *Repo) { r.atomic = true } }
func WithWatcher(paths ...string) Option     { return func(r *Repo) { r.watch = paths } }
func WithClock(c Clock) Option               { return func(r *Repo) { r.clock = c } }

func New(root string, models Registry, opts ...Option) *Repo
```

**State — máquinas explícitas.** Duas no sistema, ambas com transição validada:

```go
// internal/domain/task/state.go

// transitions is the authoritative lifecycle graph. Any move not listed here
// is rejected with AOS_TASK_INVALID_TRANSITION.
var transitions = map[Status][]Status{
	Suggestion: {Backlog, Finished},
	Backlog:    {Planning, Todo, Stopped},
	Planning:   {Todo, Backlog, Stopped},
	Todo:       {InProgress, Backlog, Stopped},
	InProgress: {InReview, Stopped, Todo},
	Stopped:    {InProgress, Todo, Backlog},
	InReview:   {Finished, InProgress},
	Finished:   {},
}

func (s Status) CanMoveTo(next Status) bool
```

**Singleflight — adicionado.** Não existe no original e resolve um problema concreto: 20 workers resolvendo o mesmo workspace ao mesmo tempo.

```go
// internal/domain/workspace/service.go
func (s *Service) Resolve(ctx context.Context, id string) (*Runtime, error) {
	v, err, _ := s.sf.Do(id, func() (any, error) { return s.build(ctx, id) })
	// ...
}
```

## Decisões e divergências

> [!decision] Sem Abstract Factory cerimonial
> `WorkspaceService.Resolve` é uma factory de fato, mas não ganha hierarquia de tipos. Devolve um struct `*Runtime` com campos concretos. Em Go, factory é uma função.

> [!decision] Observer com canal, não com callback
> O `eventbus` entrega por canal bufferizado com timeout de entrega. Um assinante lento não bloqueia o publicador: ao estourar o buffer, o evento é descartado com log em nível `warn` e contador. É deliberadamente diferente do original, cujo `NotificationBuilder` chama assinantes em sequência.

> [!decision] Decorator em vez de middleware genérico
> Um `func(next Tool) Tool` seria mais flexível. Optamos por decoradores nomeados (`truncating`, `approving`, `hooking`, `measuring`) porque cada um tem estado próprio e testes próprios, e porque a ordem entre eles é semântica — aprovação antes de execução, truncagem depois.

## Testes

- **Decorator:** cada camada testada isolada com um `Tool` fake; teste de composição verifica a ordem (aprovação nega antes de a tool interna ser chamada).
- **Chain:** handler que bloqueia impede os seguintes; contexto adicional acumula na ordem de registro.
- **State:** tabela de transições testada exaustivamente, incluindo toda transição inválida.
- **Singleflight:** N goroutines, 1 build.
- **Options:** construtor sem opções produz o default documentado.

## Critério de pronto

- [ ] Toda tool passa por `Wrap` — nenhuma registrada crua no loop
- [ ] Transições de [[Task (Go)]] e [[Gateway (Go)]] só por máquina de estados
- [ ] `eventbus` não bloqueia o publicador sob assinante lento (teste com `-race`)
- [ ] Nenhum construtor público com mais de 4 parâmetros posicionais
