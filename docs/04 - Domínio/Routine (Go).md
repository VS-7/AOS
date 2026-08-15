---
tags: [dominio, routine, automacao, cron]
aliases: [Routine Go, Rotina]
fase: 6
status: especificado
origem: "[[Routine]]"
---

# Routine (Go)

> Pai: [[Agent (Go)]] · Origem no original: [[Routine]] · Fase: 6

## Objetivo

Ponto de entrada durável para trabalho autônomo: *"durable entry points for autonomous scheduled or event-driven work"*.

## Comportamento do original

Rotinas **pertencem a agentes**, não ao workspace — o `{agent}` está no caminho ([[Routine]]). Runs ficam em coleção separada; `onDeleted` remove o diretório inteiro.

Três gatilhos, união discriminada por `type`:

| Tipo | Config | Nota |
|---|---|---|
| `scheduled` | `{cron}` | Avaliado no tick de 15 min — essa é a granularidade real |
| `webhook` | `{token}` | POST autenticado dispara |
| `activity` | `{namespace, event}` + `filters` | Reage a eventos internos |

O terceiro é o mecanismo de **automação reativa**: "quando uma task do tipo bug entrar em in_review, rode esta rotina".

Modo rotina no prompt: usuário pode não estar presente, não esperar input, executar completamente, **documentar a run para auditoria** (*"A run without history is only a promise that something happened"*), e agir apenas dentro do escopo declarado.

## Design em Go

```go
// internal/domain/routine/entity.go

type Routine struct {
	Agent string `yaml:"-" json:"agent" collection:"path"`
	ID    string `yaml:"-" json:"id"    collection:"path"`

	Name     string    `yaml:"name"     json:"name"`
	Triggers []Trigger `yaml:"triggers" json:"triggers"`
	Status   Status    `yaml:"status"   json:"status"` // enabled | disabled

	// Scope declares what this routine is allowed to do, mirroring the prompt's
	// rule: "Do not create new routines, agents, permissions, or external
	// effects from a routine unless explicitly allowed by its configuration."
	Scope Scope `yaml:"scope,omitempty" json:"scope,omitempty"`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt"`

	Content string `yaml:"-" json:"content" collection:"content"` // the routine prompt
}

// Trigger is a discriminated union. UnmarshalYAML dispatches on Type.
type Trigger struct {
	Type    TriggerType    `yaml:"type"    json:"type"`
	Config  TriggerConfig  `yaml:"config"  json:"config"`
	Filters []Filter       `yaml:"filters,omitempty" json:"filters,omitempty"`
}

type TriggerConfig struct {
	Cron      string `yaml:"cron,omitempty"      json:"cron,omitempty"`
	Token     string `yaml:"token,omitempty"     json:"token,omitempty"`
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Event     string `yaml:"event,omitempty"     json:"event,omitempty"`
}

type Filter struct {
	Field    string `yaml:"field"    json:"field"`
	Operator string `yaml:"operator" json:"operator"` // eq | neq | contains
	Value    any    `yaml:"value"    json:"value"`
}

type Scope struct {
	AllowCreateTasks   bool     `yaml:"allowCreateTasks"   json:"allowCreateTasks"`
	AllowExternalCalls bool     `yaml:"allowExternalCalls" json:"allowExternalCalls"`
	AllowedTools       []string `yaml:"allowedTools,omitempty" json:"allowedTools,omitempty"`
}
```

### Execução

```go
// internal/domain/routine/service.go

type Service interface {
	List(ctx context.Context, q Query) ([]Routine, error)
	Get(ctx context.Context, agent, id string) (*Routine, error)
	Create(ctx context.Context, in CreateInput) (*Routine, error)
	Update(ctx context.Context, in UpdateInput) (*Routine, error)
	Delete(ctx context.Context, agent, id string) error

	// Fire runs the routine now, recording a run regardless of outcome.
	Fire(ctx context.Context, in FireInput) (*Run, error)

	// ProcessScheduled is the queue job: it evaluates cron triggers against the
	// current tick window and fires the ones that are due.
	ProcessScheduled(ctx context.Context, now time.Time) (int, error)
}

// Run is the audit record. A routine without a run history is exactly what the
// master prompt warns about: "only a promise that something happened".
type Run struct {
	ID        string    `json:"id"`
	Routine   string    `json:"routine"`
	Agent     string    `json:"agent"`
	Trigger   string    `json:"trigger"`
	Payload   any       `json:"payload,omitempty"`
	ChatID    string    `json:"chatId"`
	Status    RunStatus `json:"status"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
	Error     string    `json:"error,omitempty"`
	Usage     TokenUsage `json:"usage"`
}
```

### Cron e a janela do tick

```go
// dueInWindow decides whether a cron expression fires within this tick window.
// The scheduler runs every TickInterval (15 min by default), so a cron that
// would fire several times inside one window fires ONCE — the effective
// resolution of the system. This is documented in the routine's own output so
// nobody discovers it by surprise.
func dueInWindow(expr string, last, now time.Time, loc *time.Location) (bool, error)
```

## Decisões e divergências

> [!decision] Campo `scope` explícito
> O prompt-mestre proíbe uma rotina de criar rotinas, agentes ou efeitos externos *"unless explicitly allowed by its configuration"* — mas o original não tem onde configurar isso. Aqui existe, e o [[Agent Loop]] filtra o registry de tools pelo `Scope` quando o modo é rotina.

> [!decision] A granularidade é documentada, não escondida
> `routines get` devolve o intervalo efetivo junto do cron declarado. O original deixa o usuário descobrir sozinho que `* * * * *` não roda a cada minuto.

> [!decision] Token de webhook rotacionável e hasheado em repouso
> O original guarda o token em claro no frontmatter, que é versionado em Git. Aqui, o frontmatter guarda um hash; o token é mostrado uma vez na criação e pode ser rotacionado.

> [!decision] Toda execução gera run, inclusive falha
> Sem exceção. Falhar sem registro é o pior resultado possível para auditoria.

## Testes

- União discriminada: os três tipos de trigger decodificam corretamente; tipo desconhecido é rejeitado
- Cron com timezone do workspace; DST não duplica nem some com disparo
- `dueInWindow`: cron `* * * * *` dispara uma vez por janela
- Trigger `activity` com filtros `eq`/`neq`/`contains` casa e não casa corretamente
- Webhook com token errado é rejeitado (`AOS_ROUTINE_FIRE_INVALID_TOKEN`)
- Rotina desabilitada não dispara por nenhum gatilho
- `Scope` restringe o registry de tools em modo rotina
- Run registrada em sucesso, falha e timeout
- Delete remove o diretório com as runs

## Critério de pronto

- [ ] Três gatilhos funcionando
- [ ] Runs registradas com auditoria completa
- [ ] `Scope` aplicado ao registry de tools
- [ ] Granularidade efetiva reportada ao usuário
