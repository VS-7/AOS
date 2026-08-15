---
tags: [critico, command, superficies, mcp, cli]
aliases: [Command Layer, Ponte de Superfícies, Registry de Comandos]
fase: 2
status: pronto
origem: "[[Ponte CLI para MCP]]"
---

# Command Layer ★

> Pai: [[AOS]] · Origem no original: [[Ponte CLI para MCP]] · Decisão: [[ADR-0003 Command Layer unificada]] · Fase: 2

## Objetivo

Declarar cada capacidade **uma única vez** e derivar dela cinco superfícies: comando de CLI, tool MCP, tool interna do agente, endpoint HTTP e documentação `SKILL.md`. É a peça de maior alavancagem do sistema — sem ela, ~140 capacidades × 5 superfícies viram 700 pontos de sincronização manual.

## Comportamento do original

`FractalCommand` + `CommandGroupBuilder` + o framework `incur`. Três projeções da mesma definição: `.toCLI()`, `.toTools()` e a coleta implícita do `incur` para MCP ([[Ponte CLI para MCP]]).

Detalhes que importam e que reproduzimos:

- **Nome da tool = caminho unido por `_`** — `agents_create`, `tasks_todos_set_status`, `collections_records_update`. Lista ordenada alfabeticamente, para ordem estável entre execuções.
- **Schema de entrada = união de `args` e `options`.**
- **Opções de transporte injetadas automaticamente** (`--base-url`, `--token`, `--workspace`, `--agent`), removidas antes de chegar ao handler. Exceto onde o comando declara `.withoutRemoteTransportOptions()` — hoje só o gateway.
- **Coerção de tipos complexos para argv** — `Schema.stringify` transforma arrays e objetos em strings JSON aceitáveis na linha de comando.
- **`agent: true`** no contexto quando a chamada vem do MCP, para o handler adaptar a resposta.
- **Tools compostas (0.1.401)** — um grupo expõe uma tool única com campo `action`, e `schema: true` devolve a especificação da ação em vez de executar.
- **`_reasoning` obrigatório**, com mensagem de validação explícita: *"An empty string is a rejected call."*

## Design em Go

### O tipo

```go
// internal/core/command/command.go
package command

type Example struct {
	Description string `json:"description"`
	Input       any    `json:"input"`
}

// Command is the single definition of one capability. Everything the five
// surfaces need is derived from this struct plus reflection over In.
type Command[In any, Out any] struct {
	Group    string   // "memories"
	Name     string   // "store"
	Summary  string   // one line — cobra's Short
	Doc      string   // full Markdown — becomes the MCP tool description
	Examples []Example

	// Local marks a command that never talks to a remote API: no --base-url,
	// no --token. Today only the gateway group.
	Local bool

	// Registry=false exposes the command to CLI and MCP but keeps it out of the
	// agent's internal tool registry (gateway lifecycle, auth, tunnels).
	Registry bool

	// Annotations map to MCP tool annotations and drive the approval risk level.
	Annotations Annotations

	Handler func(ctx context.Context, in In) (Out, error)
}

type Annotations struct {
	Title           string
	ReadOnlyHint    bool
	DestructiveHint bool
	IdempotentHint  bool
	OpenWorldHint   bool
}
```

### O registry heterogêneo

Go não permite `[]Command[any, any]`. A ponte é uma interface não-genérica:

```go
// internal/core/command/descriptor.go

// Descriptor is the type-erased view of a Command, used by every surface.
type Descriptor interface {
	Key() string                       // "memories_store"
	Path() []string                    // ["memories", "store"]
	Group() string
	Summary() string
	Doc() string
	Examples() []Example
	Local() bool
	InRegistry() bool
	Annotations() Annotations
	InputSchema() *jsonschema.Schema
	InputType() reflect.Type
	Invoke(ctx context.Context, raw json.RawMessage) (any, error)
}

// Register erases the type parameters and stores the descriptor.
func Register[In, Out any](r *Registry, c Command[In, Out]) {
	r.add(&descriptor[In, Out]{cmd: c})
}

type descriptor[In, Out any] struct {
	cmd    Command[In, Out]
	schema *jsonschema.Schema // computed once, at registration
}

func (d *descriptor[In, Out]) Invoke(ctx context.Context, raw json.RawMessage) (any, error) {
	var in In
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, apperr.New("COMMAND_INVALID_INPUT").
			Causer(d.Key()).Msg(err.Error()).
			Status(http.StatusBadRequest).
			CTA(apperr.CallToAction{
				Label: "inspect the input schema for this action",
				Tool:  d.Key(), Input: map[string]any{"schema": true},
			})
	}
	if err := validate.Struct(in); err != nil {
		return nil, translateValidation(d.Key(), err)
	}
	return d.cmd.Handler(ctx, in)
}
```

O schema é computado **uma vez, no registro** — não a cada chamada.

### A entrada: struct com três tags

```go
// internal/domain/memory/schema.go

type StoreInput struct {
	Title      string   `json:"title"      jsonschema:"A brief summary of the memory content" validate:"required,max=200"`
	Category   Category `json:"category"   jsonschema:"Semantic category — pick the function of the knowledge, not writing style" validate:"required"`
	Content    string   `json:"content"    jsonschema:"The memory body, in Markdown" validate:"required"`
	Confidence float64  `json:"confidence" jsonschema:"0..1 — 0.9-1.0 verified, 0.6-0.8 strong inference, below 0.6 a guess" validate:"gte=0,lte=1"`
	Scopes     []string `json:"scopes,omitempty"     jsonschema:"Globs where this memory applies, e.g. src/features/**/*.ts"`
	Links      []string `json:"links,omitempty"      jsonschema:"UUIDs of related memories"`
	Supersedes []Super  `json:"supersedes,omitempty" jsonschema:"Memories this one replaces, with the reason"`

	Reasoning string `json:"_reasoning" jsonschema:"MANDATORY. Explain why this tool is being called now, what outcome you expect, and the immediate next step." validate:"required,min=1"`
}
```

| Tag | Papel |
|---|---|
| `json` | Nome do campo em toda superfície: flag do CLI (`--title`), chave no JSON Schema, campo do body HTTP |
| `jsonschema` | Descrição — cumpre o papel do `.describe()` do Zod. É o texto que o LLM lê para preencher o payload |
| `validate` | Validação por `go-playground/validator/v10`, com tradução para `apperr` |

**`_reasoning` é injetado automaticamente** se ausente, por reflexão sobre `In` no registro — nenhum comando pode esquecê-lo. Um teste falha se algum `In` não tiver o campo.

### Derivação 1 — CLI

```go
// internal/transport/clix/build.go

// BuildCommand turns a Descriptor into a cobra command. Flags come from the
// json tags; help text comes from the jsonschema tags; complex types (slices,
// structs, maps) are accepted as JSON strings and decoded after parsing.
func BuildCommand(d command.Descriptor, client *apiclient.Client) *cobra.Command {
	c := &cobra.Command{
		Use:     d.Path()[len(d.Path())-1],
		Short:   d.Summary(),
		Long:    d.Doc(),
		Example: renderExamples(d),
	}
	binder := newFlagBinder(d.InputType())
	binder.Bind(c.Flags())          // reflection over In → pflag
	if !d.Local() {
		addTransportFlags(c.Flags()) // --base-url --token --workspace --agent
	}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		raw, err := binder.Collect(cmd.Flags(), args) // → json.RawMessage
		if err != nil {
			return err
		}
		return execute(cmd.Context(), d, raw, client, outputFrom(cmd))
	}
	return c
}
```

O `flagBinder` mapeia tipos Go para flags:

| Tipo Go | Flag | Nota |
|---|---|---|
| `string`, `int`, `float64`, `bool` | nativo do pflag | |
| `[]string` | `StringSlice` | repetível ou separado por vírgula |
| struct, map, `[]T` de struct | `String` com decodificação JSON | `--rules '[{"type":"always"}]'` — resolve o mesmo problema que `Schema.stringify` |
| `time.Time` | `String` RFC3339 | |
| enum (`type X string`) | `String` com validação e completion | valores vêm do JSON Schema |

