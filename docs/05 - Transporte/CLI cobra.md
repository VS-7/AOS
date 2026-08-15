---
tags: [transporte, cli, cobra]
aliases: [CLI Go, cobra]
fase: 2
status: pronto
origem: "[[Camada @cli]]"
---

# CLI cobra

> Pai: [[AOS]] · Origem no original: [[Camada @cli]] · [[Comandos CLI]] · Fase: 2

## Objetivo

Derivar a árvore de comandos de terminal do registry, com as flags globais pensadas para consumo por LLM.

## Comportamento do original

A camada de CLI é minúscula — apenas monta os grupos declarados em cada feature ([[Camada @cli]]). 23 grupos na 0.1.401.

As flags globais são o que diferencia este CLI de um CLI comum:

| Flag | Efeito |
|---|---|
| `--format toon\|json\|yaml\|md\|jsonl` | Formato de saída, default `toon` |
| `--filter-output <keys>` | Filtra por caminho: `foo,bar.baz,a[0,3]` |
| `--token-limit` / `--token-offset` / `--token-count` | **Paginação por tokens** |
| `--schema` | JSON Schema do comando |
| `--llms` / `--llms-full` | Manifesto legível por LLM |
| `--mcp` | Sobe como servidor MCP stdio |
| `--base-url` / `--token` | Redireciona para uma API remota |

A engenharia reversa nota o porquê: *"Nenhum CLI tradicional pagina por tokens. Aqui o consumidor primário é um LLM com janela de contexto finita."*

## Design em Go

```go
// internal/transport/clix/root.go

func NewRoot(reg *command.Registry, cfg Config) *cobra.Command {
	root := &cobra.Command{Use: build.Name, Version: build.Version}

	addGlobalFlags(root.PersistentFlags())

	for _, group := range reg.Groups() {
		g := &cobra.Command{Use: group.Name, Short: group.Summary, Long: group.Doc}
		for _, d := range group.Commands {
			g.AddCommand(BuildCommand(d, cfg.Client))
		}
		root.AddCommand(g)
	}

	// Built-ins live under a namespace so they cannot shadow a domain group —
	// fixing the original's `skills` collision (defect #15).
	root.AddCommand(selfCommand(reg))   // aos self completions|mcp|skills
	return root
}
```

### Formatos de saída

```go
// internal/transport/clix/format/format.go

type Format string

const (
	TOON  Format = "toon"  // compact, token-efficient — the default
	JSON  Format = "json"
	YAML  Format = "yaml"
	MD    Format = "md"
	JSONL Format = "jsonl"
)

type Renderer interface {
	Render(w io.Writer, v any) error
}
```

TOON não tem implementação Go pública madura; escrevemos a nossa em `internal/transport/clix/format/toon`, com golden files comparando com a saída do original para os casos cobertos.

### Paginação por tokens

```go
// internal/transport/clix/format/tokens.go

// Estimate approximates token count. The heuristic is 4 characters per token
// for Latin text, with a correction for CJK. It is documented as approximate:
// the error margin is around 10%, which is fine for slicing output but must
// never be used for billing.
func Estimate(s string) int

// Slice cuts rendered output at a token budget, on a line boundary, and
// reports what was omitted so the caller can request the next page.
func Slice(s string, limit, offset int) (out string, more bool)
```

### Filtro de saída

```go
// Filter selects paths from a rendered structure: "foo,bar.baz,a[0,3]".
// Same syntax as the original, because agents were taught this syntax by the
// tool descriptions.
func Filter(v any, expr string) (any, error)
```

### Detecção de agente

```go
// isAgent reports whether the consumer is a program rather than a human: no
// TTY on stdout. When true, the default format becomes json and colours are
// disabled — the same signal the original passes as `agent: true`.
func isAgent() bool { return !term.IsTerminal(int(os.Stdout.Fd())) }
```

### Completions

`aos self completions bash|zsh|fish|powershell` — geradas pelo cobra, com valores de enum vindos do JSON Schema de cada comando.

## Decisões e divergências

> [!decision] Builtins sob `aos self`
> Corrige o defeito #15: no original, o builtin `skills` sobrescreve o grupo de domínio, tornando `create`/`install`/`discovery` inacessíveis pelo CLI.

> [!decision] Estimativa de tokens documentada como aproximada
> O original usa `tokenx` sem declarar margem. Documentamos ~10% e proibimos uso para cobrança.

> [!decision] `--base-url` e `--token` ausentes nos comandos locais
> Herdado: só o grupo `gateway` é local. Ver [[Command Layer]].

> [!decision] Default `toon`, mas `json` sem TTY
> O original usa `toon` sempre. Sem TTY, o consumidor é um programa, e JSON é mais previsível de parsear.

