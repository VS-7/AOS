# 08 · Operação

> [Índice](../README.md) · Anterior: [Agentes de código](07-agentes-de-codigo.md) · Próximo: [Desenvolvimento](09-desenvolvimento.md)

## Instalar

O caminho curto está no [README](../../README.md); os requisitos por
plataforma, o AppImage, os checksums e a solução de problemas estão em
[INSTALL.md](../../INSTALL.md). O resumo:

| Plataforma | Comando |
|---|---|
| macOS, Linux | `curl -fsSL https://raw.githubusercontent.com/VS-7/AOS/main/install.sh \| sh` |
| Windows | `AOS-setup-<versão>-windows-amd64.exe` em Releases |
| Servidor | `curl … \| AOS_SERVER=1 sh` |

Variáveis do instalador: `AOS_VERSION`, `AOS_PREFIX` (padrão `~/.local/bin`),
`AOS_NO_CLI=1`, `AOS_SERVER=1`, `AOS_WORKSPACE` (padrão `~/aos`).

## O que fica no disco

**`~/.aos` — a instalação** (diretórios `0700`, segredos `0600`, auditados no
boot; uma permissão frouxa é apertada e o log diz que foi):

```
~/.aos/
├── config.json          configuração da instalação        ← segredo
├── users.json           contas e tokens (argon2id)        ← segredo
├── local.token          a credencial do terminal          ← segredo
├── data/jobs.sqlite     a fila de jobs
├── runtime/gateway/     gateway.json · gateway.lock · gateway.log
├── runtime/update/      o que a atualização baixou
├── workspaces/<id>/     registro + índice de busca
├── themes/              temas instalados
└── tmp/outputs/         spillover de resultados de tool
```

