---
tags: [frontend, porte, fractal, migracao]
fase: 7
status: aprovado-para-plano
data: 2026-08-18
---

# Porte do frontend do Fractal para o AOS

> Decisão: substituir a reconstrução do frontend "do zero" pela cópia integral da
> interface do Fractal extraída em `_extracted/v401/web/`, adaptando-a ao backend
> Go por um único ponto de tradução.

## Objetivo

O AOS deve ter **exatamente a mesma interface** do Fractal. Essa interface já
existe, pronta, em `_extracted/v401/web/src/`. Reconstruí-la tela a tela é
trabalho que já foi feito uma vez — a decisão é copiá-la e adaptar o acoplamento
ao backend, em vez de reescrevê-la.

Este documento estabelece **como** essa adaptação acontece sem espalhar mudanças
por centenas de arquivos.

## Medições que sustentam a decisão

Levantamento feito sobre `_extracted/v401/web/src` (features + `@app` + `@core`)
e sobre `frontend/src` do AOS.

| Medição | Valor |
|---|---|
| Arquivos no front do Fractal | 574 (395 `.tsx`, 180 `.ts`) |
| Que tocam a camada de dados ou o builder de página | 87 |
| **Copiáveis sem alteração de lógica** | **487 (84%)** |
| Métodos distintos do cliente de dados | 4 (`query`, `mutate`, `useQuery`, `useMutation`) |
| Call sites desses 4 métodos | 207 |
| Páginas roteáveis que usam o builder | 21 |
| Arquivos que importam o namespace da app | 126 |

### A design system já está portada

| | AOS | Fractal | Δ |
|---|---|---|---|
| Arquivos em `components/ui` | 116 | 121 | 5 |
| Linhas totais | 29.637 | 29.743 | **106 (0,36%)** |
| Arquivos com contagem de linhas idêntica | 78 de 116 | | |

As diferenças são reescrita de import path (`@app/` → `@/`). A design system do
AOS **é** a do Fractal. Ela permanece como base — a convenção de path já está
aplicada nela. Faltam 5 arquivos: `font-family-selector.tsx`,
`goal-selector-dropdown.tsx`, `project-selector-dropdown.tsx`,
`split-page-layout.tsx`, `theme-provider.tsx`.

### A lacuna real é o backend, não o frontend

Cruzamento das 113 chamadas do front contra os 71 comandos publicados pelo Go:

| Situação | Qtd |
|---|---|
| Cobertas via registry de comandos | 30 |
| Cobertas via `/api/auth` e `/api/file` (fora do registry) | 13 |
| Comando ausente em domínio vivo | 16 |
| **Domínio inteiro ausente no Go** | **54** |

Quatro chamadas são apenas renome, duas delas em kebab-case:
`getById`→`get`, `markAsRead`→`read`, `markAllAsRead`→`read-all`,
`setStatus`→`set-status`.

> O registry publica nomes em kebab-case (`tasks_set-status`), não apenas
> camelCase. Uma extração que assuma `[a-zA-Z]+` perde três comandos reais —
> entre eles `tasks_set-status`, que é o fluxo principal de tasks.

Comandos faltantes em domínios vivos (16): `chat.stop`, `chat.toggleReaction`,
`chat.findOrCreateDm`, `chat.delete`, `chat.update`, `task.start`,
`activity.listEvents`, `auth.verifyWaitlist`, `session.updateProfile`,
`file.explorer`, `file.changes`, `file.search`, `workspace.addMember`,
`workspace.listMembers`, `workspace.removeMember`, `workspace.updateMember`.

Domínios dormentes (14): `collection`, `view`, `skill`, `project`, `goal`,
`instruction`, `template`, `toolset`, `marketplace`, `model`, `tunnel`,
`artifact`, `user`, `token`.

## Arquitetura

### O princípio

O front do Fractal chama `client.task.list.useQuery()`. O AOS expõe
`client.invoke("tasks_list", {...})`. Um `Proxy` que apresenta a primeira
fachada e traduz para a segunda torna os 574 arquivos copiáveis **sem edição de
lógica** — apenas reescrita de import path.

