---
tags: [transporte, mcp, protocolo]
aliases: [MCP Go, Servidor MCP]
fase: 2
status: em-construcao
origem: "[[MCP - Model Context Protocol]]"
---

# MCP Go SDK

> Pai: [[AOS]] · Origem no original: [[MCP - Model Context Protocol]] · [[Fractal MCP]] · Fase: 2

## Objetivo

Expor o registry de comandos como servidor MCP, em stdio e em HTTP streamable, com paridade garantida entre os dois transportes.

## Comportamento do original

O servidor MCP **não é um projeto separado**: é o CLI com `--mcp` ([[Fractal MCP]]). Dois transportes derivam da **mesma** coleta de tools, o que garante paridade.

No transporte HTTP, um pool mantém uma instância de CLI por par `workspaceId:agentId` — isolamento de identidade por conexão, sem processos extras.

Streaming vira `notifications/progress` quando o cliente envia `progressToken`.

Um problema operacional real, observado: `~/.mcp.json` registra `"command": "fractal"`, resolvido pelo PATH para uma versão três releases atrás da que está rodando ([[Versões e Artefatos]]).

## Design em Go

```go
// internal/transport/mcpserver/stdio.go

// ServeStdio runs the MCP server over stdio. This is what `aos --mcp` does and
// what a client registers in its MCP config.
func ServeStdio(ctx context.Context, reg *command.Registry, cfg Config) error {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    build.Name,
		Version: build.Version,
	}, nil)

	switch cfg.ToolShape {
	case ShapeFlat:
		RegisterFlat(s, reg)
	case ShapeComposite:
		RegisterComposite(s, reg)
	case ShapeBoth:
		RegisterFlat(s, reg)
		RegisterComposite(s, reg)
	}
	return s.Run(ctx, &mcp.StdioTransport{})
}
```

```go
// internal/transport/mcpserver/http.go

// HTTPHandler serves MCP over streamable HTTP. Identity comes from the ambient
// context, so a client can connect as a specific agent inside a specific
// workspace — the same isolation the original gets from its CLI pool, without
// keeping N CLI instances alive.
func HTTPHandler(reg *command.Registry) http.Handler
```

**Divergência estrutural:** o original mantém um pool de instâncias de CLI porque a identidade vive num `AsyncLocalStorage` do processo. Em Go, a identidade vive no `context.Context` da requisição — não há estado por conexão para isolar, e o pool é desnecessário.

### Registro de tool

O SDK oficial infere o JSON Schema das structs Go via tags `jsonschema` — ver [[Command Layer]] para as duas projeções e o formato de `ActionDetail`.

### Progresso

```go
// When a command returns a stream, each chunk becomes a progress notification,
// mirroring the original's behaviour, and the accumulated chunks are returned
// at the end.
func streamAsProgress(ctx context.Context, req *mcp.CallToolRequest, ch <-chan any) (*mcp.CallToolResult, error)
```

### `aos mcp add` e `aos mcp doctor`

```go
// Add writes the client's MCP config using the ABSOLUTE path of the running
// binary, never a bare command name. Fixes the PATH-resolution problem the
// reverse engineering observed, where the registered MCP was three releases
// behind the running daemon.
func Add(ctx context.Context, target Client) error

// Doctor compares the registered binary's version with the running daemon's
// and reports drift, missing token, and unreachable daemon.
func Doctor(ctx context.Context) (Report, error)
```

## Decisões e divergências

> [!decision] Sem pool de instâncias
> Consequência de identidade em `context` em vez de ALS. Menos memória, menos estado, mesmo isolamento.

> [!decision] Caminho absoluto no registro
> Corrige o problema mais comum de operação observado no original.

> [!decision] `doctor` compara versões
> Adição. Diagnostica a divergência silenciosa entre cliente MCP e daemon.

> [!decision] Duas projeções, selecionáveis
> Ver [[ADR-0011 Superfície de tools versionada]].