Flags globais herdadas do original: `--format toon|json|yaml|md|jsonl`, `--filter-output`, `--token-limit`, `--token-offset`, `--token-count`, `--schema`, `--llms`, `--llms-full`. Ver [[CLI cobra]].

### Derivação 2 — MCP

```go
// internal/transport/mcpserver/register.go

func RegisterFlat(s *mcp.Server, reg *command.Registry) {
	for _, d := range reg.Sorted() { // alphabetical — stable ordering
		d := d
		mcp.AddTool(s, &mcp.Tool{
			Name:        d.Key(),
			Description: d.Doc(),
			Annotations: toMCP(d.Annotations()),
		}, func(ctx context.Context, req *mcp.CallToolRequest, in json.RawMessage) (*mcp.CallToolResult, any, error) {
			out, err := d.Invoke(ctx, in)
			if err != nil {
				return errorResult(err), nil, nil
			}
			return successResult(out), out, nil
		})
	}
}
```

O SDK oficial infere o JSON Schema das structs Go via as tags `jsonschema` — é exatamente o que torna a ponte viável em Go, no lugar dos `.describe()` do Zod.

### Derivação 3 — tool composta

Segunda projeção, selecionável por `mcp.toolShape` ([[ADR-0011 Superfície de tools versionada]]):

```go
// internal/transport/mcpserver/composite.go

type CompositeInput struct {
	Action    string          `json:"action"    jsonschema:"Action to execute"`
	Input     json.RawMessage `json:"input"     jsonschema:"Payload for the chosen action"`
	Schema    bool            `json:"schema,omitempty" jsonschema:"When true, skip execution and return the full action detail (description, examples, input schema, token estimate)"`
	Reasoning string          `json:"_reasoning" jsonschema:"MANDATORY. Why this tool is being called now." validate:"required,min=1"`
}

// ActionDetail is what schema:true returns — mirrors the original's payload,
// including the per-section token estimate so the agent can budget context.
type ActionDetail struct {
	Tool        string         `json:"tool"`
	Action      string         `json:"action"`
	Description string         `json:"description"`
	Examples    []Example      `json:"examples"`
	InputSchema map[string]any `json:"inputSchema"`
	Tokens      TokenEstimate  `json:"_tokens"`
}
```

A descrição da tool composta é montada como no original:

```
<Doc do grupo>

Composite tool `Memory` with 5 actions: recall, graph, reflect, store, forget.

## Usage
Call as `Memory({ action: "<action>", input: { ... }, _reasoning: "..." })`.
Set `schema: true` on the same level as `action` to receive the full action detail
(description, examples, input schema, token estimate) instead of executing.
```

### Derivação 4 — HTTP

```go
// internal/transport/httpapi/mount.go
func Mount(r chi.Router, reg *command.Registry) {
	for _, d := range reg.All() {
		d := d
		path := "/" + strings.Join(d.Path(), "/")
		r.Post(path, func(w http.ResponseWriter, req *http.Request) {
			raw, _ := io.ReadAll(io.LimitReader(req.Body, maxBody))
			out, err := d.Invoke(req.Context(), raw)
			writeEnvelope(w, out, err)
		})
	}
}
```

### Derivação 5 — tool interna do agente

O mesmo `Descriptor`, **sem HTTP**:

```go
// internal/runtime/agentloop/registry.go
func ToolsFrom(reg *command.Registry) []toolexec.Tool {
	var out []toolexec.Tool
	for _, d := range reg.All() {
		if !d.InRegistry() {
			continue // gateway, auth, tunnels stay out of the agent's reach
		}
		out = append(out, toolexec.Wrap(descriptorTool{d}))
	}
	return out
}
```

É a propriedade central herdada do original: **o agente interno e um cliente MCP externo usam a mesma definição, o mesmo schema e a mesma descrição.** Não existe divergência possível porque não existe segunda definição.

