---
tags: [arquitetura, solid, principios]
aliases: [SOLID, Princípios]
fase: 0
status: especificado
origem: "[[Padrão Feature-Slice]]"
---

# SOLID no Go

> Pai: [[AOS]] · Ver: [[Hexagonal e Regra de Dependência]] · [[Padrões de Projeto Aplicados]] · Fase: 0

## Objetivo

Traduzir os cinco princípios em regras verificáveis neste código, não em citações. Cada seção termina com o teste ou o portão que a torna real.

## SRP — uma responsabilidade

**Regra:** uma feature é uma fatia vertical; um serviço governa um agregado; transporte não contém regra.

```go
// internal/domain/memory/service.go

// Service owns the memory aggregate. It never writes tasks, never renders
// prompts, never talks HTTP.
type Service struct {
	repo   Repository
	search Searcher
	bus    Bus
	clock  Clock
	ids    IDGen
}

// Store persists durable knowledge, enforcing the supersede protocol.
func (s *Service) Store(ctx context.Context, in StoreInput) (*Memory, error)
```

O que **não** está aqui: parse de flags, montagem de JSON de resposta, decisão de status HTTP. Isso é transporte.

**Verificação:** o handler de um `Command` é uma chamada ao serviço mais tradução de tipos. Revisão rejeita `if` de regra de negócio em `commands.go`. Um teste heurístico falha se algum arquivo `commands.go` passar de 400 linhas.

## OCP — aberto a extensão, fechado a modificação

**Regra:** adicionar um provider de LLM, um tipo de toolset ou um adaptador de hook é implementar uma interface e registrar numa factory. Nenhum arquivo do core muda.

```go
// internal/runtime/providers/registry.go

type Factory func(cfg ProviderConfig) (agentloop.LLMProvider, error)

var registry = map[string]Factory{}

// Register wires a provider id to its factory. Adapters call this from init().
func Register(id string, f Factory) { registry[id] = f }
```

```go
// internal/runtime/providers/anthropic/anthropic.go
func init() { providers.Register("anthropic", New) }
```

O mesmo padrão vale para: adaptadores de [[Toolset (Go)]] (5 tipos), adaptadores de hook ([[Event Hooks (Go)]]), formatos de saída do CLI, e tipos de trigger de [[Routine (Go)]].

**Verificação:** um teste adiciona um provider fictício via `Register` e verifica que ele aparece em `models list` e é resolvível por um agente — sem tocar em nenhum arquivo existente.

## LSP — substituição sem surpresa

**Regra:** toda implementação de um port passa a **mesma suíte de contrato**. Se um `Repository` de teste e o `fscollections` divergem em comportamento observável, um dos dois está errado.

```go
// internal/domain/testsuite/repository.go

// RunRepositoryContract exercises the behavioural contract every Repository
// implementation must satisfy: create/read round-trip, not-found error identity,
// list filtering, update conflict detection, delete idempotence, and ordering.
func RunRepositoryContract(t *testing.T, newRepo func(t *testing.T) memory.Repository)
```

Usada por `fscollections` e pelo fake em memória. Ver [[Testes de Contrato de Port]].

Contratos cobertos: `Repository`, `LLMProvider`, `ToolsetAdapter`, `Queue`, `Index`, `Approver`.

**Verificação:** o pacote de cada adaptador tem um teste que chama a suíte. Um teste de meta-cobertura falha se existir implementação de port sem contrato associado.

## ISP — interfaces pequenas, no consumidor

**Regra:** nenhuma interface monolítica. O consumidor declara o mínimo de que precisa.

Contraste direto com o original, cuja `IAgentSandboxService` tem 11 métodos ([[Sandbox]]):

```go
// internal/runtime/toolexec/port.go
type FileReader interface {
	ReadFile(ctx context.Context, path string) ([]byte, error)
	Stat(ctx context.Context, path string) (FileInfo, error)
}
type FileWriter interface {
	WriteFile(ctx context.Context, path string, data []byte) error
	Mkdir(ctx context.Context, path string) error
}
type CommandRunner interface {
	Run(ctx context.Context, c Command) (Result, error)
}
type Globber interface {
	Glob(ctx context.Context, pattern string, opts GlobOptions) ([]string, error)
}
```

O `Sandbox` concreto implementa as quatro. A tool `Read` recebe só `FileReader` — **não pode** escrever porque não tem o método, não porque uma checagem em runtime a impede. A restrição é do compilador.

**Verificação:** `TestPortsAreSmall` — nenhuma interface em `port.go` com mais de 6 métodos sem comentário justificando.

## DIP — dependa de abstração

**Regra:** `internal/domain` define os ports; `internal/adapters` os implementa; a injeção acontece **só em `cmd/`**.

```go
// cmd/aosd/wire.go
repos := fscollections.New(root, collections.Registry())
idx   := bleveindex.Open(indexPath)
mem   := memory.NewService(repos.Memories(), idx, bus, clock, ids)
```

Nenhum `new` de adaptador dentro de `internal/domain` ou `internal/runtime`. Nenhum singleton global além dos três de processo declarados em [[Visão Geral Go]].

**Verificação:** `TestDependencyRule` ([[Hexagonal e Regra de Dependência]]) impede o import; revisão impede a construção.

## Onde os princípios se aplicam neste sistema

| Princípio | Aplicação concreta | Onde |
|---|---|---|
| SRP | Uma feature = uma fatia vertical; um serviço = um agregado | [[Hexagonal e Regra de Dependência]] |
| OCP | Factory de providers, adaptadores de toolset, triggers de rotina | [[Model Providers (Go)]] · [[Toolset (Go)]] |
| LSP | Suítes de contrato por port | [[Testes de Contrato de Port]] |
| ISP | `Sandbox` quebrado em quatro interfaces por consumidor | [[Sandbox (Go)]] · [[Tool Executor e Spillover]] |
| DIP | Ports no domínio, adaptadores fora, injeção em `cmd/` | [[Visão Geral Go]] |

## Decisões e divergências

> [!decision] Sem interfaces "por precaução"
> Interface só existe quando há **dois** implementadores reais ou quando o teste exige um fake. Criar interface de uma implementação é cerimônia, não desacoplamento. Exceção: ports que atravessam a fronteira hexagonal — esses existem por regra estrutural, mesmo com um único implementador.

> [!decision] `init()` para registro, com moderação
> Registro de provider por `init()` é o padrão idiomático (é como `database/sql` funciona) e mantém o OCP limpo. Mas `init()` esconde ordem de execução: restringimos a registro puro em mapa, sem I/O e sem dependência entre pacotes.

## Testes

- `TestPortsAreSmall` — limite de tamanho de interface
- `TestEveryPortHasContract` — meta-teste de cobertura de contrato
- `TestProviderRegistrationIsOpen` — provider novo sem tocar no core
- Heurística de tamanho de `commands.go` (SRP)

## Critério de pronto

- [ ] Toda implementação de port roda a suíte de contrato correspondente
- [ ] Nenhuma interface de port com mais de 6 métodos sem justificativa
- [ ] Nenhum `new` de adaptador fora de `cmd/`
- [ ] Teste de OCP de provider verde
