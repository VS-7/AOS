---
tags: [critico, persistencia, collections, markdown]
aliases: [Collections Engine, Motor de Coleções, Repository]
fase: 1
status: em-construcao
origem: "[[Modelo de Persistência]]"
---

# Collections Engine ★

> Pai: [[AOS]] · Origem no original: [[Modelo de Persistência]] · Decisão: [[ADR-0004 Collections em Markdown]] · Fase: 1

## Objetivo

Mapear **arquivo ↔ registro**: um `.memory.md` com frontmatter YAML no repositório do usuário é lido como struct Go tipada, e vice-versa. Substitui `@igniter-js/collections`.

## Comportamento do original

Um `IgniterCollectionModel` por entidade, com padrões de caminho, schema Zod e hooks de ciclo de vida ([[Modelo de Persistência]], [[Padrão Feature-Slice]]):

```ts
IgniterCollectionModel.create<"memories", Omit<Memory,"id">>("memories")
  .withPatterns([
    ".fractal/agents/{agent}/memories/{id}.memory.md",
    ".fractal/skills/*/agents/{agent}/memories/{id}.memory.md",
  ])
  .withSchema(MemorySchema.omit({ id: true }))
  .onCreated(({ value }) => ({ ...value, agent: value.agent?.toLowerCase(), ... }))
  .build();
```

Sete propriedades que herdamos:

1. **Placeholders extraem campos do caminho.** `{agent}` e `{id}` vêm do path, não do frontmatter.
2. **Frontmatter validado; corpo Markdown é `content`.** Os schemas fazem `.omit({ content: true, id: true })` justamente porque esses dois não vivem no YAML.
3. **Múltiplos patterns por coleção.** O segundo (`.fractal/skills/*/agents/...`) é o que permite uma [[Skill (Go)]] empacotar agentes e memórias próprios.
4. **Hooks** `onCreated` / `onUpdated` / `onDeleted` para normalização (lowercase de ID, timestamps, trim do corpo) e cascade delete (remover diretório inteiro de [[Task (Go)]], [[Skill (Go)]], [[Routine (Go)]]).
5. **Watcher** sobre `.fractal/` recarregando `**/collections/**/schema.{json,ts}` e `**/views/*.view.{json,ts}` com `autoWatch: true`.
6. **`refresh()`** varre o disco e indexa tudo na inicialização.
7. **13 coleções nativas** registradas estaticamente.

E três coisas que o original **não** faz e nós faremos: escrita atômica, lock por arquivo e detecção de escrita concorrente.

## Design em Go

### Modelo de coleção

```go
// internal/core/collections/model.go
package collections

type Format int

const (
	FormatMarkdown Format = iota // YAML frontmatter + Markdown body
	FormatJSON                   // whole file is the record (chats)
)

// Model declares how one entity maps to files on disk.
type Model[T any] struct {
	Name     string   // "memories"
	Patterns []string // ".aos/agents/{agent}/memories/{id}.memory.md"
	Format   Format

	// CascadeDir returns the directory to remove entirely on delete.
	// Nil means only the record file is removed.
	CascadeDir func(v *T) string

	OnCreated func(ctx context.Context, v *T) error
	OnUpdated func(ctx context.Context, old, new *T) error
	OnDeleted func(ctx context.Context, v *T) error
}
```

### O port

```go
// internal/core/collections/repository.go

type Repository[T any] interface {
	Get(ctx context.Context, key Key) (*T, error)
	List(ctx context.Context, q Query) ([]T, error)
	Create(ctx context.Context, v *T) error
	Update(ctx context.Context, v *T, expect Version) error
	Delete(ctx context.Context, key Key) error
}

// Key holds the placeholder values that identify a record. For memories that
// is {agent, id}; for tasks just {id}.
type Key map[string]string

// Version is the optimistic-concurrency token: modtime + size, captured on
// read and checked on write. Zero means "no expectation".
type Version struct {
	ModTime time.Time
	Size    int64
}
```

`Update` com `expect` diferente do estado em disco devolve `AOS_COLLECTION_CONFLICT` com CTA para recarregar e reaplicar — resolve o cenário de dois "agent universes" editando a mesma memória ([[ADR-0012 Escrita atômica e lock por arquivo]]).

### Padrões bidirecionais — a peça central

