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
**Entrega:** criar workspace, criar agente orquestrador, gravar e recuperar memórias com grafo.

### Fase 4 — Servidor e gateway
Daemon chi na :5326 com `/api`, `/mcp`, `/ws`, `/v/*`; gateway com máquina de estados e supervisão.

**Notas:** [[HTTP chi]] · [[Realtime WebSocket]] · [[Gateway (Go)]] · [[Auth (Go)]] · [[ADR-0009 Bind em loopback por padrão]] · [[ADR-0010 Segredos com permissão restrita]]
**Entrega:** `aos gateway start|stop|status` operando o daemon.

### Fase 5 — Runtime do agente
`agentloop`, `prompt`, `sandbox`, `toolexec`, providers, hooks, aprovação.

**Notas:** [[Agent Loop]] · [[Prompt Assembly]] · [[Sandbox (Go)]] · [[Tool Executor e Spillover]] · [[Model Providers (Go)]] · [[Event Hooks (Go)]] · [[ADR-0005 Loop de agente próprio]] · [[ADR-0006 Allowlist no sandbox]] · [[ADR-0007 Canal real de aprovação de tool]]
**Entrega:** conversa real com um agente que usa tools e persiste memória.

### Fase 6 — Continuidade
`subconscious`, `task` com ciclo de 8 estados e worktree, `routine` com 3 gatilhos, fila SQLite, `activity`.

**Notas:** [[Subconsciente (Go)]] · [[Task (Go)]] · [[Todo (Go)]] · [[Comment (Go)]] · [[Routine (Go)]] · [[Activity (Go)]] · [[ADR-0008 SQLite puro Go para filas]]
**Entrega:** task executada autonomamente do início ao `in_review`, com memórias formadas sozinhas.

### Fase 7 — Desktop
Wails3 + React 19; services; bindings; WebSocket; 38 temas; aparência nativa.

**Notas:** [[Wails3 Services]] · [[React 19 e Bindings]] · [[Design System]] · [[Temas]] · [[Theme (Go)]] · [[File (Go)]] · [[ADR-0002 Wails3 no lugar de Electron]]
**Entrega:** app desktop com chat, board de tasks e grafo de memórias.

### Fase 8 — Ecossistema
`skill`, `toolset`, `collection` e `view` dinâmicas, `artifact`, `template`, `instruction`, `project`, `goal`, `bot`, `tunnel`, `marketplace`.

**Notas:** [[Skill (Go)]] · [[Toolset (Go)]] · [[Collection (Go)]] · [[View (Go)]] · [[Views Declarativas]] · [[Artifact (Go)]] · [[Artifacts e Estáticos]] · [[Template (Go)]] · [[Instruction (Go)]] · [[Project (Go)]] · [[Goal (Go)]] · [[Bot (Go)]] · [[Tunnel (Go)]] · [[Marketplace (Go)]] · [[ADR-0014 Liquid para templates]] · [[ADR-0015 Skills com permissões declaradas]]
**Entrega:** instalar uma skill que traz agente, coleção e view próprios.

### Fase 9 — Distribuição
Cross-compile, empacotamento, auto-update, `SKILL.md` publicada, completions.

**Notas:** [[Build e Cross-Compile]] · [[Empacotamento Wails3]] · [[Auto-Update]] · [[Especificação da Skill]] · [[SKILL (gerada)]]
**Entrega:** binários assinados e instaláveis.

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
