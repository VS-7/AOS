---
tags: [qualidade, testes, ci]
aliases: [Testes, Estratégia de Testes, CI]
fase: 0
status: especificado
origem: "[[PROMPT — Reconstrução em Go]]"
---

# Estratégia de Testes

> Pai: [[AOS]] · Fase: 0

## Objetivo

Definir o que se testa, como, e quais portões impedem uma fase de ser declarada pronta. A regra do projeto é dura: **nenhuma fase é concluída sem teste executado e saída anexada**.

## Design

### As cinco camadas

| Tipo | Alvo | Ferramenta | Onde |
|---|---|---|---|
| Unitário | Regra de domínio pura | stdlib + testify | `internal/domain/**` |
| Contrato de port | Toda implementação de um port roda a mesma suíte | suíte compartilhada | `internal/domain/testsuite` |
| Golden file | Prompt montado, `SKILL.md`, saída de CLI, erros | `testdata/` | vários |
| Integração | Round-trip no filesystem real | `t.TempDir()` | `internal/adapters/**` |
| E2E | CLI → HTTP → domínio → disco | binário compilado | `test/e2e` |

### Testes de domínio não tocam I/O

```go
// internal/domain/memory/memory_test.go

// Domain tests run entirely on fakes: an in-memory repository, a fixed clock,
// a deterministic id generator. They are fast enough to run on every save and
// they fail for exactly one reason — the rule under test.
func newTestService(t *testing.T) (*Service, *fakes.Repo) {
	repo := fakes.NewRepo()
	return NewService(repo, fakes.Index{}, fakes.Bus{}, clockx.Fixed(refTime), idgen.Seq()), repo
}
```

Verificado mecanicamente: nenhum pacote sob `internal/domain` usa `t.TempDir()`, `net/http` ou `os.Open`.

### Determinismo

Três fontes de não-determinismo, todas injetadas:

```go
type Clock interface{ Now() time.Time }
type IDGen interface{ New() string }
type Rand  interface{ Intn(n int) int }
```

Nenhuma chamada direta a `time.Now()`, `uuid.New()` ou `math/rand` em código de produção — verificado por linter customizado. Sem isso, golden files não são estáveis.

### Provider fake para o loop

```go
// internal/runtime/agentloop/agentlooptest/fake.go

// Script drives the loop deterministically: each entry is what the model
// "returns" on that step. It makes the entire agent runtime testable in
// milliseconds, without a network call or an API key.
type Script []Turn

type Turn struct {
	Text      string
	ToolCalls []ToolCall
}
```

Base de quase todo teste de [[Agent Loop]], [[Tool Executor e Spillover]] e [[Subconsciente (Go)]].

### Cobertura

| Pacote | Mínimo |
|---|---|
| `internal/domain/**` | 80% |
| `internal/core/**` | 80% |
| `internal/runtime/**` | 75% |
| `internal/transport/**` | 60% |
| `internal/adapters/**` | contrato obrigatório, cobertura livre |

O portão verifica por pacote, não a média global — uma média alta esconde um pacote crítico sem teste.

### Portões de CI

```bash
go vet ./...
golangci-lint run
go test -race ./...
go test -run TestDependencyRule ./internal/architecture   # regra hexagonal
task gen-skill  && git diff --exit-code pkg/skill/        # SKILL.md não divergiu
task gen-schema && git diff --exit-code frontend/src/lib/schema.ts
task gen-components && git diff --exit-code internal/domain/view/components.gen.go
task build:all                                             # 6 alvos de plataforma
node docs/_scripts/validate-graph.mjs                      # vault sem links quebrados
```

Os três `git diff --exit-code` são o mecanismo que impede documentação e tipos de mentirem: se o gerado difere do commitado, o build falha.

### Testes que documentam defeitos corrigidos

Cada defeito da lista de anti-padrões tem um teste que o nomeia:

```go
// TestSandboxRejectsOriginalBypasses walks the exact bypasses documented in the
// reverse engineering of the original's blocklist. Each case cites what it
// covers, so a future refactor cannot quietly reopen one of them.
func TestSandboxRejectsOriginalBypasses(t *testing.T) {
	cases := []struct{ name, cmd string; args []string }{
		{"defect-6/bash-c-rm", "bash", []string{"-c", "rm -rf /"}},
		{"defect-6/sh-c-dd", "/bin/sh", []string{"-c", "dd if=/dev/zero of=/x"}},
		{"defect-6/python-rmtree", "python", []string{"-c", "import shutil; shutil.rmtree('/')"}},
		{"defect-6/find-delete", "find", []string{".", "-delete"}},
		{"defect-6/node-rmsync", "node", []string{"-e", "require('fs').rmSync('/')"}},
		{"defect-6/git-clean", "git", []string{"clean", "-fdx"}},
	}
	// ...
}
```

Ver [[Segurança e Hardening]] para a matriz completa defeito → teste.

## Decisões e divergências

> [!decision] `-race` sempre
> Não é modo opcional. O sistema tem 20 workers, N instâncias paralelas do mesmo agente e um watcher de filesystem. Um teste que só passa sem `-race` está errado.

> [!decision] Cobertura por pacote, não global
> Média global permite que um pacote crítico fique sem teste enquanto o número parece bom.

> [!decision] Geração verificada por `git diff`
> É o que torna "a documentação não diverge" uma garantia e não uma intenção.

> [!decision] `goleak` em todo pacote com goroutine
> Vazamento de goroutine em daemon de longa duração aparece semanas depois, em produção, como consumo crescente de memória.

## Testes desta nota

Meta-testes que verificam a própria estratégia:

- Nenhum pacote de domínio importa I/O
- Nenhuma chamada direta a `time.Now`, `uuid.New` ou `math/rand` em produção
- Todo port com implementação tem suíte de contrato
- Todo defeito da lista de anti-padrões tem teste correspondente, nomeado

## Critério de pronto

- [ ] Todos os portões de CI configurados e verdes
- [ ] Cobertura mínima atingida por pacote
- [ ] Suítes de contrato para os seis ports
- [ ] Um teste nomeado por defeito corrigido
- [ ] `goleak` sem vazamentos
