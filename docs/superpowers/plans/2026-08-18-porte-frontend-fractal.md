# Porte do frontend do Fractal — Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Substituir o frontend reconstruído do AOS pela interface completa do Fractal, adaptando-a ao backend Go por um único ponto de tradução.

**Architecture:** Uma fachada (`lib/aos-facade.ts`) apresenta a API que o código do Fractal espera (`client.task.list.useQuery()`) e traduz para o `client.invoke("tasks_list", …)` do AOS. Isso torna 84% dos 574 arquivos copiáveis sem edição de lógica. Domínios sem backend Go respondem com um envelope de erro tipado e exibem um painel explicativo, em vez de quebrar.

**Tech Stack:** React 19, TypeScript 5.7 (strict), Vite 6, TanStack Router + Query, Tailwind 4, Wails 3, vitest 4.

**Spec:** `docs/superpowers/specs/2026-08-18-porte-frontend-fractal-design.md`

## Global Constraints

- **Nenhum pacote `@igniter-js/*` é instalado.** Verificação final: `grep -r "@igniter-js" frontend/src` deve vir vazio.
- **`frontend/src/lib/client.ts` não é modificado.** Contém as correções de transporte desktop/HTTP já depuradas.
- **O namespace é `aos`, não `igniter`.** Classes dos builders: `AosApp`, `AosPage`, `AosLayout`, `AosMiddleware`, `AosRouter`, `AosStore`, `AosTrigger`, `AosResponse`.
- **A fachada nunca lança em `query`/`mutate`** — devolve `{ data, error }`. 117 sites no front dependem disso.
- **Payload aninhado sob a chave do domínio**: Go devolve `{"tasks": [...]}`, front lê `result.data.tasks`. Não achatar.
- **Todo comando Go exige `_reasoning`** (71 comandos). A fachada injeta; nenhum call site envia.
- **Chaves de comando podem ser kebab-case**: `tasks_set-status`, `activity_read-all`, `todos_set-status`.
- Fonte da cópia: `/Users/vitorsergio/Documents/MeusProjetos/Wails/Fractal Reverse Enginner/_extracted/`. Referida abaixo como `$EXT`.
- Destino: `/Users/vitorsergio/Documents/MeusProjetos/Wails/aos/frontend`. Referido como `$AOS`.
- Path alias: `@/*` → `./src/*` (tsconfig e vite já configurados).

Defina no shell antes de começar:

```bash
export EXT="/Users/vitorsergio/Documents/MeusProjetos/Wails/Fractal Reverse Enginner/_extracted"
export AOS="/Users/vitorsergio/Documents/MeusProjetos/Wails/aos/frontend"
```

---

### Task 1: Infraestrutura de teste e perfil do compilador

**Files:**
- Modify: `$AOS/package.json`
- Modify: `$AOS/tsconfig.json`
- Create: `$AOS/tsconfig.strict.json`
- Modify: `$AOS/vite.config.ts`
- Create: `$AOS/src/lib/aos-facade.test.ts`

**Interfaces:**
- Consumes: nada (primeira task)
- Produces: `npm test` (vitest), `npm run typecheck` (relaxado, cobre o portado), `npm run typecheck:strict` (as três flags de volta sobre `src/lib/**`)

- [ ] **Step 1: Instalar o vitest**

```bash
cd "$AOS" && npm install -D vitest@^4.1.11
```

- [ ] **Step 2: Adicionar os scripts ao package.json**

Em `$AOS/package.json`, no bloco `"scripts"`, adicionar `test` e `typecheck:strict`:

```json
  "scripts": {
    "dev": "vite",
    "build": "tsc --noEmit && vite build",
    "typecheck": "tsc --noEmit",
    "typecheck:strict": "tsc -p tsconfig.strict.json",
    "test": "vitest run",
    "test:watch": "vitest",
    "preview": "vite preview"
  },
```

- [ ] **Step 3: Habilitar o vitest no vite.config.ts**

Em `$AOS/vite.config.ts`, adicionar a chave `test` ao objeto passado para `defineConfig`, no mesmo nível de `resolve` e `build`:

```ts
  // O ambiente é node, não jsdom: o que testamos é a fachada — tradução de
  // nome, montagem de payload e formato do envelope. Nada disso toca o DOM,
  // e um jsdom aqui só somaria tempo de subida a cada rodada.
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
```

- [ ] **Step 4: Relaxar as três flags no tsconfig.json**

Em `$AOS/tsconfig.json`, `compilerOptions`, trocar as três linhas. `strict` continua `true`.

```jsonc
    "strict": true,
    // Relaxadas para o código portado do Fractal, escrito sob outro perfil.
    // As duas primeiras são lint, não segurança. noUncheckedIndexedAccess é
    // segurança, e desligá-la é perda real — aceita porque o alternativo é
    // editar 413 indexações em código que não escrevemos, e cada edição é
    // divergência permanente contra a fonte. A ilha estrita
    // (tsconfig.strict.json) mantém as três sobre src/lib, que é o código
    // que este porte de fato escreve.
    "noUnusedLocals": false,
    "noUnusedParameters": false,
    "noUncheckedIndexedAccess": false,
```

- [ ] **Step 5: Criar a ilha estrita**

Criar `$AOS/tsconfig.strict.json`:

```json
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noUncheckedIndexedAccess": true
  },
  "include": ["src/lib/**/*"]
}
```

- [ ] **Step 6: Escrever um teste de fumaça que falha**

Criar `$AOS/src/lib/aos-facade.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { flattenArgs } from "./aos-facade";

describe("flattenArgs", () => {
  it("funde params, query e body numa carga única", () => {
    expect(flattenArgs({ params: { task: "t-1" }, query: { limit: 10 }, body: { title: "x" } }))
      .toEqual({ task: "t-1", limit: 10, title: "x" });
  });

  it("não deixa `enabled` vazar para a carga", () => {
    expect(flattenArgs({ query: { limit: 5 }, enabled: false })).toEqual({ limit: 5 });
  });

  it("devolve objeto vazio quando não há argumentos", () => {
    expect(flattenArgs()).toEqual({});
  });
});
```

- [ ] **Step 7: Rodar e confirmar que falha**

Run: `cd "$AOS" && npm test`
Expected: FAIL — `Failed to resolve import "./aos-facade"`

- [ ] **Step 8: Commit**

```bash
cd "$AOS/.." && git add frontend/package.json frontend/package-lock.json frontend/tsconfig.json frontend/tsconfig.strict.json frontend/vite.config.ts frontend/src/lib/aos-facade.test.ts
git commit -m "chore(frontend): add vitest and split the strict typecheck island"
```

---

### Task 2: A tabela de mapeamento

**Files:**
- Create: `$AOS/src/lib/command-map.ts`
- Create: `$AOS/src/lib/command-map.test.ts`

**Interfaces:**
- Consumes: `CommandKey` de `@/lib/schema`
- Produces:
  - `type MapEntry = CommandKey | HttpHandler | null`
  - `type HttpHandler = (payload: Record<string, unknown>) => Promise<unknown>`
  - `const COMMAND_MAP: Record<string, MapEntry>` — 113 chaves
  - `function isDormant(feature: string): boolean`
  - `const DORMANT_DOMAINS: ReadonlySet<string>` — os 14 domínios sem backend Go

- [ ] **Step 1: Escrever o teste que falha**

Criar `$AOS/src/lib/command-map.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { COMMAND_MAP, DORMANT_DOMAINS, isDormant } from "./command-map";
import { COMMAND_KEYS } from "./schema";

describe("COMMAND_MAP", () => {
  it("cobre as 113 chamadas que o front do Fractal faz", () => {
    expect(Object.keys(COMMAND_MAP)).toHaveLength(113);
  });

  it("aponta apenas para comandos que o Go publica de fato", () => {
    const publicados = new Set<string>(COMMAND_KEYS);
    const quebrados = Object.entries(COMMAND_MAP)
      .filter(([, v]) => typeof v === "string" && !publicados.has(v))
      .map(([k, v]) => `${k} -> ${String(v)}`);
    expect(quebrados).toEqual([]);
  });

  it("mapeia os renomes irregulares, inclusive os kebab-case", () => {
    expect(COMMAND_MAP["task.getById"]).toBe("tasks_get");
    expect(COMMAND_MAP["task.setStatus"]).toBe("tasks_set-status");
    expect(COMMAND_MAP["activity.markAsRead"]).toBe("activity_read");
    expect(COMMAND_MAP["activity.markAllAsRead"]).toBe("activity_read-all");
  });

  it("declara dormência com null, nunca por omissão da chave", () => {
    expect("collection.list" in COMMAND_MAP).toBe(true);
    expect(COMMAND_MAP["collection.list"]).toBeNull();
  });

  it("reconhece os 14 domínios sem backend Go", () => {
    expect(DORMANT_DOMAINS.size).toBe(14);
    expect(isDormant("collection")).toBe(true);
    expect(isDormant("task")).toBe(false);
  });
});
```

- [ ] **Step 2: Rodar e confirmar que falha**

Run: `cd "$AOS" && npm test -- command-map`
Expected: FAIL — `Failed to resolve import "./command-map"`

- [ ] **Step 3: Escrever a tabela**

