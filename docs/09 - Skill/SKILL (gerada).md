---
tags: [skill, publicada, llm]
aliases: [SKILL.md, Skill Publicada]
fase: 9
status: especificado
origem: "[[Skill]]"
---

# SKILL (gerada)

> Pai: [[Especificação da Skill]] · Fase: 9

## Objetivo

Registrar o conteúdo curado do `SKILL.md` que o sistema publica. Este arquivo é a **fonte** do que vai para `pkg/skill/SKILL.md`; as `references/*` são geradas do registry ([[Especificação da Skill]]).

## O documento

```markdown
---
name: aos
description: Use quando precisar de memória persistente entre sessões, gestão de
  tarefas com ciclo de vida, agentes especializados, coleções de dados
  estruturados, views, rotinas agendadas ou skills instaláveis. Gatilhos:
  "lembre disso", "o que decidimos sobre X", "crie uma task", "delegue para",
  "agende", "monte um CRM".
---

# AOS

Camada de infraestrutura que dá a agentes memória persistente, execução com
ciclo de vida e continuidade entre sessões.

## Início de sessão — obrigatório

Antes de qualquer trabalho relevante:

1. `aos agents me` — descubra sua identidade neste workspace
2. `aos memories recall --limit 20` — recupere o que você já sabe
3. `aos workspace introspect` — veja o que existe

## Protocolo de memória

**Antes de gravar, recupere.** Se já existe um traço, linke ou supersede —
duplicatas diluem o grafo e confundem a recuperação futura.

**Calibre a confiança honestamente:** 0.9–1.0 verificado · 0.6–0.8 inferência
forte · abaixo de 0.6 palpite. Confiança inflada é o principal jeito de
enganar seu eu futuro.

**Memórias são globais entre instâncias paralelas suas.** Não existe rascunho:
o que você grava, todo self paralelo vê, e uma depreciação afeta todos na hora.

**Antes de entregar a resposta final, antes de mover uma task para in_review,
e antes de concluir uma rotina:** reflita sobre o que aprendeu e mantenha suas
memórias.

## Tools compostas — inspecione antes de executar

Muitas tools agrupam ações sob um campo `action`. A descrição da tool é só a
visão geral do grupo; cada ação tem descrição, exemplos e schema próprios.

Na **primeira** chamada de cada ação numa sessão, passe `schema: true` no mesmo
nível de `action` para receber a especificação completa. Depois disso você
conhece o contrato e pode chamar direto.

Se uma chamada falhar validação, não repita às cegas: leia o erro, inspecione
o contrato com `schema: true`, corrija o payload.

## Roteamento

| Preciso de… | Leia |
|---|---|
| Memória, aprendizado, decisão | `references/memories.md` |
| Trabalho com ciclo de vida | `references/tasks.md` |
| Delegar a especialista | `references/agents.md` |
| Dados estruturados | `references/collections.md` |
| Interface sobre dados | `references/views.md` |
| Automação agendada | `references/routines.md` |
| Ferramenta externa | `references/toolsets.md` |
| Entregável publicável | `references/artifacts.md` |
| Política do workspace | `references/instructions.md` |

## Regras duras

- Toda chamada de tool exige `_reasoning` não-vazio
- Use `set_status` para mover tasks; nunca `update`
- Em modo task, comunique por comentários da task, não por chat
- Só mova para `in_review` com evidência de validação — o sistema recusa a
  transição com todos pendentes
- Instrução é política do workspace; memória é sua. Correção pessoal → memória.
  Correção de escopo amplo → instrução, e ela passa por aprovação humana
- Comandos fora da allowlist do seu agente falham; o erro diz exatamente o que
  pedir ao dono do workspace
- Ações que pedem aprovação **realmente perguntam** a um humano quando há canal
  disponível. Em modo headless a negação é imediata e explícita
```

## Decisões e divergências

> [!decision] Quatro seções a mais que o original
> "Tools compostas", a regra sobre globalidade de memória, a linha sobre allowlist e a linha sobre aprovação real. As três últimas descrevem comportamentos que **divergem** do original — um agente que assume o comportamento antigo erra.

> [!decision] Roteamento com dois destinos a mais
> `artifacts` e `instructions`, ausentes da tabela do original, ambos com uso real e não óbvio.

> [!decision] O `description` do frontmatter é o que faz a skill ser encontrada
> Os gatilhos em linguagem natural são o que faz um agente hospedeiro carregar esta skill no momento certo. É a parte mais importante do arquivo e a que merece mais iteração com uso real.

## Testes

- Frontmatter válido, com `name` e `description`
- Todo `references/*.md` citado no roteamento existe
- Todo comando citado no protocolo de sessão existe no registry
- O arquivo commitado bate com o publicado em `pkg/skill/`

## Critério de pronto

- [ ] `SKILL.md` publicada e sincronizada para diretórios de agentes
- [ ] Roteamento apontando só para arquivos existentes
- [ ] Comandos citados verificados contra o registry
- [ ] Divergências em relação ao original explicitadas para o agente
