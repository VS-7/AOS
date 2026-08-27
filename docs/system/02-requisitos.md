# 02 · Documento de requisitos

> [Índice](../README.md) · Anterior: [Visão geral](01-visao-geral.md) · Próximo: [Casos de uso](03-casos-de-uso.md)

Este documento consolida o que o sistema **faz** (requisitos funcionais), as
**qualidades** que precisa ter (não funcionais) e as **restrições** sob as
quais foi construído. Cada requisito aponta para onde é implementado e para
como é verificado, para que nenhum fique só no papel.

Convenções: **RF** funcional · **RNF** não funcional · **RC** restrição.
Prioridade: **M** (must, o produto não existe sem) · **S** (should) · **C** (could).

## 1. Escopo

Um daemon local que possui um workspace e o expõe a pessoas e a agentes por
aplicativo, terminal, navegador e MCP, dando a agentes de IA memória
persistente, execução com ciclo de vida e continuidade. Está **fora** do
escopo: hospedagem multi-tenant, sincronização entre máquinas, um editor de
código, e a assinatura/notarização dos binários (bloqueada em credenciais
que o projeto ainda não tem — ver [Release](10-release.md)).

## 2. Atores

| Ator | Descrição |
|---|---|
| **Pessoa** | Quem instala e opera. No beta, a única conta da instalação (`role: super`). |
| **Agente** | Uma identidade do workspace executando um turno (chat, tarefa ou rotina). Age através das mesmas tools que a pessoa, sob sandbox e allowlist. |
| **Agente de código externo** | Claude Code, Codex, Cursor etc., operando o AOS via skill + MCP/CLI. |
| **Provedor de modelo** | Anthropic, OpenAI, Google, Antigravity ou uma API compatível com OpenAI. |
| **Sistema externo** | Telegram (webhook), Cloudflare (túnel), Git, um registro de marketplace, uma API OpenAPI ou um servidor MCP usado como toolset. |

## 3. Requisitos funcionais

### 3.1 Workspace e identidade

| ID | Requisito | Pri. | Onde | Verificação |
|---|---|---|---|---|
| RF-01 | Registrar um diretório como workspace (`workspace introspect`), criando `.aos/` e `git init` quando faltar; listar, arquivar e alternar entre vários no mesmo daemon. | M | `internal/domain/workspace`, `internal/app/workspaces.go` | `internal/app/workspaces_test.go`, `TestTheDeliveryOfThePhase` |
| RF-02 | Toda chamada é escopada a um workspace (`x-workspace-id` / `AOS_WORKSPACE_ID`); o daemon resolve o runtime do workspace sob demanda, com cache. | M | `internal/app/root.go` | `internal/app/daemon_test.go` |
| RF-03 | Onboarding cria a primeira conta local (`users.json`, permissão `0600`), emite um token e grava `local.token` para o terminal. Login, logout, sessão e regeneração de token de API. | M | `internal/domain/auth`, `internal/transport/authapi` | `authapi` tests, `TestTheDeliveryOfPhaseFour` |
| RF-04 | Uma instalação nova abre já dentro de um workspace, no idioma do sistema. | S | `cmd/aos-desktop`, `frontend/src/lib/i18n` | `window_test.go`, `i18n.test.ts` |

### 3.2 Agentes

| ID | Requisito | Pri. | Onde | Verificação |
|---|---|---|---|---|
| RF-10 | CRUD de agentes como `AGENT.md` (frontmatter + instruções), com papel, líder, provedor/modelo, reasoning, voz, canais e sandbox. | M | `internal/domain/agent` | `agent` tests, testsuite de port |
| RF-11 | Um agente **orquestrador** por workspace recebe conversas sem destinatário e delega a especialistas. *Atlas* é semeado na criação do workspace. | M | `internal/app/seeder.go`, `internal/domain/agent` | `seeder` tests |
| RF-12 | `agents_me` identifica o agente que está chamando (via `AOS_AGENT_ID`/contexto), para o protocolo de início de sessão da skill. | M | `internal/domain/agent/commands.go` | `tools/genskill/main_test.go` |
| RF-13 | Sandbox por agente: `policy` allowlist/deny-all, `allow`, `denyArgs`, `allowShell`. Comando fora da allowlist falha com a mensagem do que pedir ao dono. | M | `internal/runtime/sandbox`, ADR-0006 | `sandbox` tests |

