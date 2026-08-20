import { aos } from "@/app/aos";
import type { Skill } from "@/features/skill/interfaces/skill.interfaces";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { getRelatedListings } from "@/features/marketplace/presentation/helpers/marketplace.helper";
import { isDormant } from "@/lib/command-map";
import { DormantGate } from "@/components/DormantDomain";
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

    // Task 10: the `marketplace` domain is dormant — no Go backend to call
    // yet. Short-circuits before any client call so the dormant commands'
    // empty envelopes never reach the `!plugin || !inventory` check below,
    // which would otherwise call `response.notFound()` and preempt
    // `DormantGate` (wrapping the returned JSX in `withComponent`) with
    // the 404 page instead.
    if (isDormant("marketplace")) {
      return {
        plugin: undefined as any,
        inventory: undefined as any,
        sourceUrl: "",
        isInstalled: false,
        installedSkill: undefined as any,
        related: [] as any[],
        installedNames: [] as string[],
      };
    }

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
        (item: Skill) => item.name,
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
      <DormantGate feature="marketplace">
        <MarketplaceDetailsPageInner
          plugin={plugin}
          inventory={inventory}
          sourceUrl={sourceUrl}
          isInstalled={isInstalled}
          installedSkill={installedSkill}
          related={related}
          installedNames={installedNames}
        />
      </DormantGate>
    );
  })
  .build();
export default MarketplaceDetailsPage;
