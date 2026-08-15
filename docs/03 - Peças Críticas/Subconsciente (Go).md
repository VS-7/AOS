---
tags: [critico, runtime, memoria, subconsciente]
aliases: [Subconsciente Go, Subconscious, Camada Cognitiva de Fundo]
fase: 6
status: especificado
origem: "[[Subconsciente]]"
---

# Subconsciente (Go) ★

> Pai: [[AOS]] · Origem no original: [[Subconsciente]] · Fase: 6

## Objetivo

Um **segundo LLM**, menor e mais barato, rodando em paralelo ao agente principal, que observa a sessão e decide o que vira memória. É a ideia mais distintiva do sistema e uma das três que não podem ser cortadas.

## Comportamento do original

O problema que resolve, com precisão ([[Subconsciente]]):

> O prompt instrui o agente a "refletir e manter memórias antes de entregar a resposta final". Na prática, agentes esquecem ou economizam essa etapa sob pressão de custo. O subconsciente **retira essa responsabilidade do caminho crítico** — a memória se forma mesmo quando o agente principal não pensa nisso.
>
> É a diferença entre "o agente deveria memorizar" e "o sistema memoriza".

Faz duas coisas distintas:

1. **Seleção de contexto (entrada)** — decide o que do workspace importa *agora* para o agente principal
2. **Formação de memória (saída)** — identifica aprendizados duráveis e grava memórias

Limites operacionais, deliberadamente agressivos:

| Constante | Valor |
|---|---|
| `SUBCONSCIOUS_MAX_RECENT_MESSAGES` | 8 |
| `SUBCONSCIOUS_MAX_RECENT_EVENTS` | 12 |
| `SUBCONSCIOUS_INPUT_CHAR_LIMIT` | 12.000 |

E um mecanismo essencial: **deduplicação por hash de assinatura**. Sem ele, o subconsciente rodando a cada turno recriaria a mesma memória repetidamente.

O prompt do subconsciente, transcrito no vault de RE, tem regras não-negociáveis que portamos literalmente: não responder ao usuário, não fazer roleplay do agente principal, não devolver payloads brutos, selecionar agressivamente, preferir **uma instrução precisa a dez lembretes genéricos**, e só criar rascunho de memória quando o sinal for durável.

## Design em Go

### Constantes e configuração

```go
// internal/runtime/subconscious/limits.go
package subconscious

const (
	MaxRecentMessages = 8
	MaxRecentEvents   = 12
	InputCharLimit    = 12_000
)

type Config struct {
	Model       ModelRef      // config.agents.models.subconscious
	Timeout     time.Duration // 30s — it is cheap and frequent, never deep
	MaxDrafts   int           // 3 memory drafts per observation
	MinInterval time.Duration // 60s — floor between observations of one session
}
```

### O observador

```go
// internal/runtime/subconscious/observer.go

type Observer struct {
	provider agentloop.LLMProvider
	memories memory.Service
	events   event.Reader
	prompt   string        // //go:embed subconscious.md
	sigs     *SignatureSet // dedup across turns
	cfg      Config
}

// Observe runs the background cognitive pass for one turn. It never blocks the
// main agent: the caller fires it after the turn ends and collects the result
// asynchronously. A failure here is logged and dropped — losing an observation
// must never fail a user-facing turn.
func (o *Observer) Observe(ctx context.Context, in Input) (Output, error)

type Input struct {
	AgentID    string
	SessionID  string
	Messages   []agentloop.Message // last MaxRecentMessages
	Events     []event.Record      // last MaxRecentEvents
	Inventory  prompt.Inventory    // same workspace inventory the main agent got
}

type Output struct {
	Guidance string        // surgical guidance for the main agent's next turn
	Drafts   []MemoryDraft // durable learnings worth persisting
}
```

### O pipeline

Espelha o do original, com nomes Go:

```
Observe
  ├─ recentEvents()        # últimos 12 eventos de sessão
  ├─ serializeMessages()   # últimas 8 mensagens
  ├─ truncate()            # teto de 12.000 chars
  ├─ formatContext()
  ├─ buildAgent()          # identidade derivada
  ├─ run()                 # chamada ao LLM secundário
  ├─ signature()           # deduplicação
  └─ persist()             # grava memórias
```

### Identidade derivada

```go
// buildIdentity mirrors the original: the subconscious is not a separate agent
// with its own file — it is derived from the main agent it supports.
func buildIdentity(a agent.Agent) prompt.Identity {
	return prompt.Identity{
		ID:   a.ID + "-subconscious",
		Name: a.Name + " Subconscious",
		Role: "Background cognitive layer for " + a.Name,
		Instructions: strings.Join([]string{
			"Support the active agent by selecting and synthesizing only the workspace context that materially improves the next step.",
			"Observe the session in the background and emit structured memory drafts when durable knowledge is formed.",
		}, "\n"),
	}
}
```

### Deduplicação por assinatura

```go
// internal/runtime/subconscious/signature.go

// Signature is the idempotency key for a memory draft. Without it, running the
// subconscious every turn would recreate the same memory over and over.
//
// It hashes the normalized semantic content — category plus lowercased,
// whitespace-collapsed title and body — so cosmetic rewording does not produce
// a new memory, while a genuine change does.
func Signature(d MemoryDraft) string {
	h := sha256.New()
	h.Write([]byte(d.Category))
	h.Write([]byte{0})
	h.Write([]byte(normalize(d.Title)))
	h.Write([]byte{0})
	h.Write([]byte(normalize(d.Content)))
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// SignatureSet persists seen signatures per agent so dedup survives restarts.
// Backed by the jobs SQLite database — operational state, not domain.
type SignatureSet struct{ /* ... */ }

func (s *SignatureSet) Seen(ctx context.Context, agentID, sig string) (bool, error)
func (s *SignatureSet) Mark(ctx context.Context, agentID, sig string, ttl time.Duration) error
```

