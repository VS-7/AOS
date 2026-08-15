---
tags: [adr, decisao, linguagem, go]
aliases: [ADR-0001, Go]
fase: 0
status: especificado
origem: "[[Stack Tecnológica]]"
---

# ADR-0001 — Go como linguagem

> Pai: [[AOS]] · Origem no original: [[Stack Tecnológica]] · Fase: 0

## Contexto

O original roda em **Bun compilado em binário standalone**. Os números observados ([[Versões e Artefatos]]):

| Artefato | Tamanho |
|---|---|
| `bin/fractal` (CLI) | 79,5 MB |
| `bin/fractal-server` (daemon) | 169,6 MB |
| Source maps publicados | 104 MB |
| `Fractal.app` completo | 578 MB |

O runtime inteiro do Bun é embutido em cada binário. O daemon carrega um interpretador JavaScript para servir arquivos Markdown. Além do peso, o modelo tem três custos operacionais concretos observados na engenharia reversa:

1. **`unhandledRejection` derruba o servidor** ([[Camada @server]]) — uma promise rejeitada em qualquer feature encerra a sessão de todos os workspaces.
2. **Source maps com `sourcesContent`** vazaram 100% do código-fonte próprio ([[Metodologia da Engenharia Reversa]]). É um efeito colateral direto de distribuir um bundle JS.
3. **Cross-compilation exige um build por plataforma** com o toolchain do Bun, e o pacote npm precisa de cinco `optionalDependencies` binárias mais um shim Node de resolução ([[Fractal CLI]]).

## Decisão

**Go 1.23+** para todo o backend, CLI, MCP e a camada nativa do desktop.

Consequências técnicas que motivam a escolha, em ordem de peso:

| Propriedade | Efeito concreto neste sistema |
|---|---|
| Binário único, sem runtime | O daemon deixa de embutir um interpretador. Estimativa: 25–40 MB por binário contra 80–170 MB. |
| `GOOS`/`GOARCH` | `darwin/linux/windows × amd64/arm64` de uma máquina só, sem shim de resolução de plataforma no npm. |
| Goroutines + `context.Context` | O tick de 15 min com fan-out por workspace ([[Jobs e Queues]]) vira `errgroup` com cancelamento propagado, sem event loop compartilhado. |
| `recover()` por goroutine | Um panic em uma feature degrada aquela requisição, não o processo. Corrige o defeito #16 do original. |
| Sem source maps | O binário compilado não carrega o fonte. `-ldflags="-s -w"` remove tabela de símbolos. |
| Tipagem estrutural + generics | Suficiente para `Repository[T]` e `Command[In, Out]` sem reflexão pesada. |

**Restrição de dependências:** proibido CGO. Isso elimina SQLite via `mattn/go-sqlite3` e obriga `modernc.org/sqlite` ([[ADR-0008 SQLite puro Go para filas]]). O ganho é `CGO_ENABLED=0` em todos os alvos, o que mantém a cross-compilation trivial e produz binários estaticamente ligados.

## Alternativas consideradas

| Alternativa | Análise |
|---|---|
| **Manter TypeScript, trocar Bun por Node + SEA** | Não resolve peso nem o modelo de falha. Mantém o ecossistema de dependências (a única vantagem real: o Vercel AI SDK). |
| **Rust** | Binários ainda menores, memória segura sem GC. Descartado por custo de desenvolvimento: `async` com múltiplos providers de LLM, um motor de filesystem e uma UI desktop em Rust é 2–3× o tempo de Go para o mesmo resultado. O gargalo deste sistema é I/O e latência de LLM, não CPU — a vantagem de performance do Rust não se converte em valor aqui. |
| **Go + CGO para SQLite** | Ganharia performance marginal na fila de jobs e perderia cross-compilation de uma máquina. Trade-off ruim: a fila processa dezenas de jobs por hora, não milhares por segundo. |

## Consequências

**Positivas**
- Distribuição vira um `curl` de binário, Homebrew formula ou `go install`. Sem Node, sem npm, sem shim.
- O modo desenvolvimento (`development: true` fixo no original, defeito #12) deixa de existir: não há HMR de assets no backend Go.
- Testes com `t.TempDir()` e `-race` cobrem concorrência de escrita em arquivo, que o original não trata ([[ADR-0012 Escrita atômica e lock por arquivo]]).

**Negativas — e como são pagas**
- **Não existe equivalente ao Vercel AI SDK `ToolLoopAgent`.** O loop de ferramentas é implementação própria. Ver [[ADR-0005 Loop de agente próprio]] — a análise conclui que isso é vantagem, não custo.
- **Não existe `zod`.** O papel de `.describe()` é cumprido por struct tags `jsonschema:"..."` com `invopop/jsonschema`. Ver [[Command Layer]].
- **Não existe `liquidjs`.** `osteele/liquid` cobre a sintaxe usada; divergências de filtro são registradas em [[ADR-0014 Liquid para templates]].
- **Ergonomia de erro mais verbosa.** Mitigado por um pacote `apperr` com construtores por domínio — ver [[Estratégia de Erros]].

**Neutras**
- O frontend continua sendo React + TypeScript. A troca de linguagem é do backend para dentro; a UI não muda de tecnologia. Ver [[React 19 e Bindings]].

## Status

**Aceito.** Decisão de fundação — reverter significa recomeçar.
