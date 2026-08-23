---
tags: [entrega, roteiro, fases, planejamento]
aliases: [Roteiro, Fases, Plano de Execução]
fase: 0
status: especificado
origem: "[[PROMPT — Reconstrução em Go]]"
---

# Roteiro de Fases

> Pai: [[AOS]] · Fase: 0

## Objetivo

Sequenciar a construção de modo que **cada fase entregue software que roda**. Nenhuma avança sem testes verdes com saída anexada.

## As dez fases

### Fase 0 — Fundação
`go.mod`, `Taskfile.yml`, layout de diretórios, `log/slog`, `apperr` com CTA, `env` em camadas, CI com `go vet` + `golangci-lint` + `go test -race`.

**Notas:** [[Layout de Diretórios]] · [[Estratégia de Erros]] · [[Observabilidade]] · [[Concorrência e Context]] · [[Hexagonal e Regra de Dependência]] · [[Estratégia de Testes]] · [[Config (Go)]]
**Entrega:** `aos version` roda. `TestDependencyRule` verde no CI.

### Fase 1 — Persistência
Motor de collections completo: patterns bidirecionais, frontmatter, hooks, watcher, escrita atômica, lock, CAS.

**Notas:** [[Collections Engine]] · [[ADR-0004 Collections em Markdown]] · [[ADR-0012 Escrita atômica e lock por arquivo]] · [[Fixtures e Golden Files]] · [[Testes de Contrato de Port]]
**Entrega:** criar/ler/atualizar/deletar um agente em Markdown pelo código. Round-trip verde para os 13 modelos.

### Fase 2 — Command Layer e superfícies
`core/command` + registry + derivação para cobra, MCP e HTTP.

**Notas:** [[Command Layer]] · [[ADR-0003 Command Layer unificada]] · [[CLI cobra]] · [[MCP Go SDK]] · [[ADR-0011 Superfície de tools versionada]] · [[ADR-0016 Compatibilidade de nomes com o original]]
**Entrega:** `aos agents list` e `aos --mcp` expondo `agents_list` funcionam sobre a mesma definição. Teste de paridade verde.

### Fase 3 — Domínio núcleo
`workspace`, `agent`, `memory`, `chat` completos, com scaffolding e `git init`.

**Notas:** [[Workspace (Go)]] · [[Agent (Go)]] · [[Memory (Go)]] · [[Chat (Go)]] · [[ADR-0013 Bleve para busca full-text]]
**Entrega:** criar workspace, criar agente orquestrador, gravar e recuperar memórias com grafo. ✅ `TestTheDeliveryOfThePhase`, sobre disco real. O chat fica `em-construcao`: persistência e roteamento prontos, execução assíncrona nas Fases 4 e 5.

### Fase 4 — Servidor e gateway
Daemon chi na :5326 com `/api`, `/mcp`, `/ws`, `/v/*`; gateway com máquina de estados e supervisão.

**Notas:** [[HTTP chi]] · [[Realtime WebSocket]] · [[Gateway (Go)]] · [[Auth (Go)]] · [[ADR-0009 Bind em loopback por padrão]] · [[ADR-0010 Segredos com permissão restrita]]
**Entrega:** `aos gateway start|stop|status` operando o daemon. ✅ `TestTheDeliveryOfPhaseFour`, com os dois binários compilados conversando por socket. HTTP é a quinta superfície da suíte de paridade. `Auth (Go)` fica `em-construcao`: o serviço existe e os comandos que o expõem vêm com a árvore completa da CLI.

### Fase 5 — Runtime do agente
`agentloop`, `prompt`, `sandbox`, `toolexec`, providers, hooks, aprovação.

**Notas:** [[Agent Loop]] · [[Prompt Assembly]] · [[Sandbox (Go)]] · [[Tool Executor e Spillover]] · [[Model Providers (Go)]] · [[Event Hooks (Go)]] · [[ADR-0005 Loop de agente próprio]] · [[ADR-0006 Allowlist no sandbox]] · [[ADR-0007 Canal real de aprovação de tool]]
**Entrega:** conversa real com um agente que usa tools e persiste memória. ✅ `TestTheDeliveryOfPhaseFive`: a pessoa escreve, o agente lê um arquivo com uma tool, grava o que aprendeu como memória e responde — e tudo fica em disco, com o custo do turno na mensagem que o causou. `Tool Executor e Spillover` e `Model Providers (Go)` ficam `em-construcao`: faltam os toolsets `web` e `jobs`, a tabela de preços, e nenhum adaptador foi exercido contra a API real de um provider.

### Fase 6 — Continuidade
`subconscious`, `task` com ciclo de 8 estados e worktree, `routine` com 3 gatilhos, fila SQLite, `activity`.

