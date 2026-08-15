---
tags: [dominio, memoria, grafo, core]
aliases: [Memory Go, Memória, Knowledge Graph]
fase: 3
status: especificado
origem: "[[Memory]]"
---

# Memory (Go)

> Pai: [[Agent (Go)]] · Origem no original: [[Memory]] · Ver: [[Subconsciente (Go)]] · Fase: 3

## Objetivo

A camada cognitiva. Memórias são **pessoais do agente**, não do workspace — a distinção com [[Instruction (Go)]] é central: *memory is YOURS; instruction is EVERYONE'S*.

## Comportamento do original

Grafo, não log ([[Memory]]). Cinco propriedades definem o modelo:

1. **Supersede com linhagem.** Nada é apagado. Nova memória aponta `supersedes: [{id, reason}]`; a antiga vira `deprecated` com `deprecatedBy`/`deprecatedAt`/`deprecatedReason`. É possível reconstruir por que o agente pensa o que pensa hoje.
2. **Não existe `delete`** — só depreciação. Decisão deliberada: histórico cognitivo não se apaga.
3. **`scopes`** são globs que definem onde a memória se aplica, com `scopesMode: strict|lax` na consulta.
4. **`confidence` calibrada honestamente:** 0.9–1.0 verificado, 0.6–0.8 inferência forte, abaixo de 0.6 palpite. *"Inflated confidence is the main way future-you gets misled."*
5. **Memórias são globais entre "agent universes"** — instâncias paralelas do mesmo agente compartilham tudo, sem escopo privado. Depreciação afeta todas imediatamente.

As **13 categorias cognitivas** da 0.1.401 substituíram as 9 técnicas da 0.1.314, com instrução explícita: *"Pick the function of the knowledge, not writing style."*

Os verbos foram renomeados de CRUD para cognitivos: `recall`, `graph`, `reflect`, `store`, `forget`.

## Design em Go

Entidade completa em [[Collections Engine]]. As partes de domínio:

```go
// internal/domain/memory/entity.go

type Category string

const (
	CatDecision     Category = "decision"     // design choices, trade-offs, rationale
	CatIntent       Category = "intent"       // goals, desires, expected outcomes
	CatCommitment   Category = "commitment"   // agreements, promises, action items
	CatRelationship Category = "relationship" // people, roles, teams, dynamics
	CatEvent        Category = "event"        // occurrences and milestones
	CatObservation  Category = "observation"  // patterns and signals
	CatError        Category = "error"        // bugs, incidents, missteps
	CatLearning     Category = "learning"     // root causes, fixes, insights
	CatFact         Category = "fact"         // verified objective knowledge
	CatReference    Category = "reference"    // external links and docs
	CatInstruction  Category = "instruction"  // repeatable procedures
	CatPreference   Category = "preference"   // tastes and stylistic choices
	CatContext      Category = "context"      // session/environment awareness
)

type Status string

const (
	StatusActive     Status = "active"
	StatusDeprecated Status = "deprecated"
	StatusArchived   Status = "archived"
	StatusTTLExpired Status = "ttl_expired"
)

type Super struct {
	ID     string `yaml:"id"     json:"id"`
	Reason string `yaml:"reason" json:"reason" validate:"min=5"`
}
```

```go
// internal/domain/memory/service.go

type Service interface {
	// Recall scans with filters: status, category, agent, scopes, full-text.
	Recall(ctx context.Context, in RecallInput) ([]Memory, error)

	// Graph maps nodes and edges, reporting hubs, silos and health.
	Graph(ctx context.Context, in GraphInput) (Graph, error)

	// Reflect dives into one memory by id.
	Reflect(ctx context.Context, id string) (*Memory, error)

	// Store records durable knowledge, applying the supersede protocol.
	Store(ctx context.Context, in StoreInput) (*Memory, error)

	// Forget deprecates logically, preserving lineage. There is no Delete.
	Forget(ctx context.Context, in ForgetInput) (*Memory, error)
}
```

### O protocolo de supersede

