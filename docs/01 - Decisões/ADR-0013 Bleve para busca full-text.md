---
tags: [adr, decisao, busca, bleve]
aliases: [ADR-0013, Bleve, Full-text]
fase: 3
status: especificado
origem: "[[Memory]]"
---

# ADR-0013 — Bleve para busca full-text

> Pai: [[AOS]] · Origem no original: [[Stack Tecnológica]] · Fase: 3

## Contexto

O original usa `minisearch` — busca full-text **em memória**, reconstruída a cada boot ([[Stack Tecnológica]]). Serve `memories recall` com `search`, e a listagem de coleções.

O padrão de uso descrito no [[System Prompt (BASE)]] torna a busca caminho quente:

> Every session begins with `recall` or `graph` to reorient.
> Before storing: recall first. If a trace already exists, link or supersede.

Ou seja: **toda sessão começa com uma busca, e toda escrita de memória é precedida de uma busca**. Com `scopes` (globs) e filtros de status/categoria compondo a consulta.

Um índice em memória funciona bem até algumas milhares de memórias; acima disso o custo de reconstrução no boot e a pressão de memória por workspace passam a incomodar — e o sistema é multi-workspace no mesmo processo.

## Decisão

**`github.com/blevesearch/bleve/v2`** — índice full-text persistente, puro Go, sem CGO.

```go
// internal/adapters/bleveindex/index.go

// Index is the outbound port implementation for full-text search over
// memories and collection records. One index per workspace, stored under
// ~/.aos/workspaces/{id}/index/ — derived data, safe to delete and rebuild.
type Index struct{ /* bleve.Index */ }

func (i *Index) Upsert(ctx context.Context, d Document) error
func (i *Index) Delete(ctx context.Context, id string) error
func (i *Index) Search(ctx context.Context, q Query) ([]Hit, error)
func (i *Index) Rebuild(ctx context.Context, src Iterator) error
```

Três decisões de forma:

**1. O índice é derivado, nunca fonte da verdade.** Os arquivos Markdown são a verdade ([[ADR-0004 Collections em Markdown]]). Apagar `index/` e reconstruir é sempre seguro, e é o procedimento de recuperação padrão. O índice vive fora do repositório do usuário — em `~/.aos/`, não em `.aos/` — justamente para nunca ser commitado.

**2. Indexação assíncrona, com fallback síncrono.** Escritas enfileram uma atualização de índice; a busca aceita um parâmetro de consistência:

```go
type Query struct {
	Text        string
	Filters     map[string]any
	Scopes      []string  // globs; ver doublestar
	Consistency Consistency // Eventual (default) | Strong
}
```

`Strong` drena a fila pendente antes de buscar. Usado por `memories store`, que precisa detectar duplicata recém-criada.

**3. Filtro por `scopes` fora do Bleve.** Globs (`src/features/**/*.ts`) não são consulta de índice invertido. O Bleve filtra por termos e devolve candidatos; o casamento de glob acontece depois, com `bmatcuk/doublestar/v4`, sobre o conjunto reduzido. Os modos `strict` e `lax` do original são preservados ([[Memory (Go)]]).

## Alternativas consideradas

| Alternativa | Análise |
|---|---|
| **Busca linear sobre o índice em memória** (como o original) | Suficiente até ~2.000 registros. **Adotado como fallback**: se o Bleve falhar ao abrir, o motor degrada para varredura com aviso, em vez de derrubar a busca. Não é a escolha primária porque o custo de boot cresce com o histórico, e histórico crescente é a proposta do produto. |
| **SQLite FTS5** | Excelente e já teríamos o SQLite por causa da fila ([[ADR-0008 SQLite puro Go para filas]]). **Descartado com ressalva**: `modernc.org/sqlite` inclui FTS5, mas misturaria estado operacional (fila) com índice de domínio no mesmo arquivo, ou exigiria um segundo banco — sem ganho claro sobre o Bleve, que tem analisadores e facetas melhores para este caso. Decisão revisável. |
| **Busca vetorial / embeddings** | O campo `description` do schema de memória é descrito como *"resumo otimizado para busca vetorial"* — o original prevê isso e não implementa. **Adiado**: exige um provider de embedding, custo por escrita e um índice vetorial. Registrado como evolução em [[Memory (Go)]]; o Bleve entrega o valor imediato. |

## Consequências

**Positivas**
- Busca não degrada com o crescimento do grafo de memórias, que é justamente o que o produto acumula.
- Sem CGO, coerente com [[ADR-0001 Go como linguagem]].
- Facetas do Bleve dão de graça as contagens por categoria que o [[Prompt Assembly]] injeta no contexto.

**Negativas**
- **Um índice para manter em coerência** com arquivos que podem ser editados fora do sistema. Mitigação: o watcher do [[Collections Engine]] dispara reindexação, e existe `aos workspace reindex`.
- **Espaço em disco** — tipicamente 20–40% do volume de texto indexado.
- **Bleve tem manutenção irregular.** Risco mitigado por o índice ser derivado e o port ser pequeno: trocar a implementação é reescrever um adaptador com uma suíte de contrato pronta ([[Testes de Contrato de Port]]).

## Status

**Aceito.**