```go
// internal/core/collections/pattern.go

// Pattern compiles a path template into a bidirectional mapper:
//   ".aos/agents/{agent}/memories/{id}.memory.md"
// Reading:  path → map[string]string{"agent":"luara","id":"a1b2"}
// Writing:  map  → path
//
// A literal "*" segment matches one path element and captures nothing; it is
// how the skill-scoped variants work:
//   ".aos/skills/*/agents/{agent}/memories/{id}.memory.md"
type Pattern struct {
	raw    string
	re     *regexp.Regexp
	fields []string
	glob   string // for the initial filesystem walk
}

func Compile(raw string) (*Pattern, error)

// Match extracts placeholder values, or returns ok=false when the path does
// not belong to this pattern.
func (p *Pattern) Match(rel string) (Key, bool)

// Build renders a path from placeholder values, failing when one is missing —
// a missing placeholder must never silently produce a wrong path.
func (p *Pattern) Build(k Key) (string, error)

// Glob returns the walk pattern used by refresh(), so the engine never scans
// directories that cannot contain records of this collection.
func (p *Pattern) Glob() string
```

Regras de compilação:

| Elemento | Regex gerada | Captura |
|---|---|---|
| `{name}` | `([^/]+)` | sim, no campo `name` |
| `*` | `[^/]+` | não |
| `**` | `.+` | não |
| literal | escapado | — |

`Build` falha se faltar placeholder. Isso fecha uma classe inteira de bug: gravar um registro em caminho parcialmente resolvido.

### Serialização

```go
// internal/core/collections/codec.go

// Decode parses a Markdown file: YAML frontmatter into the struct fields,
// body into the field tagged `collection:"content"`, and path placeholders
// into the fields tagged `collection:"path"`.
func Decode[T any](data []byte, key Key, m Model[T]) (*T, error)

// Encode is the exact inverse. Fields tagged `collection:"path"` and
// `collection:"content"` are excluded from the frontmatter, mirroring the
// original's .omit({ id: true, content: true }).
func Encode[T any](v *T, m Model[T]) ([]byte, error)
```

Exemplo de entidade:

```go
// internal/domain/memory/entity.go

type Memory struct {
	ID    string `yaml:"-" json:"id"    collection:"path"`    // from {id}
	Agent string `yaml:"-" json:"agent" collection:"path"`    // from {agent}

	Title       string     `yaml:"title"       json:"title"`
	Description string     `yaml:"description" json:"description"`
	Category    Category   `yaml:"category"    json:"category"`
	Tags        []string   `yaml:"tags,omitempty" json:"tags,omitempty"`

	Confidence float64  `yaml:"confidence" json:"confidence"`
	Links      []string `yaml:"links,omitempty"      json:"links,omitempty"`
	Supersedes []Super  `yaml:"supersedes,omitempty" json:"supersedes,omitempty"`

	Status           Status     `yaml:"status" json:"status"`
	DeprecatedBy     string     `yaml:"deprecatedBy,omitempty"     json:"deprecatedBy,omitempty"`
	DeprecatedAt     *time.Time `yaml:"deprecatedAt,omitempty"     json:"deprecatedAt,omitempty"`
	DeprecatedReason string     `yaml:"deprecatedReason,omitempty" json:"deprecatedReason,omitempty"`

	Scopes    []string   `yaml:"scopes,omitempty"    json:"scopes,omitempty"`
	ExpiresAt *time.Time `yaml:"expiresAt,omitempty" json:"expiresAt,omitempty"`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt"`

	Content string `yaml:"-" json:"content" collection:"content"` // the Markdown body
}
```

Um arquivo real:

```markdown
---
title: Preferência de commit em inglês
category: preference
confidence: 0.9
scopes: ["**/*"]
status: active
createdAt: 2026-08-14T19:04:12-03:00
updatedAt: 2026-08-14T19:04:12-03:00
---

O usuário escreve commits em inglês, imperativo, sem escopo entre parênteses.
```

`ID` e `Agent` vêm de `.aos/agents/luara/memories/a1b2c3.memory.md`.

### Escrita — atômica, com lock

```go
// internal/adapters/fscollections/write.go

func (r *Repo[T]) Update(ctx context.Context, v *T, expect Version) error {
	path, err := r.pathFor(v)
	if err != nil {
		return err
	}
	return r.lock.With(ctx, path, func() error {
		if cur, err := statVersion(path); err == nil && !expect.IsZero() && cur != expect {
			return errConflict(path, expect, cur)
		}
		old, _ := r.readAt(path)
		if r.model.OnUpdated != nil {
			if err := r.model.OnUpdated(ctx, old, v); err != nil {
				return err
			}
		}
		data, err := Encode(v, r.model)
		if err != nil {
			return err
		}
		if err := WriteFileAtomic(path, data, 0o644); err != nil {
			return err
		}
		r.index.Invalidate(path)
		r.bus.Publish(ctx, collections.Changed{Collection: r.model.Name, Key: keyOf(v), Op: "update"})
		return nil
	})
}
```

Ordem invariante: **lock → CAS → hook → encode → escrita atômica → invalidar índice → publicar evento.**

### Watcher

```go
// internal/adapters/fscollections/watch.go

