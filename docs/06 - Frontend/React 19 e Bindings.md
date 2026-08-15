---
tags: [frontend, react, wails, bindings]
aliases: [React 19, Bindings, Frontend]
fase: 7
status: especificado
origem: "[[Camada @app (Web UI)]]"
---

# React 19 e Bindings

> Pai: [[AOS]] · Origem no original: [[Camada @app (Web UI)]] · Ver: [[Wails3 Services]] · Fase: 7

## Objetivo

Uma SPA React 19 + TypeScript que roda igual no navegador e dentro do desktop, consumindo o mesmo domínio por dois caminhos.

## Comportamento do original

SPA React servida pelo próprio servidor na rota `/*`, com o mesmo bundle carregado no navegador e no Electron ([[Camada @app (Web UI)]]). Cada feature tem sua camada `presentation/` própria — a UI segue o mesmo feature-slice do backend.

Comunicação: HTTP `/api/*` com cliente tipado, WebSocket `/ws` para realtime, e cookie `x-workspace-id` para o workspace ativo.

## Design

### Estrutura

```
frontend/src/
├── features/          # espelha internal/domain
│   ├── chat/  task/  memory/  agent/  workspace/
│   ├── collection/  view/  skill/  routine/
│   └── (cada uma: components/ hooks/ api.ts types.ts)
├── components/ui/     # design system → [[Design System]]
├── lib/
│   ├── client.ts      # cliente unificado HTTP | Wails
│   ├── realtime.ts    # WebSocket
│   └── schema.ts      # tipos gerados do registry Go
└── bindings/          # gerado por `wails3 generate bindings`
```

### O cliente unificado

O mesmo componente React roda no navegador (HTTP) e no desktop (Wails, em processo). Uma interface, dois transportes:

```ts
// lib/client.ts

export interface Client {
  invoke<K extends CommandKey>(key: K, input: CommandInput<K>): Promise<CommandOutput<K>>;
}

// In the browser, commands go over HTTP. In the desktop, they go through the
// Wails binding — same registry, same validation, no network hop.
export const client: Client = isDesktop()
  ? { invoke: (key, input) => WorkspaceService.Invoke(key, input) }
  : { invoke: (key, input) => http.post(`/api/${key.replaceAll("_", "/")}`, input) };
```

### Tipos gerados do registry

```
task gen-schema   →  frontend/src/lib/schema.ts
```

O gerador percorre o registry Go e emite tipos TypeScript para toda entrada e saída de comando:

```ts
// lib/schema.ts — GENERATED. Do not edit.
export interface CommandMap {
  memories_store: { input: MemoriesStoreInput; output: Memory };
  tasks_set_status: { input: TasksSetStatusInput; output: Task };
  // ...
}
```

**Consequência:** mudar um schema em Go quebra a compilação do frontend. É a mesma propriedade que o original obtém com o caller tipado do Igniter — e a razão de o `schema.ts` do original ter 3.468 linhas.

### Realtime

```ts
// lib/realtime.ts

// One connection per workspace, with reconnection and backoff. Events are
// dispatched into TanStack Query's cache so components re-render without any
// manual subscription bookkeeping.
export function useRealtime(workspaceId: string): void
```

### Estado

| Necessidade | Escolha |
|---|---|
| Dados do servidor | TanStack Query, invalidado por eventos de realtime |
| Estado de UI local | `useState` / `useReducer` |
| Estado global mínimo (workspace ativo, tema) | Context |

Sem Redux. O estado do servidor é do servidor.

### Streaming de chat

`chat.delta` acumula no cache da query da conversa; o componente renderiza incrementalmente. O envio continua por HTTP — o socket é só entrega ([[Realtime WebSocket]]).

### Aprovação de tool

Um modal global escuta `approval.request`, mostra a tool, o payload formatado e o motivo, e devolve a decisão por HTTP. Ver [[ADR-0007 Canal real de aprovação de tool]].

## Decisões e divergências

> [!decision] Sem Next.js
> O bundle do original carrega partes do Next.js compiladas. Uma SPA servida por um binário Go não precisa de SSR, roteamento de servidor nem de um framework de aplicação. Vite + React Router.

> [!decision] Workspace ativo em header, não em cookie
> O original usa cookie `x-workspace-id`, que é o que torna o upgrade de WebSocket vulnerável (defeito #5). O workspace ativo passa a ser estado da aplicação, enviado em header — e o WS o recebe com autorização verificada.

> [!decision] Tipos gerados, não escritos
> Um teste de CI falha se o gerado divergir do commitado.

> [!decision] Um bundle, dois transportes
> Nenhum componente sabe se está no navegador ou no desktop.

## Testes

- Build do frontend falha se `schema.ts` divergir do registry Go
- Cliente unificado: o mesmo teste de componente roda com transporte HTTP fake e Wails fake
- Realtime: reconexão com backoff; eventos invalidam as queries certas
- Streaming renderiza incrementalmente
- Modal de aprovação aparece e resolve
- Teste de acessibilidade básico nas telas principais

## Critério de pronto

- [ ] SPA rodando no navegador e no desktop com o mesmo bundle
- [ ] Tipos gerados do registry, verificados no CI
- [ ] Realtime com reconexão
- [ ] Chat, board de tasks e grafo de memórias funcionando