Isso concentra a adaptação: em vez de 87 arquivos editados à mão, um arquivo de
tradução mais uma tabela de dados.

### Estrutura alvo

```
frontend/src/
├── app/                       NOVO — do @app/ do Fractal
│   ├── aos.tsx                ADAPTADO — monta a fachada e a app
│   ├── router.tsx             COPIADO  — árvore de rotas do Fractal
│   ├── builders/              COPIADO  — já é TanStack Router puro
│   │   └── types.ts           RECONSTRUÍDO — apagado pelo bundler
│   └── lib/{stores,triggers}  COPIADO
├── lib/
│   ├── client.ts              INTACTO  — invoke + transporte desktop/HTTP
│   ├── aos-facade.ts          NOVO     — o tradutor
│   └── command-map.ts         NOVO     — tabela feature.ação → comando Go
├── features/                  COPIADO  — do Fractal
└── components/ui/             MANTIDO  + os 5 ausentes
```

`lib/client.ts` **não é tocado**. É onde vivem as correções de transporte
desktop/HTTP já depuradas (retry, detecção de desktop, envelope). A fachada é
construída sobre ele, não no lugar dele.

### A fachada

Três responsabilidades, nesta ordem:

**1. Mapeamento de nome.** Tabela explícita, não regra de pluralização — os
casos irregulares são reais e uma regra os erraria em silêncio:

```ts
// lib/command-map.ts
export const COMMAND_MAP: Record<string, string | null> = {
  "task.list":           "tasks_list",
  "task.getById":        "tasks_get",      // renome
  "activity.markAsRead": "activity_read",  // renome
  "collection.list":     null,             // domínio dormente
};
```

`null` é uma declaração, não uma omissão: significa "o Go ainda não tem este
domínio". A ausência de uma chave é erro de programação e deve falhar alto.

**2. Injeção de `_reasoning`.** Os 71 comandos Go exigem o campo
(`"MANDATORY. NEVER FORGET."`); o front do Fractal nunca o envia. Sem isso,
**toda** chamada copiada falharia na validação.

**3. Degradação honesta.** Ver o contrato abaixo.

```ts
// lib/aos-facade.ts — o núcleo
async function call(feature: string, action: string, opts?: CallOpts) {
  const path = `${feature}.${action}`;
  const entry = COMMAND_MAP[path];
  if (entry === undefined) throw new Error(`chamada não mapeada: ${path}`);

  if (entry === null) return { data: undefined, error: dormant(feature) };

  const payload = { ...opts?.params, ...opts?.query, ...opts?.body };
  try {
    const data = typeof entry === "function"
      ? await entry(payload)                                    // HTTP dedicado
      : await client.invoke(entry, { ...payload, _reasoning: `interface: ${path}` });
    return { data, error: undefined };
  } catch (err) {
    return { data: undefined, error: asEnvelopeError(err) };    // nunca propaga
  }
}
```

Uma chave ausente do mapa lança — é erro de programação e deve falhar alto.
Um domínio dormente não lança: devolve `error`, que é o caminho que os 117
sites já sabem tratar.

A fachada é um `Proxy` de dois níveis (`feature` → `action` → 4 métodos) sobre
TanStack Query, já configurado em `App.tsx`.

**Achatamento de argumentos.** O Fractal passa
`{ query, params, body, enabled }`; o Go recebe um objeto plano. A fachada
funde `params`, `query` e `body` numa única carga. `enabled` é consumido pelo
`useQuery` e não vai para a rede.

**Contrato de retorno: envelope, nunca exceção.** `query` e `mutate` devolvem
`{ data, error }` e **não lançam** — 117 sites no front acessam `result.data`
ou `result.error` diretamente:

```ts
const result = await client.task.getById.query({ params: { task: id } });
const task = result.data?.task;      // payload aninhado sob a chave do domínio
if (result.error) { /* trata sem try/catch */ }
```

