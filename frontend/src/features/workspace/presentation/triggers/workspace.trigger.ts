import { AosTriggerGroup } from "@/app/builders/trigger";
import type { WorkspaceContext } from "../stores/workspace.store";

/**
 * Workspace-level triggers (switch workspace only).
 * Settings navigation lives in {@link settingsGroup}.
 */
export const workspaceGroup = AosTriggerGroup.create("Workspaces")
  .withOrder(6)
  .withLoader(async ({ stores }) => {
    const options: WorkspaceContext[] = stores.workspace.state.options;

    return options.map((workspace: WorkspaceContext) => ({
      id: `workspace.switch.${workspace.id}`,
      label: `Switch to ${workspace.name}`,
      icon: "RotateCw",
      group: "Workspaces",
      metadata: { active: workspace.active },
      handler: async ({ stores }) => {
        const result = await stores.workspace.actions.switch(workspace.id);
        if (result.error) {
          console.error(result.error);
          return;
        }
        if (typeof window !== "undefined") {
          window.location.reload();
        }
      },
    }));
  })
  .build();
