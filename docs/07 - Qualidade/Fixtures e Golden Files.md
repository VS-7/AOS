---
tags: [qualidade, testes, golden, fixtures]
aliases: [Golden Files, Fixtures]
fase: 1
status: em-construcao
origem: "[[PROMPT — Reconstrução em Go]]"
---

# Fixtures e Golden Files

> Pai: [[Estratégia de Testes]] · Fase: 1

## Objetivo

Tornar revisável toda mudança em texto que um LLM ou um usuário vai ler: prompt montado, descrição de tool, `SKILL.md`, saída de CLI, mensagem de erro.

Estes artefatos não têm asserção natural — não se testa "o prompt está bom". O que se testa é: **mudou, e a mudança foi revisada**.

## Design

### O helper

```go
// internal/testx/golden.go

// Assert compares got with testdata/<name>.golden. With -update it rewrites the
// file, so refreshing a golden is a deliberate act that shows up as a reviewable
// diff in the commit — never a silent adjustment inside a test run.
func Assert(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o644))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden: run with -update")
	require.Equal(t, string(want), string(got))
}
```

```bash
go test ./internal/runtime/prompt -update
git diff testdata/   # a mudança no prompt aparece no code review
```

### O que tem golden

| Artefato | Caminho | Por quê |
|---|---|---|
| Documento de contexto completo | `testdata/prompt/full.golden` | O prompt é o produto; mudanças precisam ser revisadas |
| Diretivas de papel | `testdata/prompt/{orchestrator,member}.golden` | |
| Descrição de cada grupo de tool | `testdata/docs/{grupo}.golden` | É o que o LLM lê para decidir usar |
| `SKILL.md` e cada `references/*.md` | `testdata/skill/` | Verificado também por `git diff` no CI |
| Saída de CLI por formato | `testdata/cli/{cmd}.{format}.golden` | Cinco formatos, incluindo TOON |
| Erros nas quatro superfícies | `testdata/errors/{code}.{surface}.golden` | |
| Instrução de spillover | `testdata/toolexec/instruction.golden` | |
| Contexto do subconsciente | `testdata/subconscious/context.golden` | |

### Fixtures de workspace

```go
// internal/testx/fixture/workspace.go

// Workspace materializes a complete, deterministic workspace in t.TempDir():
// agents with memories, tasks with todos and comments, skills, collections and
// views. Every timestamp comes from the fixed clock and every id from the
// sequential generator, so anything built from it is byte-stable.
func Workspace(t *testing.T, opts ...Option) *Fixture
```

Três tamanhos:

| Fixture | Conteúdo | Uso |
|---|---|---|
| `Minimal` | 1 agente, 3 memórias, 1 task | Testes rápidos |
| `Typical` | 3 agentes, 40 memórias, 12 tasks, 2 skills, 2 coleções, 3 views | Golden de prompt, testes de UI |
| `Large` | 10 agentes, 10.000 memórias, 500 tasks | Testes de escala do índice e do `Refresh()` |

### Estabilidade

Golden só é útil se o mesmo input produzir o mesmo output. Três garantias:

1. **Clock fixo e IDs sequenciais** ([[Estratégia de Testes]])
2. **Ordenação determinística** em toda listagem — a lista de tools é ordenada alfabeticamente, o inventário do workspace por nome, as memórias por `createdAt` e depois por `id`
3. **Serialização estável** — mapas Go têm ordem aleatória; toda serialização para golden passa por um encoder que ordena chaves

```go
// internal/testx/stable.go

// MarshalStable serializes with sorted map keys and fixed indentation, so a
// golden never fails because Go randomized map iteration order.
func MarshalStable(v any) ([]byte, error)
```

## Decisões e divergências

> [!decision] `-update` explícito, nunca automático
> Um golden que se atualiza sozinho não testa nada. O fluxo é: rodar com `-update`, olhar o `git diff`, decidir se a mudança é intencional.

> [!decision] Golden do prompt completo, não por seção
> O valor está em ver o documento inteiro mudar. Um golden por seção esconderia mudanças de ordem ou de estrutura entre blocos.

> [!decision] Fixture `Large` gerada, não commitada
> 10.000 memórias em `testdata/` inflaria o repositório. É gerada deterministicamente a partir de uma semente fixa.

> [!decision] Determinismo por índice, não por semente aleatória
> A nota fala em semente fixa. A implementação não usa aleatoriedade alguma: cada campo deriva do índice do registro (`stamp(i)`, `categories[i%7]`). É mais forte que uma semente — não depende do algoritmo de PRNG da versão do Go — e é o que faz `TestFixtureIsByteIdentical` valer entre máquinas.

> [!decision] As fixtures escrevem arquivos, não entidades
> Na Fase 1 as entidades de domínio ainda não existem. A fixture materializa Markdown e JSON diretamente, no formato que o motor lê — o que é suficiente para medir `Refresh()` e para o round-trip, e continua válido quando as entidades chegarem: elas leem os mesmos arquivos.

## Testes

- Golden estável: rodar duas vezes produz o mesmo resultado
- `MarshalStable` produz saída idêntica em 100 execuções (ordem de mapa)
- Fixture `Typical` construída duas vezes é byte-idêntica
- Fixture `Large` gera em menos de 5 s

## Critério de pronto

- [x] Helper de golden com `-update` — `internal/testx.Assert`, com mensagem que ensina o fluxo
- [ ] Todos os artefatos da tabela com golden — nenhum dos oito artefatos existe ainda (prompt: Fase 5; `SKILL.md`: Fase 9; saída de CLI e erros por superfície: Fases 2 e 4)
- [x] Três fixtures de workspace determinísticas — `Minimal`, `Typical`, `Large`, com `TestFixtureIsByteIdentical`
- [x] Serialização estável verificada — `TestMarshalStableIsStable`, 100 execuções idênticas

## Saída dos testes — Fase 1

```
$ go test -race ./internal/testx/...
ok  	github.com/OWNER/aos/internal/testx
```

| Fixture | Arquivos | Tempo de geração |
|---|---|---|
| `Minimal` | 8 | < 5 ms |
| `Typical` | 105 | ~20 ms |
| `Large` | 11.525 | 740 ms (orçamento: 5 s) |

O helper de golden está pronto e exercitado (`testdata/hello.golden`), mas a tabela de artefatos continua vazia porque nenhum deles existe nesta fase. É a razão de a nota permanecer `em-construcao`.
