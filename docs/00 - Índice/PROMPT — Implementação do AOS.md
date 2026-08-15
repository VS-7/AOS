---
tags: [moc, prompt, implementacao, missao]
aliases: [PROMPT Implementação, Missão de Construção, Etapa B]
fase: 0
status: especificado
origem: "[[PROMPT — Reconstrução em Go]]"
---

# PROMPT — Implementação do AOS

> **Documento de missão da Etapa B.** Este arquivo é a entrada única para o agente que vai **construir** o AOS. A Etapa A (planejar) está concluída: o vault de reconstrução existe em `docs/` com 81 notas, grafo validado, zero links quebrados.
>
> **Leia este documento inteiro antes de qualquer ação. Depois leia o que a seção 2 manda ler. Só então escreva código.**

---

## 0. Identidade e missão

Você é o **engenheiro-executor** deste projeto. Sua missão é construir o **AOS** — uma reimplementação em **Go + Wails3 + React** de um sistema operacional para agentes de IA, funcionalmente idêntico ao produto original (Fractal), que foi integralmente submetido a engenharia reversa.

A especificação já existe. Você **não** vai redesenhar nada. Vai implementar o que está especificado, na ordem especificada, provando cada passo com teste.

### O que "idêntico" significa aqui

Três níveis, com exigências diferentes:

| Nível | Exigência |
|---|---|
| **Comportamento do sistema** | **Paridade funcional total.** Toda capacidade do original existe no AOS, com a mesma semântica, os mesmos nomes de tool, os mesmos estados, os mesmos gatilhos. |
| **Interface do aplicativo** | **Paridade visual e funcional total.** As mesmas telas, os mesmos componentes, os mesmos fluxos, o mesmo layout. Ver seção 5. |
| **Implementação interna** | **Livre, dentro da especificação.** Go idiomático, não tradução linha a linha de TypeScript. |

### As três divergências que permanecem

O vault documenta divergências **deliberadas** em relação ao original — todas de segurança e robustez, todas invisíveis para quem usa o produto. Elas **não** são negociáveis e **não** contradizem a exigência de paridade:

1. **Correções de defeito** — os 20 anti-padrões catalogados em [[Segurança e Hardening]]. Reproduzir um defeito conhecido é falha de execução, não fidelidade.
2. **`ask` que pergunta de verdade** — [[ADR-0007 Canal real de aprovação de tool]]. No original, `ask` degrada silenciosamente para `deny`, o que torna a matriz de autonomia do prompt-mestre inexequível.
3. **Allowlist no lugar da blocklist** — [[ADR-0006 Allowlist no sandbox]]. A blocklist do original é contornável com `bash -c`.

Cada ADR em `docs/01 - Decisões/` que declara uma divergência tem a justificativa completa. **Não reabra essas decisões.** Se encontrar uma razão técnica forte para reabrir, pare, escreva a análise e pergunte — não decida sozinho.

### O nome

O sistema chama-se **AOS** — codinome **provisório**. Ver [[ADR-0000 Nome provisório do projeto]].

- Módulo Go: `github.com/OWNER/aos` — `OWNER` é literal e greppável
- Binários: `aos`, `aosd`, `aos-desktop`
- Diretório de estado: `~/.aos` — **jamais** `~/.fractal`, que pertence à instalação real do original nesta máquina e seria corrompido
- Branding concentrado em `internal/core/build/brand.go`

**Nunca chame o sistema novo de "Fractal".** Esse nome refere-se exclusivamente ao produto original sob engenharia reversa.

---

## 1. Onde está tudo

Três corpos de material, com autoridade decrescente:

```
Fractal Reverse Enginner/
├── docs/                    ← ★ ESPECIFICAÇÃO — a autoridade máxima (81 notas)
├── Fractal Vault/           ← engenharia reversa do original (74 notas)
└── _extracted/              ← código-fonte TypeScript original (1.139 arquivos)
```

### 1.1 `docs/` — a especificação

**É o contrato de engenharia.** Diz *como* construir. Quando `docs/` afirma algo, é isso que se implementa.

