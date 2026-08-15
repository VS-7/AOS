---
tags: [arquitetura, hexagonal, ports, dependencias]
aliases: [Hexagonal, Ports and Adapters, Regra de Dependência]
fase: 0
status: especificado
origem: "[[Padrão Feature-Slice]]"
---

# Hexagonal e Regra de Dependência

> Pai: [[AOS]] · Origem no original: [[Padrão Feature-Slice]] · Fase: 0

## Objetivo

Fixar a única regra estrutural inviolável do projeto e o mecanismo que a verifica automaticamente. Sem isso, a Command Layer degenera: transporte começa a carregar regra de negócio e as cinco superfícies deixam de ser projeções da mesma verdade.

## Comportamento do original

O original usa **feature-slice vertical** com rigor quase mecânico nas 26 features: cada uma tem `schemas/`, `interfaces/`, `errors/`, `collections/`, `services/`, `controllers/`, `procedures/`, `commands/`, `notifications/`, `queues/` ([[Padrão Feature-Slice]]).

A separação é boa, mas a direção da dependência não é verificada. A engenharia reversa registra o vazamento: *"Se um arquivo de `@core` importa uma feature, é sinal de acoplamento indevido (acontece pontualmente em `collections.ts`, que importa as definições de coleção)."*

Vazamento pontual em TypeScript é aceitável porque nada quebra. Em Go, com um teste, ele simplesmente não acontece.

## Design em Go

Mantemos a fatia vertical **e** adicionamos a regra de direção.

```
                    ┌─── ADAPTADORES DE ENTRADA ───┐
       ┌────────────┬──────────┬─────────┬─────────┴───┐
       │ HTTP (chi) │   CLI    │   MCP   │    Wails    │
       └─────┬──────┴────┬─────┴────┬────┴──────┬──────┘
             ▼           ▼          ▼           ▼
       ┌──────────────────────────────────────────────┐
       │      internal/core/command  (fonte única)    │
       └──────────────────┬───────────────────────────┘
                          ▼
       ┌──────────────────────────────────────────────┐
       │            internal/domain  (puro)           │
       │  Sem import de chi, cobra, mcp, wails, bleve │
       └──────────────────┬───────────────────────────┘
                          ▼
       ┌──────────────────────────────────────────────┐
       │       PORTS  (definidos pelo consumidor)     │
       │  Repository · Bus · LLMProvider · Sandbox    │
       │  Clock · IDGen · Approver · Index · Queue    │
       └──────────────────┬───────────────────────────┘
                          ▼
       ┌──────────────────────────────────────────────┐
       │           internal/adapters                  │
       │  fscollections · sqlitequeue · bleveindex    │
       │  openai · anthropic · google · telegram      │
       └──────────────────────────────────────────────┘
```

### A regra

> **`internal/domain` não importa nada de `internal/transport`, `internal/adapters` nem bibliotecas de terceiros ligadas a I/O.**

Concretamente, a lista de pacotes proibidos dentro de `internal/domain`:

```
net/http · os/exec · database/sql
github.com/go-chi/... · github.com/spf13/... · github.com/wailsapp/...
github.com/modelcontextprotocol/... · github.com/coder/websocket
modernc.org/sqlite · github.com/blevesearch/... · github.com/robfig/cron/...
```

`os` é permitido apenas para tipos (`os.FileMode`), nunca para chamadas de filesystem — verificado por revisão, não por lista.

### Ports definidos no consumidor

Idioma Go: *accept interfaces, return structs*. A interface vive onde é **usada**, não onde é implementada:

```go
// internal/domain/memory/port.go — o que memory precisa, declarado por memory
package memory

type Repository interface {
	Get(ctx context.Context, agentID, id string) (*Memory, error)
	List(ctx context.Context, q Query) ([]Memory, error)
	Create(ctx context.Context, m *Memory) error
	Update(ctx context.Context, m *Memory) error
}

type Searcher interface {
	Search(ctx context.Context, q SearchQuery) ([]Hit, error)
}

type Clock interface{ Now() time.Time }
type IDGen interface{ New() string }
```