Criar `$AOS/src/lib/command-map.ts`. As superfícies HTTP dedicadas reusam os
helpers que já existem em `lib/file.ts` e `lib/auth.ts` — a fachada não
reimplementa transporte.

```ts
import type { CommandKey } from "./schema";
import * as fileApi from "./file";
import * as authApi from "./auth";

/**
 * Uma chamada do front do Fractal, resolvida.
 *
 * - `CommandKey` — vai pelo registry de comandos (`client.invoke`).
 * - `HttpHandler` — vai por uma superfície HTTP própria (`/api/auth`,
 *   `/api/file`), que está fora do registry por decisão do backend.
 * - `null` — o Go ainda não tem isso. Ver o contrato de dormência.
 */
export type HttpHandler = (payload: Record<string, unknown>) => Promise<unknown>;
export type MapEntry = CommandKey | HttpHandler | null;

const s = (p: Record<string, unknown>, k: string): string => String(p[k] ?? "");

/**
 * O mapa é explícito, não uma regra de pluralização.
 *
 * Uma regra erraria em silêncio nos casos irregulares — e eles são reais:
 * `getById`→`get`, `markAsRead`→`read`, e dois kebab-case (`set-status`,
 * `read-all`) que nenhuma pluralização produziria.
 *
 * `null` é uma declaração, não uma omissão: significa "o Go ainda não tem
 * este comando". A ausência da chave é erro de programação e a fachada
 * falha alto nesse caso.
 */
export const COMMAND_MAP: Record<string, MapEntry> = {
  // ── registry de comandos ────────────────────────────────────────────────
  "activity.list": "activity_list",
  "activity.markAllAsRead": "activity_read-all",
  "activity.markAsRead": "activity_read",
  "agent.create": "agents_create",
  "agent.delete": "agents_delete",
  "agent.update": "agents_update",
  "chat.create": "chats_create",
  "chat.getById": "chats_get",
  "chat.list": "chats_list",
  "chat.send": "chats_send",
  "comment.list": "comments_list",
  "config.get": "config_get",
  "config.update": "config_update",
  "routine.create": "routines_create",
  "routine.delete": "routines_delete",
  "routine.fire": "routines_fire",
  "routine.getById": "routines_get",
  "routine.list": "routines_list",
  "routine.update": "routines_update",
  "task.delete": "tasks_delete",
  "task.getById": "tasks_get",
  "task.list": "tasks_list",
  "task.setStatus": "tasks_set-status",
  "task.update": "tasks_update",
  "theme.get": "themes_get",
  "theme.list": "themes_list",
  "todo.list": "todos_list",
  "workspace.create": "workspace_create",
  "workspace.delete": "workspace_delete",
  "workspace.list": "workspace_list",

  // ── superfícies HTTP próprias ───────────────────────────────────────────
  "auth.getStatus": () => authApi.status(),
  "auth.login": (p) => authApi.login(s(p, "identifier"), s(p, "password")),
  "auth.logout": () => authApi.logout(),
  "auth.onboarding": (p) => authApi.onboarding(s(p, "name"), s(p, "email"), s(p, "password")),
  "session.get": () => authApi.session(),
  "password.change": (p) => authApi.changePassword(s(p, "current"), s(p, "next")),
  "file.create": (p) => fileApi.write(s(p, "path"), s(p, "content")),
  "file.delete": (p) => fileApi.remove(s(p, "path")),
  "file.diff": (p) => fileApi.diff(s(p, "path")),
  "file.list": (p) => fileApi.tree(s(p, "path"), p["recursive"] === true),
  "file.move": (p) => fileApi.move(s(p, "from"), s(p, "to")),
  "file.read": (p) => fileApi.read(s(p, "path")),
  "file.write": (p) => fileApi.write(s(p, "path"), s(p, "content")),

  // ── dormentes: comando ausente em domínio vivo ──────────────────────────
  "activity.listEvents": null,
  "auth.verifyWaitlist": null,
  "chat.delete": null,
  "chat.findOrCreateDm": null,
  "chat.stop": null,
  "chat.toggleReaction": null,
  "chat.update": null,
  // O Go tem file.tree; o explorer devolve um snapshot com contextos. A
  // diferença é de formato, não de capacidade — candidato a adaptador, não
  // a implementação nova. Fora do escopo deste porte.
  "file.changes": null,
  "file.explorer": null,
  "file.search": null,
  "session.updateProfile": null,
  "task.start": null,
  "workspace.addMember": null,
  "workspace.listMembers": null,
  "workspace.removeMember": null,
  "workspace.updateMember": null,

  // ── dormentes: domínio inteiro ausente no Go ────────────────────────────
  "artifact.delete": null,
  "artifact.getById": null,
  "artifact.list": null,
  "collection.createRecord": null,
  "collection.delete": null,
  "collection.deleteRecord": null,
  "collection.getById": null,
  "collection.getRecordById": null,
  "collection.list": null,
  "collection.listRecords": null,
  "collection.updateRecord": null,
  "goal.create": null,
  "goal.delete": null,
  "goal.getById": null,
  "goal.list": null,
  "goal.update": null,
  "instruction.create": null,
  "instruction.delete": null,
  "instruction.list": null,
  "instruction.update": null,
  "marketplace.getByName": null,
  "marketplace.list": null,
  "model.list": null,
  "model.set": null,
  "project.create": null,
  "project.delete": null,
  "project.getById": null,
  "project.list": null,
  "project.update": null,
  "skill.delete": null,
  "skill.install": null,
  "skill.list": null,
  "skill.update": null,
  "template.create": null,
  "template.delete": null,
  "template.list": null,
  "template.update": null,
  "token.regenerate": null,
  "toolset.delete": null,
  "toolset.getById": null,
  "toolset.getConfig": null,
  "toolset.updateConfig": null,
  "tunnel.getStatus": null,
  "tunnel.start": null,
  "tunnel.stop": null,
  "user.create": null,
  "user.delete": null,
  "user.list": null,
  "user.update": null,
  "view.delete": null,
  "view.executeAction": null,
  "view.getById": null,
  "view.list": null,
  "view.render": null,
};

/** Os domínios que o backend Go ainda não tem, inteiros. */
export const DORMANT_DOMAINS: ReadonlySet<string> = new Set([
  "artifact", "collection", "goal", "instruction", "marketplace", "model",
  "project", "skill", "template", "token", "toolset", "tunnel", "user", "view",
]);

/** Se o domínio inteiro está dormente — o que a rota mostra como painel. */
export function isDormant(feature: string): boolean {
  return DORMANT_DOMAINS.has(feature);
}
```

- [ ] **Step 4: Adicionar o helper de senha que falta em lib/auth.ts**

`lib/auth.ts` cobre status/login/onboarding/logout/session, mas não
`POST /api/auth/password`. Adicionar ao final de `$AOS/src/lib/auth.ts`,
seguindo o formato das funções vizinhas:

```ts
/** Troca a senha da sessão corrente. `POST /api/auth/password`. */
export function changePassword(current: string, next: string): Promise<void> {
  return post("/api/auth/password", { current, next });
}
```

Se o helper interno de POST em `lib/auth.ts` tiver outro nome ou assinatura,
usar o mesmo que `login()` usa — o objetivo é não introduzir um segundo
caminho de transporte.

- [ ] **Step 5: Rodar e confirmar que passa**

Run: `cd "$AOS" && npm test -- command-map`
Expected: PASS — 5 testes

- [ ] **Step 6: Commit**

```bash
cd "$AOS/.." && git add frontend/src/lib/command-map.ts frontend/src/lib/command-map.test.ts frontend/src/lib/auth.ts
git commit -m "feat(frontend): map the Fractal call surface onto the Go command registry"
```

---

### Task 3: A fachada

**Files:**
- Create: `$AOS/src/lib/aos-facade.ts`
- Modify: `$AOS/src/lib/aos-facade.test.ts`

**Interfaces:**
- Consumes: `COMMAND_MAP`, `isDormant` de `./command-map`; `client`, `DomainError` de `./client`
- Produces:
  - `type CallOpts = { query?, params?, body?, enabled?: boolean }`
  - `type Envelope<T> = { data: T | undefined; error: EnvelopeError | undefined }`
  - `type EnvelopeError = { code: string; message: string }`
  - `function flattenArgs(opts?: CallOpts): Record<string, unknown>`
  - `function call(feature: string, action: string, opts?: CallOpts): Promise<Envelope<unknown>>`
  - `const api: AosClient` — o Proxy de dois níveis
  - `const DORMANT_CODE = "AOS_DOMAIN_DORMANT"`

- [ ] **Step 1: Estender o teste com o contrato completo**

Substituir o conteúdo de `$AOS/src/lib/aos-facade.test.ts`:

```ts
import { describe, expect, it, vi, beforeEach } from "vitest";

const invoke = vi.fn();
vi.mock("./client", () => ({
  client: { invoke: (...a: unknown[]) => invoke(...a) },
  DomainError: class DomainError extends Error {
    code = "AOS_TEST"; status = 400; issues = {}; actions = [];
  },
}));

const { flattenArgs, call, api, DORMANT_CODE } = await import("./aos-facade");

beforeEach(() => invoke.mockReset());

describe("flattenArgs", () => {
  it("funde params, query e body numa carga única", () => {
    expect(flattenArgs({ params: { task: "t-1" }, query: { limit: 10 }, body: { title: "x" } }))
      .toEqual({ task: "t-1", limit: 10, title: "x" });
  });

  it("não deixa `enabled` vazar para a carga", () => {
    expect(flattenArgs({ query: { limit: 5 }, enabled: false })).toEqual({ limit: 5 });
  });

  it("devolve objeto vazio quando não há argumentos", () => {
    expect(flattenArgs()).toEqual({});
  });
});

describe("call", () => {
  it("traduz o nome e injeta _reasoning", async () => {
    invoke.mockResolvedValue({ tasks: [] });
    await call("task", "list", { query: { limit: 10 } });
    expect(invoke).toHaveBeenCalledWith("tasks_list", {
      limit: 10,
      _reasoning: "interface: task.list",
    });
  });

  it("resolve o renome kebab-case", async () => {
    invoke.mockResolvedValue({ task: {} });
    await call("task", "setStatus", { params: { id: "t-1" }, body: { status: "todo" } });
    expect(invoke).toHaveBeenCalledWith("tasks_set-status", {
      id: "t-1", status: "todo", _reasoning: "interface: task.setStatus",
    });
  });

  it("devolve o payload sob data, preservando a chave do domínio", async () => {
    invoke.mockResolvedValue({ tasks: [{ id: "t-1" }], total: 1 });
    const r = await call("task", "list");
    expect(r.data).toEqual({ tasks: [{ id: "t-1" }], total: 1 });
    expect(r.error).toBeUndefined();
  });

  it("converte exceção em envelope, sem propagar", async () => {
    invoke.mockRejectedValue(Object.assign(new Error("recusado"), { code: "AOS_TASK_BLOCKED" }));
    const r = await call("task", "list");
    expect(r.data).toBeUndefined();
    expect(r.error?.code).toBe("AOS_TASK_BLOCKED");
  });

  it("responde dormente sem tocar a rede", async () => {
    const r = await call("collection", "list");
    expect(invoke).not.toHaveBeenCalled();
    expect(r.data).toBeUndefined();
    expect(r.error?.code).toBe(DORMANT_CODE);
  });

  it("falha alto quando a chamada não está no mapa", async () => {
    await expect(call("inventada", "list")).rejects.toThrow(/não mapeada/);
  });
});

describe("api (o Proxy)", () => {
  it("expõe query, mutate, useQuery e useMutation em qualquer feature.ação", () => {
    const node = api.task.list;
    expect(typeof node.query).toBe("function");
    expect(typeof node.mutate).toBe("function");
    expect(typeof node.useQuery).toBe("function");
    expect(typeof node.useMutation).toBe("function");
  });

  it("query passa pela mesma tradução de call", async () => {
    invoke.mockResolvedValue({ tasks: [] });
    await api.task.list.query({ query: { limit: 3 } });
    expect(invoke).toHaveBeenCalledWith("tasks_list", {
      limit: 3, _reasoning: "interface: task.list",
    });
  });
});
```

- [ ] **Step 2: Rodar e confirmar que falha**

Run: `cd "$AOS" && npm test -- aos-facade`
Expected: FAIL — `Failed to resolve import "./aos-facade"`

- [ ] **Step 3: Escrever a fachada**

Criar `$AOS/src/lib/aos-facade.ts`:

```ts
import { useMutation, useQuery, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import { client } from "./client";
import { COMMAND_MAP, type MapEntry } from "./command-map";
import type { CommandKey } from "./schema";

/** O código que a UI reconhece como "o Go ainda não tem isto". */
export const DORMANT_CODE = "AOS_DOMAIN_DORMANT";

export interface EnvelopeError {
  code: string;
  message: string;
}

/**
 * O formato que 117 sites no front já sabem ler.
 *
 * `lib/client.ts` lança `DomainError`; esta fachada inverte a convenção e
 * devolve o erro como valor. É deliberado: o código portado escreve
 * `const { error } = await client.task.update.mutate(...)` sem try/catch, e
 * uma exceção que escape daqui vira tela branca em vez de mensagem.
 */
export interface Envelope<T> {
  data: T | undefined;
  error: EnvelopeError | undefined;
}

export interface CallOpts {
  query?: Record<string, unknown>;
  params?: Record<string, unknown>;
  body?: Record<string, unknown>;
  enabled?: boolean;
}

/**
 * O Fractal separa params/query/body porque falava com uma API REST. O
 * registry do Go recebe um objeto plano por comando, então os três se fundem.
 * `enabled` é do useQuery e não pertence à carga.
 */
export function flattenArgs(opts?: CallOpts): Record<string, unknown> {
  return { ...opts?.params, ...opts?.query, ...opts?.body };
}

function toEnvelopeError(err: unknown): EnvelopeError {
  if (err && typeof err === "object" && "code" in err) {
    const e = err as { code?: unknown; message?: unknown };
    return { code: String(e.code ?? "UNKNOWN"), message: String(e.message ?? "a chamada falhou") };
  }
  return { code: "UNKNOWN", message: err instanceof Error ? err.message : "a chamada falhou" };
}

/**
 * O ponto único por onde passam os 207 call sites do front portado.
 */
export async function call(feature: string, action: string, opts?: CallOpts): Promise<Envelope<unknown>> {
  const path = `${feature}.${action}`;
  const entry: MapEntry | undefined = COMMAND_MAP[path];

  // Chave ausente é erro de programação, não estado do backend: alguém
  // escreveu uma chamada nova sem registrá-la. Falha alto, de propósito.
  if (entry === undefined) {
    throw new Error(`chamada não mapeada: ${path} — registre-a em lib/command-map.ts`);
  }

  if (entry === null) {
    return { data: undefined, error: { code: DORMANT_CODE, message: `o domínio "${feature}" ainda não existe no backend Go` } };
  }

  const payload = flattenArgs(opts);
  try {
    const data = typeof entry === "function"
      ? await entry(payload)
      : await client.invoke(entry as CommandKey, { ...payload, _reasoning: `interface: ${path}` } as never);
    return { data, error: undefined };
  } catch (err) {
    return { data: undefined, error: toEnvelopeError(err) };
  }
}

interface ActionNode {
  query(opts?: CallOpts): Promise<Envelope<unknown>>;
  mutate(opts?: CallOpts): Promise<Envelope<unknown>>;
  useQuery(opts?: CallOpts): UseQueryResult<unknown>;
  useMutation(): UseMutationResult<Envelope<unknown>, Error, CallOpts | undefined>;
}

export type AosClient = Record<string, Record<string, ActionNode>>;

function actionNode(feature: string, action: string): ActionNode {
  return {
    query: (opts) => call(feature, action, opts),
    mutate: (opts) => call(feature, action, opts),

    // O front lê `q.data?.tasks`, não `q.data.data.tasks` — então o hook
    // desembrulha o envelope e entrega o payload direto, que é o que o
    // código portado espera de um useQuery.
    useQuery: (opts) =>
      useQuery({
        queryKey: [feature, action, flattenArgs(opts)],
        queryFn: async () => {
          const r = await call(feature, action, opts);
          if (r.error && r.error.code !== DORMANT_CODE) throw r.error;
          return r.data ?? undefined;
        },
        enabled: opts?.enabled ?? true,
        // Dormente não melhora tentando de novo.
        retry: false,
      }),

    useMutation: () =>
      useMutation({
        mutationFn: (opts?: CallOpts) => call(feature, action, opts),
      }),
  };
}

/**
 * `api.task.list.useQuery()` — a fachada que o código do Fractal espera.
 *
 * Dois níveis de Proxy em vez de um objeto gerado: o mapa tem 113 entradas
 * hoje e cresce a cada domínio que o Go ganha; gerar o objeto obrigaria a
 * mantê-lo em dois lugares.
 */
export const api: AosClient = new Proxy({} as AosClient, {
  get: (_t, feature: string) =>
    new Proxy({} as Record<string, ActionNode>, {
      get: (_t2, action: string) => actionNode(feature, action),
    }),
});
```

- [ ] **Step 4: Rodar e confirmar que passa**

Run: `cd "$AOS" && npm test`
Expected: PASS — todos os testes de `command-map` e `aos-facade`

- [ ] **Step 5: Confirmar que a ilha estrita aceita a fachada**

Run: `cd "$AOS" && npm run typecheck:strict`
Expected: sem erros

- [ ] **Step 6: Commit**

```bash
cd "$AOS/.." && git add frontend/src/lib/aos-facade.ts frontend/src/lib/aos-facade.test.ts
git commit -m "feat(frontend): add the facade that speaks Fractal and calls the Go registry"
```

---

### Task 4: O painel de domínio dormente

**Files:**
- Create: `$AOS/src/components/DormantDomain.tsx`

**Interfaces:**
- Consumes: `isDormant` de `@/lib/command-map`
- Produces: `<DormantDomain feature="collection" />`, `<DormantGate feature="collection">{children}</DormantGate>`

- [ ] **Step 1: Escrever o componente**

Criar `$AOS/src/components/DormantDomain.tsx`:

```tsx
import type { JSX, ReactNode } from "react";
import { isDormant } from "@/lib/command-map";

/**
 * O que uma tela mostra quando o domínio dela ainda não existe em Go.
 *
 * Sem isto, um domínio dormente rende uma tela vazia — indistinguível de um
 * defeito para quem abre. O painel é o que torna a degradação honesta: diz
 * que a interface está adiante do backend, o que é verdade e é deliberado.
 */
export function DormantDomain({ feature }: { feature: string }): JSX.Element {
  return (
    <div className="flex h-full min-h-64 w-full items-center justify-center p-8">
      <div className="max-w-md rounded-lg border border-dashed border-[var(--border)] bg-[var(--bg-subtle)] p-6 text-center">
        <p className="text-sm font-medium text-[var(--fg)]">
          Domínio ainda não disponível
        </p>
        <p className="mt-2 text-sm text-[var(--fg-muted)]">
          A interface de <code className="font-mono">{feature}</code> já existe, mas o
          backend Go ainda não publica esse domínio. A tela acende sozinha quando ele for
          implementado.
        </p>
      </div>
    </div>
  );
}

/** Envolve uma rota: mostra o painel se o domínio dorme, o conteúdo se não. */
export function DormantGate({ feature, children }: { feature: string; children: ReactNode }): JSX.Element {
  if (isDormant(feature)) return <DormantDomain feature={feature} />;
  return <>{children}</>;
}
```

- [ ] **Step 2: Verificar que compila**

Run: `cd "$AOS" && npm run typecheck`
Expected: sem erros novos vindos de `DormantDomain.tsx`

- [ ] **Step 3: Commit**

```bash
cd "$AOS/.." && git add frontend/src/components/DormantDomain.tsx
git commit -m "feat(frontend): show an honest panel where a domain has no Go backend yet"
```

---

### Task 5: Portar os builders e reconstruir `types.ts`

**Files:**
- Create: `$AOS/src/app/builders/{index,layout,middleware,page,response,router,store,trigger}.ts(x)` — copiados
- Create: `$AOS/src/app/builders/types.ts` — reconstruído

**Interfaces:**
- Consumes: `@tanstack/react-router`, `zod`
- Produces: `AosApp`, `AosPage`, `AosLayout`, `AosMiddleware`, `AosRouter`, `AosStore`, `AosTrigger`, `AosResponse` e os 16 tipos

- [ ] **Step 1: Copiar os 9 arquivos de builder**

```bash
mkdir -p "$AOS/src/app/builders"
cp "$EXT/v401/web/src/@app/builders/"*.ts "$EXT/v401/web/src/@app/builders/"*.tsx "$AOS/src/app/builders/"
ls "$AOS/src/app/builders"
```

Esperado: `app.tsx index.ts layout.ts middleware.ts page.ts response.ts router.ts store.ts trigger.ts` (9 arquivos, 1919 linhas)

- [ ] **Step 2: Reescrever imports e renomear o namespace**

```bash
cd "$AOS/src/app/builders"
# paths
perl -pi -e 's{\@app/}{\@/}g' *.ts *.tsx
# o pacote não existe aqui: a fila de query é a do TanStack
perl -pi -e 's{import \{ useIgniterQueryClient \} from "\@igniter-js/core/client";}{import { useQueryClient } from "\@tanstack/react-query";}g' app.tsx
perl -pi -e 's{useIgniterQueryClient}{useQueryClient}g' app.tsx
# o router tipado do Igniter não existe: a fachada é o cliente
perl -pi -e 's{^import type \{ AppRouterType \} from "\@/igniter.router";\n}{}mg' app.tsx
perl -pi -e 's{AppRouterType}{unknown}g' app.tsx
# namespace
perl -pi -e 's{\bIgniter}{Aos}g' *.ts *.tsx
perl -pi -e 's{\bIAos}{IAos}g' *.ts *.tsx
grep -rn "igniter\|Igniter" . || echo "sem resíduo de Igniter"
```

- [ ] **Step 3: Escrever o `types.ts` reconstruído**

O arquivo original era type-only e o bundler o apagou — não existe em nenhuma
extração. Os builders já usam `any` internamente (`_use: any[]`,
`_loader?: any`), então generics pragmáticos bastam: o critério é as 21 páginas
compilarem e rodarem.

Criar `$AOS/src/app/builders/types.ts`:

```ts
import type { AnyRoute, RouteComponent } from "@tanstack/react-router";
import type { ReactNode } from "react";

/**
 * O contrato interno dos builders, reconstruído.
 *
 * O original era type-only e não sobreviveu ao bundle de onde esta interface
 * foi extraída. Isto não reproduz a tipagem original — reproduz o que os
 * 1919 linhas de builder efetivamente exigem. Frouxo de propósito: apertar
 * aqui só produziria erros em código portado que já roda.
 */

export type DefaultContext = Record<string, unknown>;

export interface AosStoresCollection<T = Record<string, unknown>> {
  [key: string]: T[keyof T] | unknown;
}

export interface AosAppConfig<TClient = unknown, TContext = DefaultContext, TStores = unknown, TTrigger = unknown> {
  client?: TClient;
  context?: TContext | ((args: { stores: TStores }) => TContext);
  stores?: TStores;
  triggers?: TTrigger;
  layout?: RouteComponent;
  notFoundComponent?: RouteComponent;
  defaultPreload?: "intent" | "render" | "viewport" | false;
  onStoresReady?: (args: { stores: TStores }) => Promise<void> | void;
  beforePageLoad?: (args: PageLifecycleArgs<TClient, TContext, TStores>) => Promise<void> | void;
  onPageLoad?: (args: PageLifecycleArgs<TClient, TContext, TStores>) => Promise<void> | void;
}

export interface PageLifecycleArgs<TClient = unknown, TContext = DefaultContext, TStores = unknown> {
  client: TClient;
  context: TContext;
  stores: TStores;
  request: { url: string; query: Record<string, unknown>; params: Record<string, string> };
  response: unknown;
  page: unknown;
}

export interface AosAppBuilt<TClient = unknown, TContext = DefaultContext, TStores = unknown, TTrigger = unknown> {
  client: TClient;
  context: TContext;
  stores: TStores;
  triggers: TTrigger;
  config: AosAppConfig<TClient, TContext, TStores, TTrigger>;
  page(path: string): unknown;
  layout(path: string): unknown;
  router(routes: AnyRoute[]): unknown;
}

export interface IAosAppBuilder<TClient = unknown, TContext = DefaultContext, TStores = unknown, TTrigger = unknown> {
  withClient(client: unknown): IAosAppBuilder<any, TContext, TStores, TTrigger>;
  withStores(stores: unknown, onReady?: (args: { stores: any }) => Promise<void> | void): IAosAppBuilder<TClient, TContext, any, TTrigger>;
  withContext(factory: (args: { stores: TStores }) => unknown): IAosAppBuilder<TClient, any, TStores, TTrigger>;
  withTriggers(triggers: unknown): IAosAppBuilder<TClient, TContext, TStores, any>;
  withLayout(component: RouteComponent): IAosAppBuilder<TClient, TContext, TStores, TTrigger>;
  withNotFoundComponent(component: RouteComponent): IAosAppBuilder<TClient, TContext, TStores, TTrigger>;
  withDefaultPreload(mode: "intent" | "render" | "viewport" | false): IAosAppBuilder<TClient, TContext, TStores, TTrigger>;
  build(): AosAppBuilt<TClient, TContext, TStores, TTrigger>;
}

/** O que `withComponent(({ route }) => …)` recebe. */
export interface RouteContextAPI<TLoaderData = unknown, TParams = Record<string, string>, TSearch = Record<string, unknown>> {
  useLoaderData(): TLoaderData;
  useParams(): TParams;
  useSearch(): TSearch;
  refresh(): void | Promise<void>;
  navigate(opts: { to: string; params?: Record<string, unknown>; search?: Record<string, unknown> }): void | Promise<void>;
}

export type InferContextIn<T> = T extends (args: infer A) => unknown ? A : never;
export type MutationPath = string;

export interface AosTriggerDef {
  name: string;
  handler: (...args: any[]) => unknown;
  [key: string]: unknown;
}

export interface AosTriggerAPI {
  [name: string]: (...args: any[]) => unknown;
}

export interface AosTriggerHookResult {
  trigger: (...args: any[]) => unknown;
  isPending: boolean;
  error: unknown;
}

export interface IAosTriggerBuilt<TClient = unknown, TContext = DefaultContext, TStores = unknown> {
  client?: TClient;
  context?: TContext;
  stores?: TStores;
  [name: string]: unknown;
}

export interface IAosTriggerGroupBuilt<TClient = unknown, TContext = DefaultContext, TStores = unknown>
  extends IAosTriggerBuilt<TClient, TContext, TStores> {}

export type AosAppTriggerOnSearchCallback = (query: string) => Promise<unknown[]> | unknown[];

export interface AosUseFormOptions<TValues = Record<string, unknown>> {
  defaultValues?: Partial<TValues>;
  onSubmit?: (values: TValues) => Promise<unknown> | unknown;
  onSuccess?: (result: unknown) => void;
  onError?: (error: unknown) => void;
}

export interface AosFormReturn<TValues = Record<string, unknown>> {
  values: TValues;
  setValue(name: keyof TValues, value: unknown): void;
  submit(): Promise<void>;
  isSubmitting: boolean;
  errors: Partial<Record<keyof TValues, string>>;
  children?: ReactNode;
}
```

