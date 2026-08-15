---
tags: [critico, runtime, loop, hooks]
aliases: [Agent Loop, Loop do Agente, Runtime]
fase: 5
status: especificado
origem: "[[Agent Runtime Loop]]"
---

# Agent Loop ★

> Pai: [[AOS]] · Origem no original: [[Agent Runtime Loop]] · Decisão: [[ADR-0005 Loop de agente próprio]] · Fase: 5

## Objetivo

Transformar uma definição de agente em Markdown num processo de raciocínio com ferramentas, com cinco pontos de intervenção onde política pode injetar contexto, bloquear execução ou reescrever payload.

## Comportamento do original

`FractalAgentRuntimeService` (1.454 linhas) monta um `ToolLoopAgent` do Vercel AI SDK com cinco callbacks ([[Agent Runtime Loop]]):

| Ponto | Dispara | Pode |
|---|---|---|
| `prepareCall` | `UserPromptSubmit`, `SessionStart` | injetar contexto; **bloquear o turno** |
| `prepareStep` | contexto pendente, `PreCompact` | injetar contexto; podar histórico |
| `toolApproval` | `PreToolUse` | allow / deny / **reescrever `updatedInput`** |
| `experimental_repairToolCall` | — | **retorna `null` sempre** — reparo desabilitado |
| `onEnd` | `Stop` | encerrar |

Constantes e comportamentos que herdamos:

- `AGENT_COMPACTION_THRESHOLD_CHARS = 100_000`
- Poda: `reasoning: "all"`, `toolCalls: "before-last-15-messages"`, `emptyMessages: "remove"`
- `store: false` na OpenAI, com `include: ["reasoning.encrypted_content"]` — reasoning encriptado entre turnos, nada persistido no provider
- Cascata de resolução de modelo com cinco níveis
- `ask` degrada para `deny` — **este nós corrigimos**

## Design em Go

### O loop

```go
// internal/runtime/agentloop/loop.go
package agentloop

type Loop struct {
	provider LLMProvider
	tools    *toolexec.Registry
	hooks    Hooks
	compact  Compactor
	clock    Clock
	limits   Limits
}

type Limits struct {
	MaxSteps          int           // 40 — hard stop against runaway loops
	MaxToolsPerStep   int           // 4  — parallel independent tool calls
	StepTimeout       time.Duration // 5m per model call
	TotalTimeout      time.Duration // 30m per turn
}

// Run executes one agent turn: model call, tool calls, repeat until the model
// stops asking for tools or a limit is hit.
func (l *Loop) Run(ctx context.Context, s *State) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, l.limits.TotalTimeout)
	defer cancel()

	if err := l.hooks.PrepareCall(ctx, s); err != nil {
		return nil, err // a blocking UserPromptSubmit hook aborts the whole turn
	}

	for step := 0; step < l.limits.MaxSteps; step++ {
		if err := l.hooks.PrepareStep(ctx, s); err != nil {
			return nil, err
		}
		if l.compact.ShouldCompact(s.Messages) {
			if err := l.compactNow(ctx, s); err != nil {
				return nil, err
			}
		}

		resp, err := l.provider.Generate(ctx, s.Request())
		if err != nil {
			return nil, err
		}
		s.Append(resp.Message)

		if len(resp.ToolCalls) == 0 {
			break
		}
		if err := l.runTools(ctx, s, resp.ToolCalls); err != nil {
			return nil, err
		}
	}

	if err := l.hooks.OnEnd(ctx, s); err != nil {
		return nil, err
	}
	return s.Result(), nil
}
```

### Execução de tools — paralela e aprovada