// Watch mirrors the original's withWatcher: it observes .aos/ and reloads
// dynamic schemas so an agent can create a collection and use it in the same
// session. Events are debounced (200ms) because editors write in bursts.
func (r *Repo[T]) Watch(ctx context.Context, roots []string, patterns WatchPatterns) error

type WatchPatterns struct {
	Collections string // "**/collections/**/schema.{json,yaml}"
	Views       string // "**/views/*.view.json"
	AutoWatch   bool
}
```

Um evento de mudança dispara: invalidação do índice em memória, reindexação no Bleve ([[ADR-0013 Bleve para busca full-text]]) e publicação em [[Realtime WebSocket]] para a UI atualizar.

**Ignorados sempre:** `.git/`, arquivos temporários da escrita atômica (`.tmp-*`), e o diretório de índice. Sem isso, a própria escrita dispara o watcher em laço.

### Índice em memória

```go
// internal/adapters/fscollections/index.go

// memIndex keeps the decoded frontmatter of every record so List with filters
// does not re-read the filesystem. Bodies are NOT cached — they can be large
// and are only needed on Get.
type memIndex struct {
	mu      sync.RWMutex
	byKey   map[string]entry
	byField map[string]map[string][]string // field → value → keys
}
```

Construído no `Refresh()` do boot, mantido pelo watcher. Vazio ou frio, `List` degrada para varredura com aviso — nunca falha.

### As 13 coleções nativas

`agents`, `skills`, `memories`, `templates`, `instructions`, `tasks`, `todos`, `comments`, `chats`, `routines`, `runs`, `projects`, `goals` — registradas estaticamente em `internal/core/collections/registry.go`, com os mesmos padrões do original (adaptados de `.fractal/` para `.aos/`).

### Escala

O índice em memória cobre workspaces até ~50.000 registros com folga. Acima disso, o caminho é o espelho SQLite avaliado e adiado em [[ADR-0004 Collections em Markdown]]. O gatilho para reabrir é medido, não intuído: `Refresh()` passando de 2 s no boot.

## Decisões e divergências

> [!decision] Escrita atômica e lock — o original não tem
> Defeitos #17 e #18. A concorrência é real: 20 workers de job, múltiplos "agent universes" e nove processos MCP observados. Ver [[ADR-0012 Escrita atômica e lock por arquivo]].

> [!decision] Versionamento otimista (CAS) — o original não tem
> Sem isso, dois agentes paralelos editando a mesma memória perdem uma edição em silêncio. O erro carrega CTA para recarregar e reaplicar.

> [!decision] Corpo Markdown fora do índice em memória
> O original indexa tudo. Corpos de task e de skill podem ter dezenas de KB, e o inventário do workspace só precisa de nomes ([[Prompt Assembly]]). Cachear corpo é gastar memória por workspace sem retorno.

> [!decision] `Build` falha em placeholder ausente
> Preferimos erro explícito a caminho parcialmente resolvido. É a diferença entre uma falha visível e um registro gravado no lugar errado.

## Testes

- **Round-trip por modelo (13):** criar → ler → comparar campo a campo, incluindo corpo Markdown com whitespace inicial e caracteres não-ASCII.
- **Pattern bidirecional:** tabela com todos os padrões nativos; `Build(Match(p)) == p` para todo caminho válido; `Match` rejeita caminhos de outra coleção; `Build` falha em placeholder ausente.
- **Padrão com `*`:** `.aos/skills/x/agents/luara/memories/a.memory.md` casa o padrão skill-scoped e extrai `{agent: luara, id: a}`.
- **Escrita atômica:** interromper entre write e rename (via hook de teste) deixa o arquivo anterior intacto e nenhum `.tmp-*` órfão após o `defer`.
- **Concorrência:** 50 goroutines escrevendo no mesmo caminho sob `-race`; nenhum arquivo corrompido; CAS detecta conflito.
- **Cascade delete:** apagar uma task remove o diretório com todos, comentários e runs.
- **Hooks:** `onCreated` normaliza `agent` para lowercase e preenche timestamps; `onUpdated` só toca `updatedAt`.
- **Watcher:** criar `schema.json` de coleção nova torna-a listável em menos de 1 s, sem restart.
- **Watcher não se auto-dispara:** uma escrita atômica não gera evento para o arquivo temporário.
- **Índice frio:** `List` sem índice devolve o mesmo resultado que com índice.

## Critério de pronto

- [ ] Criar, ler, atualizar e deletar um agente em Markdown pelo código
- [ ] Testes de round-trip verdes para os 13 modelos nativos
- [ ] Escrita atômica e lock verificados com `-race`
- [ ] Watcher recarregando schema dinâmico em menos de 1 s
- [ ] `Refresh()` de um workspace com 10.000 registros abaixo de 2 s
- [ ] Conflito de escrita concorrente detectado e reportado com CTA