Isso é o oposto de `lib/client.ts`, cujo `invoke()` lança `DomainError`. A
fachada **inverte** essa convenção: captura a exceção e a devolve como `error`.
Errar isso quebra os 117 sites em silêncio — um `catch` ausente vira tela
branca em vez de mensagem.

O aninhamento sob a chave do domínio já casa: o Go devolve
`{"tasks": [...], "total": n}` e `{"task": {...}}`, exatamente o que
`result.data?.tasks` e `result.data?.task` esperam.

### Contrato de domínio dormente

| Chamada | Comportamento |
|---|---|
| `query` / `mutate` | Devolve `{ data: undefined, error: { code: "AOS_DOMAIN_DORMANT" } }`. A lista renderiza vazia em vez de quebrar. |
| `useQuery` | `data` fica `null` (**não `undefined`**), `isLoading` resolve. Sem retry — dormente não melhora tentando de novo. |
| `useMutation` | Resolve com o mesmo envelope; a UI exibe o aviso pelo caminho de erro que já tem. |
| Rota do domínio | Layout exibe um painel claro: *"Este domínio ainda não existe no backend Go."* |

> **`null`, nunca `undefined`.** O TanStack Query trata `undefined` devolvido por
> um `queryFn` como erro (`"<hash> data is undefined"`) e coloca a query em
> estado de falha. Devolver `undefined` no caminho dormente destruiria
> exatamente o contrato desta seção: o `AOS_DOMAIN_DORMANT` nunca chegaria à UI
> e o painel ficaria inalcançável por qualquer hook. `null` é dado válido para o
> TanStack, e o código portado lê `q.data?.campo`, que lida com `null`
> naturalmente.

O painel é o que separa degradação honesta de tela quebrada: uma tela vazia sem
explicação é indistinguível de um defeito. Um domínio que "acorda" no Go passa a
funcionar trocando `null` pela chave do comando — **sem tocar no frontend**.

## Inventário de trabalho

A extração `v401/web` foi produzida a partir do bundle, e **o bundler apagou
todos os arquivos type-only**. É por isso que 27 arquivos `*.interfaces.ts`
faltam ali mas sobrevivem em `_extracted/index/` — lá eles carregam schemas Zod,
que são valores em runtime.

Varredura de 367 caminhos de import internos: **45 ausentes, 227 referências**.

### Copiar direto (~544 arquivos)

`features/**` exceto `features/*/errors/**`, mais `@app/{hooks,lib,builders}`.

**Excluir `features/*/errors/**` (30 arquivos).** São resíduo do slice de
backend: importam `FractalError` de `@core/builders/error.builder`, que depende
de `@igniter-js/core` e do ciclo de request do servidor. Nenhum deles é
importado pela UI. Copiá-los arrastaria a dependência que decidimos não ter.

### Recuperar de `_extracted/index/` (22 arquivos)

19 interfaces de domínio (`task`, `file`, `chat`, `agent`, `routine`, `goal`,
`project`, `workspace`, `model`, `collection`, `memory`, `skill`, `template`,
`theme`, `toolset`, `view`, `instruction`, `auth`, `config`) mais
`@core/{helpers/request-context, services/activity, services/store}`.

Reposicionar de `features/<d>/<d>.interfaces.ts` para
`features/<d>/interfaces/<d>.interfaces.ts`, que é o caminho que o front espera.

### Reconstruir à mão (11 arquivos)

Dos 27 arquivos de interface ausentes, 19 são recuperáveis acima; estes 8 não
existem em nenhuma extração e são reconstruídos a partir dos seus usos.

