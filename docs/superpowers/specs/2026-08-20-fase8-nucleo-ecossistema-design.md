---
tags: [fase-8, ecossistema, collection, view, toolset, skill]
fase: 8
status: aprovado-para-plano
data: 2026-08-20
---

# Fase 8, núcleo do Ecossistema — collection, view, toolset e skill

> Decisão: uma fatia vertical fina pelos quatro domínios que sustentam a
> entrega declarada da fase — *"instalar uma skill que traz agente, coleção e
> view próprios"* — em vez de um domínio completo por vez. A integração entre
> os quatro é onde mora o risco, e é o que esta fatia exercita primeiro.

## Objetivo

Fazer o AOS deixar de ser um sistema com domínios fixos. Depois desta fatia, o
agente define dados estruturados em runtime, compõe uma interface sobre eles,
alcança ferramentas externas, e tudo isso pode ser empacotado, instalado e
desinstalado como uma unidade.

A tese não é "mais domínios". É que **"monte um CRM" vire tabela real e tela
real sem programador**, e que uma capacidade inteira — agente, memórias,
rotinas, coleções, views — caiba num diretório que se instala.

## O que já existe, e que muda o desenho

Levantado antes de projetar, e três achados mudaram o que ia ser escrito.

### O frontend dos quatro já está portado

O branch `feat/port-fractal-frontend` trouxe as 26 features do original, entre
elas `collection`, `view`, `toolset` e `skill`, com páginas, stores, triggers e
o renderizador declarativo sobre `@json-render/core`. Elas estão **dormentes**:
`frontend/src/lib/command-map.ts` mantém `DORMANT_DOMAINS`, e cada caminho que
o Go não responde é `null` no `COMMAND_MAP`, o que rende um painel honesto em
vez de tela branca ou lista vazia mentirosa.

Acender um domínio é, do lado do frontend, apagar uma entrada de um conjunto e
preencher caminhos numa tabela. **Nenhuma UI é escrita nesta fatia.**

Os 21 caminhos que a UI chama e que hoje são `null`:

| Domínio | Comandos |
|---|---|
| `collection` | `list`, `getById`, `delete`, `listRecords`, `getRecordById`, `createRecord`, `updateRecord`, `deleteRecord` |
| `view` | `list`, `getById`, `render`, `executeAction`, `delete` |
| `skill` | `list`, `install`, `update`, `delete` |
| `toolset` | `getById`, `getConfig`, `updateConfig`, `delete` |

### A persistência da skill já existe desde a Fase 1

`internal/core/collections/registry.go` já declara o nativo `skills`
(`.aos/skills/{id}/SKILL.md`, `CascadeDelete: true`) e — mais importante — já
declara os **segundos padrões** que fazem uma skill trazer uma equipe:

```
agents        .aos/skills/{skill}/agents/{id}/AGENT.md
memories      .aos/skills/*/agents/{agent}/memories/{id}.memory.md
routines      .aos/skills/*/agents/{agent}/routines/{id}/ROUTINE.md
templates     .aos/skills/{skill}/templates/{id}.template.md
instructions  .aos/skills/{skill}/instructions/{id}.instruction.md
goals         .aos/skills/{skill}/goals/{id}/GOAL.md
```

Nada disso precisa ser projetado. Falta o **domínio** skill — entidade,
manifesto, instalador — não o lugar onde ela mora.

### O motor de coleções é genérico sobre um struct de tempo de compilação

`Model[T]`, `Decode[T]`, `Encode[T]` e `planFor(reflect.Type)` leem tags de
struct Go. Uma coleção customizada **não tem struct**: seus campos são
declarados em dado, em runtime, pelo agente.

E `Lookup`/`ModelOf` leem `byName`, um `var` de pacote computado uma vez a
partir de `natives`. Uma coleção criada às 14h precisa ser consultável às
14h01, na mesma sessão — o `autoWatch` do original.

**Esta é a tensão arquitetural da fase.** O resto é trabalho de domínio comum.

## Arquitetura

### O princípio

Uma implementação de "onde mora um registro e como ele é escrito", não duas.

A Fase 1 gastou uma fase inteira acertando escrita atômica, lock por arquivo,
CAS por `Version` e casamento bidirecional de padrões (ADR-0012, ADR-0004). Os
registros que o **usuário final** vai criar em maior volume são exatamente os
das coleções dinâmicas. Escrever um segundo caminho de persistência para eles
seria escrever de novo a parte cara, e garantir que as duas divirjam.

