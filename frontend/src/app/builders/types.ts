import type { AnyRoute, RouteComponent } from "@tanstack/react-router";
import type { ReactNode } from "react";
import type { ZodTypeAny, z } from "zod";

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
  response: unknown;
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

export interface AosAppBuilt<TClient = unknown, TContext = DefaultContext, TStores = unknown, TTrigger = unknown> {
  client: TClient;
  context: TContext;
  stores: TStores;
  triggers: TTrigger;
  config: AosAppConfig<TClient, TContext, TStores, TTrigger>;
  /** The TanStack root route produced by `AosApp.build()`; consumed by `AosRouter`. */
  rootRoute: AnyRoute;
  page(path: string): unknown;
  layout(path: string): unknown;
  router(routes: AnyRoute[]): unknown;
}

/**
 * The fluent builder interface `AosApp` implements.
 *
 * `TTrigger` is kept as a parameter for call-site compatibility (so callers
 * can still write `IAosAppBuilder<A, B, C, D>`) but deliberately does not
 * appear in any member below. The class's own `withClient`/`withContext`/
 * `withStores`/`build` all collapse the trigger type to a fresh default
 * (typically `IAosTriggerBuilt<...>`) rather than preserving the caller's
 * `TTrigger` — threading it through here would make every one of those
 * methods fail to structurally satisfy this interface, since TS can't prove
 * a general default is assignable to an arbitrary, independently-chosen
 * `TTrigger`. Making the parameter phantom sidesteps a real variance
 * limitation in the copied builder code rather than working around it with
 * unsound casts.
 */
export interface IAosAppBuilder<TClient = unknown, TContext = DefaultContext, TStores = unknown, TTrigger = unknown> {
  withClient(client: unknown): IAosAppBuilder<any, TContext, TStores, any>;
  withStores(stores: unknown, onReady?: (args: { stores: any }) => Promise<void> | void): IAosAppBuilder<TClient, TContext, any, any>;
  withContext(factory: (args: { stores: TStores }) => unknown): IAosAppBuilder<TClient, any, TStores, any>;
  withTriggers(triggers: unknown): IAosAppBuilder<TClient, TContext, TStores, any>;
  withLayout(component: RouteComponent): IAosAppBuilder<TClient, TContext, TStores, any>;
  withNotFoundComponent(component: RouteComponent): IAosAppBuilder<TClient, TContext, TStores, any>;
  withNotAuthorizedComponent(component: RouteComponent): IAosAppBuilder<TClient, TContext, TStores, any>;
  withDefaultPreload(mode: "intent" | "render" | "viewport" | false): IAosAppBuilder<TClient, TContext, TStores, any>;
  withBeforePageLoader(fn: unknown): IAosAppBuilder<TClient, TContext, TStores, any>;
  withPageLoader(fn: unknown): IAosAppBuilder<TClient, TContext, TStores, any>;
  build(): AosAppBuilt<TClient, TContext, TStores, any>;
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
    response: unknown;
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

/** The object `useForm` returns: a react-hook-form `UseFormReturn` extended with `isLoading`/`submit`. Left as an open shape — react-hook-form's own type is not reproduced here. */
export interface AosFormReturn<TValues = Record<string, unknown>> {
  isLoading: boolean;
  submit: (...args: any[]) => unknown;
  children?: ReactNode;
  [key: string]: unknown;
}
