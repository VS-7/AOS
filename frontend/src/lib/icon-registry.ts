/**
 * Icons looked up by name, one chunk each, loaded when something asks for one.
 *
 * The command palette renders whatever icon a trigger names — `command.icon`
 * is a string chosen by whoever declared the trigger — so there is no set of
 * named imports a bundler could resolve, and the palette reached for the whole
 * package with `import * as LucideIcons from "lucide-react"`.
 *
 * A namespace import defeats tree-shaking by definition: every icon in the
 * package is reachable, so every icon is kept. That put 1.3 MB of icons —
 * 1,568 modules — into the startup bundle for a dialog that opens on ⌘K and
 * that many sessions never open at all. Importing the package dynamically
 * does not fix it either: the same package is statically imported by dozens of
 * components for the icons they name directly, and a module that is both
 * statically and dynamically imported is emitted into the static chunk.
 *
 * `lucide-react/dynamicIconImports` is the package's own answer: a record of
 * `() => import("./icons/<name>.js")`, one entry per icon, which Rollup emits
 * as one small chunk each. The static named imports elsewhere keep shaking
 * normally, and a palette entry costs exactly the icon it names.
 */
import type { ComponentType, SVGProps } from "react";

export type IconComponent = ComponentType<SVGProps<SVGSVGElement> & { className?: string }>;

type IconLoaders = Record<string, () => Promise<{ default: IconComponent }>>;

/**
 * The map itself is fetched on demand too.
 *
 * It is one arrow function per icon — about 150 kB of them — and imported at
 * the top of this file it lands in the startup bundle whole, which is most of
 * what moving the icons out of it saved.
 */
let loaders: IconLoaders | null = null;
let loadingMap: Promise<IconLoaders> | null = null;

function ensureLoaders(): Promise<IconLoaders> {
  if (loaders) return Promise.resolve(loaders);
  if (loadingMap) return loadingMap;

  loadingMap = import("lucide-react/dynamicIconImports").then((module) => {
    loaders = module.default as unknown as IconLoaders;
    loadingMap = null;
    return loaders;
  });
  return loadingMap;
}

/** Icons already in memory, by the name the caller asked for. */
const resolved = new Map<string, IconComponent>();
/** Loads in flight, so N components naming one icon cause one import. */
const pending = new Map<string, Promise<IconComponent | undefined>>();

/**
 * The key `dynamicIconImports` files an icon under.
 *
 * Trigger definitions name icons the way the package exports them — Pascal
 * case, `"MessageSquare"` — and the dynamic map is keyed by the file name,
 * kebab case, `"message-square"`. Both spellings are accepted so a trigger
 * that already uses the kebab form keeps working.
 *
 * The digit rule matters: the package files `Volume2` as `volume-2`, so a
 * digit starts its own segment.
 */
function loaderKey(name: string, available: IconLoaders): string {
  if (name in available) return name;
  return name
    .replace(/([a-z0-9])([A-Z])/g, "$1-$2")
    .replace(/([A-Za-z])([0-9])/g, "$1-$2")
    .toLowerCase();
}

/** An icon already loaded, or undefined. Synchronous, for render. */
export function iconByName(name: string | undefined): IconComponent | undefined {
  if (!name) return undefined;
  return resolved.get(name);
}

/**
 * Loads one icon by name, resolving to undefined for a name the package does
 * not have — a trigger naming an icon that does not exist is a typo in a
 * label, not a broken screen, and the caller draws no icon.
 */
export function loadIcon(name: string | undefined): Promise<IconComponent | undefined> {
  if (!name) return Promise.resolve(undefined);

  const already = resolved.get(name);
  if (already) return Promise.resolve(already);

  const inFlight = pending.get(name);
  if (inFlight) return inFlight;

  const promise = ensureLoaders()
    .then((available) => {
      const load = available[loaderKey(name, available)];
      if (!load) return undefined;
      return load().then((module) => {
        resolved.set(name, module.default);
        return module.default;
      });
    })
    .catch(() => undefined)
    .finally(() => {
      pending.delete(name);
    });

  pending.set(name, promise);
  return promise;
}

/**
 * Loads every icon in a list, and resolves once they are all in `resolved`.
 *
 * The command palette calls this with the icons of the commands it is about
 * to render, then re-renders — which is one round trip for a list, rather
 * than one re-render per icon.
 */
export async function loadIcons(names: Array<string | undefined>): Promise<void> {
  const wanted = Array.from(new Set(names.filter((n): n is string => Boolean(n))));
  const missing = wanted.filter((n) => !resolved.has(n));
  if (missing.length === 0) return;
  await Promise.all(missing.map(loadIcon));
}