### Mudança 1 — `collections.Record`

```go
// Record is the T of a collection whose fields were declared at runtime.
//
// Everything the engine does around a record — matching a path to a pattern,
// choosing a writable pattern for a key, atomic write, per-file lock, the CAS
// check on Version, the Changed event — looks at the pattern and the key, never
// inside T. That is why a dynamic collection can be a Model[Record] rather than
// a second engine.
type Record struct {
    Key     Key
    Fields  map[string]any
    Content string
}
```

O codec ganha um ramo para `Record`: em vez de refletir sobre campos, serializa
`Fields` como front matter YAML (`FormatMarkdown`) ou como o objeto JSON
inteiro (`FormatJSON`), com `Content` como corpo Markdown quando houver.

`KeyOf`, `FieldOf` e `WithoutBody` ganham o mesmo ramo. `Repository[Record]` é
o repositório existente, sem alteração.

### Mudança 2 — o registry vira instância

```go
// Registry holds what collections exist. The natives are fixed at
// construction; the dynamic ones come and go while the daemon runs, which is
// the whole point of a collection an agent can create.
type Registry struct {
    mu      sync.RWMutex
    natives map[string]Descriptor
    dynamic map[string]Descriptor
}

func NewRegistry() *Registry
func (r *Registry) Register(d Descriptor) error   // rejects a reserved name
func (r *Registry) Unregister(name string) error  // native name is refused
func (r *Registry) Lookup(name string) (Descriptor, bool)
func (r *Registry) Names() []string
```

As funções de pacote `Lookup`, `Natives` e `ModelOf` continuam existindo,
delegando a um registry default — os 14 domínios que já as chamam não mudam
uma linha.

**Nome reservado é do registry, não do domínio.** A nota `Collection (Go)`
propõe uma `var reserved` no domínio. Fica aqui: o registry é quem sabe o que
já existe, e uma lista duplicada no domínio desatualiza no dia em que esta
mesma fase acrescentar `views`, `toolsets` e `collections` aos nativos.

### Mudança 3 — o publisher ganha um caller

`fscollections.WithPublisher` e `WithWatchPublisher` existem desde a Fase 1 e
**nunca tiveram caller**. É o resíduo A2 do branch do frontend: `files:changed`
está mapeado para `collection.changed`, que nada no Go publica — os três
painéis de arquivo ficam escuros e a árvore não se atualiza sozinha.

O `autoWatch` das coleções dinâmicas precisa exatamente do watcher ligado ao
bus. Ligar um fecha o outro.

## Os quatro domínios

### `collection`

Duas camadas, deliberadamente distintas.

**A definição** é um nativo novo:

```
collections  .aos/collections/{id}/schema.json
             .aos/skills/{skill}/collections/{id}/schema.json
```

A entidade é a da nota `Collection (Go)`: `Scope`, `Format`, `[]Field` com
`Type` no subconjunto `string | number | boolean | date | enum | ref | list`,
`Required`, `Enum`, `Ref`, `Default`, `Unique`.

**Os registros** são `Model[Record]`. Cada coleção dinâmica recebe **seu
próprio `Descriptor`, nomeado com o id dela** — uma coleção `contacts` vira um
descritor de nome `contacts`, e o id da coleção fica assado no padrão em vez de
ser um placeholder:

```
Descriptor{Name: "contacts", Format: FormatMarkdown, CascadeDelete: false}
    .aos/collections/contacts/records/{id}.md
```

Assim `List`, `Get` e o casamento de padrão funcionam exatamente como para um
nativo, e `Query.Key` continua significando o que sempre significou. Uma
coleção com `Scope: skill` ganha um segundo padrão de leitura,
`.aos/skills/{skill}/collections/contacts/records/{id}.md`, pela mesma regra
que os nativos já usam — padrão com wildcard é somente-leitura por construção.

O `Register` no `*Registry` acontece em dois momentos: quando a definição é
criada, e quando o watcher vê um `schema.json` que ele não conhecia. O segundo
é o que faz uma coleção criada fora do processo — por outro `aos`, ou por
alguém editando o arquivo à mão, que é o ponto inteiro do ADR-0004 — ficar
utilizável sem restart.

