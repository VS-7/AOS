import type {
  InstalledSkillRecord,
  MarketplacePluginDetail,
  MarketplaceRegistryListing,
  MarketplaceSkillComponentItem,
  MarketplaceSkillComponentKind,
  MarketplaceSkillInventory,
  MarketplaceSkillListing,
  MarketplaceToolsetConnectionType,
} from "@/features/marketplace/interfaces/marketplace.interfaces";
import {
  MARKETPLACE_ALLOWED_CATEGORIES,
  MARKETPLACE_FEATURED_PLUGIN_SLUGS,
  MARKETPLACE_INVENTORY_ORDER,
} from "@/features/marketplace/presentation/consts/marketplace";

/** The section a listing with no recognised category is shown under. */
export const MARKETPLACE_OTHER_CATEGORY = "Other";

/**
 * The category a registry listing belongs to: the first of its tags that
 * names one of the marketplace's categories, whatever its case. Go's Listing
 * has tags and no category at all.
 */
function categoryFromTags(tags: string[]): string {
  for (const tag of tags) {
    const match = MARKETPLACE_ALLOWED_CATEGORIES.find((category) => category.toLowerCase() === tag.trim().toLowerCase());
    if (match) return match;
  }
  return MARKETPLACE_OTHER_CATEGORY;
}

/**
 * A registry's listing, in the shape the marketplace screens render.
 *
 * The screens were written against a richer listing than Go's, and read
 * `displayName`, `keywords` and `capabilities` off it unguarded — the page
 * crashed the moment any registry answered. Routed by `source`, the only key
 * marketplace_get can find a listing by.
 */
export function toMarketplaceListing(listing: MarketplaceRegistryListing): MarketplaceSkillListing {
  const source = listing.source ?? "";
  const description = listing.description ?? "";
  const tags = Array.isArray(listing.tags) ? listing.tags : [];
  return {
    name: source,
    displayName: listing.name?.trim() || source,
    shortDescription: description,
    description,
    logo: null,
    homepage: "",
    author: source.includes("/") ? source.split("/")[0] : "",
    category: categoryFromTags(tags),
    keywords: tags,
    capabilities: [],
    brandColor: null,
    source,
    registry: listing.registry,
    version: listing.version,
    stars: listing.stars,
    updatedAt: listing.updatedAt,
    permissions: listing.permissions,
  };
}

const CONNECTION_TYPES: ReadonlySet<string> = new Set(["custom", "mcp-server::stdio", "mcp-server::http", "rest-api", "cli"]);

function emptyInventory(): MarketplaceSkillInventory {
  return Object.fromEntries(MARKETPLACE_INVENTORY_ORDER.map((kind) => [kind, []])) as unknown as MarketplaceSkillInventory;
}

/**
 * What an installed skill brought, as the inventory sections list it — read
 * from the skill's own `metadata`, which is the record of what the install
 * actually applied.
 */
function inventoryFromSkill(skill: InstalledSkillRecord): MarketplaceSkillInventory {
  const inventory = emptyInventory();
  const metadata = skill.metadata ?? {};
  const add = (kind: MarketplaceSkillComponentKind, refs: Array<{ id: string; type?: string }> | undefined) => {
    for (const ref of refs ?? []) {
      if (!ref?.id) continue;
      const item: MarketplaceSkillComponentItem = { name: ref.id, id: ref.id, kind, label: ref.id, description: "" };
      if (kind === "toolsets" && ref.type && CONNECTION_TYPES.has(ref.type)) {
        item.connectionType = ref.type as MarketplaceToolsetConnectionType;
      }
      inventory[kind].push(item);
    }
  };
  add("toolsets", metadata.toolsets);
  add("collections", metadata.collections);
  add("views", metadata.views);
  add("hooks", metadata.hooks);
  add("artifacts", metadata.artifacts);
  add("templates", metadata.templates);
  add("instructions", metadata.instructions);
  return inventory;
}

/**
 * The plugin page for an installed skill, merged with its registry listing
 * when there is one. Installed skills are not registry listings, so their
 * page used to be "Page not found" — the only route asked marketplace_get.
 */
export function installedSkillDetail(
  skill: InstalledSkillRecord | undefined,
  listing?: MarketplaceSkillListing,
): MarketplacePluginDetail {
  const name = skill?.id ?? listing?.name ?? "";
  return {
    plugin: {
      name,
      displayName: listing?.displayName ?? skill?.name ?? name,
      description: listing?.description || skill?.description || "",
      category: listing?.category ?? MARKETPLACE_OTHER_CATEGORY,
      author: listing?.author ?? "",
      version: skill?.version || listing?.version,
      source: skill?.source || listing?.source,
      registry: listing?.registry,
      permissions: skill?.permissions ?? listing?.permissions,
    },
    inventory: skill ? inventoryFromSkill(skill) : emptyInventory(),
    isInstalled: Boolean(skill),
    installedSkill: skill
      ? {
          id: skill.id,
          name: skill.name,
          active: skill.active,
          path: `.aos/skills/${skill.id}`,
          hasManifest: false,
          skillMdPath: `.aos/skills/${skill.id}/SKILL.md`,
        }
      : undefined,
  };
}

