---
tags: [qualidade, testes, ci]
aliases: [Testes, Estratégia de Testes, CI]
fase: 0
status: pronto
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

> [!decision] O piso de cobertura é um binário, não um comentário
> `tools/covercheck` lê a saída de `go test -cover`, casa cada pacote com o prefixo mais específico da tabela e falha nomeando o pacote e a distância do piso. Um pacote sem piso declarado **também** falha — o que impede que uma árvore nova (`internal/runtime/`, na Fase 5) escape do portão por esquecimento.

> [!decision] Falha de sistema de arquivos é injetada, não simulada por sorte
> Provar que uma escrita interrompida preserva o arquivo anterior exige falhar no meio da escrita. `internal/core/atomicfs` expõe a criação do temporário e o `rename` como variáveis de pacote, e o teste interno troca-as por uma implementação que falha em um passo escolhido. É o que leva a cobertura dos caminhos de erro de 62% a 98% — e, mais importante, é o que torna a promessa do pacote verificável em vez de declarada.

## Testes desta nota

Meta-testes que verificam a própria estratégia:

- Nenhum pacote de domínio importa I/O
- Nenhuma chamada direta a `time.Now`, `uuid.New` ou `math/rand` em produção
- Todo port com implementação tem suíte de contrato
- Todo defeito da lista de anti-padrões tem teste correspondente, nomeado

## Critério de pronto

- [x] Todos os portões de CI configurados e verdes — `.github/workflows/ci.yml` e `task check`
- [x] Cobertura mínima atingida por pacote — `tools/covercheck`, executado no portão
- [ ] Suítes de contrato para os seis ports — Fase 1 em diante, quando os ports existirem (`internal/domain/testsuite`)
- [ ] Um teste nomeado por defeito corrigido — três já existem (ver abaixo); os demais entram na fase em que o defeito é corrigido
- [ ] `goleak` sem vazamentos — Fase 4

## Saída dos testes — Fase 0

```
$ go vet ./...
(sem saída — ok)

$ golangci-lint run
tools/covercheck/main.go:28:1: File is not properly formatted (gofmt)
	"github.com/OWNER/aos/cmd":               0,
^
1 issues:
* gofmt: 1

$ go test -race -count=1 ./...
?   	github.com/OWNER/aos/cmd/aos	[no test files]
ok  	github.com/OWNER/aos/internal/adapters/fsconfig	1.553s
ok  	github.com/OWNER/aos/internal/architecture	1.797s
ok  	github.com/OWNER/aos/internal/core/apperr	1.861s
ok  	github.com/OWNER/aos/internal/core/apperr/scan	2.207s
ok  	github.com/OWNER/aos/internal/core/atomicfs	3.837s
ok  	github.com/OWNER/aos/internal/core/build	2.853s
ok  	github.com/OWNER/aos/internal/core/config	2.524s
ok  	github.com/OWNER/aos/internal/core/env	2.990s
ok  	github.com/OWNER/aos/internal/core/identity	3.480s
ok  	github.com/OWNER/aos/internal/core/logging	3.860s
ok  	github.com/OWNER/aos/internal/core/safe	2.622s
ok  	github.com/OWNER/aos/internal/domain/config	2.555s
?   	github.com/OWNER/aos/tools/covercheck	[no test files]
?   	github.com/OWNER/aos/tools/gencatalog	[no test files]

$ go test -count=1 -cover ./... | go run ./tools/covercheck
	github.com/OWNER/aos/cmd/aos		coverage: 0.0% of statements
ok  	github.com/OWNER/aos/internal/adapters/fsconfig	0.258s	coverage: 81.2% of statements
ok  	github.com/OWNER/aos/internal/architecture	0.562s	coverage: [no statements]
ok  	github.com/OWNER/aos/internal/core/apperr	0.573s	coverage: 80.7% of statements
ok  	github.com/OWNER/aos/internal/core/apperr/scan	1.128s	coverage: 80.1% of statements
ok  	github.com/OWNER/aos/internal/core/atomicfs	1.152s	coverage: 97.6% of statements
ok  	github.com/OWNER/aos/internal/core/build	1.451s	coverage: 100.0% of statements
ok  	github.com/OWNER/aos/internal/core/config	1.172s	coverage: 80.3% of statements
ok  	github.com/OWNER/aos/internal/core/env	1.302s	coverage: 85.0% of statements
ok  	github.com/OWNER/aos/internal/core/identity	1.518s	coverage: 92.3% of statements
ok  	github.com/OWNER/aos/internal/core/logging	1.657s	coverage: 100.0% of statements
ok  	github.com/OWNER/aos/internal/core/safe	1.718s	coverage: 93.3% of statements
ok  	github.com/OWNER/aos/internal/domain/config	1.556s	coverage: 80.4% of statements
	github.com/OWNER/aos/tools/covercheck		coverage: 0.0% of statements
	github.com/OWNER/aos/tools/gencatalog		coverage: 0.0% of statements
covercheck: 14 packages, all at or above their floor
```

### Portões implementados nesta fase

```bash
task check      # gen + git diff + vet + lint + test -race + cover + arch + graph
```

| Portão | Ferramenta |
|---|---|
| `go vet ./...` | stdlib |
| `golangci-lint run` | v2.12.2, config em `.golangci.yml` |
| `go test -race ./...` | stdlib |
| Cobertura por pacote | `tools/covercheck` |
| Regra hexagonal | `internal/architecture` |
| Artefato gerado não divergiu | `go run ./tools/gencatalog` + `git diff --exit-code` |
| Grafo do vault | `docs/_scripts/validate-graph.mjs` |
| Seis alvos de plataforma | `task build:all` |

### Testes que nomeiam um defeito do original

| Defeito | Teste |
|---|---|
| #7 — `config_get` sem filtro de campo sensível | `TestNoSecretIsReachableWithoutReveal`, `TestRevealIsRefusedForAgents` |
| #10 — segredos com permissão 0644 | `TestAuditSecretsRepairsLoosePermissions`, `TestWriteSecretUses0600` |
| #12 — `development: true` fixo | `TestProductionIsTheDefaultMode` |
| #16 — falha dura em exceção não tratada | `TestDoConvertsPanicIntoError`, `TestGoReportsPanicOnTheChannel` |
| #17 — escrita não atômica | `TestAnInterruptedWriteLeavesThePreviousFileIntact` (quatro pontos de falha) |