`Validate(c Collection, data map[string]any) error` é pura: obrigatório
ausente, enum fora do conjunto, tipo errado, `unique` duplicado, `ref`
apontando para coleção que não existe. **Schema é dado, nunca avaliado** — é
por isso que carregar um schema escrito por agente não executa nada.

Hooks são declarativos: `setTimestamp`, `slugify`, `defaultTo`, `computeFrom`,
com parâmetros. Nunca strings de código-fonte, que é o que o original aceita e
grava num `schema.ts` — um agente gerando código executável no workspace, sem
sandbox nem revisão. Caso legítimo que exija mais lógica vira uma
`Routine (Go)` com trigger `activity`, que passa pelo sandbox e é auditada.

### `view`

Nativo novo:

```
views  .aos/views/{id}.view.json
       .aos/skills/{skill}/views/{id}.view.json
```

Entidade conforme a nota `View (Go)`: `Source` (coleção, filtro, ordenação,
limite), `Tree` de `Node` (componente, props, `bind`, filhos, `actions`).

**Validação na escrita, não na renderização.** Uma view inválida não é gravada,
e o erro nomeia o componente e a prop. Cinco recusas:

1. componente que não existe no catálogo;
2. prop obrigatória ausente;
3. prop de tipo errado;
4. `bind` para campo que a coleção-fonte não declara;
5. `action` nomeando comando que não está no registry.

O original valida ao renderizar; o agente descobre depois que a tela está em
branco. Aqui ele corrige na hora.

`Render` resolve a fonte e devolve a árvore com dado anexado.
`Components` é a introspecção que o agente chama antes de compor — o
equivalente a ler a documentação do design system.
`Scaffold` gera uma view inicial a partir de uma coleção, inferindo componentes
dos tipos de campo: é o que faz o agente produzir uma view válida sem adivinhar.
`ExecuteAction` roda uma das `Actions` da view — é o comando que a UI chama
quando alguém clica no botão, e ele **não executa nada por conta própria**:
resolve a `Action` declarada, monta o input e despacha pelo registry, de modo
que a autorização e a validação são as mesmas de qualquer outra superfície.

**`Actions` chamam comandos do registry**, com a mesma validação e a mesma
autorização de qualquer outra superfície. A view não é um caminho paralelo para
mutação.

### `toolset`

Nativo novo:

```
toolsets  .aos/toolsets/{id}.toolset.md
          .aos/skills/{skill}/toolsets/{id}.toolset.md
```

Um adaptador nesta fatia: `mcp-server::stdio`. O SDK oficial
(`github.com/modelcontextprotocol/go-sdk`) já é dependência — o AOS é servidor
MCP hoje; ser **cliente** é o que falta.

```go
// Adapter is the strategy every connection type implements. The other four
// types are added by implementing this and registering a factory; nothing in
// the core changes.
type Adapter interface {
    Connect(ctx context.Context, t Toolset) error
    ListTools(ctx context.Context) ([]ToolSpec, error)
    Call(ctx context.Context, name string, in json.RawMessage) (any, error)
    Close() error
}
```

**A indireção é mantida.** As tools de um toolset não entram no registry do
agente; ele as alcança por `toolsets_call`. As três razões do original valem —
o contexto não cresce com o número de integrações, a descoberta é sob demanda —
e a quarta é a que mais importa: **a fronteira de execução externa fica
auditável em um ponto só**.

Toda chamada registra em `Activity`: toolset, tool, duração, desfecho.
**Nunca o payload**, que pode carregar dado sensível do usuário.

Interpolação `${env.VAR}` resolve contra as camadas de env do workspace e
**falha alto nomeando a variável ausente** — nunca substitui string vazia, que
produz um 401 confuso muito mais tarde.

### `skill`

O lugar já existe. Falta o domínio.

Entidade conforme a nota `Skill (Go)`: `Metadata` como inventário do pacote
(`rules`, `resources`, `toolsets`, `collections`, `views`, `hooks`,
`artifacts`, `goals`, `templates`, `instructions`) e `Permissions` como
manifesto (ADR-0015).

```go
type Installer struct {
    repo     Repository
    fetcher  Fetcher       // local directory in this slice
    verifier Verifier
    approver approval.Approver
}
```