```
docs/
├── 00 - Índice/
│   ├── AOS.md                          ← MOC, comece aqui
│   └── PROMPT — Implementação do AOS.md ← este arquivo
├── 01 - Decisões/    (17 ADRs)         ← por que o sistema é assim
├── 02 - Arquitetura/ (7 notas)         ← a forma do código
├── 03 - Peças Críticas/ (7 notas)      ← ★ as que sustentam o resto
├── 04 - Domínio/     (28 notas)        ← uma por feature
├── 05 - Transporte/  (6 notas)         ← HTTP, MCP, CLI, Wails, WS, estáticos
├── 06 - Frontend/    (4 notas)         ← React, design system, views, temas
├── 07 - Qualidade/   (5 notas)         ← testes, contratos, golden, segurança, observabilidade
├── 08 - Entrega/     (4 notas)         ← build, empacotamento, update, roteiro
├── 09 - Skill/       (2 notas)         ← a skill publicada
└── _scripts/validate-graph.mjs         ← validação do grafo
```

Toda nota tem a mesma estrutura: **Objetivo → Comportamento do original → Design em Go → Decisões e divergências → Testes → Critério de pronto**. O código Go nas notas é real e compilável, não pseudocódigo — use-o como ponto de partida literal.

### 1.2 `Fractal Vault/` — a engenharia reversa

**Diz o que o original faz.** Consulte quando `docs/` referenciar uma nota daqui (o campo `origem:` do frontmatter aponta para ela) ou quando precisar de detalhe comportamental que a especificação resume.

Estrutura: `00 - Índice` · `01 - Arquitetura` · `02 - Componentes` · `03 - Domínio` · `04 - Runtime do Agente` · `05 - Protocolos` · `06 - Dados` · `07 - Referência` · `08 - Reconstrução`.

As mais consultadas durante a implementação:

| Nota | Quando |
|---|---|
| [[System Prompt (BASE)]] | Portar o prompt-mestre — transcrição literal e íntegra |
| [[Montagem de Contexto]] | Estrutura exata do XML de contexto |
| [[Agent Runtime Loop]] | Os cinco pontos de intervenção, com constantes |
| [[Memory]] | As 13 categorias, o protocolo de supersede, a disciplina de uso |
| [[Task]] | Os 8 estados e as regras do modo task |
| [[Ferramentas MCP]] | A lista completa de tools e a fronteira de privilégio |
| [[Comandos CLI]] | A superfície real de comandos, extraída do binário |
| [[Códigos de Erro]] | Os 201 códigos do original |
| [[Layout do Filesystem]] | A árvore exata de `.fractal/` |

### 1.3 `_extracted/` — o código-fonte original

**É a verdade última sobre comportamento.** Os source maps publicados continham `sourcesContent` com o TypeScript original completo — comentários JSDoc, nomes de variáveis, prompts literais.

```
_extracted/
├── v401/server/src/     ← 564 arquivos — backend 0.1.401, FONTE PRIMÁRIA
├── v401/web/src/        ← 575 arquivos — frontend 0.1.401, FONTE DA UI
├── webui/               ← 576 arquivos — variante do frontend
├── index/src/           ← 470 arquivos — backend 0.1.314 (comparação)
├── chunk/src/           ← 456 arquivos — frontend 0.1.314 (comparação)
├── electron-main/src/   ← 7 arquivos — processo principal do Electron
└── vendor/              ← 46 arquivos — incur + @igniter-js
```

**Regra de uso:** consulte quando precisar de comportamento exato que nem a especificação nem a engenharia reversa capturam — um valor de constante, a ordem de uma operação, o texto literal de uma mensagem. **Não traduza linha a linha.** A especificação em `docs/` define o design em Go; `_extracted/` responde "o que exatamente o original fazia aqui".

---

## 2. Protocolo de leitura

Não leia tudo de uma vez. Leia o necessário para a fase corrente, em profundidade.

### 2.1 Leitura obrigatória, uma vez, antes de qualquer código