| Arquivo | Refs | Origem da reconstrução |
|---|---|---|
| `app/builders/types.ts` | 3 | 16 tipos, derivados dos 1.919 linhas de builder que temos completas |
| `workspace/interfaces/directory.interfaces.ts` | 8 | uso |
| `activity/interfaces/activity.interfaces.ts` | 7 | uso |
| `marketplace/interfaces/marketplace.interfaces.ts` | 7 | uso (dormente — mínimo) |
| `chat/presentation/helpers/chat-kind.helper.ts` | 7 | uso |
| `@core/builders/notification.ts` | 3 | uso |
| `auth/interfaces/user.interfaces.ts` | — | uso |
| `routine/interfaces/run.interfaces.ts` | — | uso + `routines_runs` do Go |
| `artifact/interfaces/artifact.interfaces.ts` | — | uso (dormente — mínimo) |
| `task/interfaces/{comment,todo}.interfaces.ts` | — | extrair de `task.interfaces.ts` do `index/` |

`builders/types.ts` é plumbing genérico interno; os builders já usam `any`
internamente (`_use: any[]`, `_loader?: any`). Generics pragmáticos bastam — não
é necessário reproduzir a tipagem original.

### Escrever novo (4 arquivos)

`lib/aos-facade.ts`, `lib/command-map.ts`, `app/aos.tsx`,
`components/DormantDomain.tsx`.

### Substituir/remover

As 7 telas artesanais do AOS que a interface do Fractal substitui:
`task/TaskBoard.tsx`, `memory/MemoryGraph.tsx`, `file/{FilesPage,FileTree,MonacoViewer}.tsx`,
e o `router.tsx` atual (árvore de rotas fixa) pela árvore do Fractal.

Os 25 arquivos de `chat`/`agent` já adaptados à mão são sobrescritos pelos 34
originais do Fractal: o trabalho de adaptação não se perde, migra para a
fachada. As correções de transporte estão em `lib/client.ts`, que não é tocado.

### Assets ausentes (9)

Imagens (`logo-icon-{dark,light}.png`, `backgrounds/landscape.png`,
`screenshots/*.png`, `marketplace-prompts-bg-*.jpg`). Não estão em nenhuma
extração. Substituir por placeholders declarados; não bloqueiam compilação nem
navegação.

## Decisões de nomenclatura

**Nenhum pacote `@igniter-js/*` é instalado.** Os builders são código local
sobre TanStack Router — permanecem, como nosso código.

**O namespace `igniter` é renomeado para `aos`.** Aparece em 126 arquivos de
feature; mantê-lo sugeriria permanentemente uma dependência que não temos. Como
todos os imports já serão reescritos por script, é uma regra `sed` adicional. As
classes internas dos builders (`IgniterPage` → `AosPage`, etc.) acompanham — são
9 arquivos que já estamos tocando.

Os tipos `Spec` e `IgniterCollectionViewRenderResult` de `@igniter-js/collections`,
usados por `features/view/`, viram declarações locais. `view` é domínio dormente,
então o contrato mínimo basta.

## Compilador e testes

### Estritura do TypeScript

O `tsconfig.json` do AOS liga `noUnusedLocals`, `noUnusedParameters` e
`noUncheckedIndexedAccess`. O código do Fractal foi escrito sob outro perfil:
são **413 indexações de array em 108 arquivos**, cada uma virando erro sob
`noUncheckedIndexedAccess`.

Decisão: **relaxar as três flags globalmente e manter uma ilha estrita** sobre o
código que este porte de fato escreve.

```jsonc
// tsconfig.json — passa a cobrir o código portado
"strict": true,                    // mantido: segurança de tipo real
"noUnusedLocals": false,           // lint, não segurança
"noUnusedParameters": false,       // lint, não segurança
"noUncheckedIndexedAccess": false, // ver justificativa abaixo

// tsconfig.strict.json — as três de volta, include: ["src/lib/**/*"]
```

`noUnusedLocals` e `noUnusedParameters` são lint: desligá-las não perde
segurança. `noUncheckedIndexedAccess` **é** segurança, e desligá-la é uma perda
real — aceita porque o alternativo é editar 413 pontos em código que não
escrevemos, e cada edição é divergência permanente contra a fonte. Os dois
projetos são independentes e `noEmit`; não há project references envolvidas.

### Testes

