import { aos } from "@/app/aos";
import type { Config } from "@/features/config/interfaces/config.interfaces";
import { api } from "@/lib/aos-facade";

/**
 * Writes a dotted-path patch to the installation configuration and makes the
 * answer the state every configuration-backed settings section reads.
 *
 * Returns whether anything was sent — an empty patch is not a save, and a
 * form that toggled a switch and back within the autosave delay has nothing
 * to report. A refusal throws, so the form shows the daemon's message.
 */
export async function saveConfigSettings(set: Record<string, unknown>): Promise<boolean> {
  if (Object.keys(set).length === 0) return false;
  const config = await api.config.update.mutateOrThrow<Config>({ body: { set } });
  aos.stores.config.actions.adopt(config);
  return true;
}