### Derivação 6 — `SKILL.md`

`pkg/skill` percorre o registry e gera `SKILL.md` + `references/{grupo}.md` a partir de `Doc` e `Examples`. Um teste de CI compara o commitado com o gerado. Ver [[Especificação da Skill]].

### Fronteira de privilégio

Herdada do original e explícita no tipo: `Registry: false` mantém `gateway`, `auth`, `tunnels` e `events` fora do alcance do agente. O agente opera o domínio, não a identidade nem a exposição de rede ([[Ferramentas MCP]]).

## Decisões e divergências

> [!decision] Reflexão no registro, não na chamada
> Schema, flags e binder são computados uma vez no boot. O custo é ~140 structs refletidas em milissegundos; a chamada é `json.Unmarshal` puro.

> [!decision] `validate` no descriptor, não no handler
> A validação roda antes do handler, na fronteira genérica. Assim as cinco superfícies validam igual, e o handler recebe entrada já válida.

> [!decision] Erro de validação carrega CTA de introspecção
> Segue a regra do prompt-mestre: *"If a call fails validation, do NOT retry blindly: read the error, inspect the contract with `schema: true`, then fix."* O erro **traz** o caminho da introspecção pronto.

> [!decision] As duas projeções coexistem
> Plana e composta, por configuração. Evita repetir a quebra de integrações que o original causou ao migrar de uma para outra ([[ADR-0011 Superfície de tools versionada]]).

> [!decision] `_reasoning` é exigido nas superfícies de tool, não no terminal
> A nota põe o campo em `In` e diz que é obrigatório em toda tool. O original é mais preciso: o schema de CLI é `args + options`, e o schema de tool é isso **mais** `_reasoning` (`AgentSchema.object`). Pedir a um humano que justifique o comando que ele acabou de digitar seria absurdo. `Invoke` recebe a superfície e valida o campo apenas em MCP e no registry do agente; o binder do CLI ignora todo campo cujo nome JSON começa com `_`.

> [!decision] `command.Reasoning` embutido, e registro que falha sem ele
> A nota escreve o campo por extenso em cada struct de entrada. Embutir um tipo faz com que o texto, a regra de validação e a descrição do schema existam uma vez só. `Register` **falha** se o tipo de entrada não tiver o campo — é mais forte que o teste que a nota pede, porque transforma a classe inteira de esquecimento em falha de boot.

> [!decision] O schema inferido é reparado e conferido no registro
> **A biblioteca de inferência descarta silenciosamente os campos de uma struct embutida.** Um tipo que embute `command.Reasoning` volta de `jsonschema.For` sem nenhuma propriedade `_reasoning` — e sem erro. O efeito seria uma superfície inteira publicando um schema que não menciona o único campo que toda chamada precisa carregar, e nenhum modelo o enviaria. `completeSchema` injeta a propriedade quando falta (é o que "injetado automaticamente por reflexão sobre `In` no registro" significa na prática) **e** confere que todo campo visível em JSON está no schema, para que o próximo campo descartado quebre o build em vez de sumir do contrato.

> [!decision] `_reasoning` viaja fora de `input` na tool composta
> Como no original: o schema por ação é construído sem o campo, e ele fica ao lado de `action`. O handler continua esperando-o na própria entrada, então o servidor emenda os dois antes de invocar — o que também é o que faz uma chamada pela forma composta produzir exatamente o mesmo resultado que pela forma plana.

> [!decision] Uma string em branco não é uma razão
> `validate:"required,min=1"` aceita `"   "`. O original usa `.trim().min(1)`. Registramos a regra `notblank`, porque aproximar com `min=1` deixaria passar exatamente a chamada que a mensagem do original diz rejeitar.

> [!decision] `Registry.Freeze()` fecha o registro depois do boot
> Adição. Tudo é declarado na montagem; uma superfície capaz de acrescentar um comando depois tornaria a lista publicada um alvo móvel, e a lista é contrato.

## Testes

