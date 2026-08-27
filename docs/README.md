# Documentação do AOS

Dois conjuntos, para dois leitores.

## Documentação do sistema — `docs/system/`

Para quem instala, opera, integra ou decide. Markdown simples, links
relativos, diagramas em Mermaid — feito para ser lido aqui no GitHub.

| # | Documento | Responde |
|---|---|---|
| 01 | [Visão geral](system/01-visao-geral.md) | O que é o AOS, para quem, que problema resolve, vocabulário |
| 02 | [Requisitos](system/02-requisitos.md) | O que o sistema deve fazer e sob quais restrições — RF, RNF, critérios |
| 03 | [Casos de uso](system/03-casos-de-uso.md) | Quem faz o quê, passo a passo, com diagramas de sequência |
| 04 | [Arquitetura](system/04-arquitetura.md) | Os três binários, as camadas, o caminho de uma chamada, o que vai para o disco |
| 05 | [Domínio](system/05-dominio.md) | Entidades, estados, relações e onde cada arquivo vive |
| 06 | [Canais](system/06-canais.md) | Aplicativo, terminal, navegador, MCP, Telegram, webhooks, túnel |
| 07 | [Agentes de código](system/07-agentes-de-codigo.md) | Skill e MCP para Claude Code, Codex, Cursor, Gemini CLI, OpenCode |
| 08 | [Operação](system/08-operacao.md) | Instalação, diretórios de estado, variáveis, configuração, segurança, diagnóstico |
| 09 | [Desenvolvimento](system/09-desenvolvimento.md) | Ambiente, tarefas, gates, layout, como estender |
| 10 | [Release](system/10-release.md) | Versionamento, CI, pipeline de release, artefatos |

## Vault de especificação — `docs/00…09 - *`

O contrato de engenharia com o qual o sistema foi construído: 16 ADRs, sete
peças críticas, uma nota por domínio, transportes, frontend, qualidade e
entrega. São notas Obsidian (wikilinks, frontmatter) validadas no CI por
`docs/_scripts/validate-graph.mjs` — zero links quebrados, zero órfãs.

Comece por **[AOS — Índice](00%20-%20Índice/AOS.md)**.

| Pasta | Conteúdo |
|---|---|
| `00 - Índice` | Mapa do vault e o prompt de implementação |
| `01 - Decisões` | ADR-0000 … ADR-0016 |
| `02 - Arquitetura` | Hexagonal, regra de dependência, erros, concorrência, layout |
| `03 - Peças Críticas` | Command Layer, Collections Engine, Prompt Assembly, Agent Loop, Tool Executor, Sandbox, Subconsciente |
| `04 - Domínio` | Uma nota por domínio (28) |
| `05 - Transporte` | HTTP, MCP, CLI, Wails, WebSocket, artefatos |
| `06 - Frontend` | React 19, design system, views declarativas, temas |
| `07 - Qualidade` | Testes, fixtures, observabilidade, segurança |
| `08 - Entrega` | Build, empacotamento, auto-update, roteiro de fases |
| `09 - Skill` | A skill publicada e sua especificação |

## Outros

- [README](../README.md) — o resumo
- [INSTALL](../INSTALL.md) — instalação, por plataforma
- [AGENTS.md](../AGENTS.md) — instruções para agentes de código que trabalham neste repositório
- [CHANGELOG](../CHANGELOG.md) — o que mudou por versão
- [pkg/skill/SKILL.md](../pkg/skill/SKILL.md) — a skill que os agentes leem, gerada do registro de comandos
