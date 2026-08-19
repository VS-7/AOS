import * as React from "react";
import { useMutation, useQuery, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import type { UseMutationOptions } from "@tanstack/react-query";
import { client } from "./client";
import { COMMAND_MAP, type CommandDescriptor, type MapEntry } from "./command-map";
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
  /** Passed straight through to `@tanstack/react-query`'s `useQuery`. */
  staleTime?: number;
  /**
   * The original generated query hook called this on every successful
   * fetch — a React Query v4-era `useQuery` option v5 (installed here)
   * dropped in favor of `useEffect`-on-`data`. `ActionNode.useQuery`
   * simulates it that way (see its implementation below) rather than
   * teaching every ported call site the v5 pattern; first hit at
   * `use-chat.ts`, the same "one file's error, many files' problem"
   * shape `useMutation`'s own options fix (above) already documents.
   */
  onSuccess?: (data: any) => void;
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
 * Applies `descriptor.coerceIn`'s per-field type/shape fixes, leaving
 * every other key as it was. Runs before `applyRenameIn` — see
 * `CommandDescriptor.coerceIn`'s own doc comment for the two return
 * shapes a coercion function can produce (replace the field in place, or
 * return an object whose keys get merged into the payload in the field's
 * place). A no-op when there's nothing to coerce — the common case.
 */
function applyCoerceIn(
  payload: Record<string, unknown>,
  coerceIn?: Record<string, (value: unknown) => unknown>,
): Record<string, unknown> {
  if (!coerceIn) return payload;
  const coerced: Record<string, unknown> = { ...payload };
  for (const [key, transform] of Object.entries(coerceIn)) {
    if (!(key in coerced)) continue;
    const result = transform(coerced[key]);
    delete coerced[key];
    if (result !== null && typeof result === "object" && !Array.isArray(result)) {
      Object.assign(coerced, result as Record<string, unknown>);
    } else if (result !== undefined) {
      coerced[key] = result;
    }
    // `result === undefined` (and not an object): the coercion dropped
    // the field entirely, deliberately — already handled by the `delete`
    // above running unconditionally.
  }
  return coerced;
}

/**
 * Renames the keys `descriptor.renameIn` names, leaving every other key as
 * it was. A no-op when there's nothing to rename — the common case, most
 * commands need no adaptation at all.
 */
function applyRenameIn(payload: Record<string, unknown>, renameIn?: Record<string, string>): Record<string, unknown> {
  if (!renameIn) return payload;
  const renamed: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(payload)) {
    renamed[renameIn[key] ?? key] = value;
  }
  return renamed;
}

/** Normalizes a plain `CommandKey` string into the same shape a `CommandDescriptor` already has, so `call()` only has one path to follow. */
function resolveDescriptor(entry: CommandKey | CommandDescriptor): CommandDescriptor {
  return typeof entry === "string" ? { key: entry } : entry;
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

  const rawPayload = flattenArgs(opts);
  try {
    if (typeof entry === "function") {
      const data = await entry(rawPayload);
      return { data, error: undefined };
    }

    // `client.invoke<K extends CommandKey>` infers K from both arguments at
    // once. `descriptor.key` narrows to CommandKey already, but a wide,
    // dynamically-looked-up CommandKey gives no single K TypeScript can pin
    // down, so it can't check the payload against that command's specific
    // input schema. `as CommandKey` here is the same type restated, not a
    // widening; `as never` on the payload is the narrowest way to hand over
    // a shape whose fields vary per command — the real safety net is Go's
    // own input validation on the other side.
    const descriptor = resolveDescriptor(entry);
    const payload = applyRenameIn(applyCoerceIn(rawPayload, descriptor.coerceIn), descriptor.renameIn);
    const raw = await client.invoke(descriptor.key as CommandKey, { ...payload, _reasoning: `interface: ${path}` } as never);
    // Only a defined result gets wrapped — an empty/void response has
    // nothing worth nesting, and wrapping `undefined` as `{ [key]:
    // undefined }` would just trade one falsy shape for a different one.
    const data = descriptor.wrapOut && raw !== undefined ? { [descriptor.wrapOut]: raw } : raw;
    return { data, error: undefined };
  } catch (err) {
    return { data: undefined, error: toEnvelopeError(err) };
  }
}