**Notas:** [[Subconsciente (Go)]] · [[Task (Go)]] · [[Todo (Go)]] · [[Comment (Go)]] · [[Routine (Go)]] · [[Activity (Go)]] · [[ADR-0008 SQLite puro Go para filas]]
**Entrega:** task executada autonomamente do início ao `in_review`, com memórias formadas sozinhas.
**Entregue** em `v0.7.0-fase6`. `TestTheDeliveryOfPhaseSix`: uma rotina dispara por gatilho de activity, o agente leva a task de `in_progress` a `in_review` fechando cada passo com evidência e comentando o progresso, e o subconsciente forma uma memória que ninguém pediu. Pendências registradas nas notas: assinatura do subconsciente ainda em processo, `Guidance` registrada e não injetada, webhook sem superfície HTTP, índice de inbox por varredura.

### Fase 7 — Desktop
Wails3 + React 19; services; bindings; WebSocket; 38 temas; aparência nativa.

**Notas:** [[Wails3 Services]] · [[React 19 e Bindings]] · [[Design System]] · [[Temas]] · [[Theme (Go)]] · [[File (Go)]] · [[ADR-0002 Wails3 no lugar de Electron]]
**Entrega:** app desktop com chat, board de tasks e grafo de memórias.
**Entregue** em `v0.9.0-fase7`, em duas etapas. `v0.8.0-fase7` compilava e rodava — 38 temas com contraste WCAG AA verificado, cliente unificado, realtime com backoff, as três telas — mas sem o design system, sem as outras rotas, sem testes de frontend e sem [[File (Go)]], e a janela nunca tinha sido aberta.

O branch `feat/port-fractal-frontend` fechou isso: os 120 componentes e as 26 features vieram da extração em `_extracted/v401/` em vez de reescritos, sobre a árvore de rotas do TanStack Router, com 48 testes e `wails3 dev` funcionando. [[File (Go)]] tem backend, HTTP e UI. A marca do produto original saiu junto, e a varredura achou quatro defeitos vivos que ela escondia.

**Uma decisão de arquitetura da fase quase se perdeu no caminho:** o `//go:embed all:dist` do desktop engolia os 135 MB de sourcemap que o vite emite, levando o binário a 199 MB. O argumento inteiro de [[ADR-0002 Wails3 no lugar de Electron]] é tamanho de binário. A cópia para `cmd/aos-desktop/dist` agora exclui `*.map` — os mapas continuam em `frontend/dist` para simbolizar stack trace — e o binário voltou a 64 MB.

**Não entregue, e movido para a Fase 8:** o catálogo de componentes gerado (`task gen-components`), que é pré-requisito de [[View (Go)]] e por isso pertence à fase que a implementa. [[Design System]] fica `em-construcao` por ele e pela acessibilidade nunca medida por ferramenta.

### Fase 8 — Ecossistema
`skill`, `toolset`, `collection` e `view` dinâmicas, `artifact`, `template`, `instruction`, `project`, `goal`, `bot`, `tunnel`, `marketplace`.

**Notas:** [[Skill (Go)]] · [[Toolset (Go)]] · [[Collection (Go)]] · [[View (Go)]] · [[Views Declarativas]] · [[Artifact (Go)]] · [[Artifacts e Estáticos]] · [[Template (Go)]] · [[Instruction (Go)]] · [[Project (Go)]] · [[Goal (Go)]] · [[Bot (Go)]] · [[Tunnel (Go)]] · [[Marketplace (Go)]] · [[ADR-0014 Liquid para templates]] · [[ADR-0015 Skills com permissões declaradas]]
**Entrega:** instalar uma skill que traz agente, coleção e view próprios.

**Núcleo entregue.** A fatia vertical de `collection`, `view`, `toolset` e
`skill` está construída, registrada em `wire.go` e acesa na interface: os
quatro saíram de `DORMANT_DOMAINS` e os caminhos que a UI chama foram
exercitados por HTTP real, autenticados, todos despachando ao domínio. O
watcher que registra uma coleção quando um `schema.json` aparece em disco
existe (`internal/app/watch.go`, `onSchemaChanged`/`reconcileCollections`),
resolvendo também o resíduo A2 — `files:changed`/`collection.changed`
propaga para a árvore de arquivos sem restart. `TestTheDeliveryOfPhaseEight`
(`internal/app/ecosystem_test.go`) é verde, afirmando o percurso completo —
instalar, criar registro na mesma sessão, renderizar, desinstalar — ponta a
ponta.

