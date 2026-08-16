---
tags: [dominio, activity, eventos, inbox]
aliases: [Activity Go, Inbox, Atividades]
fase: 6
status: pronto
origem: "[[Activity]]"
---

# Activity (Go)

> Pai: [[Workspace (Go)]] · Origem no original: [[Activity]] · Ver: [[Realtime WebSocket]] · Fase: 6

## Objetivo

O log durável de eventos do workspace — a caixa de entrada de notificações e o **barramento de eventos reativo** que alimenta gatilhos de [[Routine (Go)]].

## Comportamento do original

Cada mutação de domínio dispara uma notificação que segue dois caminhos ([[Activity]]):

```
Mutação → NotificationBuilder ─┬─→ Activity (persistido, inbox)
                               └─→ Realtime (efêmero, WebSocket)
```

Registros carregam `namespace` (ex.: `task`), `event` (ex.: `status_changed`) e `data` estruturado. Estado de leitura é **por ator**, não global.

O que o torna mais que um log: [[Routine (Go)]] pode ter trigger `activity`, com filtros sobre `data`. É como "quando uma task do tipo bug entrar em in_review, rode esta rotina" funciona.

## Design em Go

```go
// internal/domain/activity/entity.go

type Activity struct {
	ID        string          `json:"id"`
	Namespace string          `json:"namespace"` // task | memory | skill | toolset | ...
	Event     string          `json:"event"`     // created | status_changed | published | ...
	Title     string          `json:"title"`     // human-readable
	Body      string          `json:"body,omitempty"`
	Icon      string          `json:"icon,omitempty"`
	Data      map[string]any  `json:"data,omitempty"`

	Actor     string    `json:"actor"`     // who caused it
	ActorType string    `json:"actorType"` // agent | user | system
	ReadBy    []string  `json:"readBy,omitempty"` // per-actor read state
	CreatedAt time.Time `json:"createdAt"`
}
```

```go
type Service interface {
	List(ctx context.Context, q Query) ([]Activity, error)
	Get(ctx context.Context, id string) (*Activity, error)

	// Publish records the activity and fans it out to realtime and to routine
	// triggers. It is the single fan-out point.
	Publish(ctx context.Context, in PublishInput) (*Activity, error)

	MarkAsRead(ctx context.Context, id string) error
	MarkAllAsRead(ctx context.Context) error
	Delete(ctx context.Context, id string) error

	// Purge removes entries older than the retention window.
	Purge(ctx context.Context, olderThan time.Duration) (int, error)
}
```

### Persistência

```go
// The original keeps a single .fractal/activity.json array, rewritten on every
// mutation. That is O(n) writes on a file that only grows, and it is the kind
// of thing that works for a week and then does not.
//
// We append JSONL under .aos/activity/{yyyy-mm}.jsonl, with an in-memory index
// for the inbox view and a monthly retention job.
type Writer struct{ /* ... */ }
```

### Fan-out

```go
// Publish is the single place where a domain mutation becomes visible:
//   1. append to the activity log
//   2. push to the workspace realtime channel
//   3. evaluate routine triggers of type "activity"
//
// Steps 2 and 3 are best-effort and never fail the mutation that caused them.
func (s *service) Publish(ctx context.Context, in PublishInput) (*Activity, error)
```

## Decisões e divergências

> [!decision] JSONL mensal em vez de um único JSON reescrito
> Escrita O(1), leitura por índice, retenção por arquivo. A abordagem do original degrada linearmente e reescreve o arquivo inteiro a cada notificação — com risco de corrupção sem escrita atômica.

> [!decision] Estado de leitura por ator, mantido
> Herdado. Um agente e um humano têm inboxes independentes sobre o mesmo fluxo.

> [!decision] Retenção configurável, default 90 dias
> O original não tem retenção (há um código de erro `INVALID_PURGE_AGE`, mas nenhuma política). Um log que nunca expira acaba dominando o repositório.

> [!decision] Fan-out best-effort
> Falha de realtime ou de avaliação de trigger não derruba a mutação de domínio que a originou.

## Testes

- `Publish` grava, empurra para realtime e avalia triggers
- Falha de realtime não afeta a gravação
- Estado de leitura por ator é independente
- Filtros de trigger `eq`, `neq`, `contains` sobre `data`
- Rotação mensal e purga por retenção
- Índice em memória bate com o conteúdo em disco após restart
- Concorrência: 100 publicações simultâneas sob `-race` não perdem registro

## Critério de pronto

- [x] Inbox funcionando na UI, com estado de leitura por ator
- [x] Triggers de rotina reagindo a atividades
- [x] Rotação e retenção operando
- [x] Fan-out resiliente a falha de consumidor

## Saída dos testes — Fase 6

`go test ./internal/domain/activity/` — **86,6% de cobertura**, 17 testes.

| O que a nota pede | Teste |
|---|---|
| `Publish` grava, empurra e avalia triggers | `TestPublishWritesThenFansOut` + `TestTheDeliveryOfPhaseSix` |
| Falha de realtime não afeta a gravação | `TestAConsumerThatBlowsUpDoesNotFailTheMutation` |
| Estado de leitura por ator é independente | `TestReadStateIsPerActor` |
| Filtros de trigger sobre `data` | `TestATriggerKeyMatchesTheNamespaceAndOptionallyTheEvent` + a suíte de rotina |
| Rotação mensal e purga por retenção | `TestPurgeDropsWholeMonthsAndSaysWhichItHadToRewrite` |
| 100 publicações simultâneas sob `-race` | `TestConcurrentPublishesAllLand` |

**Decisão não prevista: estado de leitura fora do log.** O `readBy` do original
mora no registro, o que significa reescrever o histórico para marcar uma
notificação como lida. Aqui é um overlay separado em `activity/read.json`, com
marca d'água por ator — o que torna "marcar tudo como lido" O(1) e mantém o log
apend-only por construção, não por disciplina.

**`Publish` não é comando.** Uma atividade registra algo que o sistema fez; uma
superfície capaz de escrever uma deixaria um agente fabricar o registro de uma
mudança que nunca aconteceu. Toda entrada vem da mutação que a causou.
`TestRegisterPublishesTheWholeGroup` afirma a ausência.

**Ator `system`.** Um tick ou uma purga não tem pessoa nem agente atrás, e
atribuir a quem estava logado é uma mentira que a trilha de auditoria carregaria
para sempre.

**A saída de `purge` separa `Dropped` de `Rewritten`.** Um mês inteiro expirado é
um arquivo removido; um mês parcialmente expirado é histórico editado no lugar.
Só o segundo pode perder uma entrada num crash, e quem chama tem direito de saber
que esse risco foi corrido.

**Não verificado:** o índice em memória para a inbox. A nota pede um; a leitura
hoje varre as partições que a janela alcança, e o nome do arquivo já filtra o que
não pode conter um resultado. Vale até algumas dezenas de milhares de entradas com
retenção de 90 dias. Registrado como pendente em vez de fingido.
