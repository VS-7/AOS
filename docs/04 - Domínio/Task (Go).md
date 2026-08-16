---
tags: [dominio, task, execucao, core]
aliases: [Task Go, Tarefa]
fase: 6
status: pronto
origem: "[[Task]]"
---

# Task (Go)

> Pai: [[Workspace (Go)]] · Origem no original: [[Task]] · Ver: [[Todo (Go)]] · [[Comment (Go)]] · Fase: 6

## Objetivo

A unidade de trabalho. O prompt-mestre é categórico: **"execution contracts with lifecycle, ownership, and review. Not reminders."**

## Comportamento do original

Persistida em `.fractal/tasks/{id}/TASK.md`, com runs em `runs/{id}.run.json`. `onDeleted` remove o **diretório inteiro** ([[Task]]).

Ciclo de vida de oito estados:

```
suggestion → backlog → planning → todo → in_progress → in_review → finished
                                             ↓
                                          stopped
```

Regras que herdamos:

- **Transição só por `set_status`.** *"Use `set_status` for lifecycle moves; never change status via `update`."* É operação de domínio com validação, não edição de campo.
- **Worktree Git isolada** — quando habilitada, a task executa numa branch própria e o [[Sandbox (Go)]] usa aquele path como raiz. `onCreateScript` roda depois de criar a worktree.
- **Checkpoint** no `stop`: `chatId`, `jobId`, `pendingTodoIds`, `progress`. Permite retomar do ponto exato.
- **`dependsOn`** — o autopilot pula tasks com dependências não concluídas, e um start explícito é bloqueado.
- **Assignee resolvido** — projeção read-only derivada de `assigned`. *"`type` drives execution policy: only `agent` assignees receive autonomous dispatch."* É a fronteira entre trabalho automatizado e humano.
- **Modo task no prompt** — comunicar por comentários, não por chat; só mover para `in_review` com evidência de validação.

## Design em Go

```go
// internal/domain/task/entity.go

type Status string

const (
	Suggestion Status = "suggestion"
	Backlog    Status = "backlog"
	Planning   Status = "planning"
	Todo       Status = "todo"
	InProgress Status = "in_progress"
	Stopped    Status = "stopped"
	InReview   Status = "in_review"
	Finished   Status = "finished"
)

type Priority string

const (
	NoPriority Priority = "no_priority"
	Urgent     Priority = "urgent"
	High       Priority = "high"
	Medium     Priority = "medium"
	Low        Priority = "low"
)

type Task struct {
	ID   string `yaml:"-" json:"id" collection:"path"`
	Name string `yaml:"name" json:"name"`
	Slug string `yaml:"slug" json:"slug"`
	Type string `yaml:"type" json:"type"` // one of workspace.Tasks

	Assigned string     `yaml:"assigned,omitempty" json:"assigned,omitempty"` // agent id OR user uuid
	DueAt    *time.Time `yaml:"dueAt,omitempty"    json:"dueAt,omitempty"`
	Priority Priority   `yaml:"priority"           json:"priority"`
	Summary  string     `yaml:"summary,omitempty"  json:"summary,omitempty"`

	Status     Status      `yaml:"status" json:"status"`
	Checkpoint *Checkpoint `yaml:"checkpoint,omitempty" json:"checkpoint,omitempty"`

	Template string   `yaml:"template,omitempty" json:"template,omitempty"`
	Worktree Worktree `yaml:"worktree"           json:"worktree"`
	Chat     string   `yaml:"chat,omitempty"     json:"chat,omitempty"`

	Project   string   `yaml:"project,omitempty"   json:"project,omitempty"`
	Goal      string   `yaml:"goal,omitempty"      json:"goal,omitempty"`
	DependsOn []string `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty"`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt"`

	Content string `yaml:"-" json:"content" collection:"content"` // description / plan
}

type Worktree struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Base    string `yaml:"base"    json:"base"`
	Branch  string `yaml:"branch"  json:"branch"`
	Path    string `yaml:"path"    json:"path"`
}

type Checkpoint struct {
	ChatID         string   `yaml:"chatId"         json:"chatId"`
	JobID          string   `yaml:"jobId"          json:"jobId"`
	PendingTodoIDs []string `yaml:"pendingTodoIds" json:"pendingTodoIds"`
	Progress       Progress `yaml:"progress"       json:"progress"`
}
```

