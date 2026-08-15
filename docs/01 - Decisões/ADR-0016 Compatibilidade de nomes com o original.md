---
tags: [adr, decisao, compatibilidade, nomes]
aliases: [ADR-0016, Nomenclatura, Compatibilidade]
fase: 2
status: especificado
origem: "[[Ponte CLI para MCP]]"
---

# ADR-0016 — Compatibilidade de nomes com o original

> Pai: [[AOS]] · Origem no original: [[Ponte CLI para MCP]] · Fase: 2

## Contexto

O sistema é consumido por LLMs. Isso muda o cálculo de nomenclatura de um jeito que não vale para software consumido por humanos: **um modelo que já viu `memories_store`, `tasks_todos_set_status` ou a tool `Read` em treino e em outras ferramentas acerta o formato na primeira tentativa.** Um nome original e mais bonito custa tentativas erradas, tokens e latência.

Duas evidências no original apontam para o mesmo lugar:

1. As tools nativas de filesystem chamam-se `Read`, `Write`, `Edit`, `Glob`, `Grep`, `Bash` — **deliberadamente idênticas às do Claude Code**. A nota de [[Tool Executor]] registra a razão: *"reduz a curva de aprendizado do modelo, que já viu esses nomes em treino"*.
2. O sistema de hooks implementa o **contrato do Claude Code**, com adaptador dedicado (`claude-code.adapter.ts`), o que permite hooks escritos para uma ferramenta rodarem na outra ([[Eventos e Hooks]]).

Ao mesmo tempo, há nomes do original que são defeito e não devem ser copiados: `memories_forgot` (erro gramatical), e a colisão entre o grupo `skills` builtin do `incur` e o grupo de domínio `skills`, que faz `fractal skills --help` mostrar apenas `add` e `list` — as operações de domínio ficam inacessíveis pelo CLI ([[Comandos CLI]], defeito #15).

## Decisão

**Três faixas de nomenclatura**, com regras diferentes.

**Faixa 1 — nomes que copiamos exatamente.** Onde a compatibilidade com o ecossistema de agentes vale mais que originalidade:

| Categoria | Nomes |
|---|---|
| Tools de filesystem | `Read`, `Write`, `Edit`, `Glob`, `Grep`, `Bash` |
| Tools de web | `WebSearch`, `WebFetch` |
| Eventos de hook | `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `SubagentStart`, `SubagentStop`, `Stop`, `PreCompact` |
| Campo obrigatório | `_reasoning` |
| Regra de nome de tool | caminho unido por `_`: `memories_store`, `tasks_todos_set_status` |
| Introspecção de tool composta | `schema: true` no mesmo nível de `action` |

**Faixa 2 — nomes que corrigimos.** Onde o original erra:

| Original | Aqui | Razão |
|---|---|---|
| `memories_forgot` | `memories_forget` | Erro gramatical; alias mantido ([[ADR-0011 Superfície de tools versionada]]) |
| `skills` (builtin colidindo com domínio) | builtins sob `aos self ...` | Elimina a colisão do defeito #15 |
| `tasks comment create` / tool `tasks_comment_add` | `tasks_comment_create` | O original diverge entre CLI e MCP para a mesma operação |
| `tasks todos transition` / `tasks_todos_set_status` | `tasks_todos_set_status` nos dois | Mesma divergência |
| `DEFAULT_WORSKSPACE_*` | grafia correta | Typo no fonte original |

**Faixa 3 — nomes de domínio, herdados sem cerimônia.** `workspace`, `agent`, `memory`, `task`, `todo`, `comment`, `skill`, `instruction`, `template`, `toolset`, `collection`, `view`, `artifact`, `routine`, `project`, `goal`, `chat`, `activity`, `event`, `auth`, `config`, `theme`, `model`, `file`, `bot`, `tunnel`, `gateway`, `marketplace`. São os nomes certos para os conceitos; inventar sinônimos seria perda pura.

**Nomes de categoria de memória:** adotamos as **13 categorias cognitivas da 0.1.401** (`decision`, `intent`, `commitment`, `relationship`, `event`, `observation`, `error`, `learning`, `fact`, `reference`, `instruction`, `preference`, `context`), não as 9 técnicas da 0.1.314. A instrução do schema original explica a escolha: *"Pick the function of the knowledge, not writing style."* Ver [[Memory (Go)]].

**Verbos cognitivos no grupo de memória:** `recall`, `graph`, `reflect`, `store`, `forget` — a renomeação da 0.1.401 é uma melhoria real, porque comunica ao LLM que a operação é cognitiva e não CRUD.

## Alternativas consideradas

| Alternativa | Análise |
|---|---|
| **Nomenclatura toda nova** | Marca própria, zero herança de defeito. Custo: cada modelo reaprende do zero, e os nomes de tool de filesystem perdem o benefício de reconhecimento. Trocaria valor mensurável por diferenciação cosmética. |
| **Copiar tudo, inclusive os defeitos** | Compatibilidade máxima. Descartado: `memories_forgot` e a colisão de `skills` são erros, e o documento de missão é explícito de que reproduzi-los é falha de execução. |
| **Prefixar tudo com o nome do produto** (`aos_memories_store`) | Evita colisão quando vários servidores MCP estão registrados. Descartado: os clientes MCP já namespaceiam por servidor (`mcp__aos__memories_store`), então o prefixo seria redundante e mais longo. |

## Consequências

**Positivas**
- Um agente que já opera Claude Code opera este sistema sem reaprender as tools de filesystem.
- Hooks escritos para o contrato do Claude Code rodam aqui via adaptador ([[Event Hooks (Go)]]).
- Automações escritas contra a superfície MCP do original migram com renomeações mínimas, documentadas.

**Negativas**
- **Herança de decisões que talvez não sejam ideais.** `tasks_todos_set_status` é longo; o padrão de tool composta não é universal entre clientes MCP. Aceito conscientemente.
- **A tabela de correções precisa ser mantida** e refletida na documentação de migração, senão vira folclore.

## Status

**Aceito.**