- [ ] **Step 4: Verificar que os builders compilam**

Run: `cd "$AOS" && npx tsc --noEmit --skipLibCheck src/app/builders/*.ts src/app/builders/*.tsx 2>&1 | head -30`

Erros esperados: apenas de módulos ainda não portados (`@/features/...`,
`@/lib/stores`). Nenhum erro deve citar `./types` ou `Aos*`. Se algum tipo em
`types.ts` estiver faltando, adicioná-lo com a forma mais frouxa que satisfaça
o uso, e repetir.

- [ ] **Step 5: Commit**

```bash
cd "$AOS/.." && git add frontend/src/app/builders
git commit -m "feat(frontend): port the route builders and rebuild their erased type contract"
```

---

### Task 6: Fatia vertical — a feature `task` ponta a ponta

Esta task existe para provar o encanamento inteiro numa feature só, antes de
copiar as outras 25. `task` é a escolha certa: é a feature mais rica (41
arquivos), usa `withLoader`, `useQuery`, `mutate`, dropdowns, kanban e lista, e
tem backend Go vivo — se ela funciona, o padrão está validado.

**Files:**
- Create: `$AOS/src/features/task/**` (41 arquivos, menos `errors/`)
- Create: `$AOS/src/features/task/interfaces/{task,comment,todo}.interfaces.ts`
- Create: `$AOS/src/app/aos.tsx`
- Modify: `$AOS/src/router.tsx`

**Interfaces:**
- Consumes: `api` de `@/lib/aos-facade`; `AosApp` de `@/app/builders`
- Produces: `aos` (a instância da app) exportada de `@/app/aos`; `FractalTask`, `FractalTaskStatus`, `FractalTodo`, `FractalComment`

- [ ] **Step 1: Copiar a feature, sem `errors/`**

```bash
mkdir -p "$AOS/src/features/task"
rsync -a --exclude 'errors/' "$EXT/v401/web/src/features/task/" "$AOS/src/features/task/"
find "$AOS/src/features/task" -type f | wc -l
```

Esperado: 37 arquivos (41 menos os 4 de `errors/`).

- [ ] **Step 2: Recuperar as interfaces de `_extracted/index/`**

`task.interfaces.ts` traz `FractalTask`, `FractalTodo` e `FractalComment` juntos;
o front espera três caminhos. Reexportar em vez de dividir mantém uma fonte só.

```bash
mkdir -p "$AOS/src/features/task/interfaces"
cp "$EXT/index/src/features/task/task.interfaces.ts" "$AOS/src/features/task/interfaces/task.interfaces.ts"
cat > "$AOS/src/features/task/interfaces/todo.interfaces.ts" <<'EOF'
// FractalTodo é declarado junto de FractalTask na fonte. Reexporta em vez de
// duplicar: uma definição, os dois caminhos que o front importa.
export type { FractalTodo, FractalTodoStatus } from "./task.interfaces";
EOF
cat > "$AOS/src/features/task/interfaces/comment.interfaces.ts" <<'EOF'
export type { FractalComment } from "./task.interfaces";
EOF
```

Se `FractalTodoStatus` não existir em `task.interfaces.ts`, remover o nome do
reexport — conferir com `grep -n "export .*FractalTodo" "$AOS/src/features/task/interfaces/task.interfaces.ts"`.

- [ ] **Step 3: Reescrever os imports da feature**

```bash
cd "$AOS/src/features/task"
perl -pi -e 's{\@app/}{\@/}g' $(find . -type f \( -name '*.ts' -o -name '*.tsx' \))
perl -pi -e 's{\@/\@core/}{\@/core/}g' $(find . -type f \( -name '*.ts' -o -name '*.tsx' \))
perl -pi -e 's{from "\@/igniter"}{from "\@/app/aos"}g' $(find . -type f \( -name '*.ts' -o -name '*.tsx' \))
perl -pi -e 's{\bigniter\.}{aos.}g' $(find . -type f \( -name '*.ts' -o -name '*.tsx' \))
perl -pi -e 's{\bigniter\b}{aos}g' $(find . -type f \( -name '*.ts' -o -name '*.tsx' \))
grep -rn "igniter" . || echo "sem resíduo"
```

- [ ] **Step 4: Escrever a instância da app**

Criar `$AOS/src/app/aos.tsx`. A versão do Fractal registrava 17 stores; aqui
começa com o mínimo que a fatia vertical precisa, e as demais entram na Task 10.

```tsx
import { AosApp } from "./builders";
import { api } from "@/lib/aos-facade";

/**
 * A instância que as páginas portadas consomem como `aos.page(...)` e
 * `aos.client.task.list.useQuery()`.
 *
 * O `client` aqui é a fachada, não `lib/client.ts`: as páginas foram escritas
 * contra a forma `client.<feature>.<ação>.<método>`, e é a fachada que a
 * apresenta.
 */
export const aos = AosApp.create()
  .withClient(api)
  .withDefaultPreload("intent")
  .build();
```

- [ ] **Step 5: Ligar as rotas de task no router atual**

Em `$AOS/src/router.tsx`, remover o import de `TaskBoard` e a rota que o usa,
e passar a montar as páginas portadas. Manter o resto do router intacto — a
árvore completa do Fractal entra na Task 10.

```tsx
import { TasksPage } from "@/features/task/presentation/pages/(main)";
import { TaskDetailsPage } from "@/features/task/presentation/pages/($id)";
```

Registrar `TasksPage` e `TaskDetailsPage` na árvore de rotas no lugar da rota
antiga de `/tasks`.

- [ ] **Step 6: Compilar e corrigir**

Run: `cd "$AOS" && npm run typecheck 2>&1 | head -40`

Erros esperados e como tratá-los:
- `Cannot find module '@/features/<outra>/...'` — a feature `task` importa
  vizinhas (`goal`, `project`, `chat`). **Não copiar essas features agora.**
  Criar stubs de tipo mínimos apenas para os tipos citados, cada um com este
  marcador exato na primeira linha — é o que a Task 9 procura para removê-los:

```ts
// stub temporário — remover na Task 9, quando a feature real for copiada
export interface FractalGoal { id: string; title: string }
```
- `Cannot find module '@/components/ui/<nome>'` — se for um dos 5 ausentes,
  copiar de `$EXT/v401/web/src/@app/components/ui/`.

- [ ] **Step 7: Rodar o app e percorrer a tela**

```bash
cd "$AOS/.." && task dev
```

Abrir `/tasks`. Confirmar, nesta ordem:
1. A lista carrega e mostra tasks reais do backend Go.
2. O detalhe de uma task abre (prova que `withLoader` + `task.getById` funcionam).
3. Mudar o status de uma task persiste (prova `tasks_set-status`, incluindo o kebab-case).
4. Na aba Network, a requisição é `POST /api/tasks/list` com `_reasoning` no corpo.

Se o item 4 falhar por `_reasoning` ausente, o defeito está na fachada, não na
página.

- [ ] **Step 8: Commit**

```bash
cd "$AOS/.." && git add frontend/src/features/task frontend/src/app/aos.tsx frontend/src/router.tsx
git commit -m "feat(frontend): port the task feature end to end, proving the facade"
```

---

### Task 7: Recuperar as 17 interfaces restantes e os arquivos `@core`

**Files:**
- Create: `$AOS/src/features/<d>/interfaces/<d>.interfaces.ts` × 17
- Create: `$AOS/src/core/helpers/{request-context.ts,schema.helper.ts}`
- Create: `$AOS/src/core/services/{activity.ts,store.ts}`
- Create: `$AOS/src/core/interfaces/response.interfaces.ts` — reconstruído

**Interfaces:**
- Produces: `Fractal*` para agent, chat, file, routine, goal, project, workspace, model, collection, memory, skill, template, theme, toolset, view, instruction, auth
- Produces: `ResponseWithCTA<T>`, `Schema`

> **Correções aplicadas na varredura pré-execução.** Os arquivos do `index/` são
> a fonte **server-side**, não a do frontend. Cinco problemas foram identificados
> e resolvidos nos passos abaixo; não os redescubra:
> 1. `config` **sai** da lista — a versão boa é a de `v401/web`, que a Task 9
>    traz. A do `index/` importa `@igniter-js/core`.
> 2. 15 dos 17 importam `ResponseWithCTA` de um arquivo que **não existe em
>    nenhuma extração** — Step 2 o reconstrói.
> 3. Quase todos importam `Schema` de `@core/helpers/schema.helper`, **ausente
>    no AOS** — Step 1 o recupera.
> 4. `collection` e `view` importam tipos de `@igniter-js/collections` — Step 4.
> 5. `agent.interfaces.ts` importa `node:child_process` e serviços de backend —
>    Step 5 poda. É a única poda manual autorizada nesta task.

- [ ] **Step 1: Copiar as 17 interfaces e os arquivos `@core` de apoio**

A extração `index/` guarda os arquivos num caminho mais raso
(`features/<d>/<d>.interfaces.ts`); o front espera
`features/<d>/interfaces/<d>.interfaces.ts`. `config` não entra — ver a nota
da task.

