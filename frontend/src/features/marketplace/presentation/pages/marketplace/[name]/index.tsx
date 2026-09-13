import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import type {
  InstalledSkillRecord,
  MarketplaceRegistryListing,
  MarketplaceSkillListing,
} from "@/features/marketplace/interfaces/marketplace.interfaces";
import {
  getRelatedListings,
  installedSkillDetail,
  toMarketplaceListing,
} from "@/features/marketplace/presentation/helpers/marketplace.helper";
import { DormantGate } from "@/components/DormantDomain";
import { MarketplaceDetailsPageInner } from "./inner";

/** Go's answer when no configured registry lists the source asked for. */
const LISTING_NOT_FOUND = "AOS_MARKETPLACE_LISTING_NOT_FOUND";

/**
 * MarketplaceDetailsPage: one plugin — an installed skill, a registry
 * listing, or both.
 *
 * `$name` is what the card routed by: an installed skill's id, or a registry
 * listing's "owner/repo" source. The page used to ask marketplace_get for
 * every name and require an `inventory` Go never returns, so it was "Page not
 * found" for every plugin — installed ones included, which are not registry
 * listings at all.
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

    const installedRes = await client.skill.list.query({ query: {} });
    const installed: InstalledSkillRecord[] = installedRes.data?.skills ?? [];
    const installedSkill = installed.find((skill) => skill.id === name) ??
      installed.find((skill) => !!skill.source && skill.source === name);

    // Only a source names a registry listing; an installed skill's page asks
    // for the listing it was installed from, when it has one.
    const source = name.includes("/") ? name : installedSkill?.source;
    let listing: MarketplaceSkillListing | undefined;
    let loadError: { code: string; message: string } | null = null;
    let related: MarketplaceSkillListing[] = [];
    if (source) {
      const [detailRes, listRes] = await Promise.all([
        client.marketplace.getByName.query({ params: { name: source } }),
        client.marketplace.list.query({ query: {} }),
      ]);
      const found = detailRes.data?.listing as MarketplaceRegistryListing | undefined;
      listing = found ? toMarketplaceListing(found) : undefined;
      if (detailRes.error && detailRes.error.code !== LISTING_NOT_FOUND) {
        loadError = { code: detailRes.error.code ?? "", message: detailRes.error.message ?? "" };
      }
      const all: MarketplaceRegistryListing[] = Array.isArray(listRes.data?.items) ? listRes.data.items : [];
      if (listing) related = getRelatedListings(all.map(toMarketplaceListing), listing);
    }

    if (!installedSkill && !listing) {
      // A registry that could not be asked is not a plugin that does not
      // exist: say why, rather than "Page not found".
      if (loadError || installedRes.error) {
        return {
          detail: null,
          related,
          installedNames: [] as string[],
          loadError: loadError ?? { code: installedRes.error?.code ?? "", message: installedRes.error?.message ?? "" },
        };
      }
      return response.notFound();
    }

    return {
      detail: installedSkillDetail(installedSkill, listing),
      related,
      installedNames: installed.flatMap((skill) => [skill.id, skill.source ?? ""]).filter(Boolean),
      loadError: null,
    };
  })
  .withComponent(({ route }) => {
    const { detail, related, installedNames, loadError } = route.useLoaderData();

    return (
      <DormantGate feature="marketplace">
        <MarketplaceDetailsPageInner
          detail={detail}
          loadError={loadError}
          related={related}
          installedNames={installedNames}
        />
      </DormantGate>
    );
  })
  .build();
export default MarketplaceDetailsPage;
