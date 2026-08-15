---
tags: [dominio, workspace, core]
aliases: [Workspace Go]
fase: 3
status: especificado
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
- **`introspect`** dá ao agente o inventário completo.
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

- [ ] Criar workspace apontando para repositório existente injeta `.aos/` sem tocar no resto
- [ ] Orquestrador nasce com o workspace
- [ ] `introspect` alimentando o [[Prompt Assembly]]
- [ ] Tick com relatório observável (`scanned`, `dispatched`, `failed`, `workspaces`)
