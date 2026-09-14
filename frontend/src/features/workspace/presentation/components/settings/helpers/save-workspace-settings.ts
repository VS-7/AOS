import { aos } from "@/app/aos";
import { api } from "@/lib/aos-facade";

/**
 * Writes a dotted-path patch to the workspace and makes the answer the
 * snapshot every workspace settings section reads.
 *
 * Returns whether anything was sent: an empty patch is not a save — the
 * daemon refuses one (`set` takes at least one field) and a form that toggled
 * a switch and back within the autosave delay has nothing to report. A
 * refusal throws, so the form's own error path shows the daemon's message.
 */
export async function saveWorkspaceSettings(
  workspaceId: string | undefined,
  set: Record<string, unknown>,
): Promise<boolean> {
  if (Object.keys(set).length === 0) return false;
  const answer = await api.workspace.update.mutateOrThrow<{ workspace?: unknown }>({
    params: { id: workspaceId },
    body: { set },
  });
  aos.stores.workspace.actions.adopt(answer?.workspace as Parameters<typeof aos.stores.workspace.actions.adopt>[0]);
  return true;
}
