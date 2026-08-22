---
tags: [dominio, toolset, integracao, mcp]
aliases: [Toolset Go, Ferramentas Externas]
fase: 8
status: especificado
origem: "[[Toolset]]"
---

# Toolset (Go)

> Pai: [[Skill (Go)]] · Origem no original: [[Toolset]] · Fase: 8

## Objetivo

Conectar ferramentas **externas** ao agente. Uma [[Skill (Go)]] declara toolsets; o agente os alcança por uma indireção deliberada.

## Estado atual

Quatro dos cinco tipos conectam: `mcp-server::stdio`, `mcp-server::http`,
`rest-api` e `cli` (`internal/adapters/mcpclient`, `internal/adapters/openapiclient`
e `internal/adapters/cliclient`). `rest-api` segue o design abaixo — lê o
documento OpenAPI de `baseUrl` via `kin-openapi` e gera um tool por operação,
sem alargar a entidade `Toolset`. `custom` decodifica e lista mas recusa
`Call` como `TOOLSET_TYPE_NOT_AVAILABLE` — não tem um único adaptador por
definição.

`cli` publica um único tool, `run`, que executa `Command`+`Args` com
argumentos e stdin adicionais vindos da chamada. A decisão abaixo — duas
portas fechadas por padrão — está implementada assim: a porta do manifesto
(o `Command` declarado bate contra `permissions.exec` da skill) é checada uma
vez, na instalação (`internal/domain/skill.VerifyManifest`), e protegida
contra deriva depois disso — `toolset.Service.UpdateConfig` recusa mudar
`Command`/`BaseURL` num toolset que uma skill instalou. A porta do sandbox do
agente é checada em toda `Call`: `internal/runtime/session` anexa o sandbox
do turno ao `context.Context` via `internal/runtime/execguard` — um pacote à
parte, porque `internal/domain/toolset` não pode importar
`internal/runtime/sandbox` — e `cliclient` lê de lá antes de rodar qualquer
coisa, recusando quando não há sandbox anexado (uma chamada fora de um turno
de agente).

A allowlist de rede por `permissions.network` que a decisão abaixo declara
para `rest-api` e `mcp-server::http` continua sem um cliente HTTP que a
aplique por host — o que existe hoje é a mesma trava de `UpdateConfig` acima,
que impede o `BaseURL` de um toolset instalado por skill de mudar depois da
checagem de instalação, mas não impede um toolset configurado diretamente
por uma pessoa de apontar para qualquer host.

## Comportamento do original

Cinco tipos de conexão, união discriminada por `type` ([[Toolset]]):

| Tipo | Mecanismo |
|---|---|
| `mcp-server::stdio` | Spawna um servidor MCP de terceiro |
| `mcp-server::http` | Consome um servidor MCP remoto |
| `rest-api` | Lê spec OpenAPI e **gera tools automaticamente** |
| `cli` | Envolve um binário local |
| `custom` | Adaptador arbitrário da skill |

Todos suportam interpolação `${env.VAR}` em headers, env e argumentos — segredos ficam no `.env`, nunca no `SKILL.md` versionado.

A **indireção é o ponto**: as tools de um toolset não entram no registry do agente. Ele as executa via `toolsets_call`. Três vantagens registradas: o número de tools no contexto não cresce com o número de integrações; a descoberta é sob demanda; e a fronteira de execução externa fica auditável em **um único ponto**.

O sistema fica dos dois lados do MCP: **servidor** (expõe o domínio a agentes externos) e **cliente** (consome servidores de terceiros).

## Design em Go

