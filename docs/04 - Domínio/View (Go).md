---
tags: [dominio, view, ui, declarativo]
aliases: [View Go, Interface Adaptativa]
fase: 8
status: especificado
origem: "[[View]]"
---

# View (Go)

> Pai: [[Collection (Go)]] · Origem no original: [[View]] · Ver: [[Views Declarativas]] · Fase: 8

## Objetivo

Interfaces declarativas sobre dados do workspace — tabelas, dashboards, pipelines, kanban — definidas em JSON e renderizadas no design system, **sem build e sem deploy**.

## Comportamento do original

Persistida em `.fractal/views/{id}.view.json`; o watcher observa `**/views/*.view.{json,ts}` com `autoWatch: true`, então criar uma view a torna renderizável imediatamente ([[View]]).

Duas tools de introspecção importam: `views_components` (catálogo de componentes disponíveis) e `views_registry`. O agente **descobre quais componentes existem** antes de compor uma tela — o equivalente a ler a documentação do design system.

A tese está no prompt: *"Create views when data needs a visible, actionable surface — they let users operate on records directly without chat."* A conversa não deve ser a única interface.

## Design em Go

```go
// internal/domain/view/entity.go

type View struct {
	ID string `json:"id" collection:"path"`

	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Scope       string `json:"scope"` // workspace | skill
	Skill       string `json:"skill,omitempty"`

	// Source binds the view to its data.
	Source Source `json:"source"`

	// Tree is the declarative component tree, validated against the registry.
	Tree Node `json:"tree"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Source struct {
	Collection string         `json:"collection"`
	Filter     map[string]any `json:"filter,omitempty"`
	Sort       []SortSpec     `json:"sort,omitempty"`
	Limit      int            `json:"limit,omitempty"`
}

// Node is one component instance. Props are validated against the component's
// declared schema in the registry — an unknown component or a bad prop is a
// write-time error, not a blank screen at render time.
type Node struct {
	Component string          `json:"component"`
	Props     map[string]any  `json:"props,omitempty"`
	Bind      map[string]string `json:"bind,omitempty"` // prop → field path
	Children  []Node          `json:"children,omitempty"`
	Actions   []Action        `json:"actions,omitempty"`
}

// Action is what makes a view operational rather than decorative: a button
// that calls a command from the registry.
type Action struct {
	Label   string         `json:"label"`
	Command string         `json:"command"` // a Descriptor key, e.g. "tasks_set_status"
	Input   map[string]any `json:"input,omitempty"`
	Confirm bool           `json:"confirm,omitempty"`
}
```

```go
type Service interface {
	List(ctx context.Context, q Query) ([]View, error)
	Get(ctx context.Context, id string) (*View, error)
	Create(ctx context.Context, in CreateInput) (*View, error)
	Delete(ctx context.Context, id string) error

	// Render resolves the data source and returns the tree with data attached,
	// ready for the frontend renderer.
	Render(ctx context.Context, id string, params RenderParams) (Rendered, error)

	// Components returns the component catalog — the introspection tool the
	// agent calls before composing a screen.
	Components(ctx context.Context) ([]ComponentSpec, error)

	// Scaffold generates a starting view for a collection, inferring sensible
	// components from the field types.
	Scaffold(ctx context.Context, collectionID string, kind Kind) (*View, error)
}
```

### Registry de componentes

```go
// internal/domain/view/registry.go

// ComponentSpec is the contract between the Go domain and the React design
// system. It is generated from the frontend at build time so the two cannot
// drift: a component removed in React fails the Go build.
type ComponentSpec struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Category    string      `json:"category"` // layout | data | input | feedback
	Props       []PropSpec  `json:"props"`
	AcceptsChildren bool    `json:"acceptsChildren"`
}
```

Ver [[Views Declarativas]] para o lado do frontend.

### Validação na escrita

```go
// Validate checks the tree against the registry before the view is persisted:
// unknown component, missing required prop, wrong prop type, binding to a
// field the source collection does not have, or an action naming a command
// that is not registered. A view that would render blank never gets written.
func (s *service) Validate(ctx context.Context, v *View) error
```

## Decisões e divergências

> [!decision] Validação na escrita, não na renderização
> O original valida ao renderizar (`FRACTAL_VIEW_RENDER_ERROR`, `FRACTAL_VIEW_INVALID_CONFIGURATION`). Aqui, uma view inválida **não é gravada**, e o erro nomeia o componente e a prop — o agente corrige na hora em vez de descobrir depois que a tela está em branco.

> [!decision] `ComponentSpec` gerado do frontend
> Divergência de processo. No original, o registry de componentes vive no frontend e o backend não o conhece — nada impede o agente de referenciar um componente inexistente. Aqui, o catálogo é gerado no build (`task gen-components`) e um teste falha se o gerado divergir do commitado.

> [!decision] `Actions` referenciam comandos do registry
> Um botão numa view chama um `Descriptor`, com a mesma validação e a mesma autorização de qualquer outra superfície. A view não é um caminho paralelo para mutação.

## Testes

- Criar view por agente e renderizar na mesma sessão
- Componente inexistente rejeitado na escrita, nomeando-o
- Prop obrigatória ausente e prop de tipo errado rejeitadas
- `bind` para campo inexistente na coleção rejeitado
- `action` com comando não registrado rejeitada
- `Components` bate com o catálogo gerado do frontend
- `Scaffold` de uma coleção produz view válida e renderizável
- View com escopo `skill` removida ao desinstalar a skill

## Critério de pronto

- [ ] View criada por agente renderizando na UI sem restart
- [ ] Validação na escrita contra o registry
- [ ] Catálogo de componentes sincronizado com o frontend por geração
- [ ] Ações operando sobre comandos do registry