### 3.3 Memória

| ID | Requisito | Pri. | Onde | Verificação |
|---|---|---|---|---|
| RF-20 | Gravar, recuperar (busca full-text + filtros), supersedir, depreciar e arquivar memórias, com 13 categorias, confiança 0–1, TTL e links entre memórias. | M | `internal/domain/memory`, `internal/adapters/bleveindex` | `memory` tests, `TestTheDeliveryOfThePhase` |
| RF-21 | Memórias são globais entre instâncias paralelas do mesmo agente e ficam em disco (`.aos/agents/<id>/memories/*.memory.md`). | M | idem | idem |
| RF-22 | O **subconsciente** observa conversas e forma memórias sem pedido do agente principal. | M | `internal/runtime/subconscious` | `TestTheDeliveryOfPhaseSix` |
| RF-23 | O grafo de memórias é navegável na interface. | S | `frontend/src/features/memory` | — |

### 3.4 Conversas e turno do agente

| ID | Requisito | Pri. | Onde | Verificação |
|---|---|---|---|---|
| RF-30 | Chats de cinco tipos (dm, channel, task, run, external) com histórico, anexos, menções e streaming de resposta por WebSocket. | M | `internal/domain/chat`, `internal/runtime/session`, `internal/transport/realtime` | `conversation_test.go`, `realtime_test.go` |
| RF-31 | O prompt é montado em XML com atributos `trust` por bloco, a partir de workspace, memórias, skills e instruções. | M | `internal/runtime/prompt` | goldens em `prompt/testdata` |
| RF-32 | O loop do agente tem cinco pontos de intervenção (hooks), executa tools com concorrência limitada e faz *spillover* de resultados grandes para disco. | M | `internal/runtime/agentloop`, `toolexec` | `TestTheDeliveryOfPhaseFive` |
| RF-33 | Ações que exigem aprovação perguntam a um humano pelo canal em tempo real, com prazo (`AOS_APPROVAL_DEADLINE`); sem canal, negam explicitamente. | M | `internal/domain/approval` (grupo `approvals`), ADR-0007 | `approvals` tests |
| RF-34 | Custo do turno (tokens, USD) fica registrado na mensagem que o causou. | S | `internal/runtime/providers/pricing.json` | `pricing_test.go` |
| RF-35 | Provedores: Anthropic, OpenAI, Google, Antigravity (login OAuth em arquivo) e qualquer API compatível com OpenAI, atrás de uma suíte de contrato única. | M | `internal/runtime/providers/*` | `providertest` |

### 3.5 Tarefas, todos e comentários

| ID | Requisito | Pri. | Onde | Verificação |
|---|---|---|---|---|
| RF-40 | Tarefas com 8 estados (`suggestion → backlog → planning → todo → in_progress ⇄ stopped → in_review → finished`), transições só por `set-status`. | M | `internal/domain/task` | `task` tests |
| RF-41 | Cada tarefa em execução recebe um **worktree Git** próprio (`--worktree`, `--base`), criado e removido pelo sistema. | M | `internal/adapters/gitcli` | `gitcli` tests |
| RF-42 | *Todos* fecham com evidência; a transição para `in_review` é recusada enquanto houver todo pendente. | M | `internal/domain/todo`, `task` | `task` tests |
| RF-43 | Comentários por tarefa são o canal de comunicação em modo tarefa; a UI e o agente os usam. | M | `internal/domain/comment` | `comment` tests |
| RF-44 | Um agente leva uma tarefa de `in_progress` a `in_review` autonomamente, fechando cada passo com evidência. | M | `internal/runtime/worker` | `TestTheDeliveryOfPhaseSix` |