> [!decision] O encoder TOON foi escrito contra o binário original, não contra uma especificação
> Não existe implementação Go, e o `@toon-format/toon` não está nas fontes extraídas: é dependência npm, e os source maps não a incluem. As regras foram lidas da saída do binário instalado nesta máquina, comparando `--format toon` com `--format json` no mesmo comando. Quatro regras saíram daí, e cada uma tem um teste que cita a saída observada:
> 1. objeto é `chave: valor`, dois espaços por nível, **na ordem de declaração**
> 2. array declara o tamanho: `chave[N]:`
> 3. array de objetos com as mesmas chaves vira tabela: `commands[1]{command,description}:` seguido de linhas separadas por vírgula — é onde o formato ganha o nome, os nomes de campo aparecem uma vez em vez de N
> 4. array não uniforme cai para lista `- chave: valor`
>
> Aspas só quando o valor seria ambíguo: `"Suggested command:"` leva aspas (dois-pontos), `0.1.400` não leva (dois pontos decimais não são número), `5326` levaria.

> [!decision] A ordem dos campos é a de declaração, não a alfabética
> O original imprime `status, pid, port, startedAt, version` — a ordem da struct. Renderizar por `map[string]any` embaralharia, e todo golden derivado de uma listagem dependeria de nada. O renderizador normaliza para uma árvore que lembra a ordem das chaves, atravessando o JSON, que é onde as tags `json` já são o contrato.

> [!decision] `CommandLineFor` é a única construtora de linha de comando
> Os exemplos do `--help` precisam ser linhas coláveis, e a suíte de paridade precisa dirigir o terminal com o mesmo payload que as outras superfícies recebem. Construir a linha à mão em dois lugares é como os dois divergem.

## Testes

- Toda entrada do registry vira comando; `--help` renderiza `Doc`
- Round-trip de tipo complexo por flag JSON
- Cada formato tem golden; TOON comparado com amostras do original
- `--filter-output` com as três formas de caminho
- `--token-limit`/`--token-offset` paginam sem cortar linha no meio
- `--schema` emite o JSON Schema do comando
- Sem TTY: formato default vira json, cores desligadas
- `aos skills --help` mostra o grupo de domínio
- Completions geradas para os quatro shells

## Critério de pronto

- [x] Árvore de comandos derivada do registry
- [x] Sete flags globais implementadas — `--format`, `--filter-output`, `--token-limit`, `--token-offset`, `--token-count`, `--schema`, mais as quatro de transporte nos comandos não-locais
- [x] TOON estável, verificado contra a saída do binário original em três formas (objeto plano, array tabular, array não uniforme)
- [x] Sem colisão entre builtins e domínio — `TestBuiltinsCannotShadowADomainGroup`

## Saída dos testes — Fase 2

```
$ go test -race ./internal/transport/clix/...
ok  	github.com/OWNER/aos/internal/transport/clix
ok  	github.com/OWNER/aos/internal/transport/clix/format
```

Os casos que a nota lista, e onde estão:

| Caso da nota | Teste |
|---|---|
| Toda entrada do registry vira comando; `--help` renderiza `Doc` | `TestEveryCommandOfTheRegistryBecomesACommand`, `TestHelpRendersDocumentationAndPasteableExamples` |
| Round-trip de tipo complexo por flag JSON | `TestComplexTypesTravelAsJSON`, `TestTheSamePayloadProducesTheSameInputThroughBothPaths` |
| TOON comparado com amostras do original | `TestTOONMatchesTheOriginalFlatObject`, `TestTOONMatchesTheOriginalTabularArray`, `TestTOONFallsBackToAListWhenTheArrayIsNotUniform` |
| `--filter-output` com as três formas de caminho | `TestFilterSelectsPaths` |
| `--token-limit`/`--token-offset` sem cortar linha | `TestTokenPaginationCutsOnALineBoundary`, `TestSliceCutsOnALineBoundary` |
| `--schema` emite o JSON Schema | `TestSchemaFlagPrintsTheContractWithoutRunning` |
| Sem TTY: formato default vira json | `TestDefaultFormatFollowsTheConsumer` |
| Grupo de domínio acessível apesar dos builtins | `TestBuiltinsCannotShadowADomainGroup` |
| Completions para os quatro shells | `TestCompletionsAreGeneratedForFourShells` |

**Pendente:** o golden por formato de saída (`testdata/cli/{cmd}.{format}.golden`) entra quando houver um comando cuja saída seja estável o bastante para valer um golden — hoje as asserções são sobre a forma, que é o que muda quando o encoder muda. `--llms`/`--llms-full` existem sob `aos self llms`; o manifesto completo ganha golden na Fase 9, junto com a `SKILL.md`.
