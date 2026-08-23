---
tags: [skill, documentacao, llm, geracao]
aliases: [Especificação da Skill, Skill Publicada]
fase: 9
status: em-construcao
origem: "[[Skill]]"
---

# Especificação da Skill

> Pai: [[AOS]] · Origem no original: [[Skill]] · [[Ponte CLI para MCP]] · Fase: 9

**Núcleo entregue.** `pkg/skill` (sem dependência do projeto, como esta nota
pede) gera `SKILL.md` + 26 `references/*.md` a partir do registry real —
`tools/genskill`, gate de CI (`task gen-skill && git diff --exit-code
pkg/skill/`), geração determinística testada. `Sync` copia para diretórios
de agente sem sobrescrever skill de terceiro com o mesmo nome, testado.

**Divergência conhecida, não escondida:** a estrutura de seção obrigatória
existe (`skill.RequiredSections`, `skill.MissingSections`) mas **não
bloqueia** a geração — verificado contra o registry real: **0 dos 26 grupos**
têm hoje as cinco seções completas (os `Doc` existentes são prosa útil, só
não neste formato). Impor isso agora significaria reescrever a documentação
voltada a LLM de 26 domínios numa única passada em vez de revisar cada um
pelo que ele realmente precisa dizer — fora do escopo de um gerador.
`genskill` reporta a lacuna a cada geração (stderr), não a esconde.

## Objetivo

Gerar e publicar uma Skill que ensina **qualquer agente** a operar este sistema — a partir do registry de comandos, nunca escrita à mão.

## Comportamento do original

O sistema se auto-documenta como skill ([[Skill]]): o workspace observado tinha a skill `fractal` instalada, com `SKILL.md` e **22 arquivos** em `references/`, um por grupo de comando, gerados pelo `incur` a partir das próprias definições.

> O sistema publica sua documentação de uso **no formato que seus agentes consomem**. É a mesma fonte de verdade servindo CLI, MCP e docs.

## Design

### Estrutura

```
skills/aos/
├── SKILL.md                # visão geral + roteamento
└── references/
    ├── agents.md      memories.md    tasks.md       skills.md
    ├── collections.md views.md       routines.md    toolsets.md
    ├── workspaces.md  instructions.md templates.md  goals.md
    ├── projects.md    artifacts.md   chats.md       activities.md
    ├── config.md      gateway.md     events.md      models.md
    └── themes.md      auth.md        tunnels.md
```

Um arquivo por grupo de comando, **gerado a partir de `Doc` e `Examples`** do registry.

### O gerador

```go
// pkg/skill/generate.go

// Generate produces SKILL.md and references/*.md from a command registry.
// It is the only writer of these files: hand edits are overwritten, and CI
// fails when the committed output differs from the generated one.
func Generate(reg Registry, out fs.FS, opts Options) error

// Registry is the minimal surface Generate needs, so pkg/skill stays usable
// outside this project.
type Registry interface {
	Groups() []Group
}

type Group struct {
	Name     string
	Summary  string
	Doc      string   // full Markdown — becomes the reference file body
	Commands []Command
}
```

### Estrutura do `Doc` de cada grupo

Copiada do original porque funciona — é o texto que o LLM lê para decidir usar a ferramenta:

```markdown
AOS Memory Management — <resumo de uma linha>

## What It Does
<parágrafo explicando o conceito e o modelo mental>

## Commands
- **recall** — <o que faz>
- **store** — <o que faz>

## When to Use This Group
- **Início de sessão:** <gatilho concreto>
- **Após trabalho relevante:** <gatilho concreto>

## Key Concepts
- **Conceito:** <definição>

## Rules
- <regra dura, imperativa>
```

Um teste verifica que **todo** grupo tem as cinco seções. Um `Doc` sem "When to Use This Group" é um grupo que o LLM não vai saber quando usar.

### Sincronização

```go
// Sync copies the generated skill into every agent directory found on the
// machine: .claude/skills/, .agents/skills/, and any path configured by the
// user. It is what `aos self skills add` does.
func Sync(ctx context.Context, src fs.FS, targets []string) error
```

### Portão de CI

```bash
task gen-skill && git diff --exit-code pkg/skill/
```

A documentação não pode divergir do código, porque não é escrita — é derivada.

## Decisões e divergências

> [!decision] `pkg/skill` sem dependência do projeto
> Recebe uma interface `Registry` mínima. É o único pacote em `pkg/`, e é útil fora daqui.

> [!decision] Estrutura de seção obrigatória, verificada
> O original tem a convenção; nós a impomos por teste. Um grupo sem "When to Use" passa despercebido em revisão humana e custa caro em uso por agente.

> [!decision] `SKILL.md` escrita à mão, `references/` geradas
> O `SKILL.md` é curadoria: protocolo de sessão, roteamento e regras duras. As referências são mecânicas. Misturar os dois faria a curadoria ser sobrescrita a cada geração.

## Testes

- Geração determinística: duas execuções produzem bytes idênticos
- Todo grupo do registry tem arquivo de referência
- Todo `Doc` tem as cinco seções obrigatórias
- Todo exemplo gerado é executável (parseia como comando válido)
- `git diff --exit-code` limpo após `task gen-skill`
- `Sync` cria os diretórios alvo e não sobrescreve skills de terceiros

## Critério de pronto

- [ ] `SKILL.md` + 23 referências geradas
- [ ] Portão de CI verificando divergência
- [ ] Estrutura de seções verificada por teste
- [ ] Sincronização para diretórios de agentes funcionando