### 3.6 Rotinas, fila e atividade

| ID | Requisito | Pri. | Onde | Verificação |
|---|---|---|---|---|
| RF-50 | Rotinas por agente com três gatilhos: `scheduled` (cron), `webhook` (URL pública) e `activity` (evento do workspace). Cada disparo é um `run` persistido. | M | `internal/domain/routine` | `routine` tests |
| RF-51 | Fila de jobs em SQLite puro Go com *claim* atômico, concorrência e *tick* configuráveis. | M | `internal/adapters/sqlitequeue`, ADR-0008 | `sqlitequeue` tests |
| RF-52 | Log de atividade do workspace (`.aos/activity/AAAA-MM.jsonl`) com cursor de leitura, consultável e usado como gatilho. | M | `internal/domain/activity` | `activity` tests |

### 3.7 Ecossistema

| ID | Requisito | Pri. | Onde | Verificação |
|---|---|---|---|---|
| RF-60 | Instalar/desinstalar **skills** (locais, de git ou http) que trazem agentes, coleções, views, instruções e hooks, com permissões declaradas e sem escrever fora do próprio diretório. | M | `internal/domain/skill`, ADR-0015 | `TestTheDeliveryOfPhaseEight` |
| RF-61 | **Toolsets** externos: OpenAPI e servidores MCP, com egress guardado (`netguard`). | M | `internal/domain/toolset`, `adapters/openapiclient`, `mcpclient` | `toolset` tests |
| RF-62 | **Coleções** definidas por `schema.json`, registros em Markdown, registradas ao aparecer em disco (watcher), com busca. | M | `internal/domain/collection`, `internal/app/watch.go` | `ecosystem_test.go` |
| RF-63 | **Views** declarativas validadas contra o catálogo de componentes gerado do frontend. | M | `internal/domain/view`, `components.json` | `view` tests |
| RF-64 | **Artefatos**: apps estáticos publicados em `/v/artifacts/{id}/*` com visibilidade `private`, `workspace` ou `by_password`. | M | `internal/domain/artifact`, `transport/artifactapi` | `artifactapi` tests |
| RF-65 | **Templates** Liquid, **instruções** com aprovação, **metas** e **projetos**. | M | `template`, `instruction`, `goal`, `project` | testes por domínio |
| RF-66 | **Marketplace** com registros git/http, listagem e instalação de plugins. | S | `internal/domain/marketplace` | `registrytest` |
| RF-67 | **Temas**: 38 temas com contraste WCAG AA verificado, tokens gerados para o frontend. | S | `internal/domain/theme`, `tools/gentokens` | `theme` tests |

### 3.8 Superfícies e canais

