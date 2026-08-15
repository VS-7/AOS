---
tags: [arquitetura, layout, estrutura]
aliases: [Layout, Estrutura de Diretórios]
fase: 0
status: em-construcao
origem: "[[Padrão Feature-Slice]]"
---

# Layout de Diretórios

> Pai: [[AOS]] · Ver: [[Hexagonal e Regra de Dependência]] · Fase: 0

## Objetivo

Fixar onde cada tipo de código mora, para que a pergunta "onde isso vai?" tenha uma resposta e não uma discussão.

## Design em Go

```
aos/
├── cmd/
│   ├── aos/                  # CLI + modo --mcp (binário principal)
│   ├── aosd/                 # daemon HTTP/WS
│   └── aos-desktop/          # app Wails3
├── internal/
│   ├── core/
│   │   ├── command/          # ★ a ponte de superfícies → [[Command Layer]]
│   │   ├── collections/      # ★ motor Markdown+frontmatter → [[Collections Engine]]
│   │   ├── config/           # ~/.aos/config.json → [[Config (Go)]]
│   │   ├── env/              # resolução em camadas
│   │   ├── identity/         # identidade ambiente + contexto de requisição
│   │   ├── apperr/           # erros tipados com CTA → [[Estratégia de Erros]]
│   │   ├── eventbus/         # pub/sub interno
│   │   ├── build/            # constantes de branding → [[ADR-0000 Nome provisório do projeto]]
│   │   └── logging/          # log/slog → [[Observabilidade]]
│   ├── domain/
│   │   ├── workspace/  agent/     memory/   chat/
│   │   ├── task/       todo/      comment/  routine/   project/  goal/
│   │   ├── skill/      instruction/ template/ toolset/  marketplace/
│   │   ├── collection/ view/      artifact/ file/      theme/
│   │   ├── config/     auth/      model/    event/     activity/
│   │   ├── bot/        tunnel/    gateway/
│   │   └── testsuite/        # suítes de contrato de port
│   ├── runtime/
│   │   ├── agentloop/        # ★ o loop com 5 hooks → [[Agent Loop]]
│   │   ├── prompt/           # ★ montagem XML → [[Prompt Assembly]]
│   │   ├── sandbox/          # ★ contenção fs + exec → [[Sandbox (Go)]]
│   │   ├── toolexec/         # ★ spillover + truncagem → [[Tool Executor e Spillover]]
│   │   ├── subconscious/     # ★ segundo LLM → [[Subconsciente (Go)]]
│   │   ├── compaction/
│   │   └── providers/        # openai · anthropic · google · oauth
│   ├── transport/
│   │   ├── httpapi/          # chi router + middlewares → [[HTTP chi]]
│   │   ├── mcpserver/        # stdio + streamable HTTP → [[MCP Go SDK]]
│   │   ├── clix/             # cobra a partir do registry → [[CLI cobra]]
│   │   ├── wailsvc/          # services expostos ao React → [[Wails3 Services]]
│   │   ├── realtime/         # WebSocket → [[Realtime WebSocket]]
│   │   └── artifacts/        # servidor estático → [[Artifacts e Estáticos]]
│   ├── adapters/
│   │   ├── fscollections/    # implementação do Repository
│   │   ├── sqlitequeue/      # fila de jobs → [[ADR-0008 SQLite puro Go para filas]]
│   │   ├── bleveindex/       # busca → [[ADR-0013 Bleve para busca full-text]]
│   │   ├── telegram/         # bot → [[Bot (Go)]]
│   │   └── cloudflared/      # tunnel → [[Tunnel (Go)]]
│   ├── gateway/              # supervisão de processo → [[Gateway (Go)]]
│   └── architecture/         # testes de regra de dependência
├── pkg/
│   └── skill/                # geração de SKILL.md → [[Especificação da Skill]]
├── frontend/                 # React 19 + TS (Wails3) → [[React 19 e Bindings]]
│   ├── src/
│   │   ├── features/         # espelha internal/domain
│   │   ├── components/ui/    # → [[Design System]]
│   │   ├── lib/
│   │   └── bindings/         # gerado por `wails3 generate bindings`
│   └── package.json
├── docs/                     # ★ este vault
├── testdata/                 # golden files → [[Fixtures e Golden Files]]
└── Taskfile.yml
```