- **Paridade de superfície:** para cada comando, executar via CLI, MCP plano, MCP composto, HTTP e registry interno produz o **mesmo** efeito e a mesma saída normalizada. Teste tabular sobre o registry inteiro.
- **`_reasoning` universal:** todo `In` tem o campo, com `validate:"required,min=1"`. Falha em chamada com string vazia.
- **Tags obrigatórias:** todo campo exportado de todo `In` tem `json` e `jsonschema`. Falha nomeando struct e campo.
- **Nome de tool:** `Key()` é o caminho unido por `_`; a lista é ordenada alfabeticamente e estável entre execuções.
- **Round-trip de tipos complexos:** `--rules '[{"type":"always","instruction":"x"}]'` produz o mesmo `In` que o body HTTP equivalente.
- **Flags de transporte:** presentes em todo comando, ausentes nos `Local`, e nunca visíveis no `In` do handler.
- **`schema: true`:** devolve `ActionDetail` e **não** executa. Verificado por serviço fake com contador de chamadas.
- **Golden do `Doc`:** a descrição de cada grupo é comparada com `testdata/docs/{grupo}.golden` ([[Fixtures e Golden Files]]).
- **Aliases:** chamar um alias deprecado funciona e devolve `_deprecated` com o nome novo.

## Critério de pronto

- [x] `agents list` e `--mcp` expondo `agents_list` funcionam sobre a mesma definição — no binário `aosd`, ver a decisão abaixo
- [x] Teste de paridade verde para **todos** os comandos registrados — quatro superfícies, e um comando sem cenário reprova a suíte
- [x] Nenhum campo de entrada sem `json` + `jsonschema` — verificado no registro, não por convenção
- [x] `_reasoning` presente e validado em toda tool — exigido no registro e no schema publicado
- [x] Projeções plana e composta implementadas e testadas — contra um cliente MCP real, por transporte em memória
- [ ] `SKILL.md` gerada bate com a commitada — `pkg/skill` é da Fase 9

## Saída dos testes — Fase 2

```
$ go vet ./...
(sem saída — ok)

$ golangci-lint run
0 issues.

$ go test -race -count=1 ./...
ok  	github.com/OWNER/aos/internal/adapters/fscollections	5.008s
ok  	github.com/OWNER/aos/internal/adapters/fsconfig	1.665s
ok  	github.com/OWNER/aos/internal/app	2.221s
ok  	github.com/OWNER/aos/internal/architecture	1.575s
ok  	github.com/OWNER/aos/internal/core/apperr	1.873s
ok  	github.com/OWNER/aos/internal/core/apperr/scan	1.854s
ok  	github.com/OWNER/aos/internal/core/atomicfs	2.598s
ok  	github.com/OWNER/aos/internal/core/build	2.055s
ok  	github.com/OWNER/aos/internal/core/clockx	2.246s
ok  	github.com/OWNER/aos/internal/core/collections	2.413s
ok  	github.com/OWNER/aos/internal/core/command	1.561s
ok  	github.com/OWNER/aos/internal/core/config	1.434s
ok  	github.com/OWNER/aos/internal/core/env	1.201s
ok  	github.com/OWNER/aos/internal/core/identity	1.390s
ok  	github.com/OWNER/aos/internal/core/logging	1.327s
ok  	github.com/OWNER/aos/internal/core/safe	1.447s
ok  	github.com/OWNER/aos/internal/core/tokens	1.392s
ok  	github.com/OWNER/aos/internal/domain/agent	1.354s
ok  	github.com/OWNER/aos/internal/domain/config	1.521s
ok  	github.com/OWNER/aos/internal/domain/fakes	1.587s
ok  	github.com/OWNER/aos/internal/testx	1.599s
ok  	github.com/OWNER/aos/internal/transport/clix	1.728s
ok  	github.com/OWNER/aos/internal/transport/clix/format	1.653s
ok  	github.com/OWNER/aos/internal/transport/mcpserver	1.785s

$ go test -count=1 -cover ./... | go run ./tools/covercheck
covercheck: 29 packages, all at or above their floor

$ go test -v -run TestSurfaceParity ./internal/app/
=== RUN   TestEveryCommandHasAParityScenario
--- PASS: TestEveryCommandHasAParityScenario (0.00s)
=== RUN   TestSurfaceParity
=== RUN   TestSurfaceParity/agents_update
=== RUN   TestSurfaceParity/agents_delete
=== RUN   TestSurfaceParity/config_get
=== RUN   TestSurfaceParity/config_update
=== RUN   TestSurfaceParity/agents_list
=== RUN   TestSurfaceParity/agents_get
=== RUN   TestSurfaceParity/agents_create
--- PASS: TestSurfaceParity (0.36s)
ok  	github.com/OWNER/aos/internal/app	0.756s
```