> [!decision] `Server.AddTool` de baixo nível, não o `AddTool` genérico
> O SDK oferece um `AddTool` genérico que infere o schema do tipo `In` do handler. Não serve aqui: o registry é heterogêneo e o handler recebe `json.RawMessage`, então a inferência produziria o schema de `json.RawMessage`. Usamos a forma de baixo nível, que aceita o schema já computado no registro — o mesmo objeto que o CLI usa para `--schema` e que a documentação publica.

> [!decision] Erro de tool, não erro de protocolo
> Um erro de protocolo encerra a chamada; um erro de tool chega ao modelo, que lê o código, os `issue` e o CTA e pode agir. Devolver `apperr` como falha de transporte jogaria fora a razão inteira de o erro ser estruturado. Os handlers retornam `IsError: true` com o JSON do erro.

> [!decision] Os testes falam o protocolo
> A suíte conecta um cliente MCP real a um servidor real pelo transporte em memória do SDK, em vez de chamar a função handler. O que se testa é o que um cliente recebe — nomes, ordem, schema publicado, anotações — e não o que a nossa função devolve.

## Testes

- Paridade stdio × HTTP: a mesma lista de tools, na mesma ordem alfabética
- Schema inferido bate com o golden por comando
- `schema: true` devolve `ActionDetail` sem executar
- Progresso emitido quando há `progressToken`; ausente quando não há
- Identidade por requisição: dois clientes com agentes diferentes veem `agents_me` diferente
- `mcp add` grava caminho absoluto
- `doctor` detecta versão divergente e token ausente
- Erro de tool devolve `isError: true` com o JSON de `apperr`, incluindo CTA

## Critério de pronto

- [x] `--mcp` expondo o registry completo — no binário `aosd`; ver a decisão em [[Command Layer]]
- [ ] `/mcp` com paridade verificada por teste — o transporte HTTP é da Fase 4; a paridade entre as duas projeções de tool já é verificada
- [ ] `mcp add` e `mcp doctor` operando — dependem do gateway, Fase 4
- [x] Duas projeções de tool disponíveis — plana, composta e as duas juntas

## Saída dos testes — Fase 2

```
$ go test -race ./internal/transport/mcpserver/
ok  	github.com/OWNER/aos/internal/transport/mcpserver
```

| Caso | Teste |
|---|---|
| Uma tool por comando, nome = caminho com `_`, ordem alfabética | `TestFlatShapePublishesOneToolPerCommand` |
| O schema publicado carrega `_reasoning` e o marca obrigatório | `TestPublishedSchemaCarriesTheReasoningField` |
| Anotações chegam ao cliente | `TestAnnotationsTravelToTheClient` |
| Uma tool por grupo, com a descrição montada como no original | `TestCompositeShapePublishesOneToolPerGroup` |
| Anotações compostas: read-only só se todas forem | `TestCompositeMergesAnnotations` |
| `schema: true` devolve `ActionDetail` **sem executar** | `TestSchemaTrueInspectsWithoutExecuting` |
| `_reasoning` fora de `input`, como um cliente real envia | `TestCompositeCarriesReasoningOutsideTheActionInput` |
| O schema por ação não repete `_reasoning` | `TestActionSchemaOmitsReasoning` |
| Ação desconhecida lista as que existem | `TestCompositeRejectsAnUnknownAction` |
| Razão em branco é chamada rejeitada, com a mensagem do original | `TestCompositeRejectsAMissingReasoning` |
| Erro de tool chega ao modelo com o CTA de introspecção | `TestAToolErrorReachesTheModel` |
| As duas formas coexistem | `TestBothShapesCoexist` |
| Alias funciona e ensina o nome novo | `TestAnAliasKeepsWorkingAndTeachesTheNewName` |
| A fronteira de privilégio é do agente, não do MCP | `TestThePrivilegeBoundaryIsNotAnMCPBoundary` |

**Pendente para a Fase 4:** o transporte HTTP streamable, a paridade stdio × HTTP, o progresso por `progressToken`, a identidade por requisição, `mcp add` e `mcp doctor`. A nota permanece `em-construcao` por isso.