Ao construí-la, a fase encontrou um defeito de motor da Fase 1 que ninguém
tinha alcançado: `Model.WritePatternFor` devolvia o primeiro padrão gravável
que a chave preenchia, e o padrão de workspace precisa só de `{id}` — então
`agents`, `templates`, `goals`, `collections` e `views` gravavam **fora** do
diretório da skill mesmo quando a chave trazia `skill`. A tese "instalar uma
skill instala uma equipe" não funcionava. Corrigido no motor, com table-test
por nativo multi-padrão.

Os oito domínios inicialmente declarados como fora do núcleo
(`artifact`, `template`, `instruction`, `project`, `goal`, `bot`, `tunnel`,
`marketplace`) estão todos wireados em `wire.go` e expostos em
`command-map.ts`; a maioria tem `status: pronto` na sua própria nota
(ver `docs/04 - Domínio/`). O que resta, rastreado por domínio:

- **Model Providers** (Fase 5, não Fase 8, mas ainda `em-construção`) —
  falta a tabela de preços (`CostUSD` sempre zero) e nenhum adaptador foi
  exercitado contra a API real de um provider.

### Fase 9 — Distribuição
Cross-compile, empacotamento, auto-update, `SKILL.md` publicada, completions.

**Notas:** [[Build e Cross-Compile]] · [[Empacotamento Wails3]] · [[Auto-Update]] · [[Especificação da Skill]] · [[SKILL (gerada)]]
**Entrega:** binários assinados e instaláveis.

**Parcialmente entregue.** [[Build e Cross-Compile]] e [[SKILL (gerada)]]
fecharam — build reprodutível verificado, gate de tamanho, `SKILL.md` +
26 referências geradas do registry real, sincronização testada.
[[Auto-Update]] tem o núcleo (Check/Download/Apply, verificação de
assinatura e checksum, espera-troca-rollback) mas não a coordenação com
`~/.mcp.json`. [[Especificação da Skill]] tem uma divergência disclosed: a
estrutura de cinco seções por grupo é verificada mas não imposta — 0 dos 26
grupos a têm hoje. **Completions** já existiam (`aos self completions`,
gerador embutido do cobra) — nada a construir.

**Não iniciado, e é o que falta para "binários assinados e instaláveis":**
[[Empacotamento Wails3]] inteira — assinatura/notarização macOS, MSI
Windows, `.deb`/`.rpm`/AppImage/Flatpak Linux. Bloqueada em duas decisões
que não são deste código: o nome definitivo do produto (a própria nota já
avisa — "Fase 9 pressupõe o nome definido") e as credenciais de assinatura
(Apple Developer ID, certificado de code-signing Windows), que só o dono do
projeto tem ou pode obter.

## Dependências entre fases

```
0 ──► 1 ──► 2 ──► 3 ──► 4 ──► 5 ──► 6 ──► 8 ──► 9
                              │             ▲
                              └──► 7 ───────┘
```

A Fase 7 (desktop) depende de 5 e pode correr em paralelo com 6. A Fase 8 precisa das duas.

## Caminho crítico

Se o tempo apertar, a ordem de valor por esforço, herdada da análise do original e confirmada por este design:

1. **Motor de coleções** — dá persistência, versionamento e editabilidade
2. **Ponte de superfícies** — dá cinco superfícies pelo preço de uma
3. **Prompt em XML com níveis de confiança** — dá qualidade de comportamento e defesa contra injeção
4. **Spillover de tools** — resolve o maior gargalo prático
5. **Subconsciente** — dá continuidade real, a proposta de valor central

Os itens 3 e 5 são os que separam este sistema de um wrapper de LLM.

## Definição de pronto por fase

1. Notas do vault atualizadas com `status: pronto`
2. Testes verdes, com saída anexada à nota
3. ADRs escritos para as decisões tomadas na fase
4. `SKILL.md` regenerada
5. Binários compilam para as seis plataformas
6. Grafo do vault válido (`validate-graph.mjs`)

## Riscos e mitigações

| Risco | Fase | Mitigação |
|---|---|---|
| Adaptadores de provider consomem mais tempo que o previsto | 5 | Começar por um (OpenAI); os outros entram atrás da suíte de contrato |
| Wails v3 ainda em evolução | 7 | Isolar a API do Wails em `wailsvc`; o resto do desktop não a conhece |
| TOON sem implementação Go madura | 2 | Implementação própria com golden; `json` como fallback |
| Volume de escrita do watcher em repositório grande | 1 | Debounce, ignores agressivos, e medição na fixture `Large` |
| Nome do produto ainda indefinido | 9 | Fases 0–8 usam o marcador; a 9 pressupõe o nome ([[ADR-0000 Nome provisório do projeto]]) |

## Critério de pronto

- [ ] Dez fases com entrega verificável definida
- [ ] Dependências mapeadas
- [ ] Riscos com mitigação por fase
