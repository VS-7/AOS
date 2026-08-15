---
tags: [dominio, todo, task]
aliases: [Todo Go]
fase: 6
status: especificado
origem: "[[Todo]]"
---

# Todo (Go)

> Pai: [[Task (Go)]] · Origem no original: [[Todo]] · Fase: 6

## Objetivo

Passos de execução dentro de uma [[Task (Go)]]. Subcoleção com coleção própria.

## Comportamento do original

Regras do prompt-mestre em modo task ([[Todo]]):

- *"If plan is missing, create it first. If todos are missing, create them before deep execution."*
- *"Keep task and todo state authoritative using task/todo tools. Do not run a comment-only execution."*
- *"Only move the task to in_review when all todos are finished and validated."*

Todo comando exige o identificador da task pai. O checkpoint da task guarda `pendingTodoIds` e `progress`.

## Design em Go

```go
// internal/domain/todo/entity.go

type Status string

const (
	Pending    Status = "pending"
	InProgress Status = "in_progress"
	Blocked    Status = "blocked"
	Finished   Status = "finished"
	Skipped    Status = "skipped"
)

type Todo struct {
	TaskID string `yaml:"-" json:"taskId" collection:"path"`
	ID     string `yaml:"-" json:"id"     collection:"path"`

	Title    string `yaml:"title"    json:"title"`
	Status   Status `yaml:"status"   json:"status"`
	Order    int    `yaml:"order"    json:"order"`
	Evidence string `yaml:"evidence,omitempty" json:"evidence,omitempty"` // what was verified

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt"`

	Content string `yaml:"-" json:"content" collection:"content"`
}
```

```go
type Service interface {
	List(ctx context.Context, taskID string) ([]Todo, error)
	Get(ctx context.Context, taskID, id string) (*Todo, error)
	Create(ctx context.Context, in CreateInput) (*Todo, error)
	Update(ctx context.Context, in UpdateInput) (*Todo, error)

	// SetStatus is the only lifecycle path, mirroring Task.
	SetStatus(ctx context.Context, in SetStatusInput) (*Todo, error)
	Delete(ctx context.Context, taskID, id string) error

	CountPending(ctx context.Context, taskID string) (int, error)
	Progress(ctx context.Context, taskID string) (Progress, error)
}
```

Path: `.aos/tasks/{taskId}/todos/{id}.todo.md`. `{taskId}` e `{id}` vêm do caminho.

## Decisões e divergências

> [!decision] Campo `evidence`
> Adição. O prompt-mestre exige evidência de validação antes de concluir; sem um campo, a evidência fica só no comentário e não é consultável. `SetStatus(finished)` sem `evidence` gera aviso — não bloqueio, porque nem todo passo é verificável.

> [!decision] Status `skipped`
> Adição. Um passo que se tornou desnecessário não é "finished". Sem esse estado, o agente é empurrado a mentir ou a deixar pendente.

## Testes

- Round-trip com `taskId` e `id` extraídos do caminho
- `SetStatus` valida transição; `Update` rejeita escrita de status
- `CountPending` alimenta o `guardReview` de [[Task (Go)]]
- Delete da task cascateia os todos
- `Progress` bate com a contagem real
- Ordem preservada na listagem

## Critério de pronto

- [ ] CRUD completo com pai obrigatório
- [ ] `CountPending` integrado ao guard de `in_review`
- [ ] Checkpoint da task usando `pendingTodoIds` reais
