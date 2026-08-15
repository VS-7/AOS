---
tags: [qualidade, observabilidade, logging, metricas]
aliases: [Observabilidade, Logging, Métricas]
fase: 0
status: em-construcao
origem: "[[Camada @core]]"
---

# Observabilidade

> Pai: [[AOS]] · Origem no original: [[Camada @core]] · Fase: 0

## Objetivo

Poder responder três perguntas sem adivinhação: **o que aconteceu**, **quanto custou** e **por que falhou** — para um daemon local que roda por semanas.

## Comportamento do original

`pino` para logging estruturado, com `X-Request-ID` em toda resposta e log de acesso que distingue streaming ([[Camada @core]], [[API HTTP]]).

Telemetria por Sentry, **opt-out**, com `installationId` identificando a instalação e um `sanitizeEvent` ([[Stack Tecnológica]]). Há também `telemetry.enabled` separado, ambos `true` por padrão.

## Design em Go

### Logging

```go
// internal/core/logging/logging.go

// New builds the structured logger. Text with colours on a TTY, JSON otherwise —
// the same "who is consuming this" heuristic the CLI uses for output format.
func New(cfg Config) *slog.Logger

// FromContext returns a logger already carrying the ambient identity, so every
// line in a request is correlatable without the caller repeating fields.
func FromContext(ctx context.Context) *slog.Logger
```

Campos padrão em toda linha: `request_id`, `workspace`, `agent`, `component`.

```go
log := logging.FromContext(ctx)
log.Info("memory stored", "memory_id", m.ID, "category", m.Category, "confidence", m.Confidence)
```

### Redação no logger

```go
// redactHandler drops any attribute whose key matches a secret pattern, and any
// value that matches a known token shape. Belt and braces: the config service
// already redacts, but a log line is written from a hundred places.
func redactHandler(next slog.Handler) slog.Handler
```

### O que se registra em cada camada

| Camada | Evento | Campos |
|---|---|---|
| HTTP | Requisição | método, rota, status, duração, `request_id` |
| Agent loop | Turno | agente, sessão, passos, tools chamadas, tokens, custo, duração |
| Tool | Execução | nome, duração, truncado, spillover, decisão de aprovação |
| Job | Ciclo | fila, job, tentativa, resultado, duração |
| Collections | Escrita | coleção, chave, operação, conflito |
| Hooks | Invocação | hook, evento, decisão, duração |

### Métricas

```go
// internal/core/metrics/metrics.go

// Metrics are exposed at /api/metrics in Prometheus text format, bound to
// loopback and behind the same auth as the rest of the API. No push, no
// external collector: a local-first tool should not phone home to be
// observable.
type Metrics struct {
	AgentTurns      *Counter   // by agent, model, outcome
	AgentTokens     *Counter   // by agent, model, kind (input/output/reasoning)
	AgentCostUSD    *Counter   // by agent, model
	ToolCalls       *Counter   // by tool, outcome
	ToolDuration    *Histogram // by tool
	SpilloverBytes  *Counter
	JobsProcessed   *Counter   // by queue, outcome
	CollectionOps   *Counter   // by collection, op
	HookDecisions   *Counter   // by hook, decision
	ApprovalOutcome *Counter   // approved | denied | timeout | unavailable
}
```

### Custo visível

O custo por turno e por sessão é registrado em [[Chat (Go)]] e agregado em `aos usage`:

```
aos usage --since 7d --by agent
```

O original registra tokens; sem preço, o usuário não vê o que gastou. Ver [[Model Providers (Go)]].

### Diagnóstico

```go
// aos doctor collects everything a bug report needs, with secrets redacted:
// versions, config (redacted), gateway state, port availability, provider
// reachability, MCP registration drift, workspace count and index health.
func Doctor(ctx context.Context) (Report, error)
```

## Decisões e divergências

> [!decision] Telemetria opt-in, não opt-out
> A divergência mais significativa desta nota. O original inicializa Sentry no boot do servidor **e** do CLI, com telemetria `true` por padrão e um identificador de instalação. Um produto local-first que promete privacidade não envia nada sem que o usuário diga sim. O onboarding pergunta uma vez, a resposta fica no config, e `aos config get` mostra o estado.

> [!decision] Métricas em pull local, sem coletor externo
> `/api/metrics` em loopback, autenticado. Quem quiser Prometheus aponta para lá; quem não quiser não tem processo enviando dados.

> [!decision] Custo em dinheiro, não só em tokens
> Tokens são unidade interna. O usuário raciocina em dinheiro.

> [!decision] `log/slog` da stdlib
> Sem `pino`, sem `zerolog`. Estruturado, sem dependência, com `Handler` customizável — que é o que permite a redação.

## Testes

- Redação: token conhecido injetado no config não aparece em nenhuma linha de log de uma sessão completa
- Correlação: todas as linhas de uma requisição compartilham `request_id`
- Métricas: contadores incrementam nos eventos corretos
- Custo: valor calculado bate com a tabela de preços por modelo
- `/api/metrics` exige autenticação e não escuta fora de loopback por default
- `doctor` produz relatório sem segredos
- Telemetria desligada por default: nenhuma conexão externa numa sessão completa (teste com rede bloqueada)

## Critério de pronto

- [ ] Logging estruturado com identidade ambiente e redação
- [ ] Métricas expostas localmente e autenticadas
- [ ] Custo por turno, sessão e agente
- [ ] `aos doctor` cobrindo os pontos de diagnóstico
- [ ] Telemetria opt-in verificada por teste de rede

## Progresso — Fase 0

> [!warning] Nota parcialmente implementada
> O [[Roteiro de Fases]] lista esta nota na Fase 0, mas metade do seu conteúdo depende de peças de fases posteriores: métricas exigem o servidor HTTP (Fase 4), custo exige os providers (Fase 5) e `doctor` exige o gateway (Fase 4). Só a camada de logging foi entregue. A nota permanece `em-construcao` em vez de ser declarada pronta com metade do critério em aberto.

**Entregue**

- `internal/core/logging` sobre `log/slog`, com `New(Config)` escolhendo texto em TTY e JSON quando redirecionado
- `FromContext` injetando `request_id`, `workspace` e `agent` a partir da identidade ambiente, e `Component` acrescentando `component`
- `redactHandler` cobrindo três caminhos: chave sensível (`password`, `token`, `secret`, `authorization`, …), valor com forma de credencial (`aos_…`, `sk-…`, `ghp_…`, `Bearer …`) e grupos aninhados
- Cobertura de 100% do pacote

```
$ go test -race -count=1 ./internal/core/logging/
ok  	github.com/OWNER/aos/internal/core/logging	coverage: 100.0% of statements
```

**Pendente, com a fase de destino**

| Item | Fase |
|---|---|
| `/api/metrics` em loopback, autenticado | 4 |
| Contadores e histogramas de `Metrics` | 4–6, conforme cada evento existir |
| Custo por turno, sessão e agente; `aos usage` | 5 |
| `aos doctor` | 4 |
| Teste de rede provando telemetria desligada por default | 4 |

O default de telemetria já é `false` ([[Config (Go)]]), que é a metade da decisão que não depende de rede.
