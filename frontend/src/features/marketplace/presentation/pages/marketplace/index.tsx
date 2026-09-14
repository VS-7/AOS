import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { Schema } from "@/core/helpers/schema.helper";
import { z } from "zod";
import { DormantGate } from "@/components/DormantDomain";
import type {
  InstalledSkillRecord,
  MarketplaceRegistryListing,
} from "@/features/marketplace/interfaces/marketplace.interfaces";
import { toMarketplaceListing } from "@/features/marketplace/presentation/helpers/marketplace.helper";
import { MarketplacePageInner } from "./inner";

const MarketplacePageSearchSchema = Schema.object({
  category: z.string().optional(),
  query: z.string().optional(),
});

/** A refusal the page shows, kept as plain data so the loader can hand it over. */
export interface MarketplaceLoadError {
  code: string;
  message: string;
}

function asLoadError(error: unknown): MarketplaceLoadError | null {
  if (!error) return null;
  const { code, message } = error as { code?: unknown; message?: unknown };
  return { code: typeof code === "string" ? code : "", message: typeof message === "string" ? message : "" };
}

/**
 * MarketplacePage: list route for the Plugin Marketplace.
 * Loads public marketplace plugins and currently installed workspace plugins.
 *
 * Both reads keep their refusal. The page used to drop them and render an
 * empty result — "No plugins matched your search. Try another query" — when
 * the truth was that no registry is configured at all.
 */
export const MarketplacePage = aos
  .page("/marketplace")
  .withMetadata({
    title: "Marketplace",
    description: "Discover and install plugins directly into your workspace.",
  })
  .withQuery(MarketplacePageSearchSchema)
  .use(WorkspacePageMiddleware())
  .withLoader(async ({ client, request }) => {
    const query = request.query || {};

    const [marketplaceRes, installedRes] = await Promise.all([
      client.marketplace.list.query({
        query: {
          category: query.category || undefined,
          query: query.query?.trim() || undefined,
        },
      }),
      client.skill.list.query({ query: {} }),
    ]);

    const registryListings: MarketplaceRegistryListing[] = Array.isArray(marketplaceRes.data?.items)
      ? marketplaceRes.data.items
      : [];
    const marketplacePlugins = registryListings.map(toMarketplaceListing);
    const installedPlugins: InstalledSkillRecord[] = installedRes.data?.skills || [];

    return {
      marketplacePlugins,
      installedPlugins,
      marketplaceError: asLoadError(marketplaceRes.error),
      installedError: asLoadError(installedRes.error),
    };
  })
  .withComponent(({ route }) => {
    const { marketplacePlugins, installedPlugins, marketplaceError, installedError } = route.useLoaderData();
    const search = route.useSearch();

    return (
      <DormantGate feature="marketplace">
        <MarketplacePageInner
          marketplacePlugins={marketplacePlugins}
          installedPlugins={installedPlugins}
          marketplaceError={marketplaceError}
          installedError={installedError}
          search={search}
        />
      </DormantGate>
    );
  })
  .build();
export default MarketplacePage;