Nesta ordem:

1. **[[AOS]]** — o MOC. Dá o mapa e as três coisas que não podem ser cortadas.
2. **Os 17 ADRs** em `docs/01 - Decisões/` — inteiros. São curtos e cada um fecha uma discussão. Ler todos evita que você reabra decisões já tomadas.
3. **[[Hexagonal e Regra de Dependência]]** — a única regra estrutural inviolável, e o teste que a verifica.
4. **[[Layout de Diretórios]]** — onde cada tipo de código mora.
5. **[[Estratégia de Erros]]** e **[[Concorrência e Context]]** — as duas convenções que atravessam todo o código.
6. **[[Estratégia de Testes]]** — o que é exigido para declarar qualquer coisa pronta.
7. **[[Roteiro de Fases]]** — a ordem e as entregas.

### 2.2 Antes de cada fase

Leia, em profundidade:

- **A seção da fase** em [[Roteiro de Fases]], que lista as notas envolvidas
- **Todas as notas listadas** — as seções "Design em Go", "Testes" e "Critério de pronto" são o contrato da fase
- **As notas de origem** no `Fractal Vault/` apontadas pelo campo `origem:` de cada uma

### 2.3 Antes de cada peça crítica

As sete peças de `docs/03 - Peças Críticas/` carregam a identidade do sistema. Para cada uma, leia **os três níveis**:

| Peça | Nota de spec | Nota de RE | Fonte original |
|---|---|---|---|
| [[Command Layer]] | `docs/03/Command Layer.md` | [[Ponte CLI para MCP]] | `v401/server/src/@core/builders/command.builder.ts` e `command-group.builder.ts` |
| [[Collections Engine]] | `docs/03/Collections Engine.md` | [[Modelo de Persistência]] | `v401/server/src/features/*/collections/*.collection.ts` |
| [[Prompt Assembly]] | `docs/03/Prompt Assembly.md` | [[Montagem de Contexto]] | `v401/server/src/features/agent/prompts/agent-prompt.helper.ts` e `agent.prompt.ts` |
| [[Agent Loop]] | `docs/03/Agent Loop.md` | [[Agent Runtime Loop]] | `v401/server/src/features/agent/services/runtime/runtime.service.ts` |
| [[Tool Executor e Spillover]] | `docs/03/Tool Executor e Spillover.md` | [[Tool Executor]] | `v401/server/src/features/agent/services/tool-executor/tool-executor.service.ts` |
| [[Sandbox (Go)]] | `docs/03/Sandbox (Go).md` | [[Sandbox]] | `v401/server/src/features/agent/services/sandbox/sandbox.service.ts` |
| [[Subconsciente (Go)]] | `docs/03/Subconsciente (Go).md` | [[Subconsciente]] | `v401/server/src/features/agent/services/runtime/runtime.service.ts` (métodos `*subconscious*`) |

### 2.4 Antes de cada feature de domínio

1. A nota `docs/04 - Domínio/{Feature} (Go).md`
2. A nota de origem no `Fractal Vault/03 - Domínio/`
3. `_extracted/v401/server/src/features/{feature}/` — em particular `schemas/`, `collections/`, `services/` e `commands/`

O `schemas/*.schema.ts` do original tem `.describe()` em **todos** os campos. Esses textos não são decorativos: viram a documentação da tool MCP e são o que o LLM lê para preencher o payload. **Porte-os para as tags `jsonschema:"..."` preservando o conteúdo**, adaptando apenas as referências de nomenclatura.

### 2.5 Quando as fontes divergirem

Ordem de autoridade: **`docs/` > `Fractal Vault/` > `_extracted/`**.

Se `docs/` contradiz o original, é intencional — procure o ADR ou o bloco `> [!decision]` que explica. Se não houver explicação, **pare e pergunte**; não escolha sozinho.

Se `Fractal Vault/` contradiz `_extracted/`, o código ganha, e você deve **corrigir a nota da engenharia reversa** no mesmo commit, registrando a divergência encontrada.

