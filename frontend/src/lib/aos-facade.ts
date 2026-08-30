import * as React from "react";
import { useMutation, useQuery, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import type { UseMutationOptions } from "@tanstack/react-query";
import { client } from "./client";
import { COMMAND_MAP, isCommandDormant, type CommandDescriptor, type MapEntry } from "./command-map";
import type { CommandKey } from "./schema";

/** The code the UI recognizes as "the Go side doesn't have this yet". */
export const DORMANT_CODE = "AOS_DOMAIN_DORMANT";



/**
 * The code the UI recognizes as "this call path was never registered in
 * `COMMAND_MAP` at all" — distinct from {@link DORMANT_CODE}, which means
 * the map explicitly knows Go has no such command. This means nobody told
 * the map the call path exists.
 */
export const NOT_MAPPED_CODE = "AOS_CALL_NOT_MAPPED";

/**
 * C4 of the final review: this used to be a plain `{code, message}` object.
 * ~54 ported call sites do `error instanceof Error ? error.message : "Failed
 * to…"` (e.g. `.../workspace/members/index.tsx:97,125,147`) — a pattern
 * copied from the original, where the generated client's errors *were*
 * real `Error`s. Against a plain object that check was always `false`, so
 * every one of those sites silently discarded the real message (dormant's
 * included) for a generic fallback string. Making this a real `Error`
 * subclass fixes every one of those 54 sites at the machinery, with no
 * per-site edit and no divergence between them — the alternative this
 * review rejected.
 */
export class EnvelopeError extends Error {
  code: string;

  constructor(code: string, message: string) {
    super(message);
    this.name = "EnvelopeError";
    this.code = code;
  }
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
 * The original split params/query/body because it talked to a REST API. The Go
 * registry takes one flat object per command, so the three get merged.
 * `enabled` belongs to useQuery and does not belong in the payload.
 */
export function flattenArgs(opts?: CallOpts): Record<string, unknown> {
  return { ...opts?.params, ...opts?.query, ...opts?.body };
}

function toEnvelopeError(err: unknown): EnvelopeError {
  if (err && typeof err === "object" && "code" in err) {
    const e = err as { code?: unknown; message?: unknown };
    return new EnvelopeError(String(e.code ?? "UNKNOWN"), String(e.message ?? "the call failed"));
  }
  return new EnvelopeError("UNKNOWN", err instanceof Error ? err.message : "the call failed");
}

/**
 * Applies `descriptor.coerceIn`'s per-field type/shape fixes, leaving
 * every other key as it was. Runs before `applyRenameIn` — see
 * `CommandDescriptor.coerceIn`'s own doc comment for the two return
 * shapes a coercion function can produce (replace the field in place, or
 * return an object whose keys get merged into the payload in the field's
 * place). A no-op when there's nothing to coerce — the common case.
 *
 * M2 of the final review: the object-merge branch used to be a bare
 * `Object.assign(coerced, result)` — non-composable, and silent about it.
 * `command-map.ts`'s `workspace.update` entry has exactly this shape:
 * `git`/`worktrees`/`tasks` each return `{set: {...}}`, and a payload
 * carrying two of the three would have the second clobber the first's
 * `set` key with no warning (today's three forms each submit only one,
 * so this hasn't fired — but nothing stopped a fourth form from
 * combining them). `mergedBy` tracks which coercion produced which output
 * key so a second one landing on the same key throws here, inside the
 * `try` in `call()`, so it still surfaces as an envelope rather than an
 * uncaught exception.
 */
function applyCoerceIn(
  payload: Record<string, unknown>,
  coerceIn?: Record<string, (value: unknown) => unknown>,
): Record<string, unknown> {
  if (!coerceIn) return payload;
  const coerced: Record<string, unknown> = { ...payload };
  const mergedBy = new Map<string, string>(); // output key -> which coerceIn field produced it
  for (const [key, transform] of Object.entries(coerceIn)) {
    if (!(key in coerced)) continue;
    const result = transform(coerced[key]);
    delete coerced[key];
    if (result !== null && typeof result === "object" && !Array.isArray(result)) {
      for (const outKey of Object.keys(result as Record<string, unknown>)) {
        const producedBy = mergedBy.get(outKey);
        if (producedBy) {
          throw new Error(
            `coerceIn collision: both "${producedBy}" and "${key}" produced the payload key ` +
              `"${outKey}" — merging both would silently drop one (see aos-facade.ts's ` +
              `applyCoerceIn doc comment).`,
          );
        }
        mergedBy.set(outKey, key);
      }
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
  // a new call without registering it. It used to throw here — outside
  // any try/catch, since this check runs before the one below. That threw
  // *past* the 117 call sites that read envelopes with no try/catch at
  // all (`agent.getById`, `memory.graph`: an unhandled rejection that
  // still cleared its own spinner via `.finally()`, a silently-wrong panel
  // with no error in sight). Loud-and-non-fatal beats loud-and-fatal: log
  // it where a developer will see it, and hand back an envelope so the
  // call site's normal error path — the one it already has, for every
  // other kind of failure — is what runs.
  if (entry === undefined) {
    const message = `call not mapped: ${path} — register it in lib/command-map.ts`;
    console.error(`[aos-facade] ${message}`);
    return { data: undefined, error: new EnvelopeError(NOT_MAPPED_CODE, message) };
  }

  if (entry === null) {
    return {
      data: undefined,
      error: new EnvelopeError(DORMANT_CODE, `the "${feature}" domain does not exist in the Go backend yet`),
    };
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
    const payload = {
      ...applyRenameIn(applyCoerceIn(rawPayload, descriptor.coerceIn), descriptor.renameIn),
      // Last, and keyed in Go's own names: a fixed field is the mapping's
      // meaning, not a default the caller may talk it out of.
      ...(descriptor.fixedIn ?? {}),
    };
    const raw = await client.invoke(descriptor.key as CommandKey, { ...payload, _reasoning: `interface: ${path}` } as never);
    // Only a defined result gets wrapped — an empty/void response has
    // nothing worth nesting, and wrapping `undefined` as `{ [key]:
    // undefined }` would just trade one falsy shape for a different one.
    // `mapOut` first, then `wrapOut`: the mapper is written against the
    // bare Go result (see CommandDescriptor.mapOut), and wrapping before
    // mapping would hand it the nested object instead.
    const mapped = descriptor.mapOut && raw !== undefined ? descriptor.mapOut(raw) : raw;
    const data = descriptor.wrapOut && mapped !== undefined ? { [descriptor.wrapOut]: mapped } : mapped;
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
  /**
   * `isDormant` says the Go side publishes no command for this path — known
   * from `COMMAND_MAP`, so it is true from the first render rather than
   * after a round trip. `data` is `null` in that case, exactly as it is
   * before the first fetch resolves, which is what keeps every ported
   * `q.data?.field` call site working unchanged.
   */
  useQuery<T = any>(opts?: CallOpts): UseQueryResult<T> & { isDormant: boolean };
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
   * `& { loading: boolean }`: the original's generated mutation
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
            // Dormant resolves rather than throwing — a ported screen reading
            // `q.data?.field` must not crash because the Go side has no
            // counterpart yet — and it resolves to `null`, which is what
            // every call site in this tree was written against.
            //
            // A first attempt at telling dormant apart from empty put a
            // marker object here instead. It broke the settings menu outright:
            // `(query.data ?? []).some(...)` stopped short-circuiting on
            // `null`, got an object, and threw `.some is not a function`
            // before Settings could render at all. The distinction belongs on
            // the *result*, not in the data — see `isDormant` below.
            if (r.error.code === DORMANT_CODE) return null;
            // C4 of the final review made `EnvelopeError` a real `Error`
            // subclass (see its own doc comment) — `r.error` already *is*
            // what this used to construct by hand (`Object.assign(new
            // Error(...), {code})`), so it can be thrown directly. `code`
            // still rides along, now as the class's own field rather than
            // a bolted-on one.
            throw r.error;
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
      //
      // Final review, second pass: checked this for a stale-closure bug (the
      // `[result.isSuccess, result.data]` dependency array doesn't include
      // `onSuccess` itself, on purpose — an unmemoized inline callback must
      // not re-fire this effect on every unrelated parent re-render). It
      // isn't one: whenever this effect actually *runs*, it does so as part
      // of the same render whose deps changed, and that render's own
      // `onSuccess` closure is what gets built and invoked — there is no
      // separate "stale" render's closure left running later. A version of
      // this comment briefly suggested routing `onSuccess` through a
      // `useRef` to fix exactly that; reverted once the reasoning above
      // didn't hold up — the ref changed nothing observable and would have
      // been unjustified complexity, the thing this whole review keeps
      // arguing against.
      //
      // The real, narrower gap: react-query's structural sharing means
      // `result.data` keeps the *same reference* across a refetch that
      // resolves to deep-equal content, so this effect's deps don't change
      // and `onSuccess` does not re-fire — unlike real React Query v4, whose
      // `onSuccess` option fired on every settled fetch regardless of
      // whether the data changed. "Simulates v4's onSuccess" above is
      // accurate for the common case (first success, or a refetch that
      // actually changed something) and imprecise for a refetch that
      // resolves to identical bytes. The one live consumer (`use-chat.ts`'s
      // `chat.getById.useQuery`, merging a chat snapshot) is unaffected in
      // practice — a merge against unchanged data is a no-op either way —
      // but a future consumer relying on "fires every fetch, full stop"
      // would be surprised by this. Documented here rather than "fixed":
      // matching real v4 (firing unconditionally per settled fetch, not per
      // distinct result) is a different, deliberate behavior change to
      // shared machinery, not a bug fix, and belongs to whoever needs it.
      const onSuccess = opts?.onSuccess;
      React.useEffect(() => {
        if (result.isSuccess) {
          onSuccess?.(result.data);
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
      }, [result.isSuccess, result.data]);

      // Whether this path has a Go counterpart at all.
      //
      // Read from `COMMAND_MAP` rather than from the answer: dormancy is a
      // static property of the mapping, known before the first fetch, and a
      // screen asking "can this work here?" should not have to wait for a
      // round trip to find out. It rides on the result rather than in
      // `data` so that `data` stays exactly the `null` every existing caller
      // already handles.
      return Object.assign(result, { isDormant: isCommandDormant(`${feature}.${action}`) });
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
 * `api.task.list.useQuery()` — the facade the ported code expects.
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
