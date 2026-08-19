import type { AnyRoute, RouteComponent } from "@tanstack/react-router";
import type { QueryClient } from "@tanstack/react-query";
import type { ReactNode } from "react";
import type { UseFormReturn } from "react-hook-form";
import type { ZodTypeAny, z } from "zod";
import type { AosLayout } from "./layout";
import type { AosMiddleware } from "./middleware";
import type { AosPage } from "./page";
import type { AosResponse } from "./response";

/**
 * The builders' internal contract, reconstructed.
 *
 * The original was type-only and did not survive the bundling process that
 * produced the extraction this project was ported from. This does not
 * reproduce the original typing — it reproduces what the 1919 lines of
 * builder actually require, verified against their real field and method
 * usage. Deliberately loose: tightening it here would only produce errors
 * in ported code that already runs.
 */

export type DefaultContext = Record<string, unknown>;

export interface AosStoresCollection<T = Record<string, unknown>> {
  [key: string]: T[keyof T] | unknown;
}

export interface PageLifecycleArgs<TClient = unknown, TContext = DefaultContext, TStores = unknown> {
  client: TClient;
  context: TContext;
  stores: TStores;
  request: { url: string; query: Record<string, unknown>; params: Record<string, string> };
  response: AosResponse;
  page: unknown;
}

export interface AosAppConfig<TClient = unknown, TContext = DefaultContext, TStores = unknown, TTrigger = unknown> {
  client?: TClient;
  /** Factory that resolves the global route context. Set by `withContext`. */
  contextFn?: (args: { client: TClient; stores: TStores | undefined; request: unknown }) => Promise<TContext> | TContext;
  stores?: TStores;
  /**
   * Runs once, before the stores collection is used. Set by `withStores`.
   * `stores` is `any` here: the caller passes the collection back in after
   * an internal `as AosStoresCollection<any>` cast that already discards
   * the specific `TStores`, so matching that loosely is the only way this
   * stays assignable without touching the builder's own logic.
   */
  storesInitHandler?: (args: { client: TClient; stores: any; request: unknown }) => Promise<void> | void;
  triggers?: TTrigger;
  layout?: RouteComponent;
  notFound?: RouteComponent;
  notAuthorized?: RouteComponent;
  defaultPreload?: "intent" | "render" | "viewport" | false;
  beforePageLoad?: (args: PageLifecycleArgs<TClient, TContext, TStores>) => Promise<void> | void;
  onPageLoad?: (args: PageLifecycleArgs<TClient, TContext, TStores>) => Promise<void> | void;
}

/**
 * `AosApp.build()`'s fallback trigger API, used when no trigger builder was
 * configured via `withTriggers`. Its shape genuinely differs from
 * {@link AosTriggerAPI} — it exposes `trigger` where the real,
 * `initialize()`-produced API exposes `dispatch` — so `AosAppBuilt.triggers`
 * is typed as a union of both rather than papering over the mismatch.
 */
export interface AosTriggerAPIFallback {
  list: () => Promise<never[]>;
  trigger: () => Promise<void>;
  use: () => AosTriggerHookResult;
}

/**
 * What `AosApp.build()` actually returns, read member-for-member off the
 * object literal in `app.tsx`. `page`/`layout`/`middleware` return the real
 * builder instances with every generic parameter that matters to a caller
 * (`TClient`, `TContext`, `TStores`, and for `page` the loader/search-schema
 * slots) threaded through concretely.
 *
 * The one slot left as `any` is `AosPage`/`AosLayout`'s *parent-route* type
 * parameter. That parameter is stored as a plain public class property
 * (`public parentRoute: TParentRoute`), which TypeScript's structural
 * variance measurement treats as invariant — so no single named type (not
 * even {@link AnyRoute}, which is TanStack's own "accept anything" escape
 * hatch) is simultaneously assignable from the concrete `typeof rootRoute`
 * app.tsx actually constructs *and* usable as a stand-in in this interface,
 * which cannot name that concrete, per-app route type. `any` is the
 * intentionally narrow exception, isolated to that one slot; every other
 * parameter here is real.
 */
export interface AosAppBuilt<TClient = unknown, TContext = DefaultContext, TStores = unknown, TTrigger = unknown> {
  /** The TanStack root route produced by `build()`; consumed by `AosRouter`. */
  rootRoute: AnyRoute;
  config: AosAppConfig<TClient, TContext, TStores>;
  client: TClient;
  stores: TStores;
  layout: () => AosLayout<any, TClient, TContext, TStores>;
  middleware: () => AosMiddleware<TClient, TContext, TStores, any>;
  page: <TNewPath extends string>(path: TNewPath) => AosPage<TNewPath, any, TClient, TContext, TStores, ZodTypeAny, any>;
  useContext: () => TContext;
  triggers: AosTriggerAPI<TClient, TContext, TStores> | AosTriggerAPIFallback;
  commands: AosTriggerAPI<TClient, TContext, TStores> | AosTriggerAPIFallback;
  useQueryClient: (queryClient?: QueryClient) => QueryClient;
  useForm: <TPath extends MutationPath<TClient>, TSchema extends ZodTypeAny>(
    options: AosUseFormOptions<TClient, TPath, TSchema>
  ) => AosFormReturn<z.infer<TSchema> & Record<string, unknown>>;
}

