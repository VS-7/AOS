import { aos } from "@/app/aos";
import type { FractalSkill } from "@/features/skill/interfaces/skill.interfaces";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { getRelatedListings } from "@/features/marketplace/presentation/helpers/marketplace.helper";
import { MarketplaceDetailsPageInner } from "./inner";

/**
 * MarketplaceDetailsPage: detail route for a marketplace plugin.
 * Uses install-aware getByName (registry + local custom skills).
 */
export const MarketplaceDetailsPage = aos
  .page("/marketplace/$name")
  .withMetadata({
    title: "Plugin Details",
    description: "Explore plugin capabilities and manage installation.",
  })
  .use(WorkspacePageMiddleware())
  .withLoader(async ({ client, request, response }) => {
    const { name } = request.params;

    const [detailRes, listRes] = await Promise.all([
      client.marketplace.getByName.query({
        params: { name },
      }),
      client.marketplace.list.query({
        query: { page: 1, pageSize: 100 },
      }),
    ]);

    const plugin = detailRes.data?.skill;
    const inventory = detailRes.data?.inventory;
    const sourceUrl = detailRes.data?.sourceUrl ?? "";
    const isInstalled = Boolean(detailRes.data?.isInstalled);
    const installedSkill = detailRes.data?.installedSkill;
    const allListings = listRes.data?.items || [];

    if (!plugin || !inventory) {
      return response.notFound();
    }

    const related = getRelatedListings(allListings, plugin);
    const installedNames = isInstalled ? [plugin.name] : [];

    // Also mark related installed plugins when possible via a light skill list.
    let resolvedInstalledNames = installedNames;
    try {
      const installedRes = await client.skill.list.query({ query: {} });
      resolvedInstalledNames = (installedRes.data?.skills || []).map(
        (item: FractalSkill) => item.name,
      );
    } catch {
      // keep fallback
    }

    return {
      plugin,
      inventory,
      sourceUrl,
      isInstalled,
      installedSkill,
      related,
      installedNames: resolvedInstalledNames,
    };
  })
  .withComponent(({ route }) => {
    const {
      plugin,
      inventory,
      sourceUrl,
      isInstalled,
      installedSkill,
      related,
      installedNames,
    } = route.useLoaderData();

    return (
      <MarketplaceDetailsPageInner
        plugin={plugin}
        inventory={inventory}
        sourceUrl={sourceUrl}
        isInstalled={isInstalled}
        installedSkill={installedSkill}
        related={related}
        installedNames={installedNames}
      />
    );
  })
  .build();
export default MarketplaceDetailsPage;