```go
// internal/domain/toolset/entity.go

type Type string

const (
	MCPStdio Type = "mcp-server::stdio"
	MCPHTTP  Type = "mcp-server::http"
	RESTAPI  Type = "rest-api"
	CLI      Type = "cli"
	Custom   Type = "custom"
)

type Toolset struct {
	ID    string `yaml:"-" json:"id" collection:"path"`
	Skill string `yaml:"skill,omitempty" json:"skill,omitempty"`

	Name        string `yaml:"name"        json:"name"`
	Description string `yaml:"description" json:"description"`
	Type        Type   `yaml:"type"        json:"type"`

	Command string            `yaml:"command,omitempty" json:"command,omitempty"`
	Args    []string          `yaml:"args,omitempty"    json:"args,omitempty"`
	URL     string            `yaml:"url,omitempty"     json:"url,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"     json:"env,omitempty"` // supports ${env.VAR}

	Status Status `yaml:"status" json:"status"` // connected | disconnected | failed
}
```

### O port e os adaptadores

```go
// internal/domain/toolset/port.go

// Adapter is the strategy every connection type implements. Adding a sixth
// type means implementing this and registering a factory — nothing in the core
// changes (OCP).
type Adapter interface {
	Connect(ctx context.Context, t Toolset) error
	ListTools(ctx context.Context) ([]ToolSpec, error)
	Call(ctx context.Context, name string, in json.RawMessage) (any, error)
	Close() error
}
```

| Adaptador | Implementação |
|---|---|
| `mcp` | Cliente do SDK oficial, stdio e HTTP |
| `openapi` | `kin-openapi` lê a spec e gera um `ToolSpec` por operação |
| `cli` | Descobre subcomandos por `--help` ou por declaração explícita da skill |
| `custom` | Ponte para a declaração da skill |

### Interpolação de segredos

```go
// internal/domain/toolset/interpolate.go

// interpolate resolves ${env.NAME} against the workspace env layers. It fails
// loudly on a missing variable instead of substituting an empty string — a
// silent empty token produces a confusing 401 much later.
func interpolate(s string, env env.Resolver) (string, error)
```

### Execução por indireção

```go
type Service interface {
	List(ctx context.Context, q Query) ([]Toolset, error)

	// Get returns the toolset with its tool specs — this is the discovery step
	// the agent takes before calling.
	Get(ctx context.Context, id string) (*Detail, error)

	// Call executes one external tool. It is the single auditable boundary for
	// external execution in the whole system.
	Call(ctx context.Context, in CallInput) (any, error)
}
```

Toda chamada registra em [[Activity (Go)]]: toolset, tool, duração, sucesso ou falha. Não o payload — que pode conter dados sensíveis do usuário.

## Decisões e divergências

> [!decision] Toolset de tipo `cli` respeita a allowlist do sandbox
> No original, um toolset `cli` executa binário local sem passar pelo sandbox do agente. Aqui, o binário precisa estar na allowlist do agente **e** ter sido declarado no manifesto da skill ([[ADR-0015 Skills com permissões declaradas]]). Duas portas, ambas fechadas por padrão.

> [!decision] Restrição de rede por manifesto
> Toolsets `rest-api` e `mcp-server::http` só alcançam hosts declarados em `permissions.network`. Um cliente HTTP com allowlist de host aplica isso.

> [!decision] Falha de interpolação é erro, não string vazia
> Segredo ausente falha na conexão, com CTA nomeando a variável.

> [!decision] Indireção mantida
> Não expomos as tools de toolset diretamente no registry do agente, mesmo sendo tecnicamente possível. As três razões do original continuam válidas — e a auditoria em ponto único é a mais importante.

## Testes

- União discriminada: os cinco tipos decodificam; tipo desconhecido é rejeitado
- Interpolação com variável ausente falha nomeando-a
- Adaptador OpenAPI gera um `ToolSpec` por operação, com schema derivado
- Adaptador MCP stdio conecta a um servidor de teste e lista tools
- Toolset `cli` com binário fora da allowlist é recusado
- Host fora de `permissions.network` é bloqueado
- `toolsets_call` registra em Activity sem gravar o payload
- Contrato de `Adapter` roda contra os quatro adaptadores ([[Testes de Contrato de Port]])

## Critério de pronto

- [ ] Cinco tipos implementados
- [ ] Interpolação de segredos com falha explícita
- [ ] Restrições de exec e de rede aplicadas
- [ ] Auditoria em ponto único
- [ ] Suíte de contrato verde
