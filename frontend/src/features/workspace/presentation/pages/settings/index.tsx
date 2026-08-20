import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { SettingsRouteHelper } from "@/features/workspace/presentation/helpers/settings-route.helper";

/**
 * Redirects `/settings` to the default settings section.
 */
export const SettingsIndexPage = aos
  .page("/settings")
  .withMetadata({
    title: "Settings",
    description: "Configure user and workspace settings.",
  })
  .use(WorkspacePageMiddleware())
  .withLoader(({ response, stores }) => {
    stores.viewport.actions.setSidebarMenu("settings");
    return response.redirect(SettingsRouteHelper.defaultPath());
  })
  .withComponent(() => null)
  .build();

export default SettingsIndexPage;
