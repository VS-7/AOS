import { AosTriggerGroup } from "@/app/builders/trigger";
import type { WorkspaceContext } from "../stores/workspace.store";
import { reloadHere } from "@/lib/wails";
import { toast } from "sonner";
import { errorMessage } from "@/lib/aos-facade";
import { t } from "@/lib/i18n";

/**
 * Workspace-level triggers (switch workspace only).
 * Settings navigation lives in {@link settingsGroup}.
 */
export const workspaceGroup = AosTriggerGroup.create("Workspaces")
  .withOrder(6)
  .withLoader(async ({ stores }) => {
    const options: WorkspaceContext[] = stores.workspace.state.options;
    // A workspace carries no `active` flag of its own; the one this window
    // addresses is the store's `current`, so that is what gets the check.
    const currentId: string | undefined = stores.workspace.state.current?.id;

    return options.map((workspace: WorkspaceContext) => ({
      id: `workspace.switch.${workspace.id}`,
      label: t("Switch to {{name}}", { name: workspace.name }),
      icon: "RotateCw",
      group: "Workspaces",
      metadata: { active: workspace.id === currentId },
      handler: async ({ stores }) => {
        const result = await stores.workspace.actions.switch(workspace.id);
        if (result.error) {
          // Picked from the command palette, which has already closed: the
          // console was the only place this refusal ever went.
          toast.error(t("Failed to switch workspace"), { description: errorMessage(result.error) });
          return;
        }
        if (typeof window !== "undefined") {
          reloadHere();
        }
      },
    }));
  })
  .build();