| ID | Requisito | Pri. | Onde | Verificação |
|---|---|---|---|---|
| RF-70 | Todo comando do registro é publicado como `POST /api/<grupo>/<nome>` com envelope `{data\|error\|notice}`; renomeações mantêm o caminho antigo com aviso. | M | `internal/transport/httpapi` | `httpapi_test.go`, `parity_test.go` |
| RF-71 | O mesmo registro é servido como **MCP** em stdio (`aosd --mcp`, `aos --mcp`) e em HTTP streamable (`/mcp`), em três formas: `composite` (uma tool por grupo, com `action` e `schema: true`), `flat` (uma por comando) ou `both`. | M | `internal/transport/mcpserver`, `mcpproxy` | `mcpserver` tests, `mcpproxy` tests |
| RF-72 | O **terminal** `aos` monta a árvore de comandos a partir do daemon (`/api/_commands`), aceita `--json`, `--set caminho=valor`, `--reason`, `--format`, paginação por tokens, e opera o gateway localmente. | M | `internal/transport/clix`, `cmd/aos` | `clix_test.go`, `supervision_test.go` |
| RF-73 | O **aplicativo** supervisiona o daemon, chama o registro em processo (`DomainService.Invoke`) com fallback HTTP, retransmite eventos em tempo real e integra a plataforma (diálogos, abrir caminhos, aparência, arrastar arquivos, deep link `aos://`). | M | `cmd/aos-desktop`, `wailsvc`, `frontend/src/lib/client.ts` | `wailsvc_test.go`, `window_test.go`, vitest |
| RF-74 | O **modo servidor** (`-tags webui`) serve a interface comprimida no próprio daemon para um navegador, com autenticação por cookie. | M | `cmd/aosd/webui_embed.go`, `httpapi/webui.go` | `webui_test.go` |
| RF-75 | **Telegram**: webhook por agente com segredo próprio, mensagens divididas no limite da API, resposta do turno entregue de volta. | S | `internal/domain/bot`, `adapters/telegramapi`, `transport/botapi` | `bot`, `telegramapi`, `botapi` tests |
| RF-76 | **Túnel** Cloudflare gerenciado pelo daemon (`tunnel start\|stop\|status`), fornecendo a URL pública que webhooks e bots usam. | S | `internal/domain/tunnel`, `adapters/cloudflaredproc` | `tunnel` tests |
| RF-77 | **Skill publicada**: `SKILL.md` curado + uma referência por grupo, gerados do registro, embutidos nos binários e instaláveis em Claude Code, Codex, Cursor, Gemini CLI, OpenCode e `.agents` (`aos self skill install`, menu do aplicativo). | M | `pkg/skill`, `tools/genskill`, `clix/skill.go`, `wailsvc/skill.go` | `pkg/skill` tests, `clix` tests, `wailsvc` tests |
| RF-78 | Auto-update: verificar, baixar, conferir checksum + assinatura Ed25519, aplicar com troca e rollback. Desligado por padrão (`AOS_UPDATE_BASE_URL` vazio). | C | `internal/domain/update`, `adapters/updateinstall` | `update` tests |

## 4. Requisitos não funcionais

| ID | Requisito | Como é atendido | Verificação |
|---|---|---|---|
| RNF-01 **Local-first** | Nenhum dado sai da máquina exceto para o provedor de modelo escolhido e integrações que a pessoa ligou. | Sem backend próprio; estado em `~/.aos` e `.aos/`. | Revisão; `netguard` em egress |
| RNF-02 **Um escritor** | O daemon é o único processo que escreve no workspace; clientes falam HTTP. | ADR-0002, regra de dependência. | `TestNoDomainInClients` |
| RNF-03 **Escrita atômica** | Todo arquivo é escrito por rename atômico sob lock por caminho; CAS para edições concorrentes. | ADR-0012, `internal/core/atomicfs`. | `atomicfs` tests |
| RNF-04 **Segurança por padrão** | Bind em `127.0.0.1`; expor além disso sem `security.enabled` aborta o boot; segredos em `0600` e auditados no boot; CORS por allowlist; origem de WebSocket restrita; cookies `HttpOnly`. | ADR-0009, ADR-0010, `guardExposure`, `AuditSecrets`. | `httpapi`, `serve` tests |
| RNF-05 **Contenção do agente** | Allowlist de binários, shell opt-in, aprovação real, `trust` no prompt, spillover. | ADR-0006, ADR-0007. | `sandbox`, `prompt` tests |
| RNF-06 **Sem divergência** | Uma definição por comando; artefatos derivados (schema TS, catálogo de erros, skill, tokens) gerados e verificados no CI. | `task gen` + `git diff --exit-code`. | CI `gates` |
| RNF-07 **Erros acionáveis** | Todo erro tem código, status HTTP e *call to action* (comando/tool a executar). | `internal/core/apperr`, catálogo gerado. | `gencatalog`, testes |
| RNF-08 **Tamanho** | `aos` ≤ 36 MB, `aosd` ≤ 54 MB, `aos-desktop` ≤ 66 MB (alvo +20%). | `-trimpath -s -w`, sem sourcemaps embutidos, sem Electron. | `task build:check-size` |
| RNF-09 **Reprodutibilidade** | Dois builds do mesmo commit com o mesmo `SOURCE_DATE_EPOCH` são idênticos byte a byte; nenhum caminho absoluto embutido. | Build estampado pelo commit. | `build:verify-reproducible`, `check-no-abspath` |
| RNF-10 **Portabilidade** | macOS arm64; Windows amd64; Linux amd64 (janela) e arm64 (servidor/terminal). | Cross-compile com `CGO_ENABLED=0` para CLI e servidor; janela nativa por plataforma. | CI `build-cli`, `build-desktop` |
| RNF-11 **Desempenho percebido** | Interface responde em tempo real por WebSocket; a janela abre sem esperar o daemon; busca por Bleve. | `realtime`, `ensureDaemon` assíncrono. | testes de frontend |
| RNF-12 **Observabilidade** | `log/slog` estruturado (texto/JSON), `requestId` por chamada, log do gateway em `~/.aos/runtime/gateway/gateway.log`. | `internal/core/logging`. | — |
| RNF-13 **Testabilidade** | Toda porta tem uma suíte de contrato executável; testes de entrega por fase sobre disco real; pisos de cobertura por pacote; race detector no CI. | `internal/domain/testsuite`, `tools/covercheck`. | CI |
| RNF-14 **Internacionalização** | Interface em inglês e português (Brasil), detectada do sistema, com catálogo verificado por teste. | `frontend/src/lib/i18n`. | `i18n.test.ts` |
| RNF-15 **Acessibilidade** | 38 temas com contraste WCAG AA verificado por teste. | `internal/domain/theme`. | `theme` tests |

