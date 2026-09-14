/**
 * There is no AOS Go backend for this domain yet, but real, checkable
 * declarations exist in the old AOS server:
 * `v401/server/src/features/marketplace/schemas/marketplace.schema.ts`'s
 * Zod schemas — the type-only `interfaces/marketplace.interfaces.ts` that
 * would normally declare these was itself erased by the bundler, same as
 * everywhere else, so the schema file is the best available ground truth.
 * Recovered, not guessed — this replaces an earlier pass of this file that
 * was reconstructed from frontend usage alone (`v401/web/src/features/
 * marketplace/presentation/**`); several fields below are required or
 * present that no usage site in the ported frontend ever touched, and one
 * (`MarketplaceSkill`) was missing over half its real fields
 * entirely. When AOS grows a Go backend for this, re-verify against that
 * instead.
 */

/** The 8 marketplace inventory sections (`MarketplaceSkillComponentKindSchema`). */
export type MarketplaceSkillComponentKind =
  | "toolsets"
  | "collections"
  | "views"
  | "hooks"
  | "instructions"
  | "templates"
  | "artifacts"
  | "routines";

/** Toolset connection kind, only meaningful when `kind === "toolsets"` (`MarketplaceToolsetConnectionTypeSchema`). */
export type MarketplaceToolsetConnectionType =
  | "custom"
  | "mcp-server::stdio"
  | "mcp-server::http"
  | "rest-api"
  | "cli";

/** One installed-plugin inventory row (a toolset, view, artifact, etc.) — `MarketplaceSkillComponentItemSchema`. */
export interface MarketplaceSkillComponentItem {
  name: string;
  kind: MarketplaceSkillComponentKind;
  label: string;
  description: string;
  /** Present for registry (not-yet-installed) items — links to source on GitHub. */
  githubUrl?: string;
  /** Runtime entity id when the item is installed locally; falls back to `name` when absent. */
  id?: string;
  /** Workspace-relative file path — present for file-backed items (edit/open-in-tab). */
  path?: string;
  connectionType?: MarketplaceToolsetConnectionType;
  /** Lifecycle status when applicable, e.g. a routine's enabled/disabled state. */
  status?: string;
  active?: boolean;
  instructionType?: string;
}

/** A marketplace search-result / card entry (`MarketplaceSkillListingSchema`). */
export interface MarketplaceSkillListing {
  /**
   * What the card routes by: a registry listing's `source` ("owner/repo"),
   * which is what marketplace_get looks a listing up by, or an installed
   * skill's id. Never the display name, which neither command knows.
   */
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
  /** The registry fields, present for a listing a registry offered. */
  source?: string;
  registry?: string;
  version?: string;
  stars?: number;
  updatedAt?: string;
  permissions?: Record<string, unknown>;
}

/**
 * What marketplace_discovery and marketplace_get actually answer
 * (`internal/domain/marketplace/entity.go`'s `Listing`). The screens render
 * {@link MarketplaceSkillListing}; `toMarketplaceListing` is the one place
 * the first becomes the second.
 */
export interface MarketplaceRegistryListing {
  registry: string;
  source: string;
  name: string;
  description?: string;
  version?: string;
  tags?: string[];
  stars?: number;
  updatedAt?: string;
  permissions?: Record<string, unknown>;
}

/**
 * An installed skill, as skills_list answers it
 * (`internal/domain/skill/entity.go`'s `Skill`) — only the fields the
 * marketplace reads.
 */
export interface InstalledSkillRecord {
  id: string;
  name: string;
  description?: string;
  active: boolean;
  version?: string;
  source?: string;
  permissions?: Record<string, unknown>;
  metadata?: {
    toolsets?: Array<{ id: string; type?: string }>;
    collections?: Array<{ id: string }>;
    views?: Array<{ id: string }>;
    hooks?: Array<{ id: string }>;
    artifacts?: Array<{ id: string }>;
    templates?: Array<{ id: string }>;
    instructions?: Array<{ id: string }>;
  };
}

/**
 * What the plugin page shows: a registry listing, an installed skill, or
 * both when the installed skill came from that listing.
 */
export interface MarketplacePluginDetail {
  plugin: {
    name: string;
    displayName: string;
    description: string;
    category: string;
    author: string;
    version?: string;
    source?: string;
    registry?: string;
    permissions?: Record<string, unknown>;
  };
  inventory: MarketplaceSkillInventory;
  isInstalled: boolean;
  installedSkill?: MarketplaceInstalledSkill;
}

/** Author block on a plugin manifest (`MarketplaceSkillAuthorSchema`). */
export interface MarketplaceSkillAuthor {
  name: string;
  email?: string;
  url?: string;
}

/** UI/marketing block on a plugin manifest (`MarketplaceSkillInterfaceSchema`). */
export interface MarketplaceSkillInterface {
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

/** Full plugin manifest, as returned by `marketplace.getByName` (`MarketplaceSkillSchema`). */
export interface MarketplaceSkill {
  name: string;
  version: string;
  description: string;
  author: MarketplaceSkillAuthor;
  homepage: string;
  repository: string;
  path: string;
  license: string;
  keywords?: string[];
  interface: MarketplaceSkillInterface;
}

/** Inventory items grouped by kind, returned alongside a plugin's manifest — every bucket is always present, empty when there is nothing installed of that kind (`MarketplaceSkillInventorySchema`). */
export interface MarketplaceSkillInventory {
  toolsets: MarketplaceSkillComponentItem[];
  collections: MarketplaceSkillComponentItem[];
  views: MarketplaceSkillComponentItem[];
  hooks: MarketplaceSkillComponentItem[];
  instructions: MarketplaceSkillComponentItem[];
  templates: MarketplaceSkillComponentItem[];
  artifacts: MarketplaceSkillComponentItem[];
  routines: MarketplaceSkillComponentItem[];
}

/** The locally-installed record for a plugin, merged into marketplace detail responses (`MarketplaceInstalledSkillSchema`). */
export interface MarketplaceInstalledSkill {
  id: string;
  name: string;
  active: boolean;
  path: string;
  hasManifest: boolean;
  skillMdPath: string;
  manifestPath?: string;
}