`Fetcher` é port desde já, com uma implementação local. A remota entra depois
sem tocar no núcleo.

**A ordem do `Install` é a decisão, e ela importa:**

1. buscar o pacote;
2. **verificar o conteúdo contra o manifesto declarado** — excesso é recusado,
   nomeando o excesso, não instalado em silêncio;
3. **consentimento humano**, por ADR-0007, com risco alto. Um agente chamando
   `skills_install` não se autoriza sozinho;
4. aplicar arquivos;
5. registrar por último.

O passo 5 vem por último para que uma falha parcial deixe um **diretório não
registrado** em vez de meia skill registrada.

`Delete` remove o diretório inteiro — o `CascadeDelete: true` do nativo já faz
isso. Hooks e toolsets registrados são **desregistrados antes** da remoção dos
arquivos.

**Colisão de nome:** os builtins vivem sob `aos self skills`, para não repetir
o defeito #15 do original, onde o builtin `skills` do `incur` sobrescreve o
grupo de domínio e torna `create`/`install`/`discovery` inalcançáveis pelo CLI.

## O catálogo de componentes

É a única geração deste projeto que anda ao contrário. `gencatalog` lê Go,
`gentokens` lê CSS como texto, `genschema` escreve TS a partir do registry Go —
todos programas Go. Este precisa **avaliar TypeScript**, porque o que
`z.object({...})` declara não é legível como texto.

Dois passos, cada um na sua linguagem:

1. `frontend/scripts/gen-components.mjs` importa `catalogDefinitions` de
   `src/features/view/presentation/components/registry/definitions/catalog.definitions.ts`,
   converte cada `props` de zod para JSON Schema, e escreve
   `internal/domain/view/components.json`.
2. Go embute o JSON com `go:embed` e o serve como `[]ComponentSpec`.

São **56 componentes**: 36 do baseline `@json-render/shadcn/catalog` mais 20 do
AOS, incluindo a família `SplitPage*`.

Duas dependências de desenvolvimento no frontend: `tsx`, para executar o `.ts`,
e `zod-to-json-schema` — o `z.toJSONSchema` nativo existe apenas no subpath v4
do pacote zod, e o catálogo é escrito em zod v3 clássico.

`task gen` ganha `gen-components`, e `task check` ganha a mesma guarda que já
protege o `catalog.gen.go`:

```
git diff --exit-code -- internal/domain/view/components.json
```

> [!decision] JSON embutido em vez de `components.gen.go`
> **Divergência declarada contra a nota `View (Go)`**, que pede um `.gen.go` e
> diz *"um componente removido no React quebra o build do Go"*.
>
> O valor real é detecção de divergência, e ela vem do `git diff --exit-code`
> no gate — o mesmo mecanismo que já guarda o catálogo de erros. Um `.gen.go`
> com 56 structs literais custa ruído de diff e um passo de `gofmt` para
> comprar erro em tempo de compilação em vez de em tempo de gate.
>
> Um teste cobre a diferença: todo componente que o `Scaffold` referencia tem
> que existir no catálogo.

## Fiação

`wire.go` ganha o registry de coleções como instância, construído ali e passado
aos domínios, mais quatro `Register(reg, svc)` na forma dos treze que já
existem:

```go
collection.Register(reg, collectionSvc)
view.Register(reg, viewSvc)
toolset.Register(reg, toolsetSvc)
skill.Register(reg, skillSvc)
```

Do lado do frontend, `DORMANT_DOMAINS` perde quatro entradas e as 21 entradas
`null` do `COMMAND_MAP` recebem o caminho real.

**O `COMMAND_MAP` é o contrato de aceitação desta fatia.** Enquanto houver
`null` nestes quatro domínios, ela não fechou.

Além dos 21 que a UI chama, entram os comandos que só o agente e o CLI usam, e
que são o que faz a tese funcionar sem humano clicando: `collection.create`,
`view.create`, `view.components`, `view.scaffold`, `toolset.list`,
`toolset.call`, `skill.create`.

## Testes e gates

Os gates são os que já existem, sem inventar novos: `task check` inteiro —
`gen` sem drift, `vet`, `lint`, `test -race`, `cover` com todo pacote no seu
piso, `arch`, `graph` — mais `npx tsc --noEmit`, `npx vitest run` e
`task build:desktop`.

