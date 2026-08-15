---
tags: [dominio, agente, core]
aliases: [Agent Go, Agente]
fase: 3
status: pronto
origem: "[[Agent]]"
---

# Agent (Go)

> Pai: [[Workspace (Go)]] · Origem no original: [[Agent]] · Runtime: [[Agent Loop]] · Fase: 3

## Objetivo

A entidade central: uma **persona persistente** com identidade, papel, modelo, memória própria e canais de comunicação.

## Comportamento do original

Persistido em `.fractal/agents/{id}/agent.md`, com subdiretórios `memories/`, `routines/` e `events/`. O segundo padrão da coleção (`.fractal/skills/{skill}/agents/{id}/agent.md`) permite que uma [[Skill (Go)]] traga agentes próprios ([[Agent]]).

Campos com semântica não óbvia:

- **`description`** não é documentação humana: o describe do Zod diz *"Instructions for the orchestrator to know when this agent should be called."* É o critério de roteamento.
- **`leader`** cria organograma — um agente reporta a outro.
- **`orchestrator`** marca o fallback do workspace para chats sem menção explícita, e determina qual diretiva vai no prompt.
- **`model` no formato `"{model} ({provider})"`** existe para contornar validação de frontmatter do VS Code.
- **`voice`** dá identidade sonora por agente.

Um agente pode **criar outros agentes** (`CreateAgent` está no registry nativo), o que, com o `ORCHESTRATOR_DIRECTIVE`, torna a equipe auto-expansível.

## Design em Go

```go
// internal/domain/agent/entity.go

type Agent struct {
	ID string `yaml:"-" json:"id" collection:"path"`

	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"` // routing criterion for the orchestrator
	Role        string `yaml:"role,omitempty"        json:"role,omitempty"`
	Skill       string `yaml:"skill,omitempty"       json:"skill,omitempty"`
	Leader      string `yaml:"leader,omitempty"      json:"leader,omitempty"` // org chart

	Provider string `yaml:"provider,omitempty" json:"provider,omitempty"`
	Model    string `yaml:"model,omitempty"    json:"model,omitempty"` // accepts "{model} ({provider})"
	Voice    string `yaml:"voice,omitempty"    json:"voice,omitempty"`

	Orchestrator bool `yaml:"orchestrator" json:"orchestrator"`

	Channels []Channel      `yaml:"channels,omitempty" json:"channels,omitempty"`
	Sandbox  *SandboxPolicy `yaml:"sandbox,omitempty"  json:"sandbox,omitempty"` // added; see ADR-0006

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt"`

	Content string `yaml:"-" json:"content" collection:"content"` // Markdown body = system instructions
}

type Channel struct {
	Provider string         `yaml:"provider" json:"provider"` // "telegram"
	Data     map[string]any `yaml:"data"     json:"data"`
}
```

```go
// internal/domain/agent/service.go