### A prova da fase

`TestSurfaceParity` roda **cada comando registrado** por quatro superfícies e compara o resultado normalizado:

| Superfície | Como é exercitada |
|---|---|
| Registry do agente | `Descriptor.Invoke` em processo, sem transporte |
| CLI | árvore cobra real, argv construído por `clix.CommandLineFor` |
| MCP plano | cliente MCP real sobre transporte em memória |
| MCP composto | o mesmo cliente, `Tool({action, input, _reasoning})` |

```
--- PASS: TestEveryCommandHasAParityScenario
--- PASS: TestSurfaceParity/agents_create
--- PASS: TestSurfaceParity/agents_delete
--- PASS: TestSurfaceParity/agents_get
--- PASS: TestSurfaceParity/agents_list
--- PASS: TestSurfaceParity/agents_update
--- PASS: TestSurfaceParity/config_get
--- PASS: TestSurfaceParity/config_update
```

Cada superfície recebe uma instalação nova, com relógio congelado, de modo que um comando que escreve não contamina a superfície seguinte e dois resultados são comparáveis byte a byte.

### O que a fase entrega, e em qual binário

> [!decision] A composição vive em `cmd/aosd`, não em `cmd/aos`
> O [[Roteiro de Fases]] descreve a entrega como `aos agents list` e `aos --mcp`. Mas [[Hexagonal e Regra de Dependência]] proíbe o binário de CLI de linkar pacotes de domínio, e essa proibição é um teste (`TestNoDomainInClients`). Construir a árvore de comandos exige os tipos `In`, que vivem no domínio; logo o binário que hospeda a composição é o que carrega o domínio — `aosd`, exatamente como [[Visão Geral Go]] descreve.
>
> O que a fase realmente prova é a ponte: uma definição, quatro superfícies, resultado idêntico. `cmd/aos` recebe a mesma árvore na Fase 4, sobre HTTP, e a suíte de paridade ganha a quinta superfície ali.

Verificado à mão, com o binário:

```
$ aosd agents create atlas --name Atlas --role Orchestrator --orchestrator --format json
{ "data": { "id": "atlas", "name": "Atlas", "role": "Orchestrator", "orchestrator": true, … } }

$ aosd agents list --format toon
data:
  agents[1]{id,name,role,orchestrator,createdAt,updatedAt}:
    atlas,Atlas,Orchestrator,true,"2026-08-15T15:14:04-03:00","2026-08-15T15:14:04-03:00"
  total: 1

$ cat .aos/agents/atlas/AGENT.md
---
name: Atlas
role: Orchestrator
orchestrator: true
…
```

### O que não foi feito nesta fase

- **HTTP** — a quinta superfície. `Mount` sobre chi é da Fase 4, com o daemon, a autenticação e o WebSocket. `SurfaceHTTP` já existe no tipo e já é validada como superfície de humano.
- **`SKILL.md`** — `pkg/skill` é da Fase 9.
- **`mcp add` e `mcp doctor`** — dependem do gateway, Fase 4.
- **Aliases em uso** — o mecanismo está pronto e testado, mas nenhum comando foi renomeado ainda; a tabela da [[ADR-0016 Compatibilidade de nomes com o original]] entra com as features que a citam.