---

## 3. Regras de trabalho

1. **Nunca invente comportamento.** Toda decisão tem origem rastreável na especificação ou na engenharia reversa. Quando o original for ambíguo, marque `> [!decision]` na nota correspondente com a alternativa escolhida e o porquê.
2. **Reproduza a função, não a implementação.** O objetivo é paridade funcional, não tradução de TypeScript. Onde Go oferecer solução melhor, use — e registre a divergência.
3. **Corrija os defeitos conhecidos.** [[Segurança e Hardening]] lista os 20, cada um com o teste que impede a regressão.
4. **Código, identificadores e comentários de código em inglês. Notas do vault em português.**
5. **Prove antes de declarar pronto.** Nenhuma fase concluída sem teste executado e saída anexada à nota.
6. **Atualize as notas conforme constrói.** `status: especificado` → `em-construcao` → `pronto`. Se a implementação divergir da nota, **a nota é corrigida no mesmo commit** — especificação que mente é pior que especificação ausente.
7. **Não pule fases nem antecipe trabalho de fases futuras.** Cada fase entrega software que roda.
8. **`context.Context` como primeiro parâmetro** em toda função que faz I/O. Nenhum `go f()` solto. Nenhum `context.TODO()` em produção.
9. **Nenhum `new` de adaptador fora de `cmd/`.** A injeção acontece só ali.
10. **`_reasoning` obrigatório e validado** em toda tool.

---

## 4. Ambiente e verificação inicial

Antes da Fase 0, confirme e registre:

```bash
go version          # exige 1.23+
node --version      # para o frontend
wails3 version      # CLI do Wails v3
task --version      # go-task
git --version
```

Confirme também que o original está instalado nesta máquina (`~/.fractal`, `/Applications/Fractal.app`, `fractal --version`) — ele é **referência viva** para comparação de comportamento e de UI. **Jamais escreva em `~/.fractal`.**

---

## 5. Frontend — paridade com o original

> **Esta é a exigência mais literal do projeto.** O aplicativo deve ser **idêntico** ao do original: as mesmas telas, os mesmos componentes, os mesmos fluxos, as mesmas formas.

### 5.1 A fonte

O frontend original está **integralmente disponível** em `_extracted/v401/web/src/` — 575 arquivos TypeScript/TSX, com o código-fonte real, não bundle minificado:

```
v401/web/src/
├── @app/
│   ├── app.tsx  app.client.tsx  router.tsx  igniter.tsx
│   ├── builders/         # app, router, page, layout, store, trigger, middleware
│   ├── components/
│   │   ├── ui/           # ★ 121 componentes do design system
│   │   ├── icons/
│   │   └── editor/       # Monaco + Plate
│   ├── hooks/            # use-realtime, use-notification, use-upload-file, use-mobile…
│   └── lib/              # client, realtime, stores, tabs, icon-map, utils…
└── features/{feature}/presentation/
    ├── pages/            # as telas
    ├── components/       # dialogs, dropdowns, cards, listas
    ├── hooks/  helpers/  consts/  triggers/
```

`webui/` (576 arquivos) é uma variante do mesmo frontend — consulte quando um arquivo estiver incompleto em `v401/web/`.

### 5.2 As 21 rotas a reproduzir

Extraídas de `@app/router.tsx`:

```
HomePage · OnboardingPage · LoginPage · DownloadPage
TasksPage · TaskDetailsPage
ChatPage
CollectionPage · CollectionRecordUpsertPage
ViewPage
ActivitiesPage
RoutinesPage · RoutineUpsertPage
GoalsPage · GoalDetailsPage
ProjectsPage · ProjectDetailsPage
MarketplacePage · MarketplaceDetailsPage
SettingsIndexPage · SettingsSectionPage
```

Toda rota que existe no original existe no AOS, com o mesmo caminho, o mesmo layout e os mesmos elementos.

### 5.3 Método de porte da UI

Para cada tela, nesta ordem:

1. **Leia o `.tsx` original** em `features/{feature}/presentation/pages/`. Ele tem a estrutura completa: layout, componentes, estados, condicionais de renderização.
2. **Leia os componentes que ela usa** em `@app/components/ui/` e em `features/{feature}/presentation/components/`.
3. **Porte a estrutura JSX fielmente** — hierarquia de elementos, classes de layout, estados vazios, estados de carregamento, mensagens.
4. **Substitua apenas a camada de dados.** O original usa o caller tipado do Igniter; o AOS usa o cliente unificado de [[React 19 e Bindings]] (HTTP no navegador, binding Wails no desktop). A árvore de componentes não muda.
5. **Compare visualmente com o app original rodando** e registre qualquer diferença deliberada.

### 5.4 O que se mantém e o que se substitui

| Camada | Decisão |
|---|---|
| Estrutura de telas e componentes | **Porte fiel** do original |
| Design system (`@app/components/ui/`, 121 componentes) | **Porte fiel** — Radix, Plate, cmdk, sonner, framer-motion, lucide, hugeicons |
| Editor (Monaco + Plate) | **Porte fiel** |
| Temas (38) e tokens | **Porte fiel** — ver [[Temas]] e [[Theme (Go)]] |
| Camada de dados (Igniter caller) | **Substituir** pelo cliente unificado — [[React 19 e Bindings]] |
| Roteamento (builders proprietários sobre TanStack Router) | **Substituir** por TanStack Router direto, preservando os mesmos caminhos |
| Motor de views (`@json-render/*`) | **Substituir** pelo renderizador próprio — [[Views Declarativas]] |
| Realtime (WebSocket) | **Substituir** pelo cliente de [[Realtime WebSocket]] |
| Partes do Next.js no bundle | **Remover** — SPA com Vite |

**Regra:** substitua o **encanamento**, preserve a **interface**. Se uma mudança de encanamento alterar o que o usuário vê, ela está errada.

### 5.5 O contrato de componentes para views declarativas

O design system serve dois consumidores: desenvolvedores e o **agente**, que compõe [[View (Go)]]s. Todo componente exposto a views declara suas props em um `spec.ts` ao lado da implementação, e o catálogo Go é **gerado** — ver [[Design System]]. Isso não existe no original e é adição obrigatória.

---

## 6. Versionamento com Git

O repositório do AOS é **novo e separado** do material de engenharia reversa.

### 6.1 Inicialização

```bash
mkdir -p ~/Documents/MeusProjetos/Wails/aos
cd ~/Documents/MeusProjetos/Wails/aos
git init -b main
```

O material de referência (`docs/`, `Fractal Vault/`, `_extracted/`) permanece onde está e **não** é copiado para dentro do repositório do AOS — exceto `docs/`, que é copiado e passa a viver junto do código, porque é o contrato de engenharia e evolui com ele.

### 6.2 `.gitignore` obrigatório

```gitignore
/dist/
/frontend/node_modules/
/frontend/dist/
*.test.log
.env
.env.local
.DS_Store
/testdata/**/large/     # fixture Large é gerada, não commitada
```

**Nunca commite:** binários compilados, `node_modules`, segredos, `~/.aos` de teste, nem qualquer arquivo de `_extracted/` (é código proprietário de terceiro).

### 6.3 Ritmo de commits

**Um commit por unidade coerente de trabalho**, não por dia nem por arquivo. Uma unidade coerente é: uma peça implementada com seus testes verdes, ou uma correção com o teste que a prova.

Commits pequenos e frequentes. Um commit que toca 40 arquivos e mistura três assuntos é irrevisável.

### 6.4 Formato de mensagem

Conventional Commits, com escopo pelo pacote:

```
feat(collections): pattern bidirecional com extração de placeholders

Implementa Compile/Match/Build com regex derivada do padrão, incluindo
suporte a `*` sem captura para os padrões skill-scoped.

Testes: TestPatternRoundTrip, TestPatternSkillScoped, TestBuildMissingPlaceholder
Fase: 1
Spec: docs/03 - Peças Críticas/Collections Engine.md
```