Três testes carregam a fase:

**`TestTheDeliveryOfPhaseEight`**, em `internal/app`, no molde dos cinco irmãos
que já existem, sobre disco real:

1. uma skill é instalada de um diretório local e traz agente, coleção e view
   próprios;
2. o agente cria um registro na coleção **na mesma sessão, sem restart**;
3. a view renderiza com o dado anexado;
4. desinstalar remove os três, com os hooks desregistrados **antes** dos
   arquivos.

**Contrato do port `Adapter`**, no padrão de `testsuite.RunRepositoryContract`
que já existe — para que os outros quatro tipos de toolset entrem depois sem
redesenho.

**Recusa de manifesto**, que é a razão de o ADR-0015 existir: conteúdo
excedendo o declarado é rejeitado **nomeando o excesso**, e instalação sem
consentimento é rejeitada.

Além desses, por domínio:

| Domínio | O que tem que estar verde |
|---|---|
| motor | round-trip de `Record` nos dois formatos; registrar e desregistrar dinâmico; nome reservado recusado; nativo não é desregistrável; corrida entre registro e leitura sob `-race` |
| collection | coleção criada é utilizável na mesma sessão; watcher detecta schema novo em menos de 1 s; validação nos cinco modos; `ref` para coleção inexistente recusado; hooks declarativos aplicam normalização |
| view | as cinco recusas de escrita, cada uma nomeando o que está errado; `Components` bate com o catálogo gerado; `Scaffold` produz view válida e renderizável |
| toolset | interpolação com variável ausente falha nomeando-a; `Call` registra em Activity **sem** o payload; adaptador stdio conecta a um servidor de teste e lista tools |
| skill | round-trip com `metadata` completo; skill com agente próprio aparece em `agents list` com `skill` preenchido; desinstalação limpa |

## Riscos

| Risco | Mitigação |
|---|---|
| Mexer no motor que 14 domínios usam | Nativos continuam pelo caminho `Model[T]` genérico, intocado. `TestDependencyRule`, a suíte da Fase 1 e o contrato de repositório são a rede. O registry por instância delega ao default, então nenhum chamador existente muda. |
| Registry mutável em runtime é estado compartilhado | `RWMutex`, e um teste sob `-race` que registra enquanto lê. É a razão de o registry ser instância em vez de `var` de pacote mutável. |
| `zod-to-json-schema` não cobrir alguma construção do catálogo | O gerador falha alto nomeando o componente e a prop, em vez de emitir schema vazio. Um componente sem schema utilizável é melhor ausente do catálogo do que presente e permissivo. |
| Adaptador MCP stdio spawna processo | Já é o território do `Sandbox (Go)`; o binário do servidor MCP precisa estar declarado no manifesto da skill. |
| Fatia vertical deixa cada domínio incompleto contra sua nota | Aceito e declarado: as notas ficam `em-construcao` com a lacuna nomeada, não `pronto`. A fase 8 fecha quando as fatias seguintes entrarem. |

## Fora de escopo, declarado

- Os outros quatro tipos de toolset: `mcp-server::http`, `rest-api` (OpenAPI),
  `cli`, `custom`. Entram atrás do contrato de `Adapter`.
- Fetch remoto de skill, e com ele `version`, `source`, `commit` e o
  `aos skills verify` contra o hash de origem.
- Os oito domínios restantes da Fase 8: `artifact`, `template`, `instruction`,
  `project`, `goal`, `bot`, `tunnel`, `marketplace`.
- Acessibilidade medida por ferramenta, que é a outra pendência de
  `Design System`.

## Critério de pronto

- [ ] `Record` e o registry por instância, com a suíte da Fase 1 ainda verde
- [ ] `collection.changed` publicado de verdade — resíduo A2 fechado
- [ ] Os quatro domínios registrados em `wire.go`
- [ ] `DORMANT_DOMAINS` sem `collection`, `view`, `toolset` e `skill`
- [ ] Zero `null` no `COMMAND_MAP` para esses quatro domínios
- [ ] `task gen-components` no `task gen`, com guarda de drift no `task check`
- [ ] `TestTheDeliveryOfPhaseEight` verde sobre disco real
- [ ] `task check` verde ponta a ponta, mais os gates do frontend
