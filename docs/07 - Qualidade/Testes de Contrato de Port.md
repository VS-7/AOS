---
tags: [qualidade, testes, ports, lsp]
aliases: [Testes de Contrato, Contract Tests]
fase: 1
status: especificado
origem: "[[PROMPT — Reconstrução em Go]]"
---

# Testes de Contrato de Port

> Pai: [[Estratégia de Testes]] · Ver: [[SOLID no Go]] · Fase: 1

## Objetivo

Garantir substituibilidade real (LSP): toda implementação de um port passa **a mesma suíte**. Se o fake e o real divergem em comportamento observável, um dos dois está errado — e é melhor descobrir no teste do que em produção.

## Design

### A forma

```go
// internal/domain/testsuite/repository.go

// RunRepositoryContract exercises the behavioural contract every Repository
// implementation must satisfy. The factory receives a fresh t so each case can
// build an isolated instance.
func RunRepositoryContract[T any](t *testing.T, factory func(t *testing.T) collections.Repository[T], sample func() *T) {
	t.Run("create then get round-trips", func(t *testing.T) { /* ... */ })
	t.Run("get missing returns ErrNotFound", func(t *testing.T) { /* ... */ })
	t.Run("create duplicate returns ErrConflict", func(t *testing.T) { /* ... */ })
	t.Run("update with stale version returns ErrConflict", func(t *testing.T) { /* ... */ })
	t.Run("delete is idempotent", func(t *testing.T) { /* ... */ })
	t.Run("list applies filters", func(t *testing.T) { /* ... */ })
	t.Run("list ordering is stable", func(t *testing.T) { /* ... */ })
	t.Run("concurrent writers do not corrupt", func(t *testing.T) { /* ... */ })
}
```

Uso:

```go
// internal/adapters/fscollections/repo_test.go
func TestRepositoryContract(t *testing.T) {
	testsuite.RunRepositoryContract(t, newFSRepo, sampleMemory)
}

// internal/domain/fakes/repo_test.go
func TestFakeRepositoryContract(t *testing.T) {
	testsuite.RunRepositoryContract(t, newFakeRepo, sampleMemory)
}
```

### Os seis contratos

| Port | Implementações | Invariantes principais |
|---|---|---|
| `Repository[T]` | `fscollections`, fake | round-trip, not-found, CAS, idempotência de delete, ordenação estável |
| `LLMProvider` | openai, anthropic, google, openrouter, crof, opencode, codex, gemini-cli, fake | forma de tool call, propagação de erro, cancelamento por `ctx`, contagem de tokens |
| `ToolsetAdapter` | mcp, openapi, cli, custom | conectar/listar/chamar, erro em ferramenta inexistente, fechamento limpo |
| `Queue` | sqlitequeue, fake | claim exclusivo, expiração de lease, recuperação de stale, retry com backoff |
| `Index` | bleveindex, fallback linear | mesmos resultados para a mesma consulta, facetas corretas, consistência forte quando pedida |
| `Approver` | wails, cli, noop | timeout nega, cancelamento nega, `Remember` persiste |

### Contrato de provider sem rede

```go
// internal/domain/testsuite/llmprovider.go

// RunLLMProviderContract has two modes. The default runs against a recorded
// HTTP transport (golden cassettes in testdata/cassettes), so it is fast,
// offline and deterministic. With -tags=live and credentials present, it
// replays the same suite against the real API — which is how a cassette gets
// refreshed when a provider changes its wire format.
func RunLLMProviderContract(t *testing.T, factory func(t *testing.T) agentloop.LLMProvider)
```

Isso resolve o problema prático: testar nove providers de verdade a cada commit é caro e instável; testar nenhum significa descobrir uma mudança de formato em produção.

### Casos que só um contrato pega

Exemplos reais que a suíte cobre e que um teste por implementação normalmente esqueceria:

- **`Repository`:** delete de registro inexistente é sucesso ou erro? A suíte fixa: sucesso (idempotente). Sem contrato, o fake responderia diferente do real.
- **`LLMProvider`:** um provider devolve tool call com argumentos como string JSON, outro como objeto. O contrato exige a forma normalizada na saída, e o adaptador que converte.
- **`Queue`:** dois workers dando claim simultâneo — exatamente um leva.
- **`Index`:** busca com acento (`memória` × `memoria`) devolve o mesmo resultado nas duas implementações.

## Decisões e divergências

> [!decision] Cassettes para providers
> Sem elas, a suíte de provider é inutilizável no dia a dia. Com elas, uma mudança de formato aparece quando a cassette é atualizada — de propósito, com revisão.

> [!decision] O fake passa o mesmo contrato do real
> É a razão de o fake ser confiável nos testes de domínio. Um fake que se comporta diferente do real transforma toda a suíte de domínio em teatro.

> [!decision] Meta-teste de cobertura de contrato
> Um teste percorre as implementações de port conhecidas e falha se alguma não tiver contrato associado. Impede que um adaptador novo entre sem suíte.

## Testes

- As seis suítes rodando contra todas as implementações
- Meta-teste de cobertura verde
- Cassettes atualizáveis com `-tags=live`
- Contrato de `Repository` rodando sob `-race` com escritores concorrentes

## Critério de pronto

- [ ] Seis suítes implementadas
- [ ] Toda implementação de port coberta
- [ ] Cassettes de provider gravadas e versionadas
- [ ] Meta-teste impedindo adaptador sem contrato