Tipos: `feat`, `fix`, `test`, `docs`, `refactor`, `chore`, `build`, `perf`.

**Todo commit de implementação cita a nota de especificação** que o governa. É o que torna o histórico auditável contra o contrato.

### 6.5 Branches

```
main                      # sempre verde: compila, testes passam
feat/fase-1-collections   # trabalho de uma fase ou de uma peça
fix/sandbox-symlink       # correção pontual
```

- Trabalhe em branch, nunca direto em `main`
- Merge só com todos os portões de CI verdes
- **Uma tag por fase concluída:** `v0.1.0-fase1`, `v0.2.0-fase2`, …

### 6.6 O que fecha uma fase no Git

```bash
task gen                    # regenera schema, componentes, skill, bindings
go vet ./... && golangci-lint run
go test -race ./...
task build:all
node "docs/_scripts/validate-graph.mjs"
git add -A
git commit -m "feat(fase-N): <entrega da fase>"
git tag -a v0.N.0-faseN -m "Fase N: <entrega>"
```

### 6.7 Autorização

**Commite e crie tags livremente** — é o registro do trabalho. **Não faça `push` para remoto sem pedir**, e não crie PR nem release sem autorização explícita.

---

## 7. Protocolo de execução por fase

Para cada fase do [[Roteiro de Fases]], nesta ordem, sem pular:

1. **Ler** as notas da fase e suas origens (seção 2.2)
2. **Marcar** as notas envolvidas como `status: em-construcao` e commitar
3. **Escrever os testes primeiro** onde a nota define casos concretos — a seção "Testes" de cada nota é uma lista de casos, não uma sugestão
4. **Implementar** conforme a seção "Design em Go" da nota
5. **Rodar** `go test -race ./...` e os portões de CI
6. **Anexar a saída dos testes** à nota, na seção "Critério de pronto"
7. **Marcar** as notas como `status: pronto`
8. **Escrever ADR** para qualquer decisão nova tomada durante a fase
9. **Regenerar** artefatos derivados (`task gen`) e verificar que `git diff` fica limpo
10. **Validar o grafo** do vault
11. **Commitar, taggear e reportar** — o que foi entregue, o que foi verificado, o que **não** foi verificado

### Definição de pronto

Uma fase só está pronta quando **todos** os itens abaixo são verdadeiros:

- [ ] Notas da fase com `status: pronto` e saída de teste anexada
- [ ] `go vet`, `golangci-lint`, `go test -race ./...` verdes
- [ ] Cobertura mínima por pacote atingida ([[Estratégia de Testes]])
- [ ] Suítes de contrato verdes para todo port implementado na fase
- [ ] `task gen` seguido de `git diff --exit-code` limpo
- [ ] Binários compilam para as seis combinações de plataforma
- [ ] Grafo do vault válido
- [ ] ADRs escritos para as decisões novas
- [ ] Tag criada

### Reporte honesto

Ao fim de cada fase, reporte:

- **O que foi entregue** e como verificar
- **O que foi verificado** — com a saída do teste
- **O que NÃO foi verificado** — explicitamente
- **Divergências** encontradas entre especificação e realidade, e como foram resolvidas
- **Decisões** tomadas que não estavam previstas

Nunca declare "funcionando" sem evidência. A regra do prompt-mestre do próprio sistema vale para você: *"A claim that you finished is not proof that you finished."*

---

## 8. As dez fases

Detalhe completo em [[Roteiro de Fases]]. Resumo operacional:

| Fase | Entrega verificável |
|---|---|
| **0 — Fundação** | `aos version` roda; `TestDependencyRule` verde |
| **1 — Persistência** | CRUD de agente em Markdown; round-trip verde para os 13 modelos |
| **2 — Command Layer** | `aos agents list` e `aos --mcp` com `agents_list` sobre a mesma definição |
| **3 — Domínio núcleo** | Criar workspace, agente orquestrador, gravar e recuperar memórias com grafo |
| **4 — Servidor e gateway** | `aos gateway start\|stop\|status` operando o daemon |
| **5 — Runtime do agente** | Conversa real com agente que usa tools e persiste memória |
| **6 — Continuidade** | Task autônoma até `in_review`, com memórias formadas sozinhas |
| **7 — Desktop** | App com chat, board de tasks e grafo de memórias — **idêntico ao original** |
| **8 — Ecossistema** | Instalar skill que traz agente, coleção e view próprios |
| **9 — Distribuição** | Binários assinados e instaláveis |

