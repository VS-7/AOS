/**
 * Addressing what a skill brought.
 *
 * Views and collections a skill installs are stored under a skill-qualified
 * key (`.aos/skills/{skill}/views/{id}.view.json`), and `views_get` or
 * `collections_get` with the id alone answers NOT_FOUND for them — by design,
 * so an id never silently resolves to some other scope's entry. Every screen
 * used to open them by id alone, and landed on "Page not found".
 */

/** The part of a listed view or collection that says where it lives. */
export interface ScopedEntry {
  id: string;
  skill?: string | null;
}

function skillAmong(id: string, entries: readonly ScopedEntry[]): { known: boolean; skill?: string } {
  const matches = entries.filter((entry) => entry.id === id);
  if (matches.length === 0) return { known: false };
  // The workspace's own entry is what the id alone already names.
  if (matches.some((entry) => !entry.skill)) return { known: true };
  const skills = new Set(matches.map((entry) => entry.skill as string));
  // Two skills shipping the same id: nothing here says which was meant.
  return { known: true, skill: skills.size === 1 ? [...skills][0] : undefined };
}

/**
 * The skill to address `id` with.
 *
 * The one the address names wins. Otherwise it comes from the entries already
 * loaded — the sidebar's and the palette's own list — and, when those do not
 * have the id at all (created since, or not loaded yet), from `list`. An
 * address with no skill still opens a skill's entry that way: a link from
 * Home, the marketplace or a reload.
 */
export async function resolveSkill(
  id: string,
  requested: unknown,
  known: readonly ScopedEntry[] | undefined,
  list: () => Promise<readonly ScopedEntry[]>,
): Promise<string | undefined> {
  if (typeof requested === "string" && requested.trim()) return requested.trim();

  const fromKnown = skillAmong(id, known ?? []);
  if (fromKnown.known) return fromKnown.skill;

  try {
    return skillAmong(id, await list()).skill;
  } catch {
    // The lookup below reports what is wrong, in the daemon's words.
    return undefined;
  }
}

/** The search part of an address, carrying `skill` only when there is one. */
export function skillSearch(skill: string | null | undefined): { skill?: string } {
  return skill ? { skill } : {};
}
