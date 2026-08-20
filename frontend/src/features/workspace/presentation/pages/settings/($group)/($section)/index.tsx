import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { SettingsShell } from "@/features/workspace/presentation/components/settings/settings-shell";
import { SettingsRouteHelper } from "@/features/workspace/presentation/helpers/settings-route.helper";
import { SETTINGS_SECTION_MAP } from "@/features/workspace/presentation/components/settings/constants";

/**
 * Renders a single settings section at `/settings/$group/$section`.
 */
export const SettingsSectionPage = aos
  .page("/settings/$group/$section")
  .withMetadata({
    title: "Settings",
    description: "Configure user and workspace settings.",
  })
  .use(WorkspacePageMiddleware())
  .withLoader(({ request, response, stores }) => {
    const { group, section } = request.params;

    const legacySectionId = SettingsRouteHelper.resolveLegacyRedirect(
      group,
      section,
    );
    if (legacySectionId) {
      return response.redirect(
        SettingsRouteHelper.sectionIdToPath(legacySectionId),
      );
    }

    const sectionId = SettingsRouteHelper.pathToSectionId(group, section);

    if (!sectionId) {
      return response.notFound();
    }

    stores.viewport.actions.setSidebarMenu("settings");

    const definition = SETTINGS_SECTION_MAP[sectionId];

    return {
      sectionId,
      title: definition.title,
      description: definition.description,
    };
  })
  .withComponent(({ route }) => {
    const { sectionId } = route.useLoaderData();
    return <SettingsShell sectionId={sectionId} />;
  })
  .build();

export default SettingsSectionPage;