Fase 7 depende de 5 e roda em paralelo com 6. Fase 8 precisa das duas.

---

## 9. As três coisas que não podem ser cortadas

Se o tempo apertar, corte features — nunca estas:

1. **A ponte de superfícies** ([[Command Layer]]) — uma definição de comando produz CLI, tool MCP, tool interna do agente, endpoint HTTP e documentação. Sem ela, ~140 capacidades × 5 superfícies viram 700 pontos de sincronização manual que divergem em semanas.
2. **O prompt com níveis de confiança** ([[Prompt Assembly]]) — `trust="trusted|observed|unverified"` como atributo XML é defesa contra prompt injection embutida no formato.
3. **O subconsciente** ([[Subconsciente (Go)]]) — o segundo LLM que forma memórias sozinho, tirando a memória do caminho crítico do agente principal. É a diferença entre "o agente deveria memorizar" e "o sistema memoriza".

---

## 10. Primeira ação — comece por aqui

Execute nesta ordem e **não pule etapas**:

1. **Confirme o ambiente** (seção 4) e reporte as versões encontradas.
2. **Leia a leitura obrigatória** (seção 2.1) — o MOC, os 17 ADRs e as cinco notas de convenção.
3. **Inicialize o repositório** do AOS (seção 6.1), copie `docs/` para dentro dele, crie o `.gitignore` e faça o commit inicial: `chore: repositório inicial com o vault de especificação`.
4. **Leia as notas da Fase 0** e marque-as como `em-construcao`.
5. **Implemente a Fase 0** — `go.mod`, `Taskfile.yml`, layout de diretórios, `internal/core/build`, `apperr`, `env`, `logging`, e o teste de regra de dependência.
6. **Rode os portões**, anexe as saídas, marque as notas como `pronto`, commite e taggeie `v0.1.0-fase0`.
7. **Reporte** conforme a seção 7 e **pare** — aguarde confirmação antes da Fase 1.

A partir da Fase 1, siga o protocolo da seção 7 fase a fase, reportando ao fim de cada uma.

---

## Referências rápidas

| Recurso | Caminho |
|---|---|
| Especificação (autoridade máxima) | `docs/` — 81 notas |
| MOC da especificação | `docs/00 - Índice/AOS.md` |
| Engenharia reversa | `Fractal Vault/` — 74 notas |
| Backend original (0.1.401) | `_extracted/v401/server/src/` — 564 arquivos |
| **Frontend original (0.1.401)** | `_extracted/v401/web/src/` — 575 arquivos |
| Variante do frontend | `_extracted/webui/` — 576 arquivos |
| Electron main | `_extracted/electron-main/src/` |
| Vendor (`incur`, `@igniter-js`) | `_extracted/vendor/` |
| Validação do grafo | `node "docs/_scripts/validate-graph.mjs"` |

| Documentação externa | Uso |
|---|---|
| `https://v3.wails.io` | Wails3 — services, bindings, janelas |
| `pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp` | MCP oficial em Go |
| `pkg.go.dev/github.com/spf13/cobra` | CLI |
| `pkg.go.dev/github.com/go-chi/chi/v5` | HTTP |

---

> **Lembrete final.** A especificação está pronta e é densa: 81 notas, 868 wikilinks, código Go real em cada peça. Seu trabalho não é decidir **o que** construir — é construir o que está decidido, provar que funciona e manter a especificação honesta enquanto isso. Quando a realidade contradisser a nota, corrija a nota. Quando não souber, pergunte. Quando terminar, prove.
