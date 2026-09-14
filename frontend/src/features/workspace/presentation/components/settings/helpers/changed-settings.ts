/**
 * The dotted `set` entries an autosaving settings form sends: only the fields
 * whose value differs from the one last saved.
 *
 * `config_update` and `workspace_update` both take a dotted-path patch, so a
 * form can change one setting without restating its neighbours. These forms
 * used to restate all of them on every autosave, from a snapshot read when
 * the page opened — so a field saved a moment earlier, or changed meanwhile by
 * an agent or another window, was written back to what the page had first
 * read. Sending only what the person changed leaves the rest to whoever
 * changed it last.
 *
 * `saved` must be the values the form was seeded from, defaults included, so
 * that a field the daemon never stored and the form shows as "" does not read
 * as a change. Lists and objects are compared by value and sent whole: the
 * patch treats a composite value as a leaf.
 */
export function changedSettings(
  prefix: string,
  next: Record<string, unknown>,
  saved: Record<string, unknown> | undefined,
): Record<string, unknown> {
  const set: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(next)) {
    if (saved && key in saved && sameValue(value, saved[key])) continue;
    set[prefix ? `${prefix}.${key}` : key] = value;
  }
  return set;
}

function sameValue(a: unknown, b: unknown): boolean {
  if (Object.is(a, b)) return true;
  if (typeof a !== "object" || typeof b !== "object" || a === null || b === null) return false;
  if (Array.isArray(a) !== Array.isArray(b)) return false;
  const left = a as Record<string, unknown>;
  const right = b as Record<string, unknown>;
  const keys = new Set([...Object.keys(left), ...Object.keys(right)]);
  for (const key of keys) {
    if (!sameValue(left[key], right[key])) return false;
  }
  return true;
}
