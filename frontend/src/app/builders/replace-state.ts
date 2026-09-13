/**
 * A value that, set on a store, replaces `previous` with `next` wholesale.
 *
 * `ctx.state.set` merges plain objects key by key, which is what a partial
 * update wants and what a record read back from the daemon does not: the
 * daemon leaves an emptied field out of its answer (`omitempty`), so the
 * merge kept the value the person had just cleared. This names every key
 * `previous` had and `next` lacks as `undefined` — the merge applies an
 * explicit `undefined` — recursing into the objects both sides share.
 */
export function replacing<T>(previous: unknown, next: T): T {
  if (!isPlainObject(previous) || !isPlainObject(next)) return next;
  const out: Record<string, unknown> = { ...next };
  for (const key of Object.keys(previous)) {
    if (!(key in out)) {
      out[key] = undefined;
    } else if (isPlainObject(previous[key]) && isPlainObject(out[key])) {
      out[key] = replacing(previous[key], out[key]);
    }
  }
  return out as T;
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
