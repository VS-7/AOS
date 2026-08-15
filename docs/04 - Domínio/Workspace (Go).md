---
tags: [dominio, workspace, core]
aliases: [Workspace Go]
fase: 3
status: pronto
origem: "[[Workspace]]"
---

# Workspace (Go)

> Pai: [[AOS]] · Origem no original: [[Workspace]] · Fase: 3

## Objetivo

A fronteira operacional. Todo recurso vive dentro de um workspace, e um mesmo processo serve N workspaces isolados.

## Comportamento do original

Dois diretórios por workspace ([[Workspace]]): metadados centralizados em `~/.fractal/workspaces/{id}/config.json`, dados **dentro do repositório do usuário** em `{path}/.fractal/`. É essa separação que permite versionar o estado do agente junto do código.

Pontos herdados:
- **O orquestrador nasce com o workspace** — criar um workspace já cria seu [[Agent (Go)]] orquestrador, com `tone`, `style` e `autonomy` que viram texto no prompt.
- **Tipos de task com instruções próprias** — `FractalWorkspaceTaskTypeSchema` aceita `instructions`, então uma task `bug` injeta orientação diferente de uma `docs`.
- **Scaffolding** cria `.fractal/{agents,skills,instructions,templates,collections}` com `.gitkeep`, roda `git init` e insere bloco gerenciado com `FRACTAL_WORKSPACE_ID` em `.env` e `.env.sample`, por *splice* (preservando o conteúdo do usuário).
- **`introspect`** auto-registra o repositório atual, derivando o nome do remote do Git. **Corrigido na Fase 3:** esta nota afirmava que `introspect` devolve o inventário completo, o que é falso — tanto o fonte extraído quanto a descrição da tool no binário instalado dizem "Auto-register a Fractal workspace from the current Git repository". Ver a decisão abaixo.
- **`tick`** varre todos os workspaces a cada 15 min, recovery-first.

## Design em Go

```go
// internal/domain/workspace/entity.go

type Workspace struct {
	ID    string `yaml:"-" json:"id" collection:"path"`
	Name  string `yaml:"name"  json:"name"`
	Path  string `yaml:"path"  json:"path"`
	Logo  string `yaml:"logo,omitempty"  json:"logo,omitempty"`
	Color string `yaml:"color,omitempty" json:"color,omitempty"`

	Tasks  []TaskType `yaml:"tasks"  json:"tasks"`
	Labels []Label    `yaml:"labels" json:"labels"`

	Worktrees WorktreeOptions `yaml:"worktrees" json:"worktrees"`
	Git       GitOptions      `yaml:"git"       json:"git"`

	Members []Member `yaml:"members,omitempty" json:"members,omitempty"`
	Domains []string `yaml:"domains,omitempty" json:"domains,omitempty"`

	Archived  bool      `yaml:"archived"  json:"archived"`
	CreatedAt time.Time `yaml:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt"`
}

type TaskType struct {
	ID           string `yaml:"id"    json:"id"`
	Label        string `yaml:"label" json:"label"`
	Color        string `yaml:"color" json:"color"`
	Instructions string `yaml:"instructions,omitempty" json:"instructions,omitempty"`
}

type WorktreeOptions struct {
	DeleteOldWorktrees bool   `yaml:"deleteOldWorktrees" json:"deleteOldWorktrees"`
	WorktreeLimit      int    `yaml:"worktreeLimit"      json:"worktreeLimit"`
	OnCreateScript     string `yaml:"onCreateScript"     json:"onCreateScript"`
}

type GitOptions struct {
	BranchPrefix       string `yaml:"branchPrefix"       json:"branchPrefix"`
	ForcePush          bool   `yaml:"forcePush"          json:"forcePush"`
	CommitInstructions string `yaml:"commitInstructions" json:"commitInstructions"`
	PRInstructions     string `yaml:"prInstructions"     json:"prInstructions"`
}
```

Defaults idênticos aos do original — cinco tipos de task (`feature`, `bug`, `refactor`, `docs`, `config`), labels vazias, `branchPrefix: "aos"`, `worktreeLimit: 15`.

```go
// internal/domain/workspace/service.go

