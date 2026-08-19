import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { Schema } from "@/core/helpers/schema.helper";
import { z } from "zod";
import { DormantGate } from "@/components/DormantDomain";
import { MarketplacePageInner } from "./inner";

const MarketplacePageSearchSchema = Schema.object({
  category: z.string().optional(),
  query: z.string().optional(),
});

/**
 * MarketplacePage: list route for the Plugin Marketplace.
 * Loads public marketplace plugins and currently installed workspace plugins.
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

    const marketplacePlugins = marketplaceRes.data?.items || [];
    const installedPlugins = installedRes.data?.skills || [];

    return { marketplacePlugins, installedPlugins };
  })
  .withComponent(({ route }) => {
    const { marketplacePlugins, installedPlugins } = route.useLoaderData();
    const search = route.useSearch();

    return (
      <DormantGate feature="marketplace">
        <MarketplacePageInner
          marketplacePlugins={marketplacePlugins}
          installedPlugins={installedPlugins}
          search={search}
        />
      </DormantGate>
    );
  })
  .build();
export default MarketplacePage;
