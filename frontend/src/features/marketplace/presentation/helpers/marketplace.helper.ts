import type {
  MarketplaceSkill,
  MarketplaceSkillListing,
} from "@/features/marketplace/interfaces/marketplace.interfaces";
import {
  MARKETPLACE_ALLOWED_CATEGORIES,
  MARKETPLACE_FEATURED_PLUGIN_SLUGS,
} from "@/features/marketplace/presentation/consts/marketplace";

export function marketplaceSectionId(label: string): string {
  return label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

export function buildInstallCommand(pluginName: string): string {
  return `aos skills install --source aos/registry --skill ${pluginName}`;
}

export function buildGithubFolderUrl(pluginName: string, folder: string): string {
  return `https://github.com/aos/registry/tree/main/skills/${encodeURIComponent(pluginName)}/${folder}`;
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
    grouped.Other = uncategorized;
  }

  return grouped;
}

/** Only curated featured slugs that exist in `listings` — no fallback padding. */
export function pickFeaturedListings(
  listings: MarketplaceSkillListing[],
): MarketplaceSkillListing[] {
  const byName = new Map(listings.map((listing) => [listing.name, listing]));

  return MARKETPLACE_FEATURED_PLUGIN_SLUGS.map((slug) => byName.get(slug)).filter(
    (listing): listing is MarketplaceSkillListing => listing !== undefined,
  );
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
      ...listing.keywords,
      ...listing.capabilities,
    ]
      .join(" ")
      .toLowerCase();

    return haystack.includes(normalizedQuery);
  });
}

/**
 * Merge marketplace search hits with matching installed-only plugins.
 * Dedupes by plugin name so installed registry plugins are not listed twice.
 */
export function mergeListingsWithInstalled(
  marketplaceListings: MarketplaceSkillListing[],
  installed: Array<{ name: string; description: string }>,
  allMarketplaceListings: MarketplaceSkillListing[],
  query?: string,
): MarketplaceSkillListing[] {
  const byName = new Map(
    marketplaceListings.map((listing) => [listing.name, listing]),
  );
  const normalizedQuery = (query ?? "").trim().toLowerCase();

  for (const plugin of installed) {
    if (byName.has(plugin.name)) continue;

    if (normalizedQuery) {
      const haystack = `${plugin.name} ${plugin.description}`.toLowerCase();
      if (!haystack.includes(normalizedQuery)) continue;
    }

    byName.set(
      plugin.name,
      resolveInstalledListing(
        plugin.name,
        plugin.description,
        allMarketplaceListings,
      ),
    );
  }

  return [...byName.values()];
}

export function getRelatedListings(
  listings: MarketplaceSkillListing[],
  current: MarketplaceSkill,
  limit = 5,
): MarketplaceSkillListing[] {
  return listings
    .filter(
      (listing) =>
        listing.name !== current.name &&
        listing.category === current.interface.category,
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
    category: "Other",
    keywords: [],
    capabilities: [],
    brandColor: null,
  };
}
