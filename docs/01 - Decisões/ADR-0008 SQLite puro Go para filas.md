---
tags: [adr, decisao, filas, sqlite]
aliases: [ADR-0008, modernc sqlite, Jobs]
fase: 6
status: pronto
origem: "[[Jobs e Queues]]"
---

# ADR-0008 — SQLite puro Go para filas

> Pai: [[AOS]] · Origem no original: [[Jobs e Queues]] · Fase: 6

## Contexto

O original usa `bunqueue` sobre SQLite em `~/.fractal/data/jobs.sqlite`, em modo WAL, com `lockDuration: 600_000` (10 min), `heartbeatInterval: 30_000` (30 s) e concorrência global 20. Quatro filas: `chat`, `task`, `routine`, `workspace`. O tick global roda em cron `*/15 * * * *` e faz fan-out por workspace, com ordenação recovery-first ([[Jobs e Queues]]).

É o **único** uso de banco no sistema — estado operacional efêmero, não domínio ([[ADR-0004 Collections em Markdown]]).

Em Go, a escolha de driver SQLite tem uma consequência que atravessa todo o projeto: `mattn/go-sqlite3` exige CGO, e CGO destrói a cross-compilation trivial que motivou a [[ADR-0001 Go como linguagem]].

## Decisão

**`modernc.org/sqlite`** — tradução do SQLite para Go puro, sem CGO — com uma implementação própria de fila em `internal/adapters/sqlitequeue`, atrás de um port de domínio:

```go
// internal/domain/job/port.go
type Queue interface {
	Enqueue(ctx context.Context, j Job) (string, error)
	Claim(ctx context.Context, queues []string, lease time.Duration) (*Job, error)
	Heartbeat(ctx context.Context, jobID string) error
	Complete(ctx context.Context, jobID string, result json.RawMessage) error
	Fail(ctx context.Context, jobID string, err error, retryIn *time.Duration) error
	RecoverStale(ctx context.Context, olderThan time.Duration) (int, error)
}
```

Parâmetros herdados do original, agora configuráveis em vez de fixos:

| Parâmetro | Valor default | Origem |
|---|---|---|
| `lease` | 10 min | `lockDuration` do original |
| `heartbeat` | 30 s | idem |
| `concurrency` | 20 | worker global do original |
| `tick` | 15 min | `*/15 * * * *` |
| journal mode | WAL | idem |

O comentário do fonte original registra a intenção não cumprida: *"Isolated into env config later."* Nós já entregamos configurável — `AOS_JOBS_CONCURRENCY`, `AOS_JOBS_TICK`.

O agendamento cron usa **`robfig/cron/v3`** com suporte a timezone, alimentado pelo campo `region.timezone` do [[Config (Go)]].

## Alternativas consideradas

| Alternativa | Análise |
|---|---|
| **`mattn/go-sqlite3`** | Mais rápido e mais maduro. **Descartado por CGO** — o custo de cross-compilation é maior que o ganho para uma fila que processa dezenas de jobs por hora. |
| **Fila em arquivos** (um JSON por job) | Coerente com o resto do modelo de persistência. **Descartado**: claim atômico entre workers exige exatamente o que um banco dá — `UPDATE ... WHERE status='pending' LIMIT 1` numa transação. Reimplementar isso em filesystem é reimplementar um banco, mal. |
| **`bbolt`** | Puro Go, embutido, sem SQL. Viável. Descartado porque consultas de operação ("quais jobs falharam nas últimas 24 h") ficam artesanais, e a fila é justamente onde SQL paga. |
| **Redis/NATS** | Mata o local-first. O original é explícito: *"não há Redis: o Fractal é single-node local-first por desenho"*. |

## Consequências

**Positivas**
- `CGO_ENABLED=0` em todos os alvos. Cross-compile de uma máquina, binário estático.
- Claim com lease e heartbeat detecta worker morto, como no original.
- `RecoverStale` mantém a ordenação recovery-first do tick.

**Negativas**
- **`modernc.org/sqlite` é mais lento** que o binding nativo (tipicamente 2–4× em escrita). Irrelevante nesta carga; documentado para que ninguém use o pacote como banco de domínio por engano.
- **Binário maior** — a tradução do SQLite adiciona alguns MB. Ainda muito abaixo do original.
- **Granularidade de 15 minutos permanece.** Uma [[Routine (Go)]] com cron `* * * * *` não dispara a cada minuto: é *avaliada* no tick. Mantemos o comportamento por compatibilidade de expectativa, mas o tick é configurável — e a nota de [[Routine (Go)]] documenta o limite explicitamente, coisa que o original não faz.

## Status

**Aceito.**

## Implementação — Fase 6

`internal/adapters/sqlitequeue` sobre `modernc.org/sqlite v1.56.0`, atrás do port
`job.Queue`. **90,9% de cobertura**, 14 testes, todos sob `-race`.

O claim é uma única instrução:

```sql
UPDATE jobs SET status = ?, lease_until = ?, claimed_by = ?, ...
 WHERE id = (SELECT id FROM jobs
              WHERE status = 'pending' AND run_at <= ? AND queue IN (...)
              ORDER BY run_at ASC, created_at ASC LIMIT 1)
RETURNING ...
```

Dois workers chegando no mesmo instante não podem levar a mesma linha: o `WHERE`
do segundo já não casa. `TestAClaimedJobIsHandedToExactlyOneWorker` roda 8 workers
concorrentes sobre 40 jobs e afirma que cada um foi entregue exatamente uma vez.

| Parâmetro | Valor | Verificado por |
|---|---|---|
| `lease` 10 min | `job.DefaultLease` | `TestAWorkerThatDiesLosesItsClaim` |
| `heartbeat` 30 s | `job.DefaultHeartbeat` | `TestAHeartbeatKeepsAClaimAlive` |
| `concurrency` 20 | `AOS_JOBS_CONCURRENCY` | `TestThePoolDrainsWhatIsQueued` |
| `tick` 15 min | `AOS_JOBS_TICK` | `TestTheTickRunsRecoveryBeforeTheRegisteredWork` |
| WAL | no DSN | `TestTheQueueSurvivesReopening` |

**`robfig/cron/v3` não entrou.** A ADR o lista para o agendamento; o cron acabou
escrito à mão em `internal/domain/routine/window.go`, com a razão registrada
naquela nota. A regra da arquitetura que proíbe `github.com/robfig/cron/` no
domínio continua no lugar e agora não tem nada a barrar.

**Adição: o backoff tem teto.** Exponencial a partir de 10 s, limitado em 10 min.
Sem teto, uma indisponibilidade transitória de provider vira um job que tenta de
novo na semana que vem.

**Adição: um job sem handler morre, não gira.** É uma incompatibilidade de versão,
não uma falha transitória, e a mensagem nomeia o `kind` — que é o que alguém
depurando precisa.

**Não verificado:** concorrência entre **processos**. O claim atômico é provado
entre goroutines de um processo; dois daemons sobre o mesmo arquivo, que é o que
o `busy_timeout` e o WAL existem para suportar, não foi exercido.
