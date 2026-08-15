---
tags: [arquitetura, visao-geral, go]
aliases: [Visão Geral, Arquitetura Go, Os Três Binários]
fase: 0
status: especificado
origem: "[[Visão Geral da Arquitetura]]"
---

# Visão Geral Go

> Pai: [[AOS]] · Origem no original: [[Visão Geral da Arquitetura]] · Fase: 0

## Objetivo

Descrever a topologia de processos do sistema e o que cada binário carrega. É o mapa que orienta onde qualquer código novo deve morar.

## Comportamento do original

Quatro processos compartilhando `~/.fractal`: servidor Bun na 5326, CLI, MCP stdio e Electron. **CLI, MCP e Electron não têm lógica de domínio** — são clientes HTTP do servidor. Única exceção: o grupo `gateway`, deliberadamente local ([[Visão Geral da Arquitetura]]).

Isso tem um custo escondido: uma chamada de tool do agente **interno** atravessa HTTP para o próprio processo. O original mitiga com um cliente in-process, mas a arquitetura continua desenhada em torno da fronteira HTTP.

## Design em Go

**Três binários**, não quatro. O MCP deixa de ser um processo separado por desenho e vira um modo do CLI — como no original, mas sem o shim Node de resolução de plataforma.

```
┌────────────────────────────────────────────────────────────────┐
│                   ~/.aos  (estado global)                      │
│  config.json · users.json · data/jobs.sqlite · workspaces/*    │
│  runtime/gateway/* · tmp/outputs/* · workspaces/{id}/index/*   │
└────────────────────────────────────────────────────────────────┘
        ▲                    ▲                      ▲
        │                    │                      │
┌───────┴────────┐  ┌────────┴─────────┐  ┌─────────┴──────────┐
│     aosd       │  │       aos        │  │    aos-desktop     │
│  daemon HTTP   │  │  CLI + --mcp     │  │   Wails3 + React   │
│  :5326         │  │                  │  │                    │
│  chi · ws      │  │  cobra · mcp-sdk │  │  services in-proc  │
└───────┬────────┘  └────────┬─────────┘  └─────────┬──────────┘
        │                    │                      │
        │◄─────── HTTP ──────┴──────────────────────┘
        │
   (domínio, runtime de agente, jobs, watcher)
```

| Binário | Contém | Não contém |
|---|---|---|
| `aosd` | Domínio completo, runtime de agente, jobs, watcher, transportes HTTP/WS/MCP-HTTP/artifacts | UI |
| `aos` | Registry de comandos, cliente HTTP, servidor MCP stdio, supervisão de gateway | Domínio — exceto o grupo `gateway`, local por desenho |
| `aos-desktop` | Frontend embutido, services Wails, cliente HTTP, supervisão de gateway | Domínio |

**Regra de conteúdo:** `aos` e `aos-desktop` importam `internal/core/command` e `internal/transport/*`, **nunca** `internal/domain/*`. O teste de regra de dependência ([[Hexagonal e Regra de Dependência]]) verifica isso mecanicamente.

### O daemon

`aosd` é um `chi.Router` único multiplexando cinco superfícies, espelhando o original:

| Rota | Transporte | Nota |
|---|---|---|
| `/api/*` | REST/RPC gerado do registry | [[HTTP chi]] |
| `/mcp`, `/mcp/*` | MCP streamable HTTP | [[MCP Go SDK]] |
| `/ws` | WebSocket server-push | [[Realtime WebSocket]] |
| `/v/{workspace}/artifacts/{id}/*` | Estático | [[Artifacts e Estáticos]] |
| `/*` | SPA embutida | [[React 19 e Bindings]] |

Bind default `127.0.0.1` ([[ADR-0009 Bind em loopback por padrão]]).

### Multi-workspace no mesmo processo

Igual ao original: o daemon não é dedicado a um workspace. Cada requisição carrega `workspaceID`, e um resolvedor monta o runtime correspondente sob demanda, com cache.

```go
// internal/domain/workspace/service.go

// Runtime is the fully wired set of services bound to one workspace.
// It is the Go equivalent of FractalWorkspaceRuntime.
type Runtime struct {
	Config   Workspace
	Repos    RepoSet            // one Repository[T] per collection
	Path     func(Scope, ...string) string
	Activity activity.Service
	Bus      eventbus.Bus
	Index    search.Index
}

type Service interface {
	// Resolve returns the runtime for a workspace, building it on first use.
	// Concurrent calls for the same id share one build (singleflight).
	Resolve(ctx context.Context, id string) (*Runtime, error)
	Invalidate(id string)
}
```

