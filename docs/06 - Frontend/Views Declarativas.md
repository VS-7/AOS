---
tags: [frontend, view, declarativo, json]
aliases: [Views Declarativas, Renderizador de Views]
fase: 8
status: especificado
origem: "[[View]]"
---

# Views Declarativas

> Pai: [[React 19 e Bindings]] · Origem no original: [[View]] · Ver: [[View (Go)]] · [[Design System]] · Fase: 8

## Objetivo

Renderizar uma árvore JSON criada pelo agente como interface real, com dados de uma [[Collection (Go)]] e ações que operam sobre o registry de comandos.

É o mecanismo que permite ao agente **criar interfaces sem escrever código** — a tese de que a conversa não deve ser a única interface.

## Comportamento do original

`@json-render/core`, `/react` e `/shadcn` mapeiam JSON para componentes reais ([[Camada @app (Web UI)]]).

O ciclo: o agente chama `views_create`, grava um `.view.json`, o watcher detecta, e a UI passa a renderizar aquela tela — **sem build, sem deploy**.

## Design

### O renderizador

```tsx
// features/view/render/Renderer.tsx

// Renderer walks a validated tree and maps each node to a registered component.
// The tree was already validated server-side when the view was written, so
// this path does not re-validate — it renders. An unknown component here is a
// bug in the generator, not user input.
export function Renderer({ tree, data, ctx }: RendererProps) {
  const Component = registry[tree.component];
  if (!Component) return <MissingComponent name={tree.component} />;

  const props = {
    ...tree.props,
    ...resolveBindings(tree.bind, data),
    actions: tree.actions?.map((a) => bindAction(a, ctx)),
  };

  return (
    <Component {...props}>
      {tree.children?.map((child, i) => (
        <Renderer key={i} tree={child} data={data} ctx={ctx} />
      ))}
    </Component>
  );
}
```

### Bindings

```tsx
// resolveBindings maps prop names to record field paths:
//   bind: { title: "name", subtitle: "company.name" }
// Missing fields render as empty, never as "undefined" — a view over partially
// filled records should look incomplete, not broken.
function resolveBindings(bind: Record<string, string> | undefined, data: unknown): Record<string, unknown>
```

### Ações

```tsx
// bindAction turns a declarative action into a call through the SAME client
// every other surface uses. A button in a view is not a side door: it runs the
// registered command, with the same validation and the same authorization.
function bindAction(a: ViewAction, ctx: RenderContext) {
  return async (record: Record<string, unknown>) => {
    if (a.confirm && !(await ctx.confirm(a.label))) return;
    await client.invoke(a.command, { ...a.input, ...pickFrom(record, a.input) });
    ctx.invalidate();
  };
}
```

### Tipos de superfície

Os quatro que o prompt menciona, cobertos por composição de componentes do [[Design System]]:

| Superfície | Composição |
|---|---|
| Tabela | `DataTable` com colunas ligadas a campos |
| Dashboard | `Grid` + `StatCard` + `Chart` |
| Pipeline | `Kanban` agrupado por um campo de enum |
| Board | `Kanban` com ações de transição |

### Scaffold

`views_scaffold` infere uma view inicial a partir dos tipos de campo da coleção: `string` vira coluna de texto, `enum` vira coluna com badge e agrupamento sugerido, `date` vira coluna formatada, `ref` vira link. O agente parte de algo que funciona e ajusta.

## Decisões e divergências

> [!decision] Renderizador próprio, não `@json-render`
> Percorrer uma árvore validada e mapear para um registry são ~200 linhas. Em troca, ganhamos controle sobre a fronteira mais sensível: como uma ação declarativa vira uma chamada de comando. Uma biblioteca genérica não teria o conceito de "comando do registry".

> [!decision] Validação no servidor, renderização confiante no cliente
> A view foi validada na escrita ([[View (Go)]]). O cliente não revalida, o que mantém o renderizador simples. `MissingComponent` existe para o caso de uma view antiga referenciar um componente removido — e aparece visivelmente, não em silêncio.

> [!decision] Ações passam pelo cliente unificado
> Uma view não tem caminho próprio de mutação. Mesmo comando, mesma validação, mesma auditoria.

> [!decision] Campo ausente renderiza vazio
> Views são criadas por um agente sobre dados que evoluem. Quebrar a tela porque um registro não tem um campo opcional seria frágil demais para o modelo de uso.

## Testes

- Árvore de exemplo renderiza os quatro tipos de superfície
- Binding para campo inexistente renderiza vazio, não quebra
- Ação chama o comando correto e invalida a query
- Ação com `confirm` exige confirmação
- Componente removido do registry renderiza `MissingComponent` visível
- View criada pelo agente aparece na UI sem reload (via watcher + realtime)
- `scaffold` produz view renderizável para cada tipo de campo

## Critério de pronto

- [ ] Renderizador cobrindo os quatro tipos de superfície
- [ ] Ações operando sobre o registry
- [ ] View criada pelo agente renderizando sem build
- [ ] Scaffold gerando ponto de partida útil