```bash
for d in agent chat file routine goal project workspace model collection memory skill template theme toolset view instruction auth; do
  src="$EXT/index/src/features/$d/$d.interfaces.ts"
  if [ -f "$src" ]; then
    mkdir -p "$AOS/src/features/$d/interfaces"
    cp "$src" "$AOS/src/features/$d/interfaces/$d.interfaces.ts"
    echo "OK   $d"
  else
    echo "FALTA $d"
  fi
done
```

Esperado: 17 linhas `OK`, nenhuma `FALTA`.

```bash
mkdir -p "$AOS/src/core/helpers" "$AOS/src/core/services" "$AOS/src/core/interfaces"
cp "$EXT/index/src/@core/helpers/request-context.ts" "$AOS/src/core/helpers/"
cp "$EXT/index/src/@core/services/activity.ts" "$AOS/src/core/services/"
cp "$EXT/index/src/@core/services/store.ts" "$AOS/src/core/services/"
# schema.helper vem da árvore do front, não do index/
cp "$EXT/v401/web/src/@core/helpers/schema.helper.ts" "$AOS/src/core/helpers/"
```

- [ ] **Step 2: Reconstruir `response.interfaces.ts`**

15 dos 17 arquivos importam `ResponseWithCTA` daqui, e o arquivo não existe em
nenhuma extração — era type-only. Verificado: o tipo só aparece em interfaces
`IFractal*Service`, que são contratos de serviço server-side; o frontend chama
o domínio pela fachada e nunca implementa nem invoca essas interfaces.

Criar `$AOS/src/core/interfaces/response.interfaces.ts`:

```ts
/**
 * Reconstruído: o arquivo era type-only e nenhuma extração o preservou.
 *
 * `ResponseWithCTA` aparece exclusivamente nas interfaces `IFractal*Service`,
 * que descrevem os serviços do backend original. O frontend não as implementa
 * nem as chama — fala com o domínio pela fachada. O tipo existe aqui para que
 * os 15 arquivos recuperados compilem sem serem editados: pruná-los seria 15
 * divergências à mão contra a fonte, que é o que a spec manda evitar.
 */
export interface ResponseCTA {
  label: string;
  command?: string;
  tool?: string;
}

export interface ResponseWithCTA<TData> {
  data?: TData;
  error?: { code?: string; message?: string };
  cta?: ResponseCTA[];
}
```

- [ ] **Step 3: Reescrever os imports**

```bash
cd "$AOS/src"
FILES=$(find features core -name '*.ts')
perl -pi -e 's{\@/\@core/}{\@/core/}g' $FILES
perl -pi -e 's{from "\@/features/([a-z]+)/\1\.interfaces"}{from "\@/features/$1/interfaces/$1.interfaces"}g' $FILES
# os recuperados referenciam vizinhos por caminho relativo raso
perl -pi -e 's{from "\.\./([a-z]+)/\1\.interfaces"}{from "\@/features/$1/interfaces/$1.interfaces"}g' $FILES
```

- [ ] **Step 4: Substituir os tipos de `@igniter-js/collections`**

`collection` e `view` importam dois tipos de um pacote que não instalamos.
Ambos os domínios estão dormentes, então o contrato mínimo basta.

Acrescentar ao topo de `$AOS/src/features/collection/interfaces/collection.interfaces.ts`,
removendo a linha `import type { IIgniterCollectionModel } from "@igniter-js/collections";`:

```ts
/** Era `IIgniterCollectionModel`. Domínio dormente — contrato mínimo. */
export interface IIgniterCollectionModel {
  name: string;
  fields?: Record<string, unknown>;
  [key: string]: unknown;
}
```

Idem em `$AOS/src/features/view/interfaces/view.interfaces.ts`, removendo
`import type { IgniterCollectionViewDefinition } from "@igniter-js/collections";`:

```ts
/** Era `IgniterCollectionViewDefinition`. Domínio dormente — contrato mínimo. */
export interface IgniterCollectionViewDefinition {
  name: string;
  tree?: unknown;
  [key: string]: unknown;
}
```

- [ ] **Step 5: Podar o bloco server-side de `agent.interfaces.ts`**

É o único arquivo recuperado que traz dependência de runtime Node para dentro
do bundle do navegador. Remover de
`$AOS/src/features/agent/interfaces/agent.interfaces.ts`:

- `import type { SpawnOptions } from "node:child_process";`
- `import type { BackgroundJobs } from "./services/jobs.service";`
- qualquer `import ... from "./services/browser.service";`
- `import { IgniterLogger } from "@igniter-js/core";`
- a interface `IFractalAgentService` e quaisquer tipos que existam apenas para
  ela e citem os símbolos acima

Manter tudo o mais: as entidades (`FractalAgent` e companhia) e seus schemas
são o que a UI importa.

Confirmar:

```bash
grep -nE "node:|@igniter-js|services/" "$AOS/src/features/agent/interfaces/agent.interfaces.ts" || echo "limpo"
```

- [ ] **Step 6: Verificar que os tipos resolvem entre si**

```bash
cd "$AOS" && grep -rn "@igniter-js" src/features src/core && echo "FALHOU: resíduo" || echo "OK: sem Igniter"
npx tsc --noEmit --skipLibCheck $(find src/features src/core -name '*.ts') 2>&1 | grep -v "Cannot find module '@/features" | head -20
```

Erros restantes devem ser só de módulos ainda não portados. Qualquer erro de
sintaxe ou tipo dentro dos próprios arquivos precisa ser corrigido aqui.

Não corrigir erros vindos do código AOS antigo de `features/chat` e
`features/agent`: os tipos recuperados usam `FractalAgent`/`FractalChat` e o
código antigo usa `Agent`/`Participant`. A Task 9 substitui esses
consumidores; a janela quebrada entre as duas tasks é esperada.

- [ ] **Step 7: Commit**

```bash
cd "$AOS/.." && git add frontend/src/features frontend/src/core
git commit -m "feat(frontend): recover the domain interfaces the bundler erased"
```

---

### Task 8: Reconstruir os 8 arquivos sem fonte

**Files:**
- Create: `$AOS/src/features/workspace/interfaces/directory.interfaces.ts`
- Create: `$AOS/src/features/activity/interfaces/activity.interfaces.ts`
- Create: `$AOS/src/features/marketplace/interfaces/marketplace.interfaces.ts`
- Create: `$AOS/src/features/artifact/interfaces/artifact.interfaces.ts`
- Create: `$AOS/src/features/auth/interfaces/user.interfaces.ts`
- Create: `$AOS/src/features/routine/interfaces/run.interfaces.ts`
- Create: `$AOS/src/features/chat/presentation/helpers/chat-kind.helper.ts`
- Create: `$AOS/src/core/builders/notification.ts`

**Interfaces:**
- Produces: `FractalDirectory`, `FractalActivity`, `FractalMarketplaceItem`, `FractalArtifact`, `FractalUserPublic`, `FractalUserUpdateMeInput`, `FractalRoutineRun`, `NotificationPayload`, e os helpers de `chat-kind`

Estes 8 não existem em nenhuma extração. Reconstruí-los é ler os usos e
declarar exatamente os campos tocados — nem mais, nem menos. Um campo inventado
a mais é uma mentira que o compilador vai aceitar.

- [ ] **Step 1: Levantar o uso real de cada tipo**

Para cada tipo, listar os campos efetivamente acessados:

```bash
cd "$EXT/v401/web/src"
for t in FractalDirectory FractalActivity FractalMarketplaceItem FractalArtifact FractalUserPublic FractalRoutineRun NotificationPayload; do
  echo "=== $t ==="
  grep -rhoE "\b[a-z][a-zA-Z]*\s*[:=][^;]*$t\b" --include='*.tsx' --include='*.ts' features | head -5
  var=$(grep -rlE "\b$t\b" --include='*.tsx' --include='*.ts' features | head -3)
  for f in $var; do grep -ohE "\b(activity|item|artifact|user|run|dir|payload)\.[a-zA-Z]+" "$f" | sort -u | head -12; done
done
```

- [ ] **Step 2: Escrever os dois que o Go verifica**

`activity` e `routine.run` são domínios vivos: o struct Go é a verdade, e o
tipo deve espelhá-lo campo a campo. Ambos já foram conferidos contra
`internal/domain/activity/entity.go:21` e `internal/domain/routine/entity.go:181`.

Criar `$AOS/src/features/activity/interfaces/activity.interfaces.ts`:

```ts
/**
 * Espelha internal/domain/activity/entity.go — o arquivo original era
 * type-only e não sobreviveu ao bundle, mas este domínio tem backend, então
 * a reconstrução é verificável e não adivinhada.
 */
export type FractalActivityActorType = "agent" | "user" | "system";

export interface FractalActivity {
  id: string;
  namespace: string;
  event: string;
  title: string;
  body?: string;
  icon?: string;
  /** Moldado pelo namespace: um evento de task carrega a task. */
  data?: Record<string, unknown>;
  actor: string;
  actorType: FractalActivityActorType;
  createdAt: string;
}

export interface FractalActivityList {
  activities: FractalActivity[];
  total: number;
  unread: number;
  actor: string;
}
```

