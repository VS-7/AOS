import { useMutation, useQuery, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import type { UseMutationOptions } from "@tanstack/react-query";
import { client } from "./client";
import { COMMAND_MAP, type MapEntry } from "./command-map";
import type { CommandKey } from "./schema";

/** The code the UI recognizes as "the Go side doesn't have this yet". */
export const DORMANT_CODE = "AOS_DOMAIN_DORMANT";

export interface EnvelopeError {
  code: string;
  message: string;
}

/**
 * The shape 117 call sites in the ported frontend already know how to read.
 *
 * `lib/client.ts` throws `DomainError`; this facade inverts that convention
 * and returns the error as a value. That's deliberate: the ported code
 * writes `const { error } = await client.task.update.mutate(...)` with no
 * try/catch, and an exception escaping here becomes a blank screen instead
 * of a message.
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
 * Fractal split params/query/body because it talked to a REST API. The Go
 * registry takes one flat object per command, so the three get merged.
 * `enabled` belongs to useQuery and does not belong in the payload.
 */
export function flattenArgs(opts?: CallOpts): Record<string, unknown> {
  return { ...opts?.params, ...opts?.query, ...opts?.body };
}

function toEnvelopeError(err: unknown): EnvelopeError {
  if (err && typeof err === "object" && "code" in err) {
    const e = err as { code?: unknown; message?: unknown };
    return { code: String(e.code ?? "UNKNOWN"), message: String(e.message ?? "the call failed") };
  }
  return { code: "UNKNOWN", message: err instanceof Error ? err.message : "the call failed" };
}

/**
 * The single point through which the 207 call sites in the ported frontend pass.
 */
export async function call(feature: string, action: string, opts?: CallOpts): Promise<Envelope<unknown>> {
  const path = `${feature}.${action}`;
  const entry: MapEntry | undefined = COMMAND_MAP[path];

  // A missing key is a programming error, not backend state: someone wrote
  // a new call without registering it. Fail loud, on purpose.
  if (entry === undefined) {
    throw new Error(`call not mapped: ${path} — register it in lib/command-map.ts`);
  }

  if (entry === null) {
    return { data: undefined, error: { code: DORMANT_CODE, message: `the "${feature}" domain does not exist in the Go backend yet` } };
  }

  const payload = flattenArgs(opts);
  try {
    // `client.invoke<K extends CommandKey>` infers K from both arguments at
    // once. `entry` narrows to CommandKey already (the `function` branch is
    // ruled out above), but a wide, dynamically-looked-up CommandKey gives
    // no single K TypeScript can pin down, so it can't check the payload
    // against that command's specific input schema. `as CommandKey` here is
    // the same type restated, not a widening; `as never` on the payload is
    // the narrowest way to hand over a shape whose fields vary per command
    // — the real safety net is Go's own input validation on the other side.
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
  /**
   * `options` accepts the same `onSuccess`/`onError`/`onSettled` callbacks
   * `@tanstack/react-query`'s own `useMutation` does — `mutationFn` is
   * fixed (it's always `call(feature, action, opts)`), everything else
   * passes through. Originally this took no parameter at all; the first
   * real ported call site (`task`'s `client.chat.stop.useMutation({
   * onSuccess, onError})`) called it with options and failed to compile
   * ("Expected 0 arguments, but got 1") — a common enough React Query
   * pattern that every other feature is likely to hit it too, so this is
   * fixed at the facade rather than worked around per call site.
   */
  useMutation(
    options?: Omit<UseMutationOptions<Envelope<unknown>, Error, CallOpts | undefined>, "mutationFn">,
  ): UseMutationResult<Envelope<unknown>, Error, CallOpts | undefined>;
}

export type AosClient = Record<string, Record<string, ActionNode>>;

function actionNode(feature: string, action: string): ActionNode {
  return {
    query: (opts) => call(feature, action, opts),
    mutate: (opts) => call(feature, action, opts),

    // The ported code reads `q.data?.tasks`, not `q.data.data.tasks` — so
    // the hook unwraps the envelope and hands back the payload directly,
    // which is what a useQuery is expected to return.
    useQuery: (opts) =>
      useQuery({
        queryKey: [feature, action, flattenArgs(opts)],
        queryFn: async () => {
          const r = await call(feature, action, opts);
          // @tanstack/query-core rejects `undefined` as query data outright
          // (it throws "... data is undefined"), so every branch here
          // returns `null` instead — never `undefined`. Dormant resolves to
          // `null` data rather than an error state: the ported code reads
          // `q.data?.field`, which treats `null` the same as "not there
          // yet" and never even notices. `query()`/`mutate()` keep the
          // `{ data: undefined, error }` shape for dormant calls — that
          // asymmetry with this hook is intentional, not a mismatch to fix.
          if (r.error) {
            if (r.error.code === DORMANT_CODE) return null;
            // A plain `{code, message}` object thrown here would type `error`
            // as non-Error at the `UseQueryResult` boundary — real `Error`
            // instances are what `instanceof Error`, `.stack`, and any
            // throwOnError/ErrorBoundary path expect. `code` still needs to
            // reach the UI, so it rides along as a property on the Error.
            throw Object.assign(new Error(r.error.message), { code: r.error.code });
          }
          return r.data ?? null;
        },
        enabled: opts?.enabled ?? true,
        // Dormant doesn't get better by retrying.
        retry: false,
      }),

    useMutation: (options) =>
      useMutation({
        ...options,
        mutationFn: (opts?: CallOpts) => call(feature, action, opts),
      }),
  };
}

/**
 * `api.task.list.useQuery()` — the facade the Fractal code expects.
 *
 * Two levels of Proxy instead of a generated object: the map has 113
 * entries today and grows with every domain the Go side gains; generating
 * the object would force keeping it in two places.
 */
export const api: AosClient = new Proxy({} as AosClient, {
  get: (_t, feature: string) =>
    new Proxy({} as Record<string, ActionNode>, {
      get: (_t2, action: string) => actionNode(feature, action),
    }),
});
