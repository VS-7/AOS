---
tags: [dominio, collection, dados, schema-dinamico]
aliases: [Collection Go, Coleção Customizada]
fase: 8
status: especificado
origem: "[[Collection]]"
---

# Collection (Go)

> Pai: [[Workspace (Go)]] · Origem no original: [[Collection]] · Ver: [[View (Go)]] · Fase: 8

## Objetivo

Dados estruturados de domínio definidos **em runtime pelo agente**. É o que permite "monte um CRM" virar tabelas reais sem programador.

## Comportamento do original

Escopo `workspace` ou `skill`; formato `json` ou `md` ([[Collection]]). Persistida em `.fractal/collections/{id}/schema.json` com registros em `records/`.

O watcher observa `**/collections/**/schema.{json,ts}` com `autoWatch: true` — quando o agente cria uma coleção, **o schema é carregado imediatamente** e os comandos de registro funcionam na mesma sessão.

Um detalhe que merece atenção: o schema de criação aceita **strings de código-fonte para hooks** (`onCreated`, `onUpdated`, `onDeleted`), escritas no `schema.ts` gerado. Ou seja: o agente pode gerar código de hook para a coleção que está criando.

Papel no prompt: *"Create collections when the workspace needs structured domain data — contacts, deals, articles, inventory."* Combinada com [[View (Go)]], forma o par **dados + interface**.

## Design em Go

```go
// internal/domain/collection/entity.go

type Scope string
const (
	ScopeWorkspace Scope = "workspace"
	ScopeSkill     Scope = "skill"
)

type Format string
const (
	FormatJSON     Format = "json"
	FormatMarkdown Format = "md"
)

type Collection struct {
	ID string `json:"id" collection:"path"`

	Name        string `json:"name"`
	Description string `json:"description"`
	Scope       Scope  `json:"scope"`
	Skill       string `json:"skill,omitempty"`
	Format      Format `json:"format"`

	// Fields is the declared schema. It is a constrained subset of JSON Schema:
	// enough to describe records, not enough to be a programming language.
	Fields []Field `json:"fields"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Field struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // string | number | boolean | date | enum | ref | list
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Enum        []string `json:"enum,omitempty"`
	Ref         string   `json:"ref,omitempty"` // another collection id
	Default     any      `json:"default,omitempty"`
	Unique      bool     `json:"unique,omitempty"`
}
```

```go
type Service interface {
	List(ctx context.Context, q Query) ([]Collection, error)
	Get(ctx context.Context, id string) (*Collection, error)
	Create(ctx context.Context, in CreateInput) (*Collection, error)
	Delete(ctx context.Context, id string) error

	Records() RecordService
}

type RecordService interface {
	List(ctx context.Context, collectionID string, q RecordQuery) ([]Record, error)
	Get(ctx context.Context, collectionID, id string) (*Record, error)
	Create(ctx context.Context, collectionID string, data map[string]any) (*Record, error)
	Update(ctx context.Context, collectionID, id string, data map[string]any) (*Record, error)
	Delete(ctx context.Context, collectionID, id string) error
}
```

### Validação dinâmica

```go
// internal/domain/collection/validate.go

// Validate checks a record against the declared fields. The schema is data,
// not code: there is no evaluation step, which is exactly why an
// agent-authored schema is safe to load at runtime.
func Validate(c Collection, data map[string]any) error
```

### Nomes reservados

```go
// reserved prevents a custom collection from shadowing a native one, which
// would break the engine's registry.
var reserved = []string{
	"agents", "skills", "memories", "templates", "instructions",
	"tasks", "todos", "comments", "chats", "routines", "runs",
	"projects", "goals",
}
```

## Decisões e divergências

> [!decision] Hooks de coleção não são código
> **Divergência importante.** O original aceita strings de código-fonte para `onCreated`/`onUpdated`/`onDeleted` e as grava num `schema.ts` — o que significa um agente gerando código executável no workspace, sem sandbox nem revisão.
>
> Aqui, hooks são **declarativos**: um pequeno conjunto de ações nomeadas (`setTimestamp`, `slugify`, `defaultTo`, `computeFrom`) com parâmetros. Cobre os casos reais observados (normalização e timestamps) sem abrir um vetor de execução arbitrária.
>
> Se um caso legítimo exigir lógica além disso, o caminho é uma [[Routine (Go)]] com trigger `activity` — que passa por [[Sandbox (Go)]] e é auditada.

> [!decision] Schema é dado, nunca avaliado
> Consequência direta da decisão acima: carregar um schema criado por agente não executa nada.

> [!decision] Campo `ref` entre coleções
> Adição. Um CRM precisa ligar `deals` a `contacts`. Sem referência declarada, a [[View (Go)]] não consegue renderizar relação.

## Testes

- Criar coleção em runtime e criar registro na mesma sessão, sem restart
- Nome reservado é rejeitado
- Validação: campo obrigatório ausente, enum inválido, tipo errado, `unique` duplicado
- `ref` apontando para coleção inexistente é rejeitado
- Hooks declarativos aplicam normalização e timestamps
- Watcher detecta schema novo em menos de 1 s
- Coleção com escopo `skill` é removida ao desinstalar a skill

## Critério de pronto

- [ ] Coleção criada por agente utilizável na mesma sessão
- [ ] Validação dinâmica completa
- [ ] Hooks declarativos, sem execução de código
- [ ] Referências entre coleções funcionando