`Resolve` usa `golang.org/x/sync/singleflight` — 20 workers do tick global chamando ao mesmo tempo constroem o runtime uma vez.

### Injeção de dependência

Sem framework. A montagem acontece **só em `cmd/`**:

```go
// cmd/aosd/main.go
func run(ctx context.Context) error {
	cfg    := config.Load()
	logger := logging.New(cfg.LogLevel)
	clock  := clockx.System{}
	bus    := eventbus.New(logger)

	repos    := fscollections.New(cfg.Root, collections.Registry(), fscollections.WithLock(), fscollections.WithAtomicWrite())
	queue    := sqlitequeue.Open(cfg.JobsDBPath)
	providers:= providers.NewRegistry(cfg.Providers)
	approver := realtime.NewApprover(bus)   // ADR-0007

	ws  := workspace.NewService(repos, bus, clock)
	reg := command.NewRegistry()
	domain.Register(reg, ws, providers, approver, queue, clock)

	srv := httpapi.New(reg, ws, cfg)
	return srv.Run(ctx)
}
```

Três serviços são **singletons de processo** porque guardam estado vivo, como no original: `config`, `tunnel`, `gateway`. Os demais são por workspace.

> [!decision] Sem container de DI, sem `wire`
> O original resolve a dependência circular `BotsRegistry ↔ WorkspaceService` com um `bootstrapContext` mutável preenchido depois da construção — solução frágil que lança erro em runtime se acessada cedo demais.
>
> Em Go, resolvemos por **ordem de construção explícita**: o registro de bots recebe uma função `func(ctx, id) (*Runtime, error)` em vez do serviço inteiro, e essa função é a closure de `ws.Resolve` — já construído. Sem ciclo, sem estado mutável de bootstrap, sem erro possível em runtime.

### Sequência de boot do daemon

```
1. Carrega config e resolve envs em camadas       → internal/core/env
2. Audita permissões de arquivos de segredo       → ADR-0010
3. Abre logger estruturado (log/slog)             → Observabilidade
4. Valida exposição de rede                       → ADR-0009
5. Abre fila SQLite e índice Bleve
6. Constrói o registry de comandos
7. Sobe o worker de jobs (errgroup)               → Jobs
8. Sobe o servidor HTTP/WS
9. Grava PID + meta do gateway
10. Sobe o tunnel, se habilitado                  → Tunnel (Go)
11. Registra bots dos workspaces                  → Bot (Go)
```

A ordem 10 → 11 é intencional e herdada: webhooks do Telegram precisam da URL pública antes do registro.

## Decisões e divergências

> [!decision] Três binários em vez de quatro
> O MCP stdio é um modo do CLI (`aos --mcp`), como no original. A diferença é a ausência do launcher Node: `aos` é um binário nativo por plataforma, distribuído direto.

> [!decision] O agente interno não passa por HTTP
> No original, mesmo o agente interno chama a API — com um cliente in-process, mas ainda pela fronteira. Aqui, o registry de tools do [[Agent Loop]] invoca `Descriptor.Invoke` diretamente sobre o serviço de domínio. Menos latência, menos serialização, e o `context.Context` (com deadline e cancelamento) atravessa intacto.

> [!decision] O desktop não supervisiona um binário de servidor embutido
> Usa o mesmo [[Gateway (Go)]] que o CLI. Uma implementação de supervisão, não duas.

## Testes

- **Regra de dependência:** `go list -deps` sobre `cmd/aos` e `cmd/aos-desktop` não pode conter `internal/domain/...`, exceto `internal/domain/gateway`.
- **Boot completo em `t.TempDir()`:** `aosd` sobe, responde `/api/health`, encerra limpo em `SIGTERM` sem goroutine vazada (`go.uber.org/goleak`).
- **`Resolve` concorrente:** 100 goroutines resolvendo o mesmo workspace constroem o runtime uma vez (contador no builder).
- **Ordem de boot:** teste com tunnel fake verifica que o registro de bots vê a URL pública.

## Critério de pronto

- [ ] Os três binários compilam para as seis combinações de plataforma
- [ ] `aosd` sobe, serve `/api/health` e encerra em `SIGTERM` sem vazamento
- [ ] Teste de regra de dependência verde no CI
- [ ] `aos` e `aos-desktop` não linkam nenhum pacote de domínio além de `gateway`
- [ ] Boot audita permissões de segredo e registra reparos
