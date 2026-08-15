---
tags: [qualidade, seguranca, hardening]
aliases: [Segurança, Hardening, Anti-padrões]
fase: 0
status: especificado
origem: "[[Autenticação e Credenciais]]"
---

# Segurança e Hardening

> Pai: [[AOS]] · Origem no original: [[Autenticação e Credenciais]] · Fase: 0

## Objetivo

Consolidar os 20 defeitos identificados no original, onde cada um é corrigido, e qual teste impede que volte. **Reproduzir um defeito conhecido é falha de execução, não fidelidade.**

## Matriz defeito → correção → teste

| # | Defeito no original | Correção | Onde | Teste |
|---|---|---|---|---|
| 1 | Source maps publicados (104 MB expondo o fonte) | Sem source maps; `-ldflags="-s -w"` | [[Build e Cross-Compile]] | Verifica ausência de símbolos no binário de release |
| 2 | Bind padrão `0.0.0.0` | Default `127.0.0.1`; expor exige flag + auth | [[ADR-0009 Bind em loopback por padrão]] | Boot com `--host 0.0.0.0` sem auth aborta |
| 3 | Playground com `security: () => true` | Autenticado e desabilitado em produção | [[HTTP chi]] | `/api/docs` sem token → 401 |
| 4 | Token aceito em query string | Só header | [[HTTP chi]] | Token em `?token=` é ignorado |
| 5 | WebSocket confia no cookie sem autorizar | Autorização no upgrade | [[Realtime WebSocket]] | Cookie forjado → 403 |
| 6 | Blocklist de comandos por basename | Allowlist + política por agente | [[ADR-0006 Allowlist no sandbox]] | Os 6 contornos documentados, negados |
| 7 | `config_get` sem filtro de campos sensíveis | Redação por tag, sempre para agentes | [[ADR-0010 Segredos com permissão restrita]] | Nenhum campo `secret` alcançável |
| 8 | `ask` degradando para `deny` | Canal real de aprovação | [[ADR-0007 Canal real de aprovação de tool]] | `ask` abre aprovação; headless nega com motivo distinto |
| 9 | Senha mínima de 6 caracteres | 12+ e lista de vazamentos embutida | [[Auth (Go)]] | Senha de 11 e senha vazada rejeitadas |
| 10 | Segredos em arquivos `0644` | `0600` na criação, auditado no boot | [[ADR-0010 Segredos com permissão restrita]] | Permissão frouxa é reparada e registrada |
| 11 | `ignore-certificate-errors` global no Electron | Não existe | [[ADR-0002 Wails3 no lugar de Electron]] | Ausência verificada no código |
| 12 | `development: true` fixo no servidor | Vem de `AOS_ENV`, default produção | [[HTTP chi]] | Boot default não expõe stack |
| 13 | `.withEnvironment('dev')` fixo nos jobs | Vem de configuração | [[ADR-0008 SQLite puro Go para filas]] | Ambiente refletido na fila |
| 14 | Nomes de tool mudando entre versões | Superfície versionada com aliases | [[ADR-0011 Superfície de tools versionada]] | Alias deprecado funciona e avisa |
| 15 | Colisão `skills` builtin × domínio | Builtins sob `aos self` | [[CLI cobra]] | `aos skills --help` mostra o domínio |
| 16 | `unhandledRejection` derruba o servidor | `recover()` por fronteira | [[Concorrência e Context]] | Panic em handler → 500, daemon vivo |
| 17 | Sem lock em escrita de arquivo | Lock por caminho + escrita atômica | [[ADR-0012 Escrita atômica e lock por arquivo]] | 50 escritores concorrentes sob `-race` |
| 18 | Gateway sem lock de PID | Lockfile com `flock` | [[Gateway (Go)]] | Dois `start` concorrentes → um processo |
| 19 | Senha de artifact derivada de segredo por processo | Hash persistido | [[Artifact (Go)]] | Senha válida após restart |
| 20 | Skills de terceiros sem sandbox nem assinatura | Manifesto + consentimento + procedência | [[ADR-0015 Skills com permissões declaradas]] | Conteúdo além do manifesto é recusado |

## O que o original faz bem — e mantemos

Registrado com o mesmo rigor, porque copiar acertos também é decisão:

- **argon2id** com `m=65536, t=2, p=1` — parâmetros adequados
- **Autoria de comentário server-side** — o agente não forja autoria e só edita os próprios
- **`.git` read-only no sandbox** — protege o histórico do repositório
- **Defesa contra injeção de template** — dados persistidos nunca passam pelo motor
- **Níveis de confiança no contexto** — `trust="unverified"` para conteúdo externo
- **`store: false` na OpenAI** — conversas não persistidas no provider
- **Hooks podem negar tool calls** — camada de política programável
- **Fronteira de privilégio no MCP** — `auth`, `events` e `tunnels` fora do alcance do agente
- **Normalização de path antes da checagem de contenção** — fecha path traversal clássico

## Modelo de ameaça

Ser explícito sobre o que este sistema defende e o que não defende:

| Ameaça | Postura |
|---|---|
| Outro usuário da mesma máquina | **Defendido** — permissões `0600`, bind loopback |
| Máquina da mesma rede local | **Defendido** — bind loopback por default; expor exige auth |
| Agente semi-confiável agindo por engano | **Defendido** — allowlist, aprovação, `.git` read-only |
| Skill de terceiro maliciosa | **Mitigado** — manifesto e consentimento reduzem, não eliminam |
| Prompt injection via conteúdo externo | **Mitigado** — níveis de confiança e prompt-mestre reduzem, não eliminam |
| Malware rodando como o usuário | **Não defendido** — tem acesso aos mesmos arquivos |
| Adversário com acesso físico | **Não defendido** |

Dizer o que não é defendido é parte da postura. O original apresenta um sandbox que sugere contenção que não entrega; preferimos o oposto.

## Evoluções registradas

| Item | Gatilho para reabrir |
|---|---|
| Sandbox do SO (seatbelt, seccomp/landlock, AppContainer) | Execução de código não confiável de terceiros |
| Keychain do SO para chaves de provider | Demanda de uso corporativo |
| Assinatura de publisher para skills | Registry com terceiros publicando |
| Container por execução de toolset `cli` | Toolsets de fontes não confiáveis |

## Testes

Além da coluna da matriz:

- **`govulncheck`** no CI, falhando em vulnerabilidade conhecida com correção disponível
- **Varredura de segredo em log:** teste que injeta um token conhecido no config e verifica que ele não aparece em nenhuma saída de log em uma sessão completa
- **Varredura de segredo em erro:** nenhum campo `secret:"true"` alcançável via `Issue` de `apperr`
- **Auditoria de permissão no boot** repara e registra

## Critério de pronto

- [ ] 20 defeitos corrigidos, cada um com teste nomeado
- [ ] Modelo de ameaça documentado e honesto
- [ ] `govulncheck` no CI
- [ ] Nenhum segredo alcançável por log ou erro