**`<repo>/.aos` — o workspace.** Markdown e JSON, para versionar junto com o
código. Ver [Domínio](05-dominio.md#onde-cada-coisa-vive).

`AOS_HOME_DIR` muda a primeira; `AOS_WORKSPACE_PATH`, a segunda.

## Operar o daemon

```sh
aos gateway status      # está de pé? em que porta? saudável?
aos gateway start
aos gateway stop
aos gateway restart
```

O gateway guarda o registro em `~/.aos/runtime/gateway/gateway.json`, protege
a operação com um lock, e mantém o log em `gateway.log` (nunca truncado num
restart — é ele que explica a saída anterior). Um registro que aponta para um
processo morto é detectado e limpo.

O daemon também pode ser executado à mão:

```sh
aosd serve                      # o daemon
aosd --mcp                      # MCP em stdio, sem passar pelo `aos`
aosd self llms --full           # a superfície inteira
```

## Variáveis de ambiente

Todas com o prefixo `AOS_`, resolvidas em camadas (ambiente → `.env` do
workspace → padrão).

| Variável | Padrão | O que faz |
|---|---|---|
| `AOS_HOME_DIR` | `~/.aos` | Raiz do estado da instalação |
| `AOS_WORKSPACE_PATH` | o diretório atual, se puder ser um workspace | O diretório que este daemon serve |
| `AOS_WORKSPACE_ID` | vazio | Qual workspace um comando endereça |
| `AOS_AGENT_ID` | vazio | O agente que está chamando, quando é um. O `aos` e o `aos --mcp` o enviam ao daemon — é o que faz `agents_me` responder e uma memória ter dono |
| `AOS_SERVER_HOSTNAME` | `127.0.0.1` | Endereço de bind (ADR-0009) |
| `AOS_SERVER_PORT` | `5326` | Porta do daemon |
| `AOS_TOKEN` | vazio → `~/.aos/local.token` | Credencial dos clientes |
| `AOS_ENV` | — | `production` desliga o navegador de schemas em `/api/docs` |
| `AOS_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `AOS_LOG_FORMAT` | `auto` | `text`, `json`, `auto` |
| `AOS_ALLOWED_ORIGINS` | vazio | Origens de navegador permitidas (CORS e WebSocket) |
| `AOS_JOBS_CONCURRENCY` | `20` | Workers da fila |
| `AOS_JOBS_TICK` | `15m` | Intervalo do agendador |
| `AOS_TOOL_CONCURRENCY` | `4` | Tools em paralelo por turno |
| `AOS_APPROVAL_DEADLINE` | `120s` | Quanto um pedido de aprovação espera |
| `AOS_SHUTDOWN_TIMEOUT` | `15s` | Espera pelas requisições em voo |
| `AOS_FSYNC` | ligado | `off` desliga o fsync (só para testes) |
| `AOS_UPDATE_BASE_URL` | vazio | Feed de releases; vazio desliga a verificação |
| `AOS_DAEMON_PATH` | ao lado do cliente | Caminho explícito do `aosd` |
| `AOS_NO_COLOR` | — | Desliga cor na saída do terminal |

## Configuração

`~/.aos/config.json`, relido a cada requisição — dá para editar com o daemon
rodando. Pelo terminal:

```sh
aos config get
aos config update --set security.enabled=true --reason "expondo por proxy"
```

Seções: `user`, `agents`, `region`, `general`, `notifications`, `security`,
`telemetry`, `tunnel`, `marketplace`, `mcp`.

## Segurança

**O que é garantido por desenho**

- **Loopback por padrão.** Expor além disso com `security.enabled=false`
  **aborta o boot** (`AOS_SERVER_EXPOSED_WITHOUT_AUTH`) em vez de servir um
  workspace aberto.
- **Toda superfície autenticada.** `/api`, `/mcp` e `/ws` respondem ao mesmo
  middleware. Uma instalação que pede autenticação e não tem quem a faça
  recusa a requisição — falha fechada.
- **Segredos com permissão restrita**, auditados a cada boot.
- **Duas credenciais, dois clientes.** O token do terminal
  (`~/.aos/local.token`) e a sessão da janela são independentes: sair da conta
  em uma não derruba a outra.
- **Sandbox por allowlist.** Binários nomeados, `denyArgs`, shell com opt-in
  próprio. Um executável dentro do workspace nunca satisfaz uma entrada da
  allowlist só por ter o nome dela.
- **Aprovação real.** Ações sensíveis perguntam a um humano pelo canal em
  tempo real; sem canal, a negação é imediata e explícita.
- **Confinamento de caminho.** Identificadores que viram diretório são
  validados como um único segmento; caminhos são resolvidos através de
  symlinks antes de qualquer comparação.
- **Redação de segredos no log**, por chave e por formato de credencial.

**O que fica com você**

- Colocar um proxy com TLS na frente antes de publicar.
- Ligar `security.enabled` antes de expor.
- Revisar a allowlist dos agentes que executam comandos.
- Tratar conteúdo de terceiros como não confiável — o prompt já marca
  `trust="unverified"`, mas uma allowlist generosa anula o cuidado.

## Limitações do beta

**Sem assinatura.** Nenhum binário tem certificado de desenvolvedor. No macOS
o instalador contorna a quarentena (o `curl` não marca o download); no Windows
o SmartScreen avisa toda vez. Baixe só do repositório oficial e
[confira os checksums](../../INSTALL.md#verificando-o-download).

**Sem atualização automática.** O núcleo existe (verificar, baixar, conferir
checksum e assinatura, aplicar com rollback) mas o feed está desligado
(`AOS_UPDATE_BASE_URL` vazio). Atualizar hoje é rodar o instalador de novo.

**Plataformas.** A janela: macOS Apple Silicon, Windows e Linux x86-64. O
servidor e o terminal também em `arm64` no Linux.

**Uma conta por instalação.** O modelo de contas existe (papéis, tokens,
membros de workspace), mas o fluxo é de uma pessoa por máquina.

## Diagnóstico

```sh
aos gateway status                 # o estado do daemon
curl -s localhost:5326/api/health  # ele responde?
aos self tools                     # o que este build publica
aos version --json                 # versão, commit, data
tail -f ~/.aos/runtime/gateway/gateway.log
AOS_LOG_LEVEL=debug aosd serve     # o daemon falando
```

| Sintoma | Causa provável |
|---|---|
| `aos --help` mostra só quatro comandos | Não há conta ainda, ou o daemon está fora do ar. A árvore vem dele. |
| A janela diz que não alcança o daemon | O `aosd` precisa estar na mesma pasta do executável do aplicativo. |
| Linux: a janela não aparece | Falta GTK4 ou WebKitGTK. Rode `aos-desktop` no terminal: ele nomeia a biblioteca. |
| macOS diz que o aplicativo está danificado | Veio do navegador, com quarentena. Reinstale pelo `curl`, ou `xattr -dr com.apple.quarantine /Applications/AOS.app`. |
| A porta 5326 está ocupada | `aos gateway status` diz o que está lá; ou mude `AOS_SERVER_PORT`. |
| A interface não atualiza sozinha | O canal de eventos não abriu. Com `AOS_LOG_LEVEL=debug`, o daemon diz por quê. |

## Backup

O que importa são dois diretórios:

- **`<repo>/.aos`** — o trabalho. Já está no Git, se você o versionou.
- **`~/.aos`** — contas, configuração, fila e índices. `users.json`,
  `config.json` e `local.token` são segredos; `data/` e `workspaces/*/index`
  são reconstruíveis.

> Próximo: [09 · Desenvolvimento](09-desenvolvimento.md)
