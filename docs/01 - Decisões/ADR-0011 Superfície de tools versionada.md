---
tags: [adr, decisao, mcp, compatibilidade]
aliases: [ADR-0011, Versionamento de Tools, Aliases]
fase: 2
status: especificado
origem: "[[Ferramentas MCP]]"
---

# ADR-0011 — Superfície de tools versionada com aliases

> Pai: [[AOS]] · Origem no original: [[Ferramentas MCP]] · Fase: 2

## Contexto

Entre 0.1.314 e 0.1.401 o original mudou a superfície MCP de duas maneiras que quebram integrações ([[Ferramentas MCP]], [[Versões e Artefatos]]):

1. **Renomeação semântica.** `memories list/get/create/forgot` viraram `recall/reflect/store/forget`.
2. **Mudança de forma.** De ~150 tools planas para tools compostas: `memories_create` → `Memory({action:"store", ...})`.

A nota de referência é explícita sobre o efeito:

> Automações que dependem de `memories_create` quebram com a 0.1.401, onde a operação é `Memory({ action: "store" })`.

Na máquina observada, isso se manifesta em produção: o `~/.mcp.json` registra `"command": "fractal"`, resolvido pelo `PATH` para a versão do nvm — três versões atrás do servidor rodando. As tools disponíveis, as categorias de memória e os grupos existentes divergem silenciosamente entre cliente e servidor.

O consumidor primário destas tools é um LLM que aprendeu os nomes numa sessão e os usa na seguinte. Renomear é caro de um jeito que não aparece em teste.

## Decisão

Três regras.

**1. Nomes de tool são contrato.** Uma vez publicado, um nome nunca é removido — só marcado como deprecado, com alias para o sucessor:

```go
// internal/core/command/alias.go
type Alias struct {
	From       string // "memories_create"
	To         string // "memories_store"
	Since      string // "0.4.0" — versão em que a migração começou
	RemoveAt   string // "1.0.0" — nunca antes de um major
	Deprecated bool
}
```

Chamar um alias funciona e devolve o resultado normal, com um campo `_deprecated` no envelope indicando o nome novo. O LLM lê isso e aprende sozinho na próxima chamada.

**2. As duas projeções coexistem, selecionáveis por configuração.**

```jsonc
// ~/.aos/config.json
"mcp": {
  "toolShape": "flat",     // "flat" | "composite" | "both"
  "surfaceVersion": "v1"
}
```

- `flat` — `memories_store`, `tasks_todos_set_status`. Compatível com o modelo mental de qualquer cliente MCP.
- `composite` — `Memory({action, input, schema?, _reasoning})`. Reduz drasticamente o contexto consumido na listagem de tools.
- `both` — expõe as duas. Custo de contexto maior; útil em migração.

Default: `composite`, que é para onde o ecossistema foi. Mas quem precisa de `flat` não é forçado a reescrever automações.

**3. Registro MCP com caminho absoluto do binário.** O gerador de configuração (`aos mcp add`) grava o caminho resolvido, nunca `"command": "aos"`:

```json
{ "mcpServers": { "aos": {
    "command": "/opt/homebrew/bin/aos",
    "args": ["--mcp"],
    "env": { "AOS_TOKEN": "..." } } } }
```

E `aos mcp doctor` compara a versão do binário registrado com a do daemon em execução, sinalizando divergência — exatamente o problema observado na máquina.

## Alternativas consideradas

| Alternativa | Análise |
|---|---|
| **Renomear livremente entre versões** | É o que o original faz. Quebra integrações e desperdiça o aprendizado do modelo. |
| **Congelar a superfície** | Impede evolução semântica legítima (`list` → `recall` **é** uma melhoria: comunica ao LLM que a operação é cognitiva, não CRUD). |
| **Versionar por endpoint** (`/mcp/v1`, `/mcp/v2`) | Faz sentido em HTTP, não em stdio, onde o cliente registra um comando e não uma URL. |

## Consequências

**Positivas**
- Uma integração escrita hoje continua funcionando após um rename, e o próprio retorno ensina o nome novo.
- `aos mcp doctor` detecta a classe de problema mais comum de operação (versão errada resolvida pelo PATH).
- Corrige os defeitos #14 e o de resolução de PATH descrito em [[Versões e Artefatos]].

**Negativas**
- **Aliases acumulam.** Sem disciplina, a tabela vira um cemitério. Mitigação: remoção permitida só em major, e um teste que falha se um alias passar de `RemoveAt` sem decisão registrada.
- **Manter duas projeções custa código e teste.** Contido: as duas derivam do mesmo `Descriptor`, e o teste de paridade verifica que executar `memories_store` e `Memory({action:"store"})` produz o mesmo efeito.

## Status

**Aceito.** Ver [[Command Layer]] para a implementação das projeções e [[ADR-0016 Compatibilidade de nomes com o original]] para a escolha dos nomes iniciais.
