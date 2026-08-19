/**
 * There is no AOS Go backend for this domain yet, but real, checkable
 * declarations exist in the old Fractal server:
 * `v401/server/src/features/marketplace/schemas/marketplace.schema.ts`'s
 * Zod schemas — the type-only `interfaces/marketplace.interfaces.ts` that
 * would normally declare these was itself erased by the bundler, same as
 * everywhere else, so the schema file is the best available ground truth.
 * Recovered, not guessed — this replaces an earlier pass of this file that
 * was reconstructed from frontend usage alone (`v401/web/src/features/
 * marketplace/presentation/**`); several fields below are required or
 * present that no usage site in the ported frontend ever touched, and one
 * (`FractalMarketplaceSkill`) was missing over half its real fields
 * entirely. When AOS grows a Go backend for this, re-verify against that
 * instead.
 */

/** The 8 marketplace inventory sections (`FractalMarketplaceSkillComponentKindSchema`). */
export type FractalMarketplaceSkillComponentKind =
  | "toolsets"
  | "collections"
  | "views"
  | "hooks"
  | "instructions"
  | "templates"
  | "artifacts"
  | "routines";

/** Toolset connection kind, only meaningful when `kind === "toolsets"` (`FractalMarketplaceToolsetConnectionTypeSchema`). */
export type FractalMarketplaceToolsetConnectionType =
  | "custom"
  | "mcp-server::stdio"
  | "mcp-server::http"
  | "rest-api"
  | "cli";

/** One installed-plugin inventory row (a toolset, view, artifact, etc.) — `FractalMarketplaceSkillComponentItemSchema`. */
export interface FractalMarketplaceSkillComponentItem {
  name: string;
  kind: FractalMarketplaceSkillComponentKind;
  label: string;
  description: string;
  /** Present for registry (not-yet-installed) items — links to source on GitHub. */
  githubUrl?: string;
  /** Runtime entity id when the item is installed locally; falls back to `name` when absent. */
  id?: string;
  /** Workspace-relative file path — present for file-backed items (edit/open-in-tab). */
  path?: string;
  connectionType?: FractalMarketplaceToolsetConnectionType;
  /** Lifecycle status when applicable, e.g. a routine's enabled/disabled state. */
  status?: string;
  active?: boolean;
  instructionType?: string;
}

/** A marketplace search-result / card entry (`FractalMarketplaceSkillListingSchema`). */
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

/** Author block on a plugin manifest (`FractalMarketplaceSkillAuthorSchema`). */
export interface FractalMarketplaceSkillAuthor {
  name: string;
  email?: string;
  url?: string;
}

/** UI/marketing block on a plugin manifest (`FractalMarketplaceSkillInterfaceSchema`). */
export interface FractalMarketplaceSkillInterface {
  displayName: string;
  shortDescription: string;
  longDescription?: string;
  developerName?: string;
  category: string;
  capabilities?: string[];
  websiteURL?: string;
  privacyPolicyURL?: string;
  termsOfServiceURL?: string;
  defaultPrompt?: string[];
  brandColor?: string;
  composerIcon?: string;
  logo?: string;
  screenshots?: string[];
}

/** Full plugin manifest, as returned by `marketplace.getByName` (`FractalMarketplaceSkillSchema`). */
export interface FractalMarketplaceSkill {
  name: string;
  version: string;
  description: string;
  author: FractalMarketplaceSkillAuthor;
  homepage: string;
  repository: string;
  path: string;
  license: string;
  keywords?: string[];
  interface: FractalMarketplaceSkillInterface;
}

/** Inventory items grouped by kind, returned alongside a plugin's manifest — every bucket is always present, empty when there is nothing installed of that kind (`FractalMarketplaceSkillInventorySchema`). */
export interface FractalMarketplaceSkillInventory {
  toolsets: FractalMarketplaceSkillComponentItem[];
  collections: FractalMarketplaceSkillComponentItem[];
  views: FractalMarketplaceSkillComponentItem[];
  hooks: FractalMarketplaceSkillComponentItem[];
  instructions: FractalMarketplaceSkillComponentItem[];
  templates: FractalMarketplaceSkillComponentItem[];
  artifacts: FractalMarketplaceSkillComponentItem[];
  routines: FractalMarketplaceSkillComponentItem[];
}

/** The locally-installed record for a plugin, merged into marketplace detail responses (`FractalMarketplaceInstalledSkillSchema`). */
export interface FractalMarketplaceInstalledSkill {
  id: string;
  name: string;
  active: boolean;
  path: string;
  hasManifest: boolean;
  skillMdPath: string;
  manifestPath?: string;
}