Criar `$AOS/src/features/routine/interfaces/run.interfaces.ts`:

```ts
/** Espelha internal/domain/routine/entity.go (type Run). */
export type FractalRunStatus = "running" | "succeeded" | "failed" | "timed_out" | "skipped";

export interface FractalRoutineRun {
  agent: string;
  routine: string;
  id: string;
  trigger: string;
  payload?: Record<string, unknown>;
  chatId?: string;
  status: FractalRunStatus;
  startedAt: string;
  endedAt?: string;
  error?: string;
}

export interface FractalRoutineRunList {
  routine: string;
  runs: FractalRoutineRun[];
}
```

- [ ] **Step 3: Escrever os seis restantes a partir dos usos**

`directory`, `marketplace`, `artifact`, `user`, `chat-kind` e `notification`
não têm backend Go contra o qual conferir. Declarar exatamente os campos que o
Step 1 mostrou serem acessados — nem mais, nem menos. Um campo inventado a mais
é uma afirmação sobre o backend que ninguém verificou, e o compilador a aceita
em silêncio.

Cabeçalho obrigatório em cada um:

```ts
/**
 * Reconstruído a partir dos usos: o arquivo era type-only e o bundler o
 * apagou. Não há backend Go para este domínio ainda — quando houver, este
 * contrato passa a ser verificável e deve ser conferido contra ele.
 */
```

- [ ] **Step 4: Verificar**

Run: `cd "$AOS" && npm run typecheck 2>&1 | grep -E "interfaces|chat-kind|notification" | head -20`
Expected: sem erros nesses arquivos

- [ ] **Step 5: Commit**

```bash
cd "$AOS/.." && git add frontend/src/features frontend/src/core
git commit -m "feat(frontend): rebuild the eight type files no extraction preserved"
```

---

### Task 9: Copiar as 25 features restantes

**Files:**
- Create: `$AOS/src/features/**` — todas as features menos `task` (já feita)
- Create: `$AOS/src/hooks/**`, `$AOS/src/app/lib/**`
- Create: os 5 componentes UI ausentes
- Delete: os stubs de tipo criados na Task 6, Step 6

**Interfaces:**
- Consumes: tudo das tasks 2–8
- Produces: a árvore completa de features

- [ ] **Step 1: Copiar as features, sem `errors/`**

```bash
cd "$EXT/v401/web/src/features"
for d in */; do
  d="${d%/}"
  [ "$d" = "task" ] && continue
  mkdir -p "$AOS/src/features/$d"
  rsync -a --exclude 'errors/' "$d/" "$AOS/src/features/$d/"
done
find "$AOS/src/features" -type f \( -name '*.ts' -o -name '*.tsx' \) | wc -l
```

`errors/` fica de fora: são 30 arquivos que importam `FractalError` de um
builder server-side acoplado ao `@igniter-js/core`, e nenhum é importado pela
UI. Copiá-los arrastaria de volta a dependência que decidimos não ter.

- [ ] **Step 2: Copiar hooks, lib e os 5 UI ausentes**

```bash
mkdir -p "$AOS/src/app/lib"
cp "$EXT/v401/web/src/@app/hooks/"*.ts "$AOS/src/hooks/"
cp "$EXT/v401/web/src/@app/lib/"{stores.ts,triggers.ts,tabs.ts,realtime.ts} "$AOS/src/app/lib/" 2>/dev/null
for f in font-family-selector goal-selector-dropdown project-selector-dropdown split-page-layout theme-provider; do
  cp "$EXT/v401/web/src/@app/components/ui/$f.tsx" "$AOS/src/components/ui/"
done
```

Não sobrescrever os arquivos de `lib/` que o AOS já tem (`utils.ts`, `blob.ts`,
`springs.ts`, `icon-map.tsx`, `suggestion.ts`, `font-weight.ts`,
`icon-context.tsx`, `shape-context.tsx`, `debounce.ts`,
`block-discussion-index.ts`, `unsaved-prompt.bridge.ts`): já estão portados com
os imports reescritos.

- [ ] **Step 3: Reescrita global de imports**

```bash
cd "$AOS/src"
FILES=$(find features hooks app components -type f \( -name '*.ts' -o -name '*.tsx' \))
perl -pi -e 's{\@app/}{\@/}g' $FILES
perl -pi -e 's{\@/\@core/}{\@/core/}g' $FILES
perl -pi -e 's{from "\@/igniter"}{from "\@/app/aos"}g' $FILES
perl -pi -e 's{from "\@/lib/stores"}{from "\@/app/lib/stores"}g' $FILES
perl -pi -e 's{from "\@/lib/triggers"}{from "\@/app/lib/triggers"}g' $FILES
perl -pi -e 's{from "\@/features/([a-z]+)/\1\.interfaces"}{from "\@/features/$1/interfaces/$1.interfaces"}g' $FILES
perl -pi -e 's{\bigniter\.}{aos.}g' $FILES
perl -pi -e 's{\bigniter\b}{aos}g' $FILES
grep -rn "@igniter-js\|@app/" . | head -20 || echo "sem resíduo"
```

- [ ] **Step 4: Soltar `use-realtime.ts` do `@igniter-js/store`**

`$AOS/src/hooks/use-realtime.ts` importa três tipos genéricos do pacote só para
a assinatura — o corpo já usa `any` e chama `realtime.on(event, handler)`, que é
código nosso. Substituir a assinatura por uma frouxa:

```ts
import { useEffect, useRef } from "react";
import { realtime } from "@/lib/realtime";

/**
 * Escuta um evento do canal realtime.
 *
 * A tipagem genérica do original derivava os nomes de evento do registry de
 * stores do Igniter. Sem esse pacote, o nome é uma string: o ganho de inferir
 * `"chat:refresh"` de um registry não paga trazer a dependência de volta.
 *
 * @example useRealtime("chat:refresh", (p) => { if (p.chatId === id) refetch(); }, [id])
 */
export function useRealtime(
  event: string,
  callback: (payload: any) => void,
  deps: React.DependencyList = [],
  enabled = true,
) {
```

Manter o corpo da função exatamente como veio — a lógica de `callbackRef`,
`useEffect` e `unsubscribe` não muda.

- [ ] **Step 5: Substituir os tipos de `@igniter-js/collections` na feature `view`**

`features/view/` importa `Spec` e `AosCollectionViewRenderResult` de um pacote
que não instalamos. `view` é dormente, então o contrato mínimo basta.

Criar `$AOS/src/features/view/interfaces/collections.interfaces.ts`:

```ts
/**
 * O que `features/view` usava de @igniter-js/collections, declarado aqui.
 *
 * O domínio view está dormente — o renderizador declarativo não tem backend
 * Go ainda. Quando tiver, este contrato passa a ser verificado contra ele;
 * por ora é o mínimo que faz a tela compilar.
 */
export interface Spec {
  name: string;
  fields?: Record<string, unknown>;
  [key: string]: unknown;
}

export interface CollectionViewRenderResult {
  tree?: unknown;
  data?: unknown;
  [key: string]: unknown;
}
```

Repontar os imports:

```bash
cd "$AOS/src/features/view"
perl -pi -e 's{from "\@igniter-js/collections"}{from "\@/features/view/interfaces/collections.interfaces"}g' $(find . -type f \( -name '*.ts' -o -name '*.tsx' \))
perl -pi -e 's{\bAosCollectionViewRenderResult\b}{CollectionViewRenderResult}g' $(find . -type f \( -name '*.ts' -o -name '*.tsx' \))
```

- [ ] **Step 6: Remover os stubs temporários da Task 6**

```bash
grep -rn "stub temporário\|TODO: Task 9" "$AOS/src/features/task" || echo "nenhum stub restante"
```

Apagar os que aparecerem — as features reais já estão no lugar.

- [ ] **Step 7: Compilar e corrigir em rodadas**

Run: `cd "$AOS" && npm run typecheck 2>&1 | tee /tmp/tsc.log | tail -5`
Run: `grep -oE "error TS[0-9]+" /tmp/tsc.log | sort | uniq -c | sort -rn | head`

Trabalhar por classe de erro, da mais frequente para a menos. Regra: corrigir no
menor número de arquivos possível. Se uma mesma classe de erro aparece em dezenas
de arquivos portados, a correção provavelmente pertence à fachada, a um tipo, ou
a uma flag do compilador — não a dezenas de edições.

- [ ] **Step 8: Commit**

```bash
cd "$AOS/.." && git add frontend/src
git commit -m "feat(frontend): copy the remaining Fractal features and rewrite their imports"
```

---

### Task 10: Shell, stores e a árvore de rotas

**Files:**
- Modify: `$AOS/src/app/aos.tsx` — registrar stores, triggers, layout
- Create: `$AOS/src/app/router.tsx` — a árvore do Fractal
- Modify: `$AOS/src/App.tsx`
- Delete: `$AOS/src/router.tsx`, `$AOS/src/features/task/TaskBoard.tsx`, `$AOS/src/features/memory/MemoryGraph.tsx`, `$AOS/src/features/file/{FilesPage,FileTree,MonacoViewer}.tsx`