/**
 * `Envelope<T = any>` — a generic parameter, not a fixed `<any>` — per
 * review round 2. The Go registry is untyped JSON per command, and this
 * facade has no per-command response-type map (`CommandKey` only names the
 * call, not its shape), so every one of the 26 ported features reads
 * `.data.someField` straight off a query/mutation result the way the
 * original generated Igniter client's *typed* client let them. `unknown`
 * forced every such read through a cast or narrowing; across the Task 9
 * bulk copy that was 187 errors in 70 files, all the same shape ("Property
 * X does not exist on type '{}'" — `unknown`'s `||`/`??` fallback narrows
 * to the structural empty-object type, not the array/object the ported
 * code assumes). Defaulting `T` to `any` keeps every one of those 70 call
 * sites compiling unchanged today (source-compatible, zero edits) while
 * opening a seam the follow-up per-feature contract-verification task can
 * use incrementally: `api.task.list.useQuery<TaskListOut>()` opts one call
 * site into a real response type without a big-bang facade rewrite or a
 * `CommandKey` → response-shape map built up front. `call()`'s own
 * `Envelope<unknown>` (the internal function, not this public interface)
 * is left alone — internal code benefits from the stricter type; only the
 * ported-code-facing surface takes the generic.
 */
interface ActionNode {
  query<T = any>(opts?: CallOpts): Promise<Envelope<T>>;
  mutate<T = any>(opts?: CallOpts): Promise<Envelope<T>>;
  useQuery<T = any>(opts?: CallOpts): UseQueryResult<T>;
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
   *
   * `& { loading: boolean }`: the Fractal original's generated mutation
   * hook exposed `.loading`, not React Query's own `.isPending` — 30 call
   * sites across 18 freshly-copied files (Task 9) read `.loading`
   * directly. Adding the alias here, once, is the fix; teaching every one
   * of those files React Query's naming is the alternative this avoids.
   */
  useMutation<T = any>(
    options?: Omit<UseMutationOptions<Envelope<T>, Error, CallOpts | undefined>, "mutationFn">,
  ): UseMutationResult<Envelope<T>, Error, CallOpts | undefined> & { loading: boolean };
}

export type AosClient = Record<string, Record<string, ActionNode>>;

function actionNode(feature: string, action: string): ActionNode {
  return {
    // The implementation is the same untyped `call()` regardless of what
    // `T` a caller asks for — `T` is a pure type-level opt-in (see this
    // interface's doc comment above), so the cast here is exactly as
    // honest as the old fixed `<any>` was, just deferred to the call site.
    query: (opts) => call(feature, action, opts) as any,
    mutate: (opts) => call(feature, action, opts) as any,

    // The ported code reads `q.data?.tasks`, not `q.data.data.tasks` — so
    // the hook unwraps the envelope and hands back the payload directly,
    // which is what a useQuery is expected to return.
    useQuery: (opts): any => {
      const result = useQuery({
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
        staleTime: opts?.staleTime,
        // Dormant doesn't get better by retrying.
        retry: false,
      });

      // Simulates React Query v4's `onSuccess` callback — see `CallOpts.
      // onSuccess`'s doc comment for why this facade offers it at all.
      const onSuccess = opts?.onSuccess;
      React.useEffect(() => {
        if (result.isSuccess) {
          onSuccess?.(result.data);
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
      }, [result.isSuccess, result.data]);

      return result;
    },

    useMutation: (options): any => {
      const mutation = useMutation({
        ...options,
        mutationFn: (opts?: CallOpts) => call(feature, action, opts) as any,
      });
      // See the `.loading` doc comment on `ActionNode.useMutation` above.
      return { ...mutation, loading: mutation.isPending };
    },
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