type Service interface {
	List(ctx context.Context) ([]Workspace, error)
	Get(ctx context.Context, id string) (*Workspace, error)
	Create(ctx context.Context, in CreateInput) (*Workspace, error)
	Update(ctx context.Context, in UpdateInput) (*Workspace, error)
	Delete(ctx context.Context, id string) error

	// Introspect returns the full inventory an agent needs to orient itself.
	Introspect(ctx context.Context, id string) (Inventory, error)

	// Resolve builds (and caches) the runtime bound to one workspace.
	Resolve(ctx context.Context, id string) (*Runtime, error)

	// Tick fans out recovery and due-work jobs across every workspace.
	Tick(ctx context.Context, runID string) (TickReport, error)
}

type CreateInput struct {
	Name         string             `json:"name"          jsonschema:"Workspace name" validate:"required"`
	Path         string             `json:"path,omitempty" jsonschema:"Absolute path to an existing repository; omit to create under the state dir"`
	Orchestrator *OrchestratorSpec  `json:"orchestrator,omitempty" jsonschema:"Tone, style and autonomy of the orchestrator agent created with the workspace"`
	Reasoning    string             `json:"_reasoning" validate:"required,min=1"`
}

type OrchestratorSpec struct {
	Name     string  `json:"name"     jsonschema:"Orchestrator agent name"`
	Tone     string  `json:"tone"     jsonschema:"efficient | friendly | professional | candid"`
	Style    string  `json:"style"    jsonschema:"concise | balanced | detailed"`
	Autonomy float64 `json:"autonomy" jsonschema:"0..1 — modulates the autonomy matrix in the system prompt" validate:"gte=0,lte=1"`
}
```

### Scaffolding

```go
// scaffold creates the workspace skeleton. Every step is idempotent: running it
// on an existing repository adds what is missing and touches nothing else.
func (s *service) scaffold(ctx context.Context, ws *Workspace) error {
	dirs := []string{"agents", "skills", "instructions", "templates", "collections", "views"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(ws.Path, build.StateDir, d), 0o755); err != nil {
			return err
		}
		_ = touch(filepath.Join(ws.Path, build.StateDir, d, ".gitkeep"))
	}
	if err := spliceManagedBlock(filepath.Join(ws.Path, ".env"), managedEnv(ws)); err != nil {
		return err
	}
	if err := spliceManagedBlock(filepath.Join(ws.Path, ".env.sample"), managedEnvSample()); err != nil {
		return err
	}
	return gitInitIfAbsent(ctx, ws.Path) // failure is logged, never fatal
}

