---
tags: [adr, decisao, persistencia, markdown]
aliases: [ADR-0004, Persistência em Markdown]
fase: 1
status: especificado
origem: "[[Modelo de Persistência]]"
---

# ADR-0004 — Collections em Markdown

> Pai: [[AOS]] · Origem no original: [[Modelo de Persistência]] · Detalhe técnico: [[Collections Engine]] · Fase: 1

## Contexto

O original não tem banco de dados para o domínio. Agentes, memórias, tarefas, skills, instruções, templates e rotinas são **arquivos Markdown com YAML frontmatter dentro do repositório do usuário** ([[Modelo de Persistência]]). SQLite aparece só na fila de jobs — estado operacional efêmero.

As consequências dessa escolha são a proposta de valor, não um detalhe de implementação:

- O estado do agente é **versionável em Git** — o histórico do agente vira histórico do projeto
- É **editável à mão** — o usuário abre o `.md` e corrige a memória do agente
- É **portátil** — copiar `.aos/` move o agente inteiro
- É **transparente para IDEs** — Cursor, VS Code e Claude Code enxergam os arquivos naturalmente

E há um efeito de rede documentado em [[Layout do Filesystem]]:

> Comitar `.fractal/` significa que a memória, as instruções e o histórico de trabalho do agente entram no repositório. Um novo desenvolvedor que clona o projeto herda o agente já treinado no contexto daquele código.

## Decisão

Reproduzir integralmente o modelo: **filesystem como banco de dados de domínio**, com um motor de coleções em `internal/core/collections` que mapeia arquivo ↔ registro.

Elementos não-negociáveis, herdados do original:

1. **Padrões de caminho com placeholders que extraem campos.** `{agent}` e `{id}` vêm do path, não do frontmatter. Parser bidirecional: pattern → regex para leitura; pattern + valores → path para escrita.
2. **Frontmatter YAML validado contra struct Go**; corpo Markdown é o campo `Content`.
3. **Múltiplos patterns por coleção.** O segundo padrão (`.aos/skills/{skill}/agents/...`) é o que permite uma [[Skill (Go)]] empacotar agentes próprios. Sem ele, instalar uma skill não instala uma equipe.
4. **Hooks de ciclo de vida** para normalização e cascade delete.
5. **Watcher** sobre `.aos/` recarregando schemas dinâmicos de [[Collection (Go)]] e [[View (Go)]] em runtime — é o que permite ao agente criar uma coleção e usá-la na mesma sessão.

Elementos **adicionados**, que o original não tem:

6. **Escrita atômica** — temp + `os.Rename`. Ver [[ADR-0012 Escrita atômica e lock por arquivo]].
7. **Lock por caminho** para escrita concorrente.
8. **Índice em memória com invalidação pelo watcher**, para que `List` com filtro não faça varredura completa do disco a cada chamada.

## Alternativas consideradas

| Alternativa | Análise |
|---|---|
| **SQLite para tudo** | Consultas mais rápidas, transações reais, sem lock artesanal. **Descartado**: mata Git, edição manual e transparência para IDEs — as três razões pelas quais o modelo existe. Trocaria a proposta de valor por conveniência de implementação. |
| **Markdown + índice SQLite espelhado** | O melhor dos dois: arquivos como verdade, SQLite como cache de consulta. **Adiado, não descartado.** Adiciona um problema de coerência (índice desatualizado após edição externa) antes de haver evidência de que a varredura de arquivos é gargalo. Um workspace com 10.000 memórias justifica reabrir; ver [[Collections Engine]], seção Escala. |
| **JSON em vez de Markdown** | Perde o corpo livre, que é onde o conteúdo real vive (instruções do agente, descrição da task, corpo da memória). O original usa JSON só em [[Chat (Go)]], por razão técnica explícita — mensagens do SDK de IA não serializam em schema. Reproduzimos essa exceção. |

## Consequências

**Positivas**
- `git diff` mostra o que o agente aprendeu. `git blame` mostra quando.
- Backup é `cp -r`. Migração entre máquinas é `git clone`.
- Testes de integração usam `t.TempDir()` e verificam o arquivo real — sem mock de banco.

**Negativas**
- **Consulta é varredura.** `memories recall --category decision` lê o índice em memória; se o índice não existir ou estiver frio, varre. Mitigado por índice + [[ADR-0013 Bleve para busca full-text]].
- **Concorrência é problema nosso.** O original não trata; nós tratamos com lock por caminho e escrita atômica. Isso é código que um banco daria de graça.
- **Sem transação multi-arquivo.** Criar uma task com todos é N escritas sem atomicidade entre elas. Mitigação: operações compostas gravam o agregado (`TASK.md`) por último, de modo que uma falha parcial deixe órfãos invisíveis em vez de um agregado inconsistente. Documentado em [[Task (Go)]].
- **Nomes de arquivo carregam semântica.** Renomear um arquivo à mão renomeia o registro. É recurso, não bug — mas exige que os hooks normalizem (lowercase de IDs de agente, como no original).

## Status

**Aceito.** Ver [[Collections Engine]] para o design do motor.
