# AGENTS.md

Instruções para agentes de código (Claude Code, Codex, Cursor, Gemini CLI,
OpenCode…) que trabalham **neste repositório**. Para *usar* o AOS a partir de
um agente, veja [docs/system/07-agentes-de-codigo.md](docs/system/07-agentes-de-codigo.md).

## O que é isto

AOS é um sistema operacional para agentes de IA: um daemon Go (`aosd`) que
possui o workspace, um CLI (`aos`) e um aplicativo desktop (`aos-desktop`,
Wails3 + React 19). Leia [docs/system/04-arquitetura.md](docs/system/04-arquitetura.md)
antes de mudar qualquer coisa estrutural; o contrato de engenharia inteiro
está no vault em `docs/` (comece por `docs/00 - Índice/AOS.md`).

## Regras que o CI impõe (não negocie com elas)

- **Regra de dependência** (`internal/architecture`): `internal/domain` não
  importa transporte, adaptadores, runtime nem bibliotecas de I/O; `cmd/aos` e
  `cmd/aos-desktop` não linkam domínio (exceção: `gateway`); `pkg/` não
  importa `internal/`. `TestDependencyRule` e `TestNoDomainInClients` falham
  o build.
- **Artefatos gerados são derivados, nunca editados à mão**:
  `internal/core/apperr/catalog.gen.go`, `frontend/src/lib/schema.ts`,
  `internal/domain/view/components.json`, `pkg/skill/**`. Rode `task gen` e
  commite o resultado; `git diff --exit-code` sobre eles é um gate.
- **Um comando = uma definição** (`command.Command[In,Out]` registrado em
  `internal/domain/<x>/commands.go`). Ele vira CLI, tool MCP, rota HTTP
  (`POST /api/<grupo>/<nome>`), tool do agente e documentação. Nunca crie uma
  rota HTTP ou tool MCP à mão para algo que é um comando.
- **Erros são `apperr`** com código, status e *call to action*. Um erro sem
  classificação vira `HTTP_INTERNAL` e é um defeito.
- **Todo domínio tem** `entity.go`, `service.go`, `port.go` (`TestFeatureLayout`).
- **Cobertura por pacote** tem piso (`tools/covercheck`). Testes usam
  `t.TempDir()` e `AOS_HOME_DIR`; nunca tocam em `~/.aos`.

## Como rodar

```sh
task check                       # gen + diff + vet + lint + test -race + cover + arch + graph
go test -race ./internal/...     # só o Go
cd frontend && npm run typecheck && npx vitest run
task dev                         # a janela com hot reload (wails3 dev)
task build:desktop && task build:cli
AOS_HOME_DIR=/tmp/aos-dev go run ./cmd/aosd serve     # um daemon isolado
AOS_HOME_DIR=/tmp/aos-dev go run ./cmd/aosd self llms  # a superfície inteira
```

Variáveis de ambiente levam o prefixo `AOS_` (`AOS_SERVER_PORT`,
`AOS_WORKSPACE_PATH`, `AOS_TOKEN`, `AOS_LOG_LEVEL`…). A lista completa está em
[docs/system/08-operacao.md](docs/system/08-operacao.md#variáveis-de-ambiente).

## Convenções

- Go 1.25, `gofmt`, `golangci-lint` v2 (`.golangci.yml`). Comentários
  explicam **por quê**, não o quê; o código-base é escrito assim de ponta a
  ponta e um comentário que só repete a linha seguinte não passa em revisão.
- Frontend: TypeScript estrito, React 19, TanStack Router/Query, Tailwind 4.
  Strings de interface passam por `t("...")` e precisam existir em
  `frontend/src/lib/i18n/locales/en.json` **e** `pt-BR.json` (há um teste).
- O frontend fala com o daemon só por `lib/client.ts` (registro) ou pelas
  superfícies próprias (`/api/auth`, `/api/file`) via `lib/daemon-origin.ts`.
  Nunca `fetch("/api/...")` solto — dentro da janela isso alcança o host de
  assets, não o daemon.
- Commits em inglês, imperativo, com escopo: `fix(desktop): …`, `feat(skill): …`.
- Não commite `dist/`, `frontend/dist/`, `cmd/*/dist/*`, `.aos/`, `.env`.

## Onde as coisas estão

| Quero… | Vá a |
|---|---|
| Adicionar um comando a um domínio | `internal/domain/<x>/commands.go` → `task gen` |
| Um novo domínio | `internal/domain/<novo>/{entity,service,port,commands}.go` + `internal/app/wire.go` |
| Uma rota fora do registro (raro) | `internal/transport/httpapi` — e justifique no doc do pacote |
| Um adaptador (disco, rede, processo) | `internal/adapters/<nome>` implementando o port do domínio |
| A janela | `cmd/aos-desktop`, `internal/transport/wailsvc`, `frontend/src` |
| A skill que agentes leem | Texto curado em `tools/genskill/main.go`; referências geradas |
| CI / release | `.github/workflows/{ci,release}.yml`, `Taskfile.yml`, `install.sh` |
