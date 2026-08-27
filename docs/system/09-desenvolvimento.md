# 09 · Desenvolvimento

> [Índice](../README.md) · Anterior: [Operação](08-operacao.md) · Próximo: [Release](10-release.md)

## Ambiente

| | |
|---|---|
| **Go** | 1.25+ (`go.mod` manda) |
| **Node** | 22+ |
| **go-task** | `go install github.com/go-task/task/v3/cmd/task@latest` |
| **wails3** | `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.8` |
| **golangci-lint** | v2.12.2 |
| **Linux** | GTK4 e WebKitGTK 6.0 para compilar a janela |

```sh
git clone https://github.com/VS-7/AOS.git && cd AOS
cd frontend && npm ci --legacy-peer-deps && cd ..
task check
```

`--legacy-peer-deps` não é opcional: a árvore tem faixas de *peer* que o npm
recusa resolver no modo estrito.

## Tarefas

```sh
task check              # todos os gates — o que o CI roda
task gen                # regenera tudo que é derivado
task build:desktop      # a janela (frontend + embed + binário)
task build:cli          # aos e aosd
task build:server       # aosd com a interface dentro (-tags webui)
task build:all          # os seis alvos
task dev                # wails3 dev, com hot reload
task run                # a janela já compilada
task package:desktop    # .app / instalador NSIS / AppImage
task clean
```

Gates individuais: `task vet lint test cover arch graph`.

## O que é gerado, e por quê

Nada disso se edita à mão. O CI compara o que está commitado com o que o
gerador produz — se divergir, o build falha.

| Comando | Produz | A partir de |
|---|---|---|
| `task gen-catalog` | `internal/core/apperr/catalog.gen.go` | as chamadas `apperr.New` na árvore |
| `task gen-schema` | `frontend/src/lib/schema.ts` | o registro de comandos |
| `task gen-tokens` | `internal/domain/theme/tokens.txt` | o CSS que o frontend lê |
| `task gen-components` | `internal/domain/view/components.json` | os schemas zod do frontend |
| `task gen-skill` | `pkg/skill/SKILL.md` + `references/*` | o registro de comandos |

A consequência prática: um campo renomeado em Go quebra o build do frontend
na hora, e não a tela na frente de alguém.

## Layout

```
cmd/            aos · aosd · aos-desktop — a composição, e nada mais
internal/
  core/         command · collections · apperr · atomicfs · pathx · env · config · logging · search
  domain/       31 fatias verticais: entity · service · port · commands
  runtime/      agentloop · prompt · sandbox · toolexec · providers · session · subconscious · worker
  adapters/     fs* · gitcli · sqlitequeue · bleveindex · telegramapi · cloudflaredproc · supervise · …
  transport/    httpapi · mcpserver · mcpproxy · clix · wailsvc · realtime · authapi · fileapi · botapi · artifactapi
  app/          wire.go · serve.go · watch.go — onde tudo se encontra
  architecture/ as regras estruturais, como testes
  testx/        fixtures e utilidades de teste
pkg/skill/      a skill publicada — o único pacote útil fora deste projeto
frontend/src/   app · features/<domínio> · components · lib · core
tools/          os geradores e o covercheck
build/          empacotamento por plataforma
docs/           o vault de especificação + docs/system
```

## As regras que o CI impõe

**Regra de dependência** (`internal/architecture`):

- `internal/domain` não importa `net/http`, `os/exec`, `database/sql`, chi,
  cobra, Wails, MCP, WebSocket, SQLite, Bleve, nem `transport`/`adapters`/`runtime`.
- `cmd/aos` e `cmd/aos-desktop` não linkam domínio — exceto `gateway`.
- `internal/core/command` não conhece `internal/domain`.
- `pkg/` não importa `internal/`.

**Layout de feature**: todo domínio tem `entity.go`, `service.go`, `port.go`.

**Cobertura**: piso por pacote (`tools/covercheck`). 125 pacotes, todos acima
do seu piso.

**Grafo do vault**: zero wikilinks quebrados, zero notas órfãs, frontmatter
obrigatório presente.

## Como adicionar um comando

1. Escreva `In` e `Out` no domínio, com tags `json`, `jsonschema` e `validate`:

```go
type ArchiveInput struct {
    ID string `json:"id" jsonschema:"Identifier of the task to archive." validate:"required,notblank"`
    command.Reasoning
}
```

2. Registre em `internal/domain/task/commands.go`:

```go
must(command.Register(reg, command.Command[ArchiveInput, ArchiveOutput]{
    Group: "tasks", Name: "archive",
    Summary:  "Archive a task.",
    Registry: true,
    Examples: []command.Example{{Description: "a finished task", Input: ArchiveInput{ID: "t-1"}}},
    Handler:  svc.Archive,
}))
```

3. `task gen` e commite o que ele escreveu.

Pronto: existe `aos tasks archive`, a ação `archive` na tool `Task`,
`POST /api/tasks/archive`, a tool interna do agente, o tipo em `schema.ts` e a
entrada em `references/tasks.md`.

## Como adicionar um domínio

1. `internal/domain/<novo>/{entity,service,port,commands}.go`
2. Um adaptador em `internal/adapters/<nome>` para cada port que toca o mundo
3. Fiação em `internal/app/wire.go`
4. `task gen`, e a feature no frontend em `frontend/src/features/<novo>/`

## Erros

Todo erro é um `apperr` com código, status HTTP e *call to action*:

```go
return apperr.New("TASK_REVIEW_BLOCKED").
    Causer("task.Service.SetStatus").
    Msgf("%d todos are still open", open).
    Issue("task", id).
    Status(apperr.StatusConflict).
    CTA(apperr.CallToAction{
        Label:   "close the remaining steps with evidence first",
        Command: build.Name + " todos list --set task=" + id,
        Tool:    "todos_list",
    })
}
```

Um erro que escapa sem classificação vira `HTTP_INTERNAL` e é um defeito — a
interface o mostra como "não foi possível completar", que não ajuda ninguém.

## Testes

- **Contrato de port**: `internal/domain/testsuite` — toda implementação de um
  port roda a mesma suíte.
- **Entrega por fase**: `TestTheDeliveryOfPhaseN` em `internal/app`, sobre
  disco real, ponta a ponta.
- **Paridade de superfícies**: o mesmo payload pela CLI, pelo MCP e pelo HTTP
  chega igual ao handler.
- **Frontend**: `npm run typecheck` e `vitest`; o catálogo de i18n é
  verificado (uma chave sem tradução falha).
- **Race detector** em tudo.

Testes usam `t.TempDir()` e `AOS_HOME_DIR`; nenhum toca `~/.aos`.

## Estilo

**Go.** `gofmt`, `golangci-lint` v2. Comentários explicam **por quê**, não o
quê — o código-base inteiro é escrito assim, e um comentário que repete a
linha seguinte não passa em revisão.

**TypeScript.** Estrito, React 19, TanStack Router/Query, Tailwind 4. Toda
string de interface passa por `t("...")` e precisa existir em `en.json` **e**
`pt-BR.json`. O frontend só fala com o daemon por `lib/client.ts` ou pelas
superfícies próprias via `lib/daemon-origin.ts` — um `fetch("/api/...")`
solto alcança o host de assets dentro da janela, não o daemon.

**Commits.** Inglês, imperativo, com escopo: `fix(desktop): …`,
`feat(skill): …`, `docs(readme): …`.

Agentes de código que trabalham neste repositório: [AGENTS.md](../../AGENTS.md).

> Próximo: [10 · Release](10-release.md)
