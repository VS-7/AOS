import { aos } from "@/app/aos";
import { toast } from "sonner";
import { reloadAt } from "@/lib/wails";

/**
 * Switches the active workspace, syncs the cookie via the store action,
 * and hard-navigates to `/` so loaders and the WebSocket rebind.
 *
 * The navigation goes through `reloadAt` rather than `location.replace`. The
 * desktop window carries the daemon's address in its own query string — it is
 * served from `wails://localhost` and has no other way to know where the API
 * is — so replacing the URL with a bare `/` left the window with no API
 * origin, no event channel and no `window.aos`, and switching workspace was
 * the fastest way to break the application until it was restarted.
 *
 * @param id - Target workspace id.
 * @returns `true` when the switch succeeded, `false` otherwise.
 *
 * @example
 * ```typescript
 * await switchWorkspace("my-workspace");
 * ```
 */
export async function switchWorkspace(id: string): Promise<boolean> {
  const result = await aos.stores.workspace.actions.switch(id);

  if (result.error) {
    toast.error(result.error.message || "Failed to switch workspace");
    return false;
  }

  reloadAt("/");
  return true;
}