/**
 * A toolset's environment after saving what was typed.
 *
 * toolsets_update-config replaces Env wholesale, so the map sent has to be the
 * whole of it: every variable the toolset already had, with only the fields
 * typed this time (non-blank) laid over it. Sending just the typed fields
 * erased the rest.
 */
export function mergeEnv(
  current: Record<string, string> | undefined,
  typed: Record<string, string>,
): Record<string, string> {
  const next: Record<string, string> = { ...(current ?? {}) };
  for (const [key, value] of Object.entries(typed)) {
    if (value.trim()) next[key] = value.trim();
  }
  return next;
}

export function marketplaceSectionId(label: string): string {
  return label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

export function groupListingsByCategory(
  listings: MarketplaceSkillListing[],
): Record<string, MarketplaceSkillListing[]> {
  const grouped = Object.fromEntries(
    MARKETPLACE_ALLOWED_CATEGORIES.map((category) => [
      category,
      [] as MarketplaceSkillListing[],
    ]),
  ) as Record<string, MarketplaceSkillListing[]>;

  const uncategorized: MarketplaceSkillListing[] = [];

  for (const listing of listings) {
    if (grouped[listing.category]) {
      grouped[listing.category].push(listing);
    } else {
      uncategorized.push(listing);
    }
  }

  if (uncategorized.length > 0) {
    grouped[MARKETPLACE_OTHER_CATEGORY] = uncategorized;
  }

  return grouped;
}

/**
 * Only curated featured slugs that exist in `listings` — no fallback padding.
 * A slug is a listing's short name: the display name, or the repository half
 * of its "owner/repo" source.
 */
export function pickFeaturedListings(
  listings: MarketplaceSkillListing[],
): MarketplaceSkillListing[] {
  const slugOf = (listing: MarketplaceSkillListing) =>
    [listing.name, listing.displayName, listing.name.split("/").pop() ?? ""].map((value) => value.toLowerCase());

  return MARKETPLACE_FEATURED_PLUGIN_SLUGS.map((slug) =>
    listings.find((listing) => slugOf(listing).includes(slug)),
  ).filter((listing): listing is MarketplaceSkillListing => listing !== undefined);
}

export function filterListings(
  listings: MarketplaceSkillListing[],
  options: { query?: string; category?: string },
): MarketplaceSkillListing[] {
  const normalizedQuery = (options.query ?? "").trim().toLowerCase();
  const normalizedCategory = (options.category ?? "").trim().toLowerCase();

  return listings.filter((listing) => {
    const matchesCategory =
      !normalizedCategory ||
      listing.category.toLowerCase() === normalizedCategory;

    if (!matchesCategory) return false;
    if (!normalizedQuery) return true;

    const haystack = [
      listing.name,
      listing.displayName,
      listing.description,
      listing.shortDescription,
      listing.author,
      listing.category,
      ...(listing.keywords ?? []),
      ...(listing.capabilities ?? []),
    ]
      .join(" ")
      .toLowerCase();

    return haystack.includes(normalizedQuery);
  });
}

/**
 * The card an installed skill shows as: the registry listing it was installed
 * from when one is listed (matched by source), otherwise a listing of its own,
 * routed by the skill's id.
 */
export function listingForInstalled(
  skill: InstalledSkillRecord,
  marketplacePlugins: MarketplaceSkillListing[],
): MarketplaceSkillListing {
  const fromRegistry = skill.source
    ? marketplacePlugins.find((listing) => listing.source === skill.source)
    : undefined;
  if (fromRegistry) return fromRegistry;
  return {
    ...resolveInstalledListing(skill.id, skill.description ?? "", []),
    displayName: skill.name || skill.id,
    source: skill.source,
    version: skill.version,
  };
}

export function getRelatedListings(
  listings: MarketplaceSkillListing[],
  current: { name: string; source?: string; category: string },
  limit = 5,
): MarketplaceSkillListing[] {
  return listings
    .filter(
      (listing) =>
        listing.name !== current.name &&
        listing.name !== current.source &&
        listing.category === current.category,
    )
    .slice(0, limit);
}

export function resolveInstalledListing(
  pluginName: string,
  description: string,
  marketplacePlugins: MarketplaceSkillListing[],
): MarketplaceSkillListing {
  const fromMarketplace = marketplacePlugins.find(
    (listing) => listing.name === pluginName,
  );

  if (fromMarketplace) {
    return fromMarketplace;
  }

  return {
    name: pluginName,
    displayName: pluginName,
    shortDescription: description,
    description,
    logo: null,
    homepage: "",
    author: "",
    category: MARKETPLACE_OTHER_CATEGORY,
    keywords: [],
    capabilities: [],
    brandColor: null,
  };
}
