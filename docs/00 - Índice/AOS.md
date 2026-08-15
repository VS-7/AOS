---
tags: [moc, indice, reconstrucao, go]
aliases: [AOS, Vault de Reconstrução, MOC Go]
fase: 0
status: especificado
origem: "[[PROMPT — Reconstrução em Go]]"
---

# AOS — Vault de Reconstrução em Go

> [!warning] O nome ainda não existe
> **`AOS`** é um codinome provisório (*Agent Operating System*). O produto original chama-se Fractal; **este sistema não é o Fractal e não deve levar esse nome**. Toda ocorrência de `aos`, `AOS` e `github.com/OWNER/aos` neste vault e no código é um marcador substituível em uma única operação. Ver [[ADR-0000 Nome provisório do projeto]].

Este é o **contrato de engenharia** da reconstrução. O vault de engenharia reversa ([[Fractal OS]]) descreve **o que** o sistema original faz; este vault descreve **como** cada peça será construída em Go. Nenhuma linha de código é escrita antes de a nota correspondente existir e estar coerente.

| Campo | Valor |
|---|---|
| Codinome | `AOS` — provisório |
| Linguagem | Go 1.23+ |
| Desktop | Wails v3 + React 19 + TypeScript |
| Módulo Go | `github.com/OWNER/aos` — placeholder |
| Binários | `aos` (CLI + MCP) · `aosd` (daemon) · `aos-desktop` |
| Porta | 5326 |
| Raiz de estado | `~/.aos` |
| Origem funcional | [[Fractal OS]] — 74 notas de engenharia reversa |

---

## Como navegar

### Construir — a Etapa B
- **[[PROMPT — Implementação do AOS]]** — ⭐ documento de missão da implementação: protocolo de leitura, paridade de frontend, versionamento e execução fase a fase

### Decisões — por que o sistema é assim
- [[ADR-0000 Nome provisório do projeto]] — o marcador e o procedimento de renomeação
- [[ADR-0001 Go como linguagem]] · [[ADR-0002 Wails3 no lugar de Electron]]
- [[ADR-0003 Command Layer unificada]] · [[ADR-0004 Collections em Markdown]]
- [[ADR-0005 Loop de agente próprio]] · [[ADR-0006 Allowlist no sandbox]]
- [[ADR-0007 Canal real de aprovação de tool]] · [[ADR-0008 SQLite puro Go para filas]]
- [[ADR-0009 Bind em loopback por padrão]] · [[ADR-0010 Segredos com permissão restrita]]
- [[ADR-0011 Superfície de tools versionada]] · [[ADR-0012 Escrita atômica e lock por arquivo]]
- [[ADR-0013 Bleve para busca full-text]] · [[ADR-0014 Liquid para templates]]
- [[ADR-0015 Skills com permissões declaradas]] · [[ADR-0016 Compatibilidade de nomes com o original]]

### Arquitetura — a forma do código
- [[Visão Geral Go]] — os três binários e o que cada um carrega
- [[Hexagonal e Regra de Dependência]] — a regra inviolável
- [[SOLID no Go]] · [[Padrões de Projeto Aplicados]]
- [[Layout de Diretórios]] · [[Concorrência e Context]] · [[Estratégia de Erros]]

### Peças críticas — as sete que sustentam o resto
- [[Command Layer]] ★ — uma definição, cinco superfícies
- [[Collections Engine]] ★ — o motor Markdown + frontmatter
- [[Prompt Assembly]] ★ — XML com níveis de confiança
- [[Agent Loop]] ★ — os cinco pontos de intervenção
- [[Tool Executor e Spillover]] ★ — o gargalo prático
- [[Sandbox (Go)]] ★ — contenção por allowlist
- [[Subconsciente (Go)]] ★ — o segundo LLM

### Domínio — 28 fatias verticais
**Núcleo:** [[Workspace (Go)]] · [[Agent (Go)]] · [[Memory (Go)]] · [[Chat (Go)]]
**Execução:** [[Task (Go)]] · [[Todo (Go)]] · [[Comment (Go)]] · [[Routine (Go)]] · [[Project (Go)]] · [[Goal (Go)]]
**Capacidade:** [[Skill (Go)]] · [[Instruction (Go)]] · [[Template (Go)]] · [[Toolset (Go)]] · [[Marketplace (Go)]]
**Dados & UI:** [[Collection (Go)]] · [[View (Go)]] · [[Artifact (Go)]] · [[File (Go)]] · [[Theme (Go)]]
**Infra:** [[Config (Go)]] · [[Auth (Go)]] · [[Model Providers (Go)]] · [[Event Hooks (Go)]] · [[Activity (Go)]] · [[Bot (Go)]] · [[Tunnel (Go)]] · [[Gateway (Go)]]

### Transporte — os adaptadores de entrada
- [[HTTP chi]] · [[MCP Go SDK]] · [[CLI cobra]] · [[Wails3 Services]] · [[Realtime WebSocket]] · [[Artifacts e Estáticos]]

### Frontend
- [[React 19 e Bindings]] · [[Design System]] · [[Views Declarativas]] · [[Temas]]

### Qualidade
- [[Estratégia de Testes]] · [[Testes de Contrato de Port]] · [[Fixtures e Golden Files]] · [[Segurança e Hardening]] · [[Observabilidade]]

### Entrega
- [[Build e Cross-Compile]] · [[Empacotamento Wails3]] · [[Auto-Update]] · [[Roteiro de Fases]]

### Skill publicada
- [[Especificação da Skill]] · [[SKILL (gerada)]]

---

## As três coisas que não podem ser cortadas

Se o tempo apertar, corte features — nunca estas:

1. **A ponte de superfícies** — uma definição de comando produz CLI, tool MCP, tool interna do agente, endpoint HTTP e documentação. Ver [[Command Layer]].
2. **O prompt com níveis de confiança** — `trust="trusted|observed|unverified"` como atributo XML é defesa contra prompt injection embutida no formato. Ver [[Prompt Assembly]].
3. **O subconsciente** — um segundo LLM que forma memórias sozinho, tirando a memória do caminho crítico do agente principal. Ver [[Subconsciente (Go)]].

O resto é execução competente. Estas três são a identidade do produto.

---

## Modelo mental em uma frase

```
O usuário fala → um Agente com identidade persistente recebe o pedido →
monta contexto XML a partir de Workspace + Memória + Skills + Instruções →
executa Tools que são a mesma definição do CLI →
que chamam serviços de domínio →
que gravam Markdown no repositório do usuário →
que voltam a alimentar o contexto da próxima sessão.
```

Esse ciclo fechado é o que o produto entrega. O nome disso é **continuidade**.

---

## Estado das notas

Toda nota carrega `status` no frontmatter:

| Status | Significado |
|---|---|
| `especificado` | Design fechado, pronto para implementar |
| `em-construcao` | Código sendo escrito contra esta especificação |
| `pronto` | Implementado, testado, com saída de teste anexada |

Nenhuma fase do [[Roteiro de Fases]] é declarada concluída sem que suas notas estejam em `pronto`.

## Validação do grafo

```bash
node "docs/_scripts/validate-graph.mjs"
```

Verifica: zero wikilinks quebrados, zero notas órfãs, frontmatter obrigatório presente em toda nota. Roda no CI — ver [[Estratégia de Testes]].
