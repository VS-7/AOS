---
tags: [dominio, project, organizacao]
aliases: [Project Go, Projeto]
fase: 8
status: pronto
origem: "[[Project]]"
---

# Project (Go)

> Pai: [[Workspace (Go)]] · Origem no original: [[Project]] · Fase: 8

## Objetivo

Contêiner durável para trabalho relacionado que atravessa múltiplas [[Task (Go)]]s e [[Goal (Go)]]s.

## Comportamento do original

Papel no prompt ([[Project]]):

> *"Top-level workspace boundaries organizing related goals, tasks, and files into coherent bodies of work. Use projects to structure long-running efforts that span multiple tasks; associate tasks and goals with the right project to keep work traceable."*

A hierarquia `Workspace → Project → {Goal, Task → {Todo, Comment}}` não é rígida: uma task pode existir sem projeto e sem goal.

## Design em Go

```go
// internal/domain/project/entity.go

type Project struct {
	ID string `yaml:"-" json:"id" collection:"path"`

	Name        string `yaml:"name"        json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Status      Status `yaml:"status"      json:"status"` // active | paused | done | archived
	Color       string `yaml:"color,omitempty" json:"color,omitempty"`

	Paths []string `yaml:"paths,omitempty" json:"paths,omitempty"` // globs this project owns

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt"`

	Content string `yaml:"-" json:"content" collection:"content"`
}
```

```go
type Service interface {
	List(ctx context.Context, q Query) ([]Project, error)
	Get(ctx context.Context, id string) (*Project, error)
	Create(ctx context.Context, in CreateInput) (*Project, error)
	Update(ctx context.Context, in UpdateInput) (*Project, error)

	// Delete never cascades: tasks and goals are unlinked, not removed.
	Delete(ctx context.Context, id string) error
}
```

## Decisões e divergências

> [!decision] Delete desassocia, não cascateia
> Diferente de [[Task (Go)]] e [[Skill (Go)]], apagar um projeto não apaga o trabalho feito nele. Cascatear aqui destruiria histórico legítimo por uma operação de organização.

> [!decision] Campo `paths`
> Adição, simétrica ao `paths` de [[Instruction (Go)]] e ao `scopes` de [[Memory (Go)]]. Permite ao agente inferir o projeto a partir do arquivo em que está trabalhando.

## Testes

- Round-trip completo
- Delete desassocia tasks e goals sem removê-los
- `paths` casando com globs `doublestar`
- Listagem por status

## Critério de pronto

- [x] CRUD completo — `TestRoundTrip`
- [x] Desassociação verificada em delete — `TestDeleteUnlinksWithoutRemovingTheReferencingWork`
- [x] Projeto aparecendo no inventário do [[Prompt Assembly]] — `TestTheAssembledPromptCarriesEveryInventoryCategory` (`internal/app`)