/**
 * The fluent builder interface `AosApp` implements.
 *
 * `TClient`/`TContext`/`TStores` are threaded through `withClient`/
 * `withContext`/`withStores`/`withTriggers` as real generic type parameters,
 * matching how the class itself narrows them on every call.
 *
 * `TTrigger`, by contrast, is kept as a parameter for call-site compatibility
 * (so callers can still write `IAosAppBuilder<A, B, C, D>`) but deliberately
 * does not appear in the *output* position of `withClient`/`withContext`/
 * `withStores`/`build` — only `withTriggers` propagates a real `TNewTrigger`.
 * This is not the same erasure as the other three params: the class's own
 * `withClient`/`withContext`/`withStores` each collapse the trigger type to a
 * *fresh* `IAosTriggerBuilt<...>` default rather than preserving whatever
 * `TTrigger` the caller instantiated the interface with, so threading the
 * caller's `TTrigger` through those three specifically would make the class
 * provably fail to structurally satisfy the interface — TS cannot prove a
 * general default is assignable to an arbitrary, independently-chosen
 * `TTrigger`. Erasing `TTrigger` to `any` only at those four output
 * positions (not at `TClient`/`TContext`/`TStores`, which the class does
 * preserve exactly) sidesteps that real variance limitation in the copied
 * builder code rather than working around it with unsound casts.
 */
export interface IAosAppBuilder<TClient = unknown, TContext = DefaultContext, TStores = unknown, TTrigger = unknown> {
  withClient<TNewClient = unknown>(client: TNewClient): IAosAppBuilder<TNewClient, TContext, TStores, any>;
  withStores<TNewStores extends AosStoresCollection<any> = AosStoresCollection<any>>(
    stores: TNewStores,
    onReady?: (args: { client: TClient; stores: TNewStores; request: unknown }) => Promise<void> | void
  ): IAosAppBuilder<TClient, TContext, TNewStores, any>;
  withContext<TNewContext = unknown>(
    factory: (args: { client: TClient; stores: TStores; request: unknown }) => Promise<TNewContext> | TNewContext
  ): IAosAppBuilder<TClient, TNewContext, TStores, any>;
  withTriggers<TNewTrigger extends IAosTriggerBuilt<TClient, TContext, TStores> = IAosTriggerBuilt<TClient, TContext, TStores>>(
    triggers: TNewTrigger
  ): IAosAppBuilder<TClient, TContext, TStores, TNewTrigger>;
  withLayout(component: RouteComponent): IAosAppBuilder<TClient, TContext, TStores, TTrigger>;
  withNotFoundComponent(component: RouteComponent): IAosAppBuilder<TClient, TContext, TStores, TTrigger>;
  withNotAuthorizedComponent(component: RouteComponent): IAosAppBuilder<TClient, TContext, TStores, TTrigger>;
  withDefaultPreload(mode: "intent" | "render" | "viewport" | false): IAosAppBuilder<TClient, TContext, TStores, TTrigger>;
  withBeforePageLoader(fn: unknown): IAosAppBuilder<TClient, TContext, TStores, TTrigger>;
  withPageLoader(fn: unknown): IAosAppBuilder<TClient, TContext, TStores, TTrigger>;
  build(): AosAppBuilt<TClient, TContext, TStores, TTrigger>;
}

/** What `withComponent(({ route }) => …)` receives. */
export interface RouteContextAPI<TLoaderData = unknown, TParams = Record<string, string>, TSearch = Record<string, unknown>> {
  useLoaderData(): TLoaderData;
  useParams(): TParams;
  useSearch(): TSearch;
  refresh(): void | Promise<void>;
  navigate(opts: { to: string; params?: Record<string, unknown>; search?: Record<string, unknown> }): void | Promise<void>;
}

export type InferContextIn<T> = T extends (args: infer A) => unknown ? A : never;

/** A dotted "controller.action" path into the API client. Kept as `string` — deriving the real union from `TClient`'s shape is out of scope. */
export type MutationPath<TClient = unknown> = string;

export interface AosTriggerDef<
  TId extends string = string,
  TSchema extends ZodTypeAny = ZodTypeAny,
  TResult = unknown,
  TClient = unknown,
  TContext = DefaultContext,
  TStores = unknown,
  TMetadata = unknown
