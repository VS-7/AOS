/**
 * Reconstructed from usage: the file was type-only and the bundler erased
 * it. There is no Go backend for this domain yet — when there is, this
 * contract becomes verifiable and should be checked against it.
 *
 * Usage sites (v401/web/src, features/marketplace/presentation/**):
 * `helpers/marketplace.helper.ts`'s `resolveInstalledListing` fallback
 * object literal is the most authoritative source for
 * `FractalMarketplaceSkillListing` (every field it sets is a field real
 * code reads elsewhere: `plugin-card.component.tsx`,
 * `plugin-section.component.tsx`, `filterListings`, `groupListingsByCategory`).
 * `pages/marketplace/[name]/inner.tsx` and
 * `components/plugin-detail-section.component.tsx` are the sources for
 * `FractalMarketplaceSkill`, `FractalMarketplaceSkillInventory`,
 * `FractalMarketplaceSkillComponentItem/Kind`, and
 * `FractalMarketplaceInstalledSkill`.
 */

/** The 8 marketplace inventory sections, from `PLUGIN_INVENTORY_SECTION_META`. */
export type FractalMarketplaceSkillComponentKind =
  | "toolsets"
  | "collections"
  | "views"
  | "hooks"
  | "instructions"
  | "templates"
  | "artifacts"
  | "routines";

/** One installed-plugin inventory row (a toolset, view, artifact, etc.). */
export interface FractalMarketplaceSkillComponentItem {
  id?: string;
  name?: string;
  kind: FractalMarketplaceSkillComponentKind;
  label: string;
  description?: string;
  /** Workspace file path — present for file-backed items (edit/open-in-tab). */
  path?: string;
  /** Present for registry (not-yet-installed) items — links to source on GitHub. */
  githubUrl?: string;
  instructionType?: string;
  /** Toolset connection kind (e.g. "custom"); only meaningful for `kind === "toolsets"`. */
  connectionType?: string;
}

/** A marketplace search-result / card entry. */
export interface FractalMarketplaceSkillListing {
  name: string;
  displayName: string;
  shortDescription: string;
  description: string;
  logo: string | null;
  homepage: string;
  author: string;
  category: string;
  keywords: string[];
  capabilities: string[];
  brandColor: string | null;
}

/** Full plugin manifest, as returned by `marketplace.getByName`. */
export interface FractalMarketplaceSkill {
  name: string;
  description?: string;
  author: {
    name: string;
  };
  interface: {
    category: string;
    displayName: string;
    logo?: string | null;
    brandColor?: string | null;
    shortDescription: string;
    longDescription?: string;
    defaultPrompt?: string[];
  };
}

/** Inventory items grouped by kind, as returned alongside a plugin's manifest. */
export type FractalMarketplaceSkillInventory = Partial<
  Record<FractalMarketplaceSkillComponentKind, FractalMarketplaceSkillComponentItem[]>
>;

/** The locally-installed record for a plugin (vs. its marketplace listing). */
export interface FractalMarketplaceInstalledSkill {
  id: string;
  active?: boolean;
  skillMdPath?: string;
  manifestPath?: string;
}