```go
// runTools executes independent tool calls concurrently, honouring the master
// prompt's rule: "Call multiple independent tools in parallel." Dependent calls
// arrive in separate steps by construction of the model.
func (l *Loop) runTools(ctx context.Context, s *State, calls []ToolCall) error {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(l.limits.MaxToolsPerStep)

	results := make([]ToolResult, len(calls))
	for i, c := range calls {
		i, c := i, c
		g.Go(func() error {
			dec, err := l.hooks.ApproveTool(gctx, &c)
			if err != nil {
				return err
			}
			if !dec.Allow {
				results[i] = denied(c, dec.Reason)
				return nil // a denial is a result, not a failure
			}
			if len(dec.UpdatedInput) > 0 {
				c.Input = dec.UpdatedInput // PreToolUse rewrote the payload
			}
			res := l.tools.Invoke(gctx, c) // already Wrapped: spillover, metrics
			if err := l.hooks.AfterTool(gctx, &c, &res); err != nil {
				return err
			}
			results[i] = res
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	s.AppendToolResults(results)
	return nil
}
```

Note: **negação não é erro.** Vira um `ToolResult` que o modelo lê e sobre o qual raciocina — comportamento correto sob a Two-Strike Rule.

### Os hooks

```go
// internal/runtime/agentloop/hooks.go

type Hooks interface {
	PrepareCall(ctx context.Context, s *State) error
	PrepareStep(ctx context.Context, s *State) error
	ApproveTool(ctx context.Context, c *ToolCall) (Decision, error)
	AfterTool(ctx context.Context, c *ToolCall, r *ToolResult) error
	OnEnd(ctx context.Context, s *State) error
}

type Decision struct {
	Allow        bool
	Reason       string
	UpdatedInput json.RawMessage // rewrites the tool payload before execution
	AddContext   string
}
```

A implementação padrão (`eventHooks`) traduz para os nove eventos de [[Event Hooks (Go)]]:

```go
func (h *eventHooks) ApproveTool(ctx context.Context, c *ToolCall) (Decision, error) {
	out, err := h.bus.Emit(ctx, event.PreToolUse{
		SessionID: c.SessionID, Tool: c.Name, Input: c.Input,
	})
	if err != nil {
		return Decision{}, err
	}

	switch out.PermissionDecision {
	case "deny":
		return Decision{Allow: false, Reason: out.Reason}, nil

	case "ask":
		// DIVERGENCE FROM THE ORIGINAL, WHICH DEGRADES ask TO deny.
		// We ask a human. See ADR-0007.
		res, err := h.approver.RequestApproval(ctx, approval.Request{
			SessionID: c.SessionID, ToolName: c.Name, Input: c.Input,
			Reason: out.Reason, Risk: riskOf(c), Deadline: h.approvalDeadline,
		})
		if err != nil {
			// No approval channel available (headless): deny with a distinct,
			// honest reason instead of pretending the hook denied it.
			return Decision{Allow: false, Reason: "tool approval channel unavailable"}, nil
		}
		return Decision{Allow: res.Approved, Reason: res.Reason, UpdatedInput: res.UpdatedInput}, nil

	default:
		return Decision{Allow: true, UpdatedInput: out.UpdatedInput, AddContext: out.AdditionalContext}, nil
	}
}
```

### Compactação

```go
// internal/runtime/compaction/compact.go

const ThresholdChars = 100_000 // same as the original

type Policy struct {
	Reasoning      string // "all"
	ToolCalls      string // "before-last-15-messages"
	EmptyMessages  string // "remove"
}

// Prune implements the original's pruneMessages policy. It is safe only
// because three escape valves preserve what matters: memories written by the
// subconscious, task comments, and tool-output spillover on disk.
func Prune(msgs []Message, p Policy) []Message
```

O `PreCompact` dispara **antes** da poda, dando à extensão a chance de injetar contexto que deve sobreviver.

> [!important] A arquitetura de três camadas de memória
> A compactação só é segura porque nada do que importa vive apenas no contexto:
>
> | Camada | Tempo de vida |
> |---|---|
> | Contexto ativo | Podado acima de 100k chars |
> | `~/.aos/tmp/outputs/` | TTL 24 h — [[Tool Executor e Spillover]] |
> | `.aos/agents/{id}/memories/` | Permanente — [[Memory (Go)]] |

### Resolução de modelo

Cascata de cinco níveis, herdada:

```go
// internal/runtime/agentloop/model.go

// Resolve mirrors the original's fallback chain:
//   1. agent.Provider / agent.Model from frontmatter
//   2. config default provider, when only the model was given
//   3. the whole config default
//   4. a literal last-resort model
//   5. no provider at all → AOS_AGENT_PROVIDER_NOT_ENABLED
//
// It also parses the "{model} ({provider})" form the original introduced to
// work around VS Code frontmatter validation, e.g. "Gemini 3 Flash (google)".
func Resolve(a Agent, cfg Config) (ModelRef, error)

var modelWithProvider = regexp.MustCompile(`^(.+?)\s*\((.+)\)$`)
```

O nível de reasoning vem de `config.agents.models.default.reasoning`, com default `medium` — **nunca do agente individual**, como no original.

### Privacidade por provider

```go
// internal/runtime/providers/openai/openai.go

// Options mirror the original's privacy stance: conversations are not stored
// on the provider, and reasoning stays encrypted between turns.
func (p *Provider) options() openai.Options {
	return openai.Options{
		Store:   openai.Bool(false),
		Include: []string{"reasoning.encrypted_content"},
	}
}
```

### Reparo de tool call

Mantido **desabilitado**, como no original. A responsabilidade é do agente, guiado pela Two-Strike Rule do prompt-mestre. Um erro de validação carrega CTA de introspecção (`schema: true`), o que dá ao modelo o caminho concreto de correção ([[Command Layer]]).

## Decisões e divergências

> [!decision] `ask` pergunta de fato
> A divergência mais importante em relação ao original. Ver [[ADR-0007 Canal real de aprovação de tool]]. Em contexto headless a negação é imediata e com motivo distinto — não fingimos que o hook negou.

> [!decision] Limites duros no loop
> `MaxSteps: 40` e `TotalTimeout: 30m` não existem no original de forma explícita (há `stopWhen`). Um loop de agente sem teto é um incidente de custo esperando acontecer.

> [!decision] Negação de tool é resultado, não erro
> O modelo precisa ver a negação para raciocinar sobre ela. Propagar como erro Go abortaria o turno.

> [!decision] Reparo automático continua desligado
> Concordamos com a escolha do original: reparo automático mascara problemas de schema e gasta chamadas. O caminho certo é o erro que ensina.

## Testes

- **Provider fake roteirizado:** sequência determinística de tool calls exercita o loop inteiro sem rede. Base de quase todos os testes desta peça.
- **Bloqueio:** hook `UserPromptSubmit` com `decision: block` aborta o turno com `AOS_AGENT_HOOK_BLOCKED`; nenhuma chamada ao provider acontece.
- **Reescrita:** hook `PreToolUse` com `updatedInput` faz a tool receber o payload novo. Verificado por tool fake que registra a entrada.
- **Aprovação:** `ask` com aprovador fake que aprova → executa; que nega → resultado de negação; sem aprovador → negação imediata com motivo distinto.
- **Paralelismo:** três tool calls independentes executam concorrentes, respeitando `MaxToolsPerStep`; verificado por contador de concorrência máxima.
- **Compactação:** histórico acima de 100k chars dispara `PreCompact` e poda; reasoning some, tool calls das últimas 15 mensagens sobrevivem. Golden do resultado.
- **Limites:** modelo que pede tool indefinidamente para em `MaxSteps` com erro claro.
- **Cancelamento:** `ctx` cancelado no meio de uma tool encerra o loop e mata o processo filho ([[Concorrência e Context]]).
- **Resolução de modelo:** tabela cobrindo os cinco níveis da cascata e o formato `"{model} ({provider})"`.
- **Contrato de provider:** a mesma suíte roda contra OpenAI, Anthropic, Google e o fake ([[Testes de Contrato de Port]]).

## Critério de pronto

- [ ] Conversa real com um agente que usa tools e persiste memória
- [ ] Os nove eventos disparando nos pontos corretos
- [ ] `ask` abrindo aprovação real no desktop e no CLI interativo
- [ ] Compactação com golden estável
- [ ] Limites de passos e tempo aplicados
- [ ] Suíte de contrato verde para os três providers
