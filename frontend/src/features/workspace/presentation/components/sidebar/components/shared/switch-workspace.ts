import { aos } from "@/app/aos";
import { toast } from "sonner";

/**
 * Switches the active workspace, syncs the cookie via the store action,
 * and hard-navigates to `/` so loaders and the WebSocket rebind.
 *
 * @param id - Target workspace id.
 * @returns `true` when the switch succeeded, `false` otherwise.
 *
 * @example
 * ```typescript
 * await switchWorkspace("fractal");
 * ```
 */
export async function switchWorkspace(id: string): Promise<boolean> {
  const result = await aos.stores.workspace.actions.switch(id);

  if (result.error) {
    toast.error(result.error.message || "Failed to switch workspace");
    return false;
  }

  window.location.replace("/");
  return true;
}