`internal/adapters/fscollections` implementa `memory.Repository` **sem importar `memory`** — satisfaz a interface estruturalmente, via um tipo genérico `Repo[T]`. A ligação acontece em `cmd/`.

### Interface segregada, não monólito

O port `Sandbox` do original é uma interface única de 11 métodos ([[Sandbox]]). Aqui é quebrado por consumidor:

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
	Run(ctx context.Context, cmd Command) (Result, error)
}
```

A tool `Read` recebe um `FileReader`. Ela não pode escrever nem executar porque **não tem como** — a restrição é de tipo, não de checagem em runtime. Ver [[SOLID no Go]], ISP.

### Verificação automática

```go
// internal/architecture/dependency_test.go

var forbidden = map[string][]string{
	"github.com/OWNER/aos/internal/domain": {
		"net/http", "os/exec", "database/sql",
		"github.com/go-chi/", "github.com/spf13/",
		"github.com/wailsapp/", "github.com/modelcontextprotocol/",
		"modernc.org/sqlite", "github.com/blevesearch/",
		"github.com/OWNER/aos/internal/transport",
		"github.com/OWNER/aos/internal/adapters",
	},
	"github.com/OWNER/aos/internal/core/command": {
		"github.com/OWNER/aos/internal/domain",
	},
}

// TestDependencyRule walks every package under the guarded prefixes with
// packages.Load and fails listing each violating import path, so the error
// message tells the developer exactly which file to fix.
func TestDependencyRule(t *testing.T) { /* ... */ }
```

Roda em `go test ./...` — é portão de CI, não documentação ([[Estratégia de Testes]]).

> [!decision] `core/command` não importa `domain`
> Isso soa contraintuitivo: a Command Layer é o ponto de entrada dos serviços de domínio. A resolução é que o **registry recebe** comandos já construídos, via `command.Register(desc Descriptor)`. Quem os constrói é `internal/domain/{feature}/commands.go`, que importa `core/command`. A seta aponta para dentro: domínio conhece a Command Layer, a Command Layer não conhece o domínio.
>
> Consequência prática: um comando novo nunca exige tocar em `core/command`. Ver [[Command Layer]].

### A fatia vertical

Cada feature de domínio tem a mesma forma, herdada do original:

```
internal/domain/memory/
├── entity.go     # Memory, Category, Status — tipos puros
├── service.go    # regra de negócio; recebe ports no construtor
├── port.go       # interfaces que este pacote consome
├── schema.go     # tipos de entrada/saída com tags json + jsonschema
├── commands.go   # os Command[In,Out] do grupo
├── errors.go     # construtores de erro do domínio
└── memory_test.go
```

Um serviço governa **um agregado**. `MemoryService` não escreve tasks.

## Decisões e divergências

> [!decision] Ports no consumidor, não no provedor
> O original declara interfaces em `interfaces/{feature}.interfaces.ts`, do lado de quem implementa. Aqui elas vivem no pacote que consome. O ganho é composição: um serviço que só lê declara uma interface de leitura, e qualquer implementação a satisfaz sem saber que ela existe.

> [!decision] A regra é teste, não convenção
> Convenção arquitetural sem verificação vira exceção documentada em três meses. O teste custa 80 linhas.

## Testes

- **`TestDependencyRule`** — a verificação acima, com mensagem que nomeia arquivo e import.
- **`TestNoDomainInClients`** — `go list -deps ./cmd/aos` não contém domínio, exceto `gateway`.
- **`TestPortsAreSmall`** — heurística: nenhuma interface em `port.go` com mais de 6 métodos sem comentário justificando.
- **Testes de domínio sem I/O** — todo teste em `internal/domain` roda com fakes; nenhum toca disco ou rede. Verificado por ausência de `t.TempDir()` nesses pacotes.

## Critério de pronto

- [ ] `TestDependencyRule` verde e no portão de CI
- [ ] Toda feature de domínio segue o esqueleto de 7 arquivos
- [ ] Nenhuma interface de port com mais de 6 métodos sem justificativa escrita
- [ ] Testes de `internal/domain` rodam sem tocar disco nem rede