> {
  id: TId;
  label: string;
  group?: string;
  hidden?: boolean;
  keybind?: string;
  schema?: TSchema;
  metadata?: TMetadata;
  handler?: (args: {
    input: unknown;
    client: TClient;
    context: TContext;
    stores: TStores;
    response: AosResponse;
  }) => Promise<TResult> | TResult;
  [key: string]: unknown;
}

export interface AosTriggerListParams {
  query?: string;
  [key: string]: unknown;
}

export interface AosTriggerHookResult<TData = unknown> {
  mutate: (input?: unknown) => Promise<TData>;
  data: TData | undefined;
  error: Error | null;
  isLoading: boolean;
}

export interface AosTriggerAPI<TClient = unknown, TContext = DefaultContext, TStores = unknown> {
  list: (params: AosTriggerListParams) => Promise<AosTriggerDef<string, ZodTypeAny, unknown, TClient, TContext, TStores, unknown>[]>;
  dispatch: (triggerId: string, input?: unknown) => Promise<unknown>;
  use: (args: {
    trigger: string;
    enabled?: boolean;
    onPressKey?: (event: KeyboardEvent) => void;
    onSuccess?: (args: { data: unknown; input: unknown }) => void;
    onError?: (args: { error: unknown; input: unknown }) => void;
  }) => AosTriggerHookResult;
}

/** What `AosTrigger.build()` returns: a registry of groups plus the runtime API factory. */
export interface IAosTriggerBuilt<TClient = unknown, TContext = DefaultContext, TStores = unknown> {
  groups?: Record<string, IAosTriggerGroupBuilt<TClient, TContext, TStores>>;
  initialize: (config: AosAppConfig<TClient, TContext, TStores>) => AosTriggerAPI<TClient, TContext, TStores>;
  [key: string]: unknown;
}

/** What `AosTriggerGroup.build()` returns. Not a sub-shape of {@link IAosTriggerBuilt} — the two are unrelated shapes despite the similar names. */
export interface IAosTriggerGroupBuilt<TClient = unknown, TContext = DefaultContext, TStores = unknown> {
  id: string;
  order: number;
  metadataSchema?: unknown;
  triggers: Record<string, AosTriggerDef<string, ZodTypeAny, unknown, TClient, TContext, TStores, unknown>>;
  loader?: AosAppTriggerOnSearchCallback<TClient, TContext, TStores>;
}

export type AosAppTriggerOnSearchCallback<TClient = unknown, TContext = DefaultContext, TStores = unknown> = (args: {
  client: TClient;
  context: TContext;
  stores: TStores;
  query?: string;
}) =>
  | Promise<AosTriggerDef<string, ZodTypeAny, unknown, TClient, TContext, TStores, unknown>[]>
  | AosTriggerDef<string, ZodTypeAny, unknown, TClient, TContext, TStores, unknown>[];

export interface AosUseFormOptions<TClient = unknown, TPath = MutationPath<TClient>, TSchema extends ZodTypeAny = ZodTypeAny> {
  schema: TSchema;
  values?: (z.infer<TSchema> & Record<string, unknown>) | undefined;
  mutation?: TPath;
  params?: Record<string, unknown>;
  query?: Record<string, unknown>;
  mode?: "onSubmit" | "onChange" | "onBlur";
  debounce?: number;
  preventNavigation?: boolean;
  onSubmit?: (values: z.infer<TSchema> & Record<string, unknown>) => Promise<unknown> | unknown;
  onResponse?: (result: { data?: unknown; error?: Error }) => void;
}

/**
 * The object `useForm` returns: a real react-hook-form `UseFormReturn`
 * (`app/builders/app.tsx`'s `useForm` does `Object.assign(form, {isLoading,
 * submit})` on the actual `useHookForm(...)` result) extended with
 * `isLoading`/`submit`.
 *
 * This used to be a loose `{ isLoading; submit; children?; [key: string]:
 * unknown }` shape with no relation to react-hook-form's real type. That
 * made every accessor on the runtime-real object (`form.watch`,
 * `form.reset`, `form.control`, `form.formState`, ...) type as `unknown` to
 * callers — invisible until `task`'s forms (`dialogs/create`,
 * `comments/index.tsx`'s reply box, `todo-dialog-upsert`) were the first
 * real consumers. Extending `UseFormReturn<TValues>` gives every real
 * accessor its real type; `isLoading`/`submit` stay as the two fields this
 * builder actually adds on top.
 */
export interface AosFormReturn<TValues extends Record<string, any> = Record<string, any>> extends UseFormReturn<TValues> {
  isLoading: boolean;
  submit: (...args: any[]) => unknown;
  children?: ReactNode;
}
