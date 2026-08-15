---
tags: [frontend, design-system, ui]
aliases: [Design System, Componentes]
fase: 7
status: especificado
origem: "[[Camada @app (Web UI)]]"
---

# Design System

> Pai: [[React 19 e Bindings]] · Ver: [[Views Declarativas]] · [[Temas]] · Fase: 7

## Objetivo

Um conjunto de componentes que serve dois consumidores: os desenvolvedores que escrevem telas, e o **agente**, que compõe [[View (Go)]]s declarativas a partir de um catálogo.

O segundo consumidor é o que torna este design system diferente de um comum: cada componente precisa de um contrato de props legível por máquina.

## Comportamento do original

Design system estilo shadcn em `src/@app/components/ui/`, com `lucide-react` para ícones, Monaco para edição e `@json-render/*` como motor de views ([[Camada @app (Web UI)]]).

O agente descobre componentes por `views_components` e `views_registry` — introspecção antes de compor.

## Design

### Base

| Camada | Escolha |
|---|---|
| Primitivos | Radix UI — acessibilidade e comportamento |
| Estilo | Tailwind com tokens CSS de [[Temas]] |
| Composição | Padrão shadcn: componentes no repositório, não em `node_modules` |
| Ícones | `lucide-react` |
| Editor | Monaco, empacotado no bundle |
| Gráficos | Recharts, com paleta derivada dos tokens do tema |

### O contrato legível por máquina

Todo componente exposto a views declara suas props ao lado da implementação:

```ts
// components/ui/data-table/spec.ts

export const spec = defineComponent({
  name: "DataTable",
  category: "data",
  description: "Tabular view over a collection, with sorting and inline actions.",
  acceptsChildren: false,
  props: {
    columns: {
      type: "list",
      required: true,
      description: "Columns to render, in order. Each binds to a record field.",
    },
    pageSize: { type: "number", default: 25, description: "Rows per page." },
    density:  { type: "enum", values: ["compact", "comfortable"], default: "comfortable" },
  },
});
```

```
task gen-components  →  internal/domain/view/components.gen.go
```

O gerador lê os `spec.ts` e emite o catálogo Go. Um componente removido no React quebra o build de Go; uma prop nova aparece na introspecção do agente sem trabalho manual. Ver [[View (Go)]].

### Regras de composição

| Regra | Razão |
|---|---|
| Nenhum componente com estado de servidor próprio | Dados vêm de props ou de TanStack Query no contêiner |
| Toda cor vem de token, nunca literal | [[Temas]] precisa poder trocar tudo |
| Todo componente interativo tem estado de foco visível | Navegação por teclado é requisito, não extra |
| Nenhuma dependência de CDN | Bundle offline-first |

### As três telas que definem o produto

| Tela | Componentes centrais |
|---|---|
| **Chat** | Stream de mensagens com partes tipadas (texto, reasoning colapsável, tool call com input/output, arquivo), composer, seletor de agente |
| **Board de tasks** | Kanban por status com transição por arraste, que chama `tasks_set_status` — a mesma validação de qualquer superfície |
| **Grafo de memórias** | Grafo force-directed com cor por categoria, aresta por `link` e `supersedes`, painel lateral de reflexão |

## Decisões e divergências

> [!decision] Catálogo gerado do código, em vez de registry mantido à mão
> No original, o registry de componentes vive no frontend e o backend não o conhece — nada impede o agente de referenciar um componente inexistente. Aqui, o catálogo é gerado e a validação de view acontece na escrita ([[View (Go)]]).

> [!decision] Componentes no repositório (padrão shadcn)
> Um design system que precisa ser modificado por tema e por token não deve ser uma dependência opaca.

> [!decision] Renderizador de views próprio, não `@json-render`
> O original usa `@json-render/*`. Um renderizador que percorre uma árvore validada e mapeia para componentes registrados são ~200 linhas, e nos dá controle sobre a fronteira entre árvore declarativa e ações do registry. Ver [[Views Declarativas]].

> [!decision] Reasoning colapsado por padrão no chat
> O prompt-mestre instrui o agente a entregar síntese de decisão, não transcrição de raciocínio. A UI reflete: reasoning existe, mas não domina a tela.

## Testes

- `task gen-components` produz catálogo idêntico ao commitado (portão de CI)
- Todo `spec.ts` tem componente correspondente e vice-versa
- Nenhuma cor literal fora dos arquivos de tema (lint)
- Navegação por teclado nas três telas principais
- Contraste WCAG AA com cada tema embutido
- Renderização das partes de mensagem, incluindo tool call com output truncado

## Critério de pronto

- [ ] Catálogo gerado e sincronizado com o Go
- [ ] Três telas principais implementadas
- [ ] Zero cores literais fora dos temas
- [ ] Acessibilidade verificada nas telas principais