## 5. Restrições

| ID | Restrição | Consequência |
|---|---|---|
| RC-01 | **Nome provisório.** `AOS` e `github.com/OWNER/aos` são marcadores substituíveis numa operação ([ADR-0000](../01%20-%20Decisões/ADR-0000%20Nome%20provisório%20do%20projeto.md)). | Não há `go install` público; a instalação é pelo `install.sh` e por Releases. |
| RC-02 | **Sem certificado de assinatura.** Nenhum Apple Developer ID nem certificado Windows. | macOS: instalar por `curl` (sem quarentena). Windows: aviso do SmartScreen. Sem auto-update ligado. |
| RC-03 | **Go 1.25, Wails v3 beta.8, React 19.** | A API do Wails é isolada em `wailsvc`; o resto do desktop não a conhece. |
| RC-04 | **Linux exige GTK4 + WebKitGTK 6.0 instalados.** Empacotá-los no AppImage é problema em aberto do ecossistema (wails#4313). | O instalador e o AppImage nomeiam a biblioteca que falta. |
| RC-05 | **Regra de dependência é teste.** `internal/domain` não vê transporte/adaptadores/runtime; clientes não veem domínio; `pkg/` não vê `internal/`. | Arquitetura hexagonal mantida mecanicamente. |
| RC-06 | **Compatibilidade de nomes com o original** (ADR-0016) para grupos, comandos e arquivos do workspace. | Skills e workspaces do produto de origem funcionam sem renomear. |

## 6. Critérios de aceite do sistema

O sistema está entregue quando, num commit da `main`:

1. `task check` passa (gen sem diff, vet, lint, `go test -race`, pisos de cobertura, regra de dependência, grafo do vault).
2. `npm run typecheck` e `vitest run` passam no frontend.
3. O CI (`ci.yml`) está verde nos quatro jobs: `gates`, `frontend`, `build-cli` (6 alvos), `build-desktop` (3 plataformas).
4. Uma tag `v*` produz um release com os 21 artefatos e `checksums.txt` ([Release](10-release.md)).
5. Os fluxos dos [casos de uso](03-casos-de-uso.md) UC-01 a UC-12 são exercitáveis numa instalação limpa.

> Próximo: [03 · Casos de uso](03-casos-de-uso.md)
