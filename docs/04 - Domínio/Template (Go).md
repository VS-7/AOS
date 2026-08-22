---
tags: [dominio, template, liquid]
aliases: [Template Go]
fase: 8
status: pronto
origem: "[[Template]]"
---

# Template (Go)

> Pai: [[Workspace (Go)]] · Origem no original: [[Template]] · Decisão: [[ADR-0014 Liquid para templates]] · Fase: 8

## Objetivo

Geradores reutilizáveis para trabalho com forma repetível — briefs de task, planos, relatórios, e-mails.

## Estado atual

`templates_render` grava em disco, mas só quando o chamador pede: `write:true`
em `RenderInput` (default `false` — a única fronteira do sistema onde Liquid
de terceiro roda contra dados do chamador não escreve nada por padrão).
`Output` é ele mesmo Liquid, renderizado pelo mesmo `render()` limitado por
timeout/tamanho que `Content` usa, e o caminho resolvido é confinado ao
workspace ativo (`template.Workspaces`/`template.Files`, ligados em `wire.go`
ao mesmo `workspaceRoot`/`osfile.New()` que o file explorer já usa — não uma
segunda implementação). A anotação `ReadOnlyHint` do comando `render` foi
removida por causa disso: o canal de aprovação deriva o risco dela
([[ADR-0007 Canal real de aprovação de tool]]), e um comando que pode
escrever não pode se anunciar como somente-leitura.

## Comportamento do original

Motor LiquidJS ([[Template]]). Papel no prompt:

> *"Use a template when work has a recognizable, repeatable shape. Inspect a template's schema before rendering to know the variables it expects."*

A instrução "inspecione o schema antes de renderizar" segue o mesmo padrão *inspect-before-execute* das tools compostas.

Uma [[Task (Go)]] tem campo `template` — tasks podem ser geradas a partir de um template.

## Design em Go

```go
// internal/domain/template/entity.go

type Template struct {
	ID string `yaml:"-" json:"id" collection:"path"`

	Name        string `yaml:"name"        json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Skill       string `yaml:"skill,omitempty"       json:"skill,omitempty"`

	// Variables is the declared contract: what the template expects. It is what
	// `templates get` returns so the agent can inspect before rendering.
	Variables []Variable `yaml:"variables,omitempty" json:"variables,omitempty"`

	// Output declares where a render should land, when the template generates
	// a file rather than a string.
	Output string `yaml:"output,omitempty" json:"output,omitempty"`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt"`

	Content string `yaml:"-" json:"content" collection:"content"` // the Liquid template
}

type Variable struct {
	Name        string `yaml:"name"        json:"name"`
	Type        string `yaml:"type"        json:"type"` // string | number | boolean | list | object
	Description string `yaml:"description" json:"description"`
	Required    bool   `yaml:"required"    json:"required"`
	Default     any    `yaml:"default,omitempty" json:"default,omitempty"`
}
```

```go
type Service interface {
	List(ctx context.Context, q Query) ([]Template, error)
	Get(ctx context.Context, id string) (*Template, error)
	Create(ctx context.Context, in CreateInput) (*Template, error)
	Update(ctx context.Context, in UpdateInput) (*Template, error)
	Delete(ctx context.Context, id string) error

	// Render validates the variables against the declared contract, then runs
	// the template under a timeout and an output size cap.
	Render(ctx context.Context, in RenderInput) (RenderResult, error)
}
```

### Renderização contida

```go
// internal/domain/template/render.go

const (
	renderTimeout   = 5 * time.Second
	maxOutputBytes  = 4 << 20 // 4 MB
)

// Render is the one place in the system where user-authored Liquid actually
// executes. It is bounded in time and size, and it never sees config, secrets,
// or the process environment — only the variables the caller passed and a
// small allowlisted context.
func (s *service) Render(ctx context.Context, in RenderInput) (RenderResult, error) {
	t, err := s.repo.Get(ctx, Key{"id": in.ID})
	if err != nil {
		return RenderResult{}, err
	}
	if err := validateVariables(t.Variables, in.Variables); err != nil {
		return RenderResult{}, err // carries a CTA pointing at `templates get`
	}

	ctx, cancel := context.WithTimeout(ctx, renderTimeout)
	defer cancel()

	out, err := s.engine.Render(ctx, t.Content, s.safeVars(in.Variables))
	if err != nil {
		return RenderResult{}, errRenderFailed(t.ID, err)
	}
	if len(out) > maxOutputBytes {
		return RenderResult{}, errOutputTooLarge(t.ID, len(out))
	}
	return RenderResult{Output: out}, nil
}
```

## Decisões e divergências

> [!decision] `variables` declarado, validado antes de renderizar
> O original tem "schema de variáveis" mencionado na descrição mas o contrato não é explícito no entity. Aqui é, e a validação acontece antes — o erro de variável faltando aponta para `templates get`, seguindo o padrão inspect-before-execute.

> [!decision] Timeout e teto de saída
> Um template com laço patológico não pode travar o servidor nem estourar a memória. O original não limita.

> [!decision] Este é o único lugar onde Liquid roda sobre entrada de usuário
> O caminho do prompt **não** renderiza dados persistidos ([[ADR-0014 Liquid para templates]]). A separação é estrutural: `internal/runtime/prompt` não importa este pacote.

## Testes

- Round-trip com `variables` declaradas
- Render com variável obrigatória ausente falha com CTA para `templates get`
- Template com laço infinito é interrompido pelo timeout
- Saída acima de 4 MB é rejeitada
- Variáveis não incluem `config` nem ambiente (teste tentando `{{ config.security.secret }}`)
- Golden por filtro Liquid suportado, documentando divergências de `osteele/liquid` em relação ao LiquidJS

## Critério de pronto

- [x] CRUD + render funcionando — `TestRenderWithWriteWritesTheRenderedOutputAtTheLiquidRenderedOutputPath`
- [x] Contrato de variáveis validado antes de renderizar — `validateVariables` (`render.go`); erro nomeia a variável e aponta para `templates get`
- [x] Timeout e teto de saída aplicados — `renderTimeout`/`maxOutputBytes` (`render.go`)
- [x] Divergências de filtro documentadas com golden — `TestFilterGoldens`
