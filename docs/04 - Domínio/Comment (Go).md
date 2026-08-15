---
tags: [dominio, comment, task, colaboracao]
aliases: [Comment Go, Comentário]
fase: 6
status: especificado
origem: "[[Comment]]"
---

# Comment (Go)

> Pai: [[Task (Go)]] · Origem no original: [[Comment]] · Fase: 6

## Objetivo

Discussão e feedback dentro de uma [[Task (Go)]] — e o **canal de comunicação obrigatório** de um agente em modo task.

## Comportamento do original

Duas regras que fazem do comentário mais do que um campo de texto ([[Comment]]):

**Autoria server-side.** *"Comment authorship is server-side — use `tasks comment create` with `body` only. Agents may only update/delete their own comments (`FRACTAL_COMMENT_FORBIDDEN` otherwise)."* O agente não forja autoria: o autor vem da identidade da requisição, não do payload. A engenharia reversa registra que essa é uma das poucas restrições de autorização realmente aplicadas no domínio.

**Uso obrigatório em modo task.** *"Use task comments for all progress communication, not chat messages."* A razão: quando o agente executa autonomamente, o usuário não está no chat. O comentário é o registro durável e localizável.

Suporta `parentId` — threads aninhadas.

## Design em Go

```go
// internal/domain/comment/entity.go

type Comment struct {
	TaskID string `yaml:"-" json:"taskId" collection:"path"`
	ID     string `yaml:"-" json:"id"     collection:"path"`

	// Author is set from the ambient identity, never from the payload.
	Author     string `yaml:"author"     json:"author"`
	AuthorType string `yaml:"authorType" json:"authorType"` // agent | user

	ParentID string `yaml:"parentId,omitempty" json:"parentId,omitempty"`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt"`

	Content string `yaml:"-" json:"content" collection:"content"`
}
```

```go
// CreateInput deliberately has no author field. Adding one would be the bug.
type CreateInput struct {
	TaskID    string `json:"taskId"   jsonschema:"Parent task identifier" validate:"required"`
	Body      string `json:"body"     jsonschema:"Comment body in Markdown" validate:"required"`
	ParentID  string `json:"parentId,omitempty" jsonschema:"Parent comment, for threaded replies"`
	Reasoning string `json:"_reasoning" validate:"required,min=1"`
}

// guardOwnership enforces that an agent only edits or deletes what it wrote.
func (s *service) guardOwnership(ctx context.Context, c *Comment) error {
	actor, kind := identity.Actor(ctx)
	if c.Author != actor || c.AuthorType != string(kind) {
		return errForbidden(c.ID, actor)
	}
	return nil
}
```

Path: `.aos/tasks/{taskId}/comments/{id}.comment.md`.

## Decisões e divergências

> [!decision] Autoria server-side, sem exceção
> Herdado sem alteração. É o que torna o histórico de discussão de uma task atribuível de forma confiável, e um dos poucos pontos onde o original aplica autorização de verdade.

> [!decision] Threads limitadas a um nível
> `parentId` aponta para um comentário de topo; responder a uma resposta anexa à mesma thread. O prompt já orienta *"post progress updates in-thread, not scattered as new top-level comments"* — profundidade arbitrária não acrescenta e complica a renderização.

## Testes

- Autor derivado da identidade ambiente; campo `author` no payload é ignorado
- Agente A não edita nem apaga comentário do agente B (`AOS_COMMENT_FORBIDDEN`)
- Usuário `super` pode moderar (exceção explícita, testada)
- `parentId` inexistente é rejeitado
- Resposta a uma resposta anexa à thread de topo
- Delete da task cascateia os comentários

## Critério de pronto

- [ ] Autoria server-side aplicada e testada
- [ ] Restrição de propriedade em update e delete
- [ ] Threads de um nível funcionando