// spliceManagedBlock replaces only the delimited managed section, preserving
// whatever the user wrote around it.
func spliceManagedBlock(path, block string) error
```

### Runtime resolvido

Ver [[Visão Geral Go]] para `Runtime` e o uso de `singleflight`.

## Decisões e divergências

> [!decision] `members` e `domains` no schema
> O original tem esses campos no disco mas **ausentes do schema Zod** — resquício da migração multi-usuário. Declaramos no tipo desde o início.

> [!decision] `git init` falha de forma visível
> No original, o erro é silenciosamente ignorado. Aqui é logado e reportado no resultado de `create` como aviso — o usuário precisa saber que o workspace não está versionado.

> [!decision] Tick com concorrência limitada
> O original faz fan-out sem limite declarado. Ver [[Concorrência e Context]].

> [!decision] `introspect` registra; `inventory` inventaria
> A nota herdou do vault a afirmação de que `workspace introspect` devolve o inventário do workspace. Não devolve: no original ele auto-registra o diretório atual como workspace. Verificado em `workspace/commands/workspace/introspect.ts` e na descrição publicada da tool `workspace_introspect` do binário instalado.
>
> Em vez de escolher em silêncio entre a função real e a que a nota descrevia, os dois existem com nomes distintos: `workspace introspect` mantém a semântica do original ([[ADR-0016 Compatibilidade de nomes com o original]]) e `workspace inventory` é a visão panorâmica que o [[Prompt Assembly]] precisa — a que esta nota chamava de `Introspect`. Nome confirmado com o usuário na Fase 3.

> [!decision] Scaffolding atrás de um porto, não com `os` direto
> O esboço desta nota chama `os.MkdirAll` dentro do serviço, o que a regra de dependência permite. Ficou atrás de `Scaffolder` porque as duas propriedades que importam — o scaffolding é idempotente, e o splice preserva o que o usuário escreveu — seriam provadas contra um diretório temporário em vez de contra a regra. O `Scaffolder` tem suíte de contrato, e o fake e o adaptador real passam pela mesma.

> [!decision] `update` por caminho pontilhado, e não substituição
> O original faz merge raso do payload inteiro. Aqui `workspace update` recebe `set` com caminhos (`git.branchPrefix`), como `config update` — dois escritores concorrentes mudando campos diferentes não se sobrescrevem. `id`, `path` e `createdAt` são do servidor: um patch que os alcançasse órfãos todo registro que referencia o workspace.

## Testes

- Round-trip de `Workspace` com todos os campos
- Scaffolding idempotente: rodar duas vezes não duplica nem sobrescreve
- Splice do `.env`: conteúdo do usuário antes e depois do bloco preservado
- `git init` ausente → aviso no resultado, workspace criado
- Criar workspace cria o orquestrador com tom/estilo/autonomia refletidos no `agent.md`
- `Resolve` concorrente constrói uma vez
- `Tick` com um workspace quebrado não impede os demais
- `Introspect` devolve só nomes, nunca corpos

## Critério de pronto

- [x] Criar workspace apontando para repositório existente injeta `.aos/` sem tocar no resto — `TestEnvSplicePreservesWhatTheUserWrote`
- [x] Orquestrador nasce com o workspace — `TestTheOrchestratorIsBornWithTheWorkspace`
- [x] `inventory` alimentando o [[Prompt Assembly]] — nomes e contagens, nunca corpos
- [ ] Tick com relatório observável (`scanned`, `dispatched`, `failed`, `workspaces`) — **Fase 6**, junto com a fila

## Saída dos testes — Fase 3

```
$ go test -race ./internal/domain/workspace/ ./internal/adapters/fsworkspace/
ok  	github.com/OWNER/aos/internal/domain/workspace
ok  	github.com/OWNER/aos/internal/adapters/fsworkspace
```

Os casos que a nota lista, e onde estão:

| Caso da nota | Teste |
|---|---|
| Round-trip de `Workspace` com todos os campos | `TestStoreRoundTripsEveryField` |
| Scaffolding idempotente | `TestScaffoldIsIdempotent` |
| Splice do `.env` preserva o conteúdo do usuário | `TestEnvSplicePreservesWhatTheUserWrote`, `TestEnvSpliceReplacesOnlyTheManagedBlock` |
| `git init` ausente → aviso no resultado | `TestGitFailureIsVisibleAndNotFatal` |
| Orquestrador com tom/estilo/autonomia no `AGENT.md` | `TestOrchestratorDialsBecomeProse` |
| `Introspect` devolve só nomes, nunca corpos | `TestInventoryCarriesNoBodies` |
| Entrega ponta a ponta, sobre disco real | `TestTheDeliveryOfThePhase` (em `internal/app`) |

**Não verificado:** `Resolve` concorrente com `singleflight` e `Tick` com um workspace quebrado — as duas dependem do runtime multi-workspace, que chega na Fase 4 com o daemon. O `Runtime` desta fase é um `repoSet` construído por raiz na raiz de composição, sem cache.
