# Changelog

Todas as mudanças relevantes deste projeto. O formato segue
[Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/); as versões
seguem [SemVer](https://semver.org/lang/pt-BR/) com o sufixo `-faseN` que
marca a fase do [roteiro](docs/08%20-%20Entrega/Roteiro%20de%20Fases.md)
em que o release foi cortado.

## [Unreleased]

## [v0.15.1-fase9] — 2026-09-03

A rodada que fez o aplicativo funcionar de fato. Onze correções, cada uma com
teste que falha no código anterior, e uma verificação ponta a ponta contra um
daemon limpo.

### Adicionado

- **Parar um turno.** `chats_stop` cancela o turno que está rodando numa
  conversa. O botão Parar do compositor chamava um comando que não existia: o
  facade respondia o envelope dormente como *sucesso*, a tela dizia "nenhuma
  execução ativa foi encontrada" e o agente seguia trabalhando. Um turno
  parado é gravado como `interrupted`, não como erro — o `StatusInterrupted`
  existia desde o começo e nada nunca o atribuía, porque nada podia parar um
  turno.
- **Reagir a uma mensagem.** `chats_react` alterna uma reação.
  `Message.Reactions` era persistido desde o início e nenhum comando o
  tocava, então o seletor de emoji da lista de mensagens não tinha nada
  atrás. Quem reage vem da identidade da chamada, nunca do payload.
- **Uma tela de "daemon caiu".** A supervisão era um passo único de boot: um
  daemon que morria deixava o aplicativo desenhando suas telas enquanto toda
  ação falhava com "Load failed" sem tradução. A janela agora observa o
  daemon, avisa a interface, traz ele de volta e readota o workspace.

### Corrigido

**A janela estava inscrita num canal em que o daemon nunca publicou.** O
`active` do `wire.go` só tem valor quando o escopo foi *fixado* — um workspace
secundário resolvido para um chamador, ou `AOS_WORKSPACE_ID`. Numa instalação
desktop ou de terminal não há nenhum dos dois: o supervisor passa o *caminho*
do workspace, e o workspace é registrado depois, pelo `workspace_introspect`
da própria janela. Então toda atividade, toda mudança de coleção e todo pedido
de aprovação saíam em `workspace::` enquanto a janela ouvia `workspace::<id>`.

É isso inteiro o "nada atualiza sozinho" — um projeto, uma meta ou uma tarefa
criada por um agente nunca aparecia, a caixa de entrada não se mexia — e, pior,
o diálogo de aprovação nunca abria: toda ferramenta com aprovação esperava o
prazo e era negada. O agente informava que não conseguiu; a pessoa nunca foi
perguntada. Um único resolvedor preguiçoso agora responde para o sink de
atividade, o publicador de coleções, o notificador de aprovação e o próprio
turno.

**Toda repositório publica `collection.changed`**, que estava ligado só em oito
coleções, e o evento ganhou tags json — ele é a carga de um frame do WebSocket
e estava sendo serializado com os nomes de campo do Go. A interface passa a
agir sobre ele, inclusive atualizando as stores pré-carregadas por trás da
barra lateral, que invalidação de cache nenhuma alcança.

**O agente podia parar o próprio daemon.** `gateway_stop`, oferecido a um
modelo como ferramenta comum, encerra o processo em que o turno está rodando.
Os quatro comandos de `gateway` e `workspace_create`/`delete` saíram do
registro do agente.

**Todo agente que o orquestrador criava só sabia ler** — o valor zero do
sandbox é certo para um AGENT.md escrito à mão e errado para uma criação, então
todo especialista era recusado no primeiro Write ou Bash.

**Cada ferramenta aparecia duas vezes no chat e nunca como "executando"**, todo
turno terminado dizia "Worked for 0s", o raciocínio transmitido era descartado
ao salvar, e a mensagem que a pessoa enviava sumia quando o daemon confirmava.

**28 telas relatavam sucesso sobre recusas do daemon** — a `mutate` do facade
resolve com `{data, error}` e nunca rejeita, e o código portado foi escrito
como `try { await mutate() } catch`. Mover uma tarefa por `tasks_update`, que
recusa status, era a mais visível.

**Nenhuma imagem, PDF ou vídeo carregava** no painel de arquivos, e um terminal
dentro de um repositório endereçava outro workspace.

**A janela pedia senha a cada abertura**, recarregar a transformava numa aba de
navegador quebrada, e uma sessão expirada não tinha caminho de volta ao login.

**Criar projeto, meta ou agente era invisível para a caixa de entrada.** As
escritas passaram a invalidar os caches certos, mas ainda não *diziam o que
aconteceu*: `collection.changed` não carrega título, então a caixa de entrada
não tem o que mostrar e as rotinas só reagem a activities. Os três publicam
agora, sob o nome singular em que a interface indexa suas queries.

**O editor de template nunca salvava nada.** A área de texto dizia "JSON
schema" e mandava um campo `schema` que `templates_create`/`templates_update`
não têm, então o decodificador o descartava toda vez. O que o Go guarda é
`variables` — a lista que o corpo Liquid lê por nome — e é isso que o editor
edita.

**A janela pedia senha a cada abertura**, recarregar a transformava numa aba
de navegador quebrada, e uma sessão expirada não tinha caminho de volta ao
login.

### Mudado
- `agents_create` deriva o slug do nome; `agents_create`/`update` aceitam
  `image`; `goal` tem `priority`; `tasks_list` filtra por `query` e `priority`;
  `chats_send` aceita `context`, separado do que a pessoa digitou;
  `memories_store` aceita `agent`.
- `gateway_status` responde por si mesmo em vez de descrever o registro do
  supervisor, e `gateway_restart` recusa de dentro do daemon em vez de mandar
  um sinal para o próprio processo.
- `AOS_APPROVAL_DEADLINE` passa a ser lido.
- Uma chamada que não nomeia workspace é roteada para aquele a que o
  diretório do chamador pertence — o terminal e o `aos --mcp` dentro de um
  repositório param de endereçar outro.

## [v0.15.0-fase9] — 2026-08-30

### Corrigido

**O agente parava de responder no meio do trabalho.** A compactação do
histórico mantinha as chamadas de ferramenta das últimas 15 mensagens e
removia as mais antigas, por índice. Mas um turno são **duas** mensagens — a do
assistente que pede a ferramenta e a que responde — e o corte caía entre elas:
a mensagem que pedia perdia as chamadas e era descartada como vazia, enquanto o
resultado, um índice adiante, ficava dentro da janela. O que chegava ao
provider era um `function_call_output` sem o `function_call` correspondente, e a
API recusava a requisição inteira:
`No tool call found for function call output with call_id …` →
`AOS_AGENT_PROVIDER_FAILED`. Toda sessão longa o bastante para compactar morria
ali. A janela agora só se alarga: o resultado que fica leva junto a chamada que
ele responde.

**O orquestrador não conseguia fazer nada.** Um agente sem bloco `sandbox`
recebe o valor zero — somente leitura, sem execução — e o orquestrador, o único
agente que todo workspace ganha e com quem a pessoa de fato conversa, era criado
sem um. A experiência padrão do produto era um assistente que lia o workspace,
não escrevia uma linha nele e não rodava um comando: planejava, delegava, e
informava que o sandbox tinha recusado. Ele nasce com leitura, escrita, exclusão
e execução por allowlist (ADR-0006), sem shell. Um `orchestrator.sandbox` no
`workspace_create` continua valendo e pode restringir.

**O raciocínio saía colado.** Um turno são várias chamadas ao modelo e cada uma
escreve seu próprio bloco; todas iam para um único acumulador, sem separador —
"…sem suposições.Agora vou olhar…". Cada chamada vira uma parte própria, e a
interface mostra um passo por pensamento.

**"Worked for 0s".** O `chat.Message` do Go carrega `createdAt` e `runs` no
topo e não tem `metadata`; todo consumidor da interface lia
`message.metadata.createdAt`. Ou seja: `undefined` em todos, e o cabeçalho
mostrava zero em turnos de minutos. A tradução acontece uma vez, em
`command-map.ts`.

**O indicador de agente trabalhando.** O "..." repetia o que a marca animada já
diz e quebrava linha num painel estreito, virando uma fileira de pontos soltos
sob a frase. Saiu; a marca não encolhe nem quebra, e o texto foi traduzido.

### Mudado
- `workspace_create` aceita `orchestrator.sandbox` para declarar o que o
  primeiro agente pode fazer.


## [v0.14.1-fase9] — 2026-08-30

### Corrigido
- **Configurações não abriam.** A v0.14.0 fez uma *query* dormente resolver
  para um objeto marcado em vez de `null`, para a tela poder distinguir "não
  existe" de "lista vazia". Isso quebrou todo call site escrito contra o
  `null`: o menu de configurações faz `(query.data ?? []).some(...)`, o `??`
  deixou de curto-circuitar, e a tela inteira caía com
  `.some is not a function` antes de renderizar.

  A distinção agora fica no **resultado** (`query.isDormant`), lida do próprio
  `COMMAND_MAP` — vale desde o primeiro render, sem ida ao daemon — e `data`
  volta a ser `null`, que é contra o que todo o código portado foi escrito.


## [v0.14.0-fase9] — 2026-08-30

Duas auditorias fechadas: a da API/CLI e a do aplicativo desktop.

### Corrigido

**Onboarding — o copiloto sempre nascia "Atlas".** `AuthService` chamava
`afterAuth` *dentro* de `Onboarding`, antes de a resposta chegar à janela, e o
desktop registrava um workspace ali — com o nome da pasta e o orquestrador
padrão. Quando o `workspace_create` do assistente rodava, já havia workspace, e
ele desistia. O nome do workspace, o nome do copiloto, o tom, o estilo e a
autonomia coletados em cinco telas eram descartados, sempre. O assistente passa
a ser dono do primeiro workspace; o onboarding só adota.

**O canal de aprovação não tinha interface.** O ADR-0007 define que um hook
pode responder "ask" e a chamada espera por uma pessoa. O daemon tinha tudo —
broker, `approvals_list`, `approvals_decide`, evento no socket — e nenhuma tela
chamava nada disso: o agente pedia permissão e esperava o deadline até ser
negado. Agora há o modal, com Negar / Permitir sempre / Aprovar uma vez.

**Memórias eram invisíveis.** Só `memories_graph` estava ligado; guardar,
recordar, refletir e esquecer não tinham interface. A aba de memórias do agente
ganhou lista com busca, filtro por categoria, leitura completa, escrita e o
esquecer-com-motivo (que descontinua e mantém o rastro).

**Telas que não faziam nada.** `task.start` chamava um comando dormente e
mostrava "Failed to start task"; a tela de Membros mostrava lista vazia para
sempre porque o `DormantGate` dela listava um comando que havia sido ligado, e
o gate só dispara quando todos estão dormentes.

**`schema: true` nunca funcionava.** A validação rodava antes da checagem, então
o `cta` de toda mensagem de erro mandava fazer algo que caía no mesmo erro.
Agora é checado antes de decodificar, em todas as superfícies, e `--schema`
voltou a valer para comandos servidos pelo daemon.

**`self tools` / `self llms` descreviam 4 de 140 comandos.** Liam o registro
local do binário do terminal. O daemon publica `GET /api/_manifest` e eles
passam a lê-lo.

**`workspace_introspect` falhava no caso normal.** Colisão de nome respondia
`ALREADY_EXISTS` em vez de registrar; e resolvia o diretório do *daemon*, não o
de quem chamou.

**16 CTAs apontavam para comandos inexistentes** — `aos auth token issue`,
`aos gateway logs`, `tasks_set_status` (o comando é `set-status`) e outros. Um
teste estático agora falha o build quando um CTA nomeia grupo, comando ou tool
que não existe.

**A interface misturava idiomas.** A primeira tela dizia "Bem-vindo ao AOS"
sobre um botão "Get Started". As telas de entrada foram traduzidas, e um teste
novo falha quando copy renderizada não passa por `t()`.

**O onboarding mentia sobre o progresso.** As etapas avançavam por cronômetro,
fora da ordem das chamadas reais. Agora cada etapa embrulha a chamada que nomeia.

### Adicionado
- **Atualizações** e **Daemon** em Configurações: procurar, baixar-e-verificar
  e instalar uma versão nova; status e reinício do daemon.
- **Jobs** em Configurações do workspace: a fila de execução, com recuperação
  dos travados e limpeza dos concluídos.
- `GET /api/_manifest`: a superfície inteira (documentação e schemas) para
  clientes que não são o CLI.
- `--agent`, `--base-url`, `--token` e `--workspace` nos comandos servidos pelo
  daemon. Sem eles, o CLI não conseguia escrever nada como um agente.
- Conjuntos fechados viram `enum` de verdade no JSON Schema, pela interface
  `command.Enumerator` que o próprio tipo do domínio implementa.
- `workspace_get`/`update`/`delete`/`inventory` aceitam `id`, como todo o resto.

### Mudado
- O metadado do comando saiu de `issue.tool` para `issue._command`: um comando
  com campo de domínio chamado `tool` (`toolsets_call`) sobrescrevia o próprio
  metadado.
- Uma *query* dormente deixa de resolver para `null` e passa a carregar uma
  marca, para a tela poder dizer que a capacidade não existe em vez de mostrar
  lista vazia.
- Removida a segunda tela de onboarding (rota `/onboarding`), que não podia
  funcionar: mandava o corpo aninhado onde o mapa lia chaves planas, e exigia
  um `workspaceId` que a API nunca devolveu.


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