**Divergência:** o original mantém a assinatura em memória de processo. Nós persistimos, para que a deduplicação sobreviva a restart do daemon — que acontece com frequência durante desenvolvimento e a cada auto-update.

### Persistência com protocolo de memória

```go
// persist applies the same discipline the master prompt demands of the main
// agent: recall before store, link or supersede instead of duplicating.
func (o *Observer) persist(ctx context.Context, agentID string, drafts []MemoryDraft) ([]memory.Memory, error) {
	var out []memory.Memory
	for _, d := range drafts {
		sig := Signature(d)
		if seen, _ := o.sigs.Seen(ctx, agentID, sig); seen {
			continue
		}

		// Recall before storing — a near-duplicate becomes a link or a
		// supersede, never a second copy. Duplicates dilute the graph.
		similar, err := o.memories.Recall(ctx, memory.RecallInput{
			AgentID: agentID, Query: d.Title, Category: d.Category, Limit: 5,
		})
		if err != nil {
			return nil, err
		}
		if hit, ok := bestMatch(similar, d); ok {
			d.Links = append(d.Links, hit.ID)
			if d.Contradicts(hit) {
				d.Supersedes = append(d.Supersedes, memory.Super{ID: hit.ID, Reason: d.SupersedeReason})
			}
		}

		m, err := o.memories.Store(ctx, d.ToStoreInput(agentID))
		if err != nil {
			return nil, err
		}
		_ = o.sigs.Mark(ctx, agentID, sig, 30*24*time.Hour)
		out = append(out, *m)
	}
	return out, nil
}
```

### Modelo próprio

```go
// resolveModel mirrors the original's cascade:
//   1. config.agents.models.subconscious
//   2. agent.provider / agent.model
//   3. config.agents.models.default
//   4. a small last-resort model
//
// The point is to run a cheap, frequent observer alongside an expensive,
// deep main agent. On the machine observed in the reverse engineering, the
// user had not separated the slots — both pointed at the same large model.
func resolveModel(cfg Config, a agent.Agent, global config.Models) (ModelRef, error)
```

### Onde entra no ciclo

```go
// internal/runtime/agentloop/hooks.go — inside OnEnd

func (h *eventHooks) OnEnd(ctx context.Context, s *State) error {
	if _, err := h.bus.Emit(ctx, event.Stop{SessionID: s.SessionID, Last: s.LastAssistant()}); err != nil {
		return err
	}
	// Fire-and-forget: the observation runs on a detached context with its own
	// timeout so a slow or failing subconscious never delays the user's answer.
	h.subconscious.Schedule(context.WithoutCancel(ctx), s.Snapshot())
	return nil
}
```

`Schedule` faz *coalescing*: observações da mesma sessão dentro de `MinInterval` são fundidas, evitando N chamadas em uma rajada de turnos curtos.

### Relação com a compactação

Quando a [[Compactação de Contexto]] poda o histórico, informação se perde. O subconsciente, tendo observado e persistido antes, preserva o que importava. **A memória sobrevive à poda do contexto** — e é isso que torna a poda agressiva do original (`reasoning: all`) defensável.

## Decisões e divergências

> [!decision] Assinatura persistida, não em memória
> Deduplicação sobrevive a restart. O original recriaria memórias após reinício.

> [!decision] Recall antes de store, dentro do subconsciente
> O original grava os rascunhos. Nós aplicamos ao subconsciente a mesma disciplina que o prompt-mestre exige do agente principal: *"Before storing: recall first. If a trace already exists, link or supersede. Storing duplicates dilutes the graph."*

> [!decision] Execução desacoplada com `context.WithoutCancel`
> A observação não pode ser cancelada pelo fim do turno, nem atrasá-lo. Falha é registrada e descartada.

> [!decision] Coalescing por sessão
> Uma conversa com dez turnos curtos em um minuto não deve gerar dez observações. O original não trata isso.

> [!decision] O subconsciente não tem tools
> Ele observa e propõe; quem persiste é o `Observer` via `memory.Service`. Dar tools ao subconsciente o transformaria num segundo agente autônomo — que não é o desenho.

## Testes

- **Provider fake:** entrada roteirizada → rascunhos determinísticos. Base dos demais testes.
- **Deduplicação:** o mesmo rascunho em duas observações gera **uma** memória. Reformulação cosmética (maiúsculas, espaços) também dedupa; mudança semântica não.
- **Persistência da assinatura:** após reiniciar o `Observer`, o mesmo rascunho continua deduplicado.
- **Recall antes de store:** rascunho semelhante a uma memória existente vira `link`; rascunho contraditório vira `supersedes` + `forget` da anterior.
- **Limites:** entrada com 50 mensagens é cortada em 8; 100 eventos em 12; texto grande truncado em 12.000 chars.
- **Não bloqueia:** turno principal responde antes de a observação terminar (verificado por temporização com provider lento).
- **Falha isolada:** subconsciente que erra não afeta o resultado do turno.
- **Coalescing:** cinco turnos em 10 s produzem uma observação.
- **Cascata de modelo:** tabela cobrindo os quatro níveis.
- **Golden do prompt:** o contexto formatado é comparado com `testdata/subconscious/context.golden`.

## Critério de pronto

- [ ] Memórias formadas sozinhas durante uma sessão real, sem o agente principal pedir
- [ ] Deduplicação verificada, inclusive após restart
- [ ] Observação nunca atrasa nem quebra o turno principal
- [ ] Recall-antes-de-store aplicado aos rascunhos
- [ ] Slot de modelo próprio, com cascata testada
- [ ] Golden do contexto estável