type Service interface {
	List(ctx context.Context, q Query) ([]Agent, error)
	Get(ctx context.Context, id string) (*Agent, error)

	// Me resolves the caller's identity: inside an agent execution context it
	// returns that agent; from a human terminal it returns the orchestrator.
	Me(ctx context.Context) (*Agent, error)

	Create(ctx context.Context, in CreateInput) (*Agent, error)
	Update(ctx context.Context, in UpdateInput) (*Agent, error)
	Delete(ctx context.Context, id string) error
}
```

`Me` é como um agente externo (Claude Code, Cursor) descobre **quem ele é** dentro do workspace. A identidade vem do `AgentID` no contexto ambiente ([[Concorrência e Context]]).

### Invariantes

```go
// Invariants enforced by the service, not by the schema:
//  1. At most one orchestrator per workspace. Promoting a second one demotes
//     the first and records the change in Activity.
//  2. The leader chain must be acyclic — a cycle is rejected on write.
//  3. IDs are lowercase slugs derived from the name, normalized in OnCreated.
//  4. Deleting an agent cascades its memories, routines and events directory.
```

### Comandos

| Comando | Tool | Efeito |
|---|---|---|
| `agents list` | `agents_list` | Roster da equipe |
| `agents get <id>` | `agents_get` | Configuração completa |
| `agents me` | `agents_me` | Resolve a própria identidade |
| `agents create` | `agents_create` | Registra novo agente |
| `agents update` | `agents_update` | Atualização parcial |
| `agents delete` | `agents_delete` | Remove, com cascata |

## Decisões e divergências

> [!decision] `agents execute` (isolate V8) não é portado
> O original executa JavaScript/TS em isolate seguro via `meriyah` + `@secure-exec/core`, disponível só no CLI. Em Go não há equivalente direto sem embutir um runtime JS — o que contraria [[ADR-0001 Go como linguagem]]. A capacidade é coberta pelo `Bash` sob [[Sandbox (Go)]] com allowlist. Registrado como escopo cortado, não esquecido.

> [!decision] Política de sandbox no frontmatter do agente
> Campo novo. É onde a allowlist de [[ADR-0006 Allowlist no sandbox]] vive, versionada e revisável.

> [!decision] Formato `"{model} ({provider})"` mantido
> Existe por um motivo de ferramenta externa (validação de frontmatter do VS Code) que continua valendo. Parseado em [[Agent Loop]].

> [!decision] Um orquestrador por workspace, aplicado
> O original diz "deve haver no máximo um" sem impor. Aqui é invariante de serviço: promover o segundo rebaixa o primeiro, na mesma chamada. Sem isso, o roteamento de todo chat sem menção explícita depende de qual arquivo a listagem do diretório devolve primeiro.
>
> O registro em [[Activity (Go)]] fica para a **Fase 6**, quando a atividade existir. Hoje o rebaixamento acontece e é observável no registro do agente rebaixado; o que falta é a trilha.

> [!decision] Cadeia de `leader` verificada na escrita, com limite de profundidade
> Um ciclo no organograma não é cosmético: o [[Prompt Assembly]] caminha a cadeia para dizer a quem o agente reporta, e a delegação caminha para decidir escalonamento. O erro nomeia o laço encontrado.
>
> O limite de 32 níveis é a outra metade: o teste de ciclo terminaria sozinho, mas uma cadeia longa o bastante transforma uma escrita em cem leituras de arquivo — negação de serviço escrita como estrutura de dados.

> [!decision] Um líder que ainda não existe é aceito
> Equipes são montadas na ordem em que a pessoa pensa nelas, e uma referência pendente não fecha laço nenhum. Ela é visível no roster.

## Testes

- Round-trip com todos os campos, incluindo `channels` aninhado
- `Me` com `AgentID` no contexto devolve aquele agente; sem, devolve o orquestrador
- Promover segundo orquestrador rebaixa o primeiro e registra em [[Activity (Go)]]
- Cadeia de `leader` cíclica é rejeitada
- ID normalizado para lowercase no create
- Delete cascateia memórias, rotinas e eventos
- Padrão skill-scoped: agente sob `.aos/skills/x/agents/y/agent.md` é listado com `skill: x`
- Diretiva correta no prompt conforme `orchestrator`

## Critério de pronto

- [x] CRUD completo com cascata — a cascata é declaração do modelo (`CascadeDelete`), verificada em [[Collections Engine]]
- [x] `agents me` resolvendo identidade nas três origens — `TestMeResolvesTheCallingAgent`, `TestMeFromATerminalResolvesTheOrchestrator`
- [x] Invariante de orquestrador único — `TestPromotingASecondOrchestratorDemotesTheFirst`
- [ ] Política de sandbox lida e aplicada por [[Sandbox (Go)]] — **Fase 5**

## Saída dos testes — Fase 3

```
$ go test -race ./internal/domain/agent/
ok  	github.com/OWNER/aos/internal/domain/agent
```

| Caso da nota | Teste |
|---|---|
| Round-trip com todos os campos, inclusive `channels` | `TestRoundTripsEveryField` (em `fscollections`) |
| `Me` com e sem `AgentID` no contexto | `TestMeResolvesTheCallingAgent`, `TestMeFromATerminalResolvesTheOrchestrator` |
| Promover segundo orquestrador rebaixa o primeiro | `TestPromotingASecondOrchestratorDemotesTheFirst`, `TestCreatingASecondOrchestratorAlsoDemotes` |
| Cadeia de `leader` cíclica é rejeitada | `TestALeaderCycleIsRejectedOnWrite`, `TestAnAgentCannotLeadItself` |
| ID normalizado para lowercase no create | `TestSlugIsLowercased` |
| Padrão skill-scoped listado com `skill: x` | `TestSkillScopedRecordsAreReadOnly` (em `collections`) |

**Não verificado:** a diretiva de prompt conforme `orchestrator` — o [[Prompt Assembly]] é da Fase 5. `agents execute` continua fora de escopo, como registrado acima.

## Adições da Fase 5

O frontmatter ganhou dois campos que o runtime lê a cada turno.

```yaml
reasoning: high          # none | low | medium | high — default vem da config
sandbox:
  permissions: [read, write, execute]
  exec:
    policy: allowlist
    allow: [git, go, task]
    denyArgs: ["git push --force*"]
    allowShell: false
```

Ausente o bloco `sandbox`: `permissions: [read]`, `policy: deny-all`. Mais restritivo que o original, e deliberado — ver [[ADR-0006 Allowlist no sandbox]]. O `reasoning` por agente é divergência registrada em [[Model Providers (Go)]]: um revisor crítico e um triador não deveriam pensar igual.

`UpdateInput` substitui o bloco inteiro em vez de mesclar. Uma permissão que alguém quis remover precisa sair de fato, e mesclagem é como uma remoção não faz nada em silêncio.