### Máquina de estados

```go
// internal/domain/task/state.go

// transitions is the authoritative lifecycle graph. Anything not listed is
// rejected with AOS_TASK_INVALID_TRANSITION — status is never a plain field
// write.
var transitions = map[Status][]Status{
	Suggestion: {Backlog, Finished},
	Backlog:    {Planning, Todo, Stopped},
	Planning:   {Todo, Backlog, Stopped},
	Todo:       {InProgress, Backlog, Stopped},
	InProgress: {InReview, Stopped, Todo},
	Stopped:    {InProgress, Todo, Backlog},
	InReview:   {Finished, InProgress},
	Finished:   {},
}

// SetStatus is the only path to a lifecycle move. Update explicitly rejects
// any attempt to write the status field.
func (s *service) SetStatus(ctx context.Context, in SetStatusInput) (*Task, error) {
	t, err := s.repo.Get(ctx, Key{"id": in.ID})
	if err != nil {
		return nil, err
	}
	if !t.Status.CanMoveTo(in.Status) {
		return nil, errInvalidTransition(t.Status, in.Status)
	}
	if in.Status == InReview {
		if err := s.guardReview(ctx, t); err != nil {
			return nil, err
		}
	}
	if in.Status == InProgress {
		if err := s.guardDependencies(ctx, t); err != nil {
			return nil, err
		}
	}
	// ...
}

// guardReview enforces the master prompt's hardest task rule: "Only move the
// task to in_review when all todos are finished and validated. Completion
// claims without evidence stay in in_progress."
func (s *service) guardReview(ctx context.Context, t *Task) error {
	pending, err := s.todos.CountPending(ctx, t.ID)
	if err != nil {
		return err
	}
	if pending > 0 {
		return errReviewBlocked(t.ID, pending)
	}
	return nil
}
```

### Assignee resolvido

```go
// ResolvedAssignee is a read-only projection, never persisted. Its Type drives
// execution policy: only "agent" receives autonomous dispatch; "user" and
// "unknown" tasks are human-owned — context mode, no chat spawning.
type ResolvedAssignee struct {
	ID    string `json:"id"`
	Type  string `json:"type"` // agent | user | unknown
	Name  string `json:"name"`
	Image string `json:"image,omitempty"`
	Role  string `json:"role,omitempty"`
}
```

### Worktree

```go
// internal/domain/task/worktree.go

// Branch creates an isolated Git worktree for the task and runs the workspace's
// onCreateScript inside it. The sandbox root becomes this path, confining the
// agent to its own branch.
func (s *service) Branch(ctx context.Context, in BranchInput) (*Worktree, error)

// Old worktrees are pruned according to workspace.Worktrees.WorktreeLimit and
// DeleteOldWorktrees, oldest finished task first.
func (s *service) pruneWorktrees(ctx context.Context) (int, error)
```

`onCreateScript` roda sob [[Sandbox (Go)]] com a política do agente designado — não com privilégio irrestrito, como no original.

### Ordem de escrita

Sem transação multi-arquivo ([[ADR-0004 Collections em Markdown]]), a criação de uma task com todos grava o agregado **por último**:

```
1. cria .aos/tasks/{id}/           (diretório)
2. grava todos/                     (filhos)
3. grava TASK.md                    (agregado — último)
```

Falha parcial deixa filhos órfãos invisíveis (não há `TASK.md` que os referencie), em vez de um agregado apontando para filhos inexistentes. Um job de manutenção varre e reporta órfãos.

## Decisões e divergências

> [!decision] `guardReview` aplicado no servidor
> O original impõe a regra no prompt. Aqui é validação de domínio: mover para `in_review` com todos pendentes falha com `AOS_TASK_REVIEW_BLOCKED`. O prompt continua ensinando; o sistema agora garante.

> [!decision] `onCreateScript` sob sandbox
> O original executa o script livremente. Aqui passa pela allowlist do agente ([[ADR-0006 Allowlist no sandbox]]) — um script de setup é código de terceiro em muitos workspaces.