O frontend não tem test runner (96 testes em Go, zero em JS). O porte adiciona
**vitest** — não para cobrir a UI copiada, mas porque a fachada é um ponto
único por onde passam 207 call sites. Um defeito nela falha em silêncio e em
toda parte; é exatamente o código que merece teste.

Escopo dos testes: `lib/aos-facade.ts` e `lib/command-map.ts`. A UI portada não
recebe testes — ela já foi validada em produção no Fractal, e testá-la aqui
seria escrever do zero o que decidimos não reescrever.

## Fases

| # | Fase | Entrega | Verificação |
|---|---|---|---|
| 1 | Fundação | `aos-facade.ts`, `command-map.ts`, `app/aos.tsx`, `DormantDomain.tsx`, `builders/` + `types.ts` | Teste unitário da fachada: mapeia, injeta `_reasoning`, achata argumentos, degrada em dormente |
| 2 | Tipos | 19 interfaces + 3 `@core` recuperados, 11 arquivos reconstruídos | `tsc --noEmit` sobre os tipos isolados |
| 3 | Cópia | `features/**` (menos `errors/`), `@app/{hooks,lib}`, 5 UI ausentes; imports reescritos por script | `tsc --noEmit` |
| 4 | Shell | Árvore de rotas do Fractal substitui `router.tsx`; remove as 7 telas artesanais | App sobe |
| 5 | Verificação | Percorrer as telas dos 12 domínios vivos; confirmar painel dormente nos 14 restantes | Navegação manual + `task build` |

A ordem é uma cadeia de dependências, não preferência: a fachada precisa existir
antes de a cópia compilar; os tipos precisam existir antes de `tsc` passar.

## Verificação

- `npm run typecheck` (`tsc --noEmit`) limpo ao fim das fases 3 e 5.
- `task build` produz o binário desktop.
- Percurso manual: chat (envia e recebe), tasks (lista, detalhe, muda status),
  activity, memory, files, agents, config, themes, routines, workspace.
- Cada um dos 14 domínios dormentes exibe o painel, nunca uma tela em branco.
- Nenhum import de `@igniter-js/*` no bundle final:
  `grep -r "@igniter-js" frontend/src` vazio.

## Riscos e dívidas

**`_reasoning` sintético (dívida deliberada).** O campo existe para o agente
justificar por que chamou uma tool. Preenchê-lo com `"interface: task.list"`
satisfaz a validação mas esvazia o campo de sentido — o audit trail encherá de
justificativas geradas. A correção pertence ao lado Go: distinguir origem de
interface de origem de agente (`_origin: "ui"`), em vez de exigir dos dois a
mesma prova. Não bloqueia o porte; deve virar ADR.

**Interface à frente do backend.** Ao fim do porte, 14 domínios terão tela sem
domínio Go. É consequência aceita da decisão — a interface passa a ser a
especificação executável do que falta implementar.

**`builders/types.ts` reconstruído.** Não existe em nenhuma extração. A
reconstrução é pragmática, não fiel; se as 21 páginas compilarem e rodarem, está
correta o bastante. É o único item do porte sem fonte de verdade.

**Os 16 comandos faltantes em domínios vivos.** Ficam dormentes por comando (não
por domínio) até serem implementados em Go. Os mais visíveis:
`file.explorer` e `file.changes` (alimentam o explorador de arquivos e o painel
de mudanças na sidebar — elementos permanentes da interface) e os quatro
`workspace.*Member` (a tela de membros fica inteira dormente).

`file.explorer` é o candidato mais forte a um adaptador em vez de dormência: o
Go tem `file.tree`, e a diferença é de formato (o explorer devolve um snapshot
com contextos), não de capacidade. Fica registrado como oportunidade, fora do
escopo deste porte.

**Divergência da fonte.** Cada edição à mão em arquivo copiado é dívida contra
uma futura ressincronização com o Fractal. É a razão de a fachada absorver a
adaptação em vez de espalhá-la — e a razão de relaxar flags do compilador em vez
de editar 413 indexações de array em código que não escrevemos.