### Regras de colocação

| Se o código… | Vai para |
|---|---|
| …é regra de negócio de um agregado | `internal/domain/{feature}/service.go` |
| …fala um protocolo (HTTP, JSON-RPC, argv) | `internal/transport/{protocolo}/` |
| …fala com o mundo (disco, rede, processo) | `internal/adapters/{alvo}/` |
| …é infraestrutura sem domínio | `internal/core/{assunto}/` |
| …é o cérebro do agente | `internal/runtime/{peça}/` |
| …seria útil fora deste projeto | `pkg/{nome}/` — hoje só `skill` |
| …monta as peças | `cmd/{binário}/` |

### Anatomia de uma feature de domínio

Sempre os mesmos sete arquivos, herdando o rigor do [[Padrão Feature-Slice]]:

```
internal/domain/memory/
├── entity.go        # Memory, Category, Status — tipos puros, sem I/O
├── service.go       # regra de negócio; recebe ports no construtor
├── port.go          # interfaces que ESTE pacote consome
├── schema.go        # StoreInput, RecallInput... com tags json + jsonschema
├── commands.go      # os Command[In,Out] do grupo, e o Doc do grupo
├── errors.go        # construtores de erro do domínio
└── memory_test.go
```

Arquivos opcionais, quando a feature precisa: `queue.go` (jobs), `notification.go` (eventos), `state.go` (máquina de estados).

### O que `pkg/` significa aqui

`pkg/skill` gera `SKILL.md` + `references/*` a partir de um registry de comandos. É útil fora do projeto — qualquer CLI com registry semelhante poderia usá-lo. Nada mais está em `pkg/` até provar que merece.

### `internal/` é intencional

Todo o resto está sob `internal/`, o que impede import externo pelo compilador. A API pública do projeto são os **binários e as superfícies** (CLI, HTTP, MCP), não pacotes Go.

## Decisões e divergências

> [!decision] `internal/gateway` fora de `internal/domain`
> O gateway supervisiona processo do SO — é infraestrutura, não domínio. Mas é o único grupo de comando que o CLI executa **localmente**, sem passar pelo daemon (como no original, que chama `.withoutRemoteTransportOptions()`). Por isso vive em `internal/gateway`, importável por `cmd/aos` sem violar a regra de dependência. `internal/domain/gateway` existe só para os tipos e o `Command`, sem I/O.

> [!decision] `runtime/` separado de `domain/`
> O loop do agente, a montagem de prompt e o sandbox não são domínio (não têm agregado nem persistência) nem adaptador (não falam com um sistema externo específico). São o motor de execução. Misturá-los com `domain/agent` inflaria a feature e quebraria o SRP.

> [!decision] `frontend/features/` espelha `internal/domain/`
> Um nome, dois lados. Achar o código de UI de tasks é `frontend/src/features/task/`.

## Testes

- **Estrutura de feature:** teste que percorre `internal/domain/*` e falha se faltar `entity.go`, `service.go`, `port.go` ou `commands.go`.
- **Sem ciclos:** `go vet` mais um teste que rejeita import de `internal/domain/a` por `internal/domain/b` sem que a relação esteja declarada numa lista de dependências permitidas (ex.: `todo` → `task` é permitido; `memory` → `task` não).
- **`pkg/` limpo:** teste que falha se `pkg/*` importar `internal/*`.

## Critério de pronto

- [ ] Toda feature de domínio com os quatro arquivos obrigatórios
- [ ] Nenhum ciclo entre features de domínio
- [ ] `pkg/skill` sem import de `internal/`
- [ ] `Taskfile.yml` com alvos `build`, `test`, `lint`, `gen-bindings`, `gen-skill`