```go
// Store applies the supersede protocol atomically from the caller's point of
// view: the replacement is written first, then each superseded memory is
// deprecated pointing at it. If deprecation fails midway, the new memory
// exists and the old ones stay active — a visible inconsistency the graph
// health check reports, rather than a silent loss.
func (s *service) Store(ctx context.Context, in StoreInput) (*Memory, error) {
	for _, sup := range in.Supersedes {
		if _, err := s.repo.Get(ctx, key(in.AgentID, sup.ID)); err != nil {
			return nil, errSupersedeTargetMissing(sup.ID)
		}
	}

	m := s.build(ctx, in)
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}

	for _, sup := range in.Supersedes {
		if _, err := s.Forget(ctx, ForgetInput{
			ID: sup.ID, Reason: sup.Reason, DeprecatedBy: m.ID,
		}); err != nil {
			s.log.Error("supersede incomplete", "new", m.ID, "old", sup.ID, "err", err)
		}
	}
	s.bus.Publish(ctx, MemoryStored{ID: m.ID, AgentID: m.Agent})
	return m, nil
}
```

### Scopes

```go
// matchScopes decides whether a memory applies to the files in play.
//   strict — memories without scopes are excluded
//   lax    — they are included
// Globs use doublestar so "src/features/**/*.go" behaves as expected.
func matchScopes(m Memory, paths []string, mode ScopesMode) bool
```

### Grafo

```go
type Graph struct {
	Nodes  []Node `json:"nodes"`
	Edges  []Edge `json:"edges"`
	Health Health `json:"health"`
}

type Node struct {
	ID, Title, Description string
	Category   Category
	Status     Status
	Confidence float64
	Label      string   // node label
	Group      string   // = category, used for colouring
}

type Edge struct {
	From, To string
	Type     string // "reference" (via links) | "supersedes"
}

// Health surfaces what the original's graph command reports: hubs (highly
// linked), silos (unlinked), and aggregate confidence — so the agent can see
// the shape of its own knowledge, not just its contents.
type Health struct {
	Hubs          []string
	Silos         []string
	AvgConfidence float64
	DeprecatedPct float64
}
```

### Injeção no contexto

Memórias **não** entram no prompt. Só contagens por categoria e as regras gerais — ver [[Prompt Assembly]].

## Decisões e divergências

> [!decision] `forget` exige razão de no mínimo 5 caracteres
> Herdado do original. Razões vagas confundem o self futuro, e o prompt-mestre é explícito sobre quando **não** esquecer: em dúvida, baixe a confiança em vez de esquecer.

> [!decision] Supersede parcial é reportado, não escondido
> Sem transação multi-arquivo ([[ADR-0004 Collections em Markdown]]), uma falha no meio deixa estado inconsistente. Preferimos que o `Health` do grafo reporte a que finja atomicidade.

> [!decision] Sem `delete`, como no original
> A tool não existe. Apagar um arquivo à mão continua possível — é filesystem — mas nenhuma superfície do sistema oferece isso.

> [!decision] TTL processado por job, não na leitura
> `expiresAt` vencido vira `ttl_expired` num job diário, não na consulta. Evita escrita durante leitura e mantém `Recall` puro.

> [!decision] Aviso de globalidade preservado
> A documentação do grupo mantém o alerta do original: *"Memories are GLOBAL across all agent universes. What you store here, every parallel self sees. There is no 'private' flag."* É comportamento, não detalhe — e motiva o CAS de [[ADR-0012 Escrita atômica e lock por arquivo]].

## Testes

- Round-trip com todos os campos, inclusive `supersedes` aninhado
- Supersede: nova criada, antiga `deprecated` com os três campos preenchidos
- Supersede com alvo inexistente é rejeitado antes de qualquer escrita
- `forget` com razão de 4 caracteres é rejeitado
- Scopes `strict` exclui memórias sem escopo; `lax` inclui
- Glob `src/**/*.go` casa `src/a/b.go` e não casa `src/a/b.ts`
- Grafo: nós, arestas `reference` e `supersedes`, hubs e silos identificados
- Contagem por categoria bate com a faceta do índice
- Concorrência: dois "universos" atualizando a mesma memória → um sucesso, um `AOS_COLLECTION_CONFLICT`
- Job de TTL move vencidas para `ttl_expired` sem tocar nas demais

## Critério de pronto

- [ ] Gravar e recuperar memórias com grafo funcionando
- [ ] Protocolo de supersede completo, com linhagem preservada
- [ ] Scopes com os dois modos
- [ ] Contagens por categoria alimentando o [[Prompt Assembly]]
- [ ] Busca full-text sobre memórias via [[ADR-0013 Bleve para busca full-text]]
