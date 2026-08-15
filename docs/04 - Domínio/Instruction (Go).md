---
tags: [dominio, instruction, politica]
aliases: [Instruction Go, Instrução]
fase: 8
status: especificado
origem: "[[Instruction]]"
---

# Instruction (Go)

> Pai: [[Workspace (Go)]] · Origem no original: [[Instruction]] · Ver: [[Memory (Go)]] · Fase: 8

## Objetivo

Regras comportamentais **duráveis e de escopo workspace**, que moldam TODOS os agentes.

## Comportamento do original

A distinção com [[Memory (Go)]] é o ponto central, e o prompt-mestre a martela ([[Instruction]]):

> *"Memory vs instruction — memory is YOURS (your personal behavior); instruction is EVERYONE'S (workspace-wide policy). Personal correction → memory. Workspace-wide correction → instruction."*

E na hierarquia de prioridade: *"Your memories and preferences never override a workspace-wide instruction."*

Isso tem consequência direta na montagem de contexto: instruções recebem `trust="trusted"` (autoridade máxima); memórias recebem `trust="observed"` ([[Prompt Assembly]]).

O campo `paths` faz por instruções o que `scopes` faz por memórias: aplica a regra só a determinados arquivos.

## Design em Go

```go
// internal/domain/instruction/entity.go

type Instruction struct {
	ID string `yaml:"-" json:"id" collection:"path"`

	Name        string `yaml:"name"        json:"name"`
	Type        string `yaml:"type"        json:"type"` // standards | patterns | workflows | free-form
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Skill       string `yaml:"skill,omitempty"       json:"skill,omitempty"`

	// Paths scopes the rule to matching files, like Memory.Scopes.
	Paths []string `yaml:"paths,omitempty" json:"paths,omitempty"`

	Active bool `yaml:"active" json:"active"`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt"`

	Content string `yaml:"-" json:"content" collection:"content"`
}
```

```go
type Service interface {
	List(ctx context.Context, q Query) ([]Instruction, error)
	Get(ctx context.Context, id string) (*Instruction, error)
	Create(ctx context.Context, in CreateInput) (*Instruction, error)
	Update(ctx context.Context, in UpdateInput) (*Instruction, error)
	Delete(ctx context.Context, id string) error

	// Applicable returns active instructions whose Paths match the files in
	// play — or that have no Paths at all, which means workspace-wide.
	Applicable(ctx context.Context, paths []string) ([]Instruction, error)
}
```

### No contexto

Apenas os **nomes** entram no inventário, com a orientação pedagógica do original:

> *"Workspace-wide behavioral rules that shape ALL agents. Follow all active instructions during every task and conversation — they apply to you. When the user establishes a policy that should apply workspace-wide, create or update an instruction, not a memory."*

O conteúdo é buscado sob demanda, como todo o resto ([[Prompt Assembly]]).

## Decisões e divergências

> [!decision] Instrução criada por agente exige confirmação
> Adição. Uma instrução é política que afeta todos os agentes — é exatamente o "shared state or policy change" que a matriz de autonomia do prompt classifica como **consultivo**. `instructions_create` e `instructions_update` roteiam por [[ADR-0007 Canal real de aprovação de tool]] com risco médio. O original permite criação silenciosa.

> [!decision] `Applicable` com semântica de globs vazios
> Instrução sem `paths` aplica-se ao workspace inteiro. É o default e o caso comum.

## Testes

- Round-trip completo
- `Applicable` com `paths` casando e não casando
- Instrução sem `paths` sempre aplicável
- Instrução inativa nunca retorna
- Criação por agente dispara aprovação
- Instrução instalada por skill é removida com ela
- No prompt, bloco de instruções sai com `trust="trusted"`

## Critério de pronto

- [ ] CRUD completo
- [ ] `Applicable` por globs funcionando
- [ ] Bloco no prompt com autoridade máxima
- [ ] Criação por agente sob aprovação
