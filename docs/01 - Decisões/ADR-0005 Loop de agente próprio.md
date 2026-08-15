---
tags: [adr, decisao, runtime, agente]
aliases: [ADR-0005, Agent Loop próprio]
fase: 5
status: especificado
origem: "[[Agent Runtime Loop]]"
---

# ADR-0005 — Loop de agente próprio

> Pai: [[AOS]] · Origem no original: [[Agent Runtime Loop]] · Detalhe técnico: [[Agent Loop]] · Fase: 5

## Contexto

O original usa `ToolLoopAgent` do Vercel AI SDK ([[Agent Runtime Loop]]), configurado com cinco callbacks de intervenção — `prepareCall`, `prepareStep`, `toolApproval`, `experimental_repairToolCall`, `onEnd` — além de `stopWhen`, `providerOptions` e `runtimeContext`.

**Não existe equivalente em Go.** As opções disponíveis são: os SDKs oficiais de cada provider (`openai-go`, `anthropic-sdk-go`, `google.golang.org/genai`), que expõem a API de chat com tools mas não o loop; ou frameworks de orquestração jovens, cuja abstração não cobre os cinco pontos de intervenção que este sistema exige.

Vale notar o que o original faz com esses pontos: `experimental_repairToolCall` **retorna `null` sempre** — o reparo automático está desabilitado, e a responsabilidade de corrigir uma tool call malformada recai sobre o próprio agente, guiado pela regra "Two-Strike Tool Rule" do [[System Prompt (BASE)]]. Ou seja, boa parte do valor do SDK não está sendo usada.

## Decisão

Implementar o loop em `internal/runtime/agentloop`, com uma interface `LLMProvider` única e adaptadores por provider.

```go
// internal/runtime/agentloop/provider.go
type LLMProvider interface {
	Name() string
	Generate(ctx context.Context, req Request) (Response, error)
	Stream(ctx context.Context, req Request) (Stream, error)
}

type Request struct {
	Model        string
	Instructions string
	Messages     []Message
	Tools        []ToolSpec
	Reasoning    ReasoningLevel
	Options      map[string]any // provider-specific escape hatch
}
```

Os cinco pontos de intervenção viram uma interface explícita:

```go
type Hooks interface {
	PrepareCall(ctx context.Context, s *State) error                       // UserPromptSubmit + SessionStart
	PrepareStep(ctx context.Context, s *State) error                       // contexto pendente + PreCompact
	ApproveTool(ctx context.Context, c *ToolCall) (Decision, error)        // PreToolUse
	AfterTool(ctx context.Context, c *ToolCall, r *ToolResult) error       // PostToolUse / PostToolUseFailure
	OnEnd(ctx context.Context, s *State) error                             // Stop
}
```

Ver [[Agent Loop]] para o design completo e [[Event Hooks (Go)]] para os nove eventos.

## Alternativas consideradas

| Alternativa | Análise |
|---|---|
| **Um framework Go de agentes de terceiros** | Adiciona uma abstração entre nós e os cinco pontos de intervenção que são a identidade do sistema. Se o framework não expõe reescrita de payload de tool antes da execução (`updatedInput`), o sistema de hooks — que é o mecanismo de extensibilidade de skills — deixa de funcionar. Dependência de alto risco em componente crítico. |
| **Manter um serviço Node só para o loop** | Reintroduz o runtime JS que a [[ADR-0001 Go como linguagem]] eliminou, e cria uma fronteira de processo no caminho mais quente do sistema. |
| **Loop próprio** | Escolhido. |

## Consequências

**Positivas**
- **Controle total sobre os cinco pontos.** Nenhum é "experimental", nenhum tem semântica surpresa. O ponto de aprovação de tool pode, por isso, implementar aprovação real — ver [[ADR-0007 Canal real de aprovação de tool]], que **só é possível** porque o loop é nosso.
- **Teste sem rede.** Um `LLMProvider` fake que devolve uma sequência roteirizada de tool calls torna o loop testável de ponta a ponta em milissegundos. Ver [[Testes de Contrato de Port]].
- **Contabilidade de tokens e custo por passo** entram no ponto certo do loop, sem depender de o SDK expor.
- **Cancelamento por `context.Context`** atravessa o loop inteiro, incluindo chamadas de tool em andamento.

**Negativas**
- **Cada provider tem particularidades de streaming e de tool calling.** OpenAI, Anthropic e Google diferem em formato de tool call, em como devolvem reasoning e em como aceitam resultados de tool. O adaptador absorve isso, e o custo é real: estimam-se 2–4 dias por provider, mais manutenção quando as APIs mudarem.
- **Recursos que o SDK dá de graça precisam ser escritos:** retry com backoff, reparo de tool call malformada (que manteremos desabilitado, como o original), poda de mensagens (`pruneMessages` → implementação própria em [[Agent Loop]]).
- **Risco de divergir do comportamento do original em detalhes sutis** — por exemplo, a ordem exata em que o reasoning é descartado na compactação. Mitigação: golden files comparando prompts montados e sequências de mensagens ([[Fixtures e Golden Files]]).

## Status

**Aceito.** O documento de missão registra a mesma conclusão: *"Isso é vantagem: dá controle total sobre os 5 pontos de intervenção."*
