# Changelog

Todas as mudanças relevantes deste projeto. O formato segue
[Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/); as versões
seguem [SemVer](https://semver.org/lang/pt-BR/) com o sufixo `-faseN` que
marca a fase do [roteiro](docs/08%20-%20Entrega/Roteiro%20de%20Fases.md)
em que o release foi cortado.

## [Unreleased]

## [v0.13.0-fase9] — 2026-08-27

### Adicionado
- `aos --mcp`: servidor MCP em stdio no binário do terminal, roteado ao
  `/mcp` do daemon (`internal/transport/mcpproxy`). Inicia o daemon quando
  ele não está de pé. O daemon continua o único processo que escreve no
  workspace.
- `aos self skill install|targets|show`: a skill publicada vem embutida nos
  binários (`pkg/skill.Files`) e é instalada em Claude Code, Codex, Cursor,
  Gemini CLI, OpenCode e `~/.agents/skills` — por agente ou todos os
  detectados.
- O menu "Add skill to…" do aplicativo instala a skill de verdade
  (`SystemService.InstallSkill`); no navegador, copia o comando.
- Documentação do sistema em `docs/system/`: visão geral, requisitos, casos
  de uso, arquitetura, domínio, canais, agentes de código, operação,
  desenvolvimento e release. Índice em `docs/README.md`.
- `AGENTS.md` (e `CLAUDE.md`) para agentes de código que trabalham neste
  repositório. `CHANGELOG.md`.

### Corrigido
- A seção *Developers* do aplicativo instruía `aos --mcp` (que não existia no
  binário `aos`), `aos onboarding` (inexistente) e um `go install` de um
  módulo com caminho provisório. Agora instrui o instalador, o `--mcp` real e
  a instalação da skill.
- `daemonclient.Ready` perguntava por `/health`; o daemon responde em
  `/api/health`. O *ping* nunca dizia "pronto".

### Mudado
- README reescrito: o que importa, com links para a documentação.
- `docs/_scripts/validate-graph.mjs` ignora `docs/system/` e
  `docs/README.md`, que são documentação de produto e não notas do vault.

## [v0.12.3-fase9] — 2026-08-27
- Uma instalação nova abre dentro de um workspace, autenticada, no idioma do
  sistema. O CI passa a cobrir o frontend. Provedor Antigravity.

## [v0.12.2-fase9] — 2026-08-27
- O daemon do pacote *server* carrega a interface: uma VPS é um navegador.
  README e INSTALL para quem instala.

## [v0.12.1-fase9] — 2026-08-26
- AppImage do Linux funcionando (sem empacotar o WebKitGTK); instalador
  `curl` para Linux.

## [v0.12.0-fase9] — 2026-08-25
- Desktop para macOS, Windows e Linux, com o daemon junto. Sem assinatura.

## [v0.11.0-fase9] · [v0.10.0-fase9] — 2026-08-25
- Fase 9: build reprodutível, gates de tamanho e de caminhos absolutos,
  skill gerada do registro, release por tag para seis alvos.

## [v0.9.0-fase7] — 2026-08-20
- Fase 7 completa: 120 componentes e 26 features portados, design system,
  48 testes de frontend, `wails3 dev`.

## [v0.8.0-fase7] — 2026-08-15
- Desktop Wails3 sobre o daemon, 38 temas com contraste verificado.

## [v0.7.0-fase6] — 2026-08-15
- Continuidade: tarefa autônoma até `in_review`, rotina com três gatilhos,
  fila SQLite, subconsciente formando memórias.

## [v0.6.0-fase5] — 2026-08-15
- Runtime do agente: loop com cinco pontos de intervenção, prompt com
  autoridade por bloco, sandbox com allowlist, spillover, providers,
  aprovação real.

## [v0.5.0-fase4] — 2026-08-15
- Servidor e gateway: daemon HTTP, WebSocket, auth, supervisão.

## [v0.4.0-fase3] — 2026-08-15
- Domínio núcleo: workspace, agent, memory, chat.

[Unreleased]: https://github.com/VS-7/AOS/compare/v0.13.0-fase9...HEAD
[v0.13.0-fase9]: https://github.com/VS-7/AOS/compare/v0.12.3-fase9...v0.13.0-fase9
[v0.12.3-fase9]: https://github.com/VS-7/AOS/compare/v0.12.2-fase9...v0.12.3-fase9
[v0.12.2-fase9]: https://github.com/VS-7/AOS/compare/v0.12.1-fase9...v0.12.2-fase9
[v0.12.1-fase9]: https://github.com/VS-7/AOS/compare/v0.12.0-fase9...v0.12.1-fase9
[v0.12.0-fase9]: https://github.com/VS-7/AOS/compare/v0.11.0-fase9...v0.12.0-fase9
[v0.11.0-fase9]: https://github.com/VS-7/AOS/compare/v0.10.0-fase9...v0.11.0-fase9
[v0.10.0-fase9]: https://github.com/VS-7/AOS/compare/v0.9.0-fase7...v0.10.0-fase9
[v0.9.0-fase7]: https://github.com/VS-7/AOS/compare/v0.8.0-fase7...v0.9.0-fase7
[v0.8.0-fase7]: https://github.com/VS-7/AOS/compare/v0.7.0-fase6...v0.8.0-fase7
[v0.7.0-fase6]: https://github.com/VS-7/AOS/compare/v0.6.0-fase5...v0.7.0-fase6
[v0.6.0-fase5]: https://github.com/VS-7/AOS/compare/v0.5.0-fase4...v0.6.0-fase5
[v0.5.0-fase4]: https://github.com/VS-7/AOS/compare/v0.4.0-fase3...v0.5.0-fase4
[v0.4.0-fase3]: https://github.com/VS-7/AOS/releases/tag/v0.4.0-fase3
