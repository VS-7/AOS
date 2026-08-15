---
tags: [transporte, mcp, protocolo]
aliases: [MCP Go, Servidor MCP]
fase: 2
status: especificado
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

- [ ] `aos --mcp` expondo o registry completo
- [ ] `/mcp` com paridade verificada por teste
- [ ] `mcp add` e `mcp doctor` operando
- [ ] Duas projeções de tool disponíveis
