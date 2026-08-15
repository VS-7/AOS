---
tags: [adr, decisao, template, liquid]
aliases: [ADR-0014, Liquid, Templates]
fase: 8
status: especificado
origem: "[[Template]]"
---

# ADR-0014 — Liquid para templates, com fronteira de segurança

> Pai: [[AOS]] · Origem no original: [[Template]] · Fase: 8

## Contexto

O original usa **LiquidJS** em dois lugares com propósitos opostos:

1. **[[Template]]** — motor de geração de artefatos. Renderizar **é** o propósito.
2. **[[Montagem de Contexto]]** — montagem do prompt do agente, onde renderizar é **risco**.

O contrato de segurança do `AgentPromptBuilder` está escrito no fonte, e é a razão pela qual o segundo caso existe separado do primeiro:

```ts
/**
 * Security contract: `content` is persisted data and is NEVER interpreted as a
 * Liquid template unless `renderTemplate` is explicitly `true`. This prevents
 * template injection from agent content, memories, or workspace records.
 */
```

Sem isso, uma memória contendo `{{ config.security.secret }}` vazaria segredo pelo prompt. Um agente que grava a própria memória controla o conteúdo — e portanto controlaria o template.

Há ainda uma otimização no original que também é redução de superfície:

```ts
if (!/{{|{%/.test(template)) return template;   // sem placeholders, sem parse
```

## Decisão

**`github.com/osteele/liquid`** para o domínio [[Template (Go)]], com **três regras de fronteira**.

**Regra 1 — o motor de prompt e o motor de template são pacotes distintos.** `internal/runtime/prompt` **não importa** `internal/domain/template`. A separação é estrutural, verificada pelo teste de regra de dependência ([[Hexagonal e Regra de Dependência]]), não uma convenção.

**Regra 2 — dados persistidos nunca renderizam.** No [[Prompt Assembly]], `RenderTemplate` default é `false`:

```go
type Section struct {
	Title          string
	Content        any
	Kind           string
	Source         string
	Trust          Trust
	RenderTemplate bool // MUST default to false. Persisted data never renders.
}

// renderIfNeeded renders only when the caller explicitly opted in AND the text
// actually contains Liquid delimiters. Both conditions are required.
func renderIfNeeded(tpl string, vars map[string]any, allow bool) (string, error) {
	if !allow {
		return tpl, nil
	}
	if !strings.Contains(tpl, "{{") && !strings.Contains(tpl, "{%") {
		return tpl, nil
	}
	return engine.ParseAndRenderString(tpl, vars)
}
```

Só duas coisas renderizam no caminho do prompt: as instruções de sistema (constante do código, confiável) e seções com opt-in explícito.

**Regra 3 — o contexto de variáveis é uma allowlist.** Mesmo quando renderiza, o mapa de variáveis nunca contém `config`, credenciais ou o objeto de ambiente inteiro. É montado campo a campo:

```go
vars := map[string]any{
	"agent":     map[string]any{"id": a.ID, "name": a.Name, "role": a.Role},
	"workspace": map[string]any{"id": ws.ID, "name": ws.Name},
	"now":       clock.Now().Format(time.RFC3339),
}
```

Para o domínio [[Template (Go)]], onde o usuário escreve o template e a renderização é o produto, aplica-se limite de tempo e de tamanho de saída, e a renderização roda com `context.WithTimeout` — um template com laço patológico não trava o servidor.

## Alternativas consideradas

| Alternativa | Análise |
|---|---|
| **`text/template` da stdlib** | Zero dependência, mas sintaxe incompatível. Templates que o usuário migre do original quebrariam, e a sintaxe Liquid é a que aparece na documentação e nos exemplos do sistema. |
| **`flosch/pongo2`** (Django-like) | Mais recursos, sintaxe diferente do Liquid. Mesmo problema de compatibilidade. |
| **Escrever um motor mínimo** (só `{{ var }}`) | Elimina toda a superfície de injeção e o risco de laço infinito. **Descartado**: o domínio [[Template (Go)]] tem valor real em condicionais e laços — gerar um brief de task ou um relatório exige mais que substituição. Registrado como plano B caso `osteele/liquid` se mostre insuficiente ou insustentável. |

## Consequências

**Positivas**
- Paridade de sintaxe com o original: templates existentes funcionam.
- A defesa contra injeção de template é estrutural (pacotes separados + default `false` + allowlist de variáveis), não uma lembrança do desenvolvedor.
- A otimização de "sem delimitador, sem parse" é mantida — é redução de superfície, não só performance.

**Negativas**
- **`osteele/liquid` não cobre 100% dos filtros do LiquidJS.** Divergências devem ser levantadas e documentadas em [[Template (Go)]] durante a Fase 8, com um teste golden por filtro suportado.
- **Dependência de terceiro em caminho de produto.** Contido atrás de uma interface `Renderer` de duas funções, substituível.

## Status

**Aceito.**
