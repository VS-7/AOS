---
tags: [frontend, react, wails, bindings]
aliases: [React 19, Bindings, Frontend]
fase: 7
status: em-construcao
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
- [x] Tipos gerados do registry, verificados no CI
- [x] Realtime com reconexão
- [x] Chat, board de tasks e grafo de memórias funcionando

## Estado — Fase 7

`npx tsc --noEmit` limpo; `vite build` produz 255 KB (79 KB comprimido).
`task gen-schema` emite **71 comandos, 974 linhas** de TypeScript do registry Go,
e `task check` falha se o gerado divergir do commitado — que é a propriedade que
o original obtém com o caller tipado do Igniter.

**O cliente unificado existe e é uma interface com dois transportes.** Nenhum
componente sabe onde está rodando: `isDesktop()` escolhe o binding Wails ou o
`fetch`, e ambos chegam no mesmo registry. O workspace vai em header, não em
cookie — defeito #5 do original, e `TestTheWorkspaceTravelsInAHeaderAndNotACookie`
afirma que nenhum cookie sai junto.

**Realtime com backoff limitado.** Seis passos de 250 ms a 10 s, e para de
crescer: sem teto, um laptop que dormiu uma hora volta e espera mais uma; com
teto fixo curto, um daemon fora do ar apanha de cada janela aberta. Os eventos
invalidam o cache do TanStack Query em vez de virarem estado de componente,
então uma tela re-renderiza porque o dado dela mudou — venha a mudança da própria
mutação ou de um agente trabalhando em segundo plano.

### O que NÃO está pronto, e por quê

**A nota fica `em-construcao`.** Três das quatro caixas do critério estão
marcadas; a primeira — *"SPA rodando no navegador e no desktop com o mesmo
bundle"* — está construída e **não foi observada rodando**. Nada nesta fase
abriu a janela e clicou.

Nenhum teste de frontend existe. A nota pede cinco (cliente unificado com
transporte falso, reconexão, streaming incremental, modal de aprovação,
acessibilidade) e nenhum está escrito: não há runner de teste no `package.json`.
É a lacuna mais séria da fase e está aqui em vez de escondida atrás de três
caixas marcadas.

**Roteamento não existe.** A navegação é `useState` entre três telas. TanStack
Router e as 21 rotas da seção 5.2 do PROMPT são trabalho não feito.