**Interfaces:**
- Consumes: `aos` de `@/app/aos`; as páginas de `@/features/*/presentation/pages/*`
- Produces: `router` exportado de `@/app/router`

- [ ] **Step 1: Completar a instância da app**

Substituir `$AOS/src/app/aos.tsx` pela versão completa, espelhando
`$EXT/v401/web/src/@app/igniter.tsx`:

```tsx
import { AosApp } from "./builders";
import { api } from "@/lib/aos-facade";
import { stores } from "./lib/stores";
import { triggers } from "./lib/triggers";
import { WorkspaceLayout } from "@/features/workspace/presentation/components/layout";
import { NotFoundComponent } from "@/features/workspace/presentation/pages/not-found/404";

export const aos = AosApp.create()
  .withClient(api)
  .withStores(stores, async ({ stores }) => {
    await stores.workspace.init();
    await stores.namespace.set((current: Record<string, unknown>) => ({
      ...current,
      workspaceId: stores.workspace.state.current?.id,
    }));
  })
  .withContext(({ stores }) => ({
    config: stores.config.state,
    workspaces: stores.workspace.state,
  }))
  .withTriggers(triggers)
  .withLayout(WorkspaceLayout)
  .withNotFoundComponent(NotFoundComponent)
  .withDefaultPreload("intent")
  .build();
```

- [ ] **Step 2: Portar a árvore de rotas**

```bash
cp "$EXT/v401/web/src/@app/router.tsx" "$AOS/src/app/router.tsx"
cd "$AOS/src/app"
perl -pi -e 's{\@app/}{\@/}g' router.tsx
perl -pi -e 's{from "\@/igniter"}{from "./aos"}g' router.tsx
perl -pi -e 's{\bigniter\b}{aos}g' router.tsx
perl -pi -e 's{\bAosRouter\b}{AosRouter}g' router.tsx
```

- [ ] **Step 3: Envolver as rotas de domínio dormente**

Para cada página cujo domínio está em `DORMANT_DOMAINS` (collection, view,
skill, project, goal, instruction, template, toolset, marketplace, model,
tunnel, artifact, user, token), envolver o componente com `DormantGate`:

```tsx
import { DormantGate } from "@/components/DormantDomain";

// exemplo, na definição da rota de collections:
component: () => (
  <DormantGate feature="collection">
    <CollectionPage />
  </DormantGate>
),
```

- [ ] **Step 4: Apontar App.tsx para o novo router e remover as telas artesanais**

Em `$AOS/src/App.tsx`, trocar `import { router } from "@/router"` por
`import { router } from "@/app/router"`.

```bash
cd "$AOS/src"
rm -f router.tsx features/task/TaskBoard.tsx features/memory/MemoryGraph.tsx \
      features/file/FilesPage.tsx features/file/FileTree.tsx features/file/MonacoViewer.tsx
grep -rn "TaskBoard\|MemoryGraph\|FilesPage\|FileTree\|MonacoViewer" . | grep -v node_modules || echo "sem referências pendentes"
```

`features/file/{language,monaco-theme}.ts` ficam: são configuração do Monaco,
não telas, e a feature `file` portada os usa.

- [ ] **Step 5: Compilar**

Run: `cd "$AOS" && npm run typecheck`
Expected: sem erros

- [ ] **Step 6: Commit**

```bash
cd "$AOS/.." && git add -A frontend/src
git commit -m "feat(frontend): adopt the Fractal route tree and retire the hand-built screens"
```

---

### Task 11: Placeholders para os 9 assets ausentes

**Files:**
- Create: `$AOS/src/assets/placeholder.ts`
- Modify: os 9 arquivos que importam imagens ausentes

**Interfaces:**
- Produces: `PLACEHOLDER_IMAGE` — um data-URI 1×1 transparente

Os 9 assets não existem em nenhuma extração e o usuário confirmou não tê-los.

- [ ] **Step 1: Criar o placeholder**

```ts
// $AOS/src/assets/placeholder.ts

/**
 * Um pixel transparente, no lugar dos assets que não vieram na extração.
 *
 * Deliberadamente visível como ausência: um placeholder desenhado seria
 * confundido com decisão de design. Trocar por arte real é substituir este
 * import — as telas que o usam já estão corretas em tudo o mais.
 */
export const PLACEHOLDER_IMAGE =
  "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7";
```

- [ ] **Step 2: Localizar e repontar os imports**

```bash
cd "$AOS/src"
grep -rn "public/logo-icon\|public/backgrounds\|public/screenshots\|marketplace-prompts-bg" --include='*.tsx' --include='*.ts' .
```

Em cada ocorrência, trocar o import da imagem por
`import { PLACEHOLDER_IMAGE } from "@/assets/placeholder";` e usar
`PLACEHOLDER_IMAGE` no lugar da variável importada.

- [ ] **Step 3: Verificar**

Run: `cd "$AOS" && npm run typecheck && npm run build`
Expected: ambos sem erros

- [ ] **Step 4: Commit**

```bash
cd "$AOS/.." && git add frontend/src
git commit -m "chore(frontend): stand in for the nine assets the extraction lost"
```

---

### Task 12: Verificação final

**Files:** nenhum criado — esta task confirma o resultado

- [ ] **Step 1: Verificar que nenhum resíduo de Igniter sobrou**

```bash
cd "$AOS" && grep -rn "@igniter-js" src package.json && echo "FALHOU: resíduo encontrado" || echo "OK: sem Igniter"
```

- [ ] **Step 2: Rodar a bateria completa**

```bash
cd "$AOS" && npm test && npm run typecheck && npm run typecheck:strict && npm run build
```
Expected: os quatro passam

- [ ] **Step 3: Build do desktop**

```bash
cd "$AOS/.." && task build:desktop
```
Expected: binário produzido sem erro

- [ ] **Step 4: Percorrer os 12 domínios vivos**

```bash
cd "$AOS/.." && task dev
```

Confirmar em cada um que a tela carrega dados reais do backend Go:
chat, task, agent, memory, activity, config, theme, routine, todo, comment,
workspace, file.

- [ ] **Step 5: Confirmar a degradação nos 14 domínios dormentes**

Navegar até cada rota de collection, view, skill, project, goal, instruction,
template, toolset, marketplace, model, tunnel, artifact, user, token.

Cada uma deve exibir o painel de domínio dormente. **Nenhuma pode mostrar tela
branca, erro não tratado, ou spinner infinito** — os três são falhas do
contrato de degradação, não estados aceitáveis.

- [ ] **Step 6: Confirmar o `_reasoning` na rede**

Com o DevTools aberto, disparar uma ação em cada um de 3 domínios vivos
distintos. Cada `POST /api/<grupo>/<nome>` deve carregar `_reasoning` no corpo.

- [ ] **Step 7: Commit final**

```bash
cd "$AOS/.." && git add -A
git commit -m "feat(frontend): complete the Fractal interface port"
```

---

## Checklist de porte por feature

> Adicionado após a Task 6, a fatia vertical. Cada item existe porque a ausência
> dele produziu um defeito real ali. Os quatro Criticals da T6 teriam sido pegos
> só pelo item 4.

Para **cada** feature portada, nesta ordem:

1. **Copiar e reescrever imports** pelo script. Depois rodar o diff mecânico
   fonte-vs-porte e guardá-lo — é ele que revela a divergência, não a memória de
   quem portou. Relatório em prosa omite; foi verificado.
2. **Entradas.** Para cada `params`/`query`/`body`, conferir nome **e tipo** de
   cada chave contra as tags do `*Input` em Go. Tipos importam tanto quanto nomes:
   `bool` vs objeto, `int` vs string, escalar vs array — todos apareceram na T6.
   Renome sistemático de chave vai em `renameIn` no mapa, não no call site.
3. **Saídas.** Para cada campo lido de uma resposta, conferir contra a entidade ou
   o `*Output` do Go. ~32 dos 71 comandos devolvem entidade nua: se o código lê
   `data.task` e o Go devolve o `View` direto, usar `wrapOut` no mapa.
4. **Exercitar cada comando da feature uma vez por HTTP, com a carga que a UI
   realmente monta** — não um corpo mínimo escrito à mão. Esta é a regra que pega
   o que o compilador não pega, porque `params`/`body` são `Record<string, unknown>`.
5. **Conferir que todo caminho `client.x.y` da feature existe no `COMMAND_MAP`.**
   Ausência hoje só aparece quando alguém clica.

## Notas para quem executar

**A fachada é o único lugar que deve mudar.** Se um erro aparece em dezenas de
arquivos portados, a correção quase certamente pertence a `lib/aos-facade.ts`,
a `lib/command-map.ts`, a um arquivo de tipo, ou a uma flag do compilador.
Editar os arquivos portados é a última opção, não a primeira: cada edição é
divergência permanente contra a fonte, e é o que torna uma ressincronização
futura com o Fractal cara.

**Um domínio dorme por comando, não só por domínio.** `chat` está vivo mas
`chat.stop` dorme. Acordar um comando é trocar `null` pela chave em
`command-map.ts` — sem tocar em nenhuma tela.

**O `_reasoning` sintético é dívida conhecida**, registrada na spec. Não tente
melhorá-lo aqui: a correção pertence ao lado Go (distinguir origem de interface
de origem de agente) e deve virar ADR.
