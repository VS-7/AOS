---
tags: [dominio, goal, estrategia]
aliases: [Goal Go, Meta]
fase: 8
status: especificado
origem: "[[Goal]]"
---

# Goal (Go)

> Pai: [[Project (Go)]] · Origem no original: [[Goal]] · Fase: 8

## Objetivo

Resultados estratégicos que alinham o trabalho diário: *"strategic outcomes that align daily work toward a shared destination"*.

## Comportamento do original

A razão de ser da entidade está numa frase do prompt ([[Goal]]):

> *"Before planning or executing significant work, check active goals to align your efforts and avoid strategically useless results."*

**"Avoid strategically useless results"** — impedir que o agente execute trabalho tecnicamente correto e estrategicamente irrelevante. Uma [[Skill (Go)]] pode trazer goals pré-definidas no seu `metadata.goals`.

## Design em Go

```go
// internal/domain/goal/entity.go

type Goal struct {
	ID string `yaml:"-" json:"id" collection:"path"`

	Title       string `yaml:"title"       json:"title"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Status      Status `yaml:"status"      json:"status"` // active | achieved | abandoned | paused

	Project string     `yaml:"project,omitempty" json:"project,omitempty"`
	DueAt   *time.Time `yaml:"dueAt,omitempty"   json:"dueAt,omitempty"`
	Skill   string     `yaml:"skill,omitempty"   json:"skill,omitempty"` // when installed by a skill

	// Measure makes the outcome checkable instead of aspirational.
	Measure string `yaml:"measure,omitempty" json:"measure,omitempty"`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt"`

	Content string `yaml:"-" json:"content" collection:"content"`
}
```

```go
type Service interface {
	List(ctx context.Context, q Query) ([]Goal, error)
	Get(ctx context.Context, id string) (*Goal, error)
	Create(ctx context.Context, in CreateInput) (*Goal, error)
	Update(ctx context.Context, in UpdateInput) (*Goal, error)
	Delete(ctx context.Context, id string) error

	// Active returns goals the agent should align with right now. It is what
	// feeds the workspace inventory in the prompt.
	Active(ctx context.Context) ([]Goal, error)
}
```

## Decisões e divergências

> [!decision] Campo `measure`
> Adição. Uma meta sem critério verificável não permite ao agente saber se contribuiu para ela. Alinha com a exigência do prompt-mestre de distinguir planejado, implementado e verificado.

> [!decision] Delete desassocia tasks
> Como em [[Project (Go)]].

## Testes

- Round-trip completo
- `Active` devolve só `status: active`
- Goal instalada por skill carrega o campo `skill` e é removida ao desinstalar
- Delete desassocia tasks sem removê-las

## Critério de pronto

- [ ] CRUD completo
- [ ] Goals ativas no inventário do [[Prompt Assembly]]
- [ ] Goals trazidas por skill instaladas e removidas com ela