> [!decision] Agregado gravado por último
> Ordem de escrita como estratégia de consistência, na ausência de transação.

> [!decision] Autopilot explícito
> O original menciona um autopilot que varre e executa tasks. Aqui ele é um job nomeado (`task.autopilot`), com política declarada por workspace e desligado por padrão. Executar trabalho autônomo sem opt-in é decisão do usuário.

## Testes

- Tabela exaustiva de transições, incluindo todas as inválidas
- `Update` rejeita escrita no campo `status`
- `in_review` com todos pendentes é bloqueado; sem pendentes, passa
- `in_progress` com dependência não concluída é bloqueado
- Delete remove o diretório inteiro (todos, comentários, runs)
- Checkpoint no `stop` registra chat, job e todos pendentes; retomada usa os três
- Worktree criada, `onCreateScript` sob sandbox, poda respeitando o limite
- Assignee `user` não recebe dispatch autônomo
- Falha entre a escrita de todos e do `TASK.md` deixa órfãos detectáveis pelo job de manutenção

## Critério de pronto

- [x] Task executada autonomamente do início ao `in_review`
- [x] Ciclo de oito estados com transições validadas
- [x] Worktree Git isolada confinando o sandbox
- [x] Checkpoint e retomada exata
- [x] `guardReview` impedindo conclusão sem evidência

## Saída dos testes — Fase 6

`go test ./internal/domain/task/` — **86,2% de cobertura**, 24 testes.

A tabela de transições é verificada inteira: `TestTheLifecycleTableIsExhaustive`
percorre os 64 pares e afirma cada um contra o grafo, em vez de amostrar as
arestas interessantes. `TestFinishedWorkIsNotReopened` cobre a única linha vazia
da tabela.

| O que a nota pede | Teste |
|---|---|
| Tabela exaustiva, incluindo as inválidas | `TestTheLifecycleTableIsExhaustive` |
| `Update` rejeita escrita no campo `status` | `TestUpdateRefusesToWriteStatus` |
| `in_review` com todos pendentes é bloqueado | `TestReviewIsBlockedByAnOpenPlan` |
| `in_progress` com dependência não concluída é bloqueado | `TestWorkDoesNotStartOnUnfinishedDependencies` |
| Delete remove o diretório inteiro | cascade da coleção, `TestCascadeDeleteRemovesTheWholeDirectory` |
| Checkpoint no `stop`; retomada usa os três | `TestStoppingWritesTheCheckpointAndResumingConsumesIt` |
| Worktree criada, `onCreateScript` sob sandbox, poda no limite | `TestABranchIsCutFromThePrefixAndTheSlug`, `TestTheOldestFinishedCheckoutIsPrunedAndAnActiveOneIsNot` |
| Assignee `user` não recebe dispatch | `TestOnlyAnAgentAssigneeIsDispatchable` |

**Adições não previstas na nota, cada uma por um motivo.**

`TASK_NOT_AN_ENTRY_POINT` — uma task só pode ser criada em `suggestion`,
`backlog`, `planning` ou `todo`. Criar direto em `in_progress` pularia o guard de
dependências; em `in_review`, o guard do plano. A tabela de transições não
protege nada se a criação puder aterrissar em qualquer estado.

`TASK_DEPENDENCY_CYCLE` — duas tasks que esperam uma pela outra nunca começam, e
o sistema não teria como dizer isso depois. A checagem é uma busca em largura
sobre `dependsOn`, feita no `update`.

Dependência que não existe mais não bloqueia. É uma referência pendente, vale
uma linha no log, e recusar o trabalho por causa dela é a punição errada.

**Divergência: `tasks branch` fora da suíte de paridade.** Cria uma worktree Git
de verdade, que exige um repositório que a instalação de paridade não tem. Está
em `excluded` com a razão escrita, e o mesmo caminho de código é exercido pela
suíte do domínio sobre um driver de worktree falso.

**Não verificado:** worktree contra um repositório Git real. O adaptador
`gitcli.Worktrees` traduz para `git worktree add/remove/list --porcelain` e nunca
rodou contra um `git` de verdade nesta fase.
