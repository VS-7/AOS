import * as React from "react";
import type { AgentModelSelectValue } from "@/components/ui/agent-model-select";

export type SlotKey = "default" | "subconscious" | "realtime" | "voice" | "image" | "video";
export const SLOT_KEYS: SlotKey[] = ["default", "subconscious", "realtime", "voice", "image", "video"];

type SavedSlots = Partial<Record<string, unknown>> | undefined;

function fromSaved(models: SavedSlots): Partial<Record<SlotKey, AgentModelSelectValue>> {
  const out: Partial<Record<SlotKey, AgentModelSelectValue>> = {};
  for (const key of SLOT_KEYS) {
    const entry = models?.[key] as Partial<AgentModelSelectValue> | undefined;
    if (entry) {
      out[key] = { provider: entry.provider ?? "", model: entry.model ?? "", reasoning: entry.reasoning };
    }
  }
  return out;
}

/**
 * The model slots as a person sees them: what is saved, plus the one pick
 * still being saved.
 *
 * The saved configuration is the only source. A pick is shown on top of it
 * while `persist` runs, dropped the moment `persist` fails — so a refused
 * save shows what is actually saved again — and dropped on success once the
 * configuration this hook is given has changed, so the row does not flash
 * back to the old value while the route context is being re-read.
 */
export function useModelSlots(
  models: SavedSlots,
  persist: (slot: SlotKey, next: AgentModelSelectValue) => Promise<void>,
) {
  const [pending, setPending] = React.useState<{ slot: SlotKey; value: AgentModelSelectValue } | null>(null);

  React.useEffect(() => {
    setPending(null);
  }, [models]);

  const value = React.useMemo(() => {
    const saved = fromSaved(models);
    return pending ? { ...saved, [pending.slot]: pending.value } : saved;
  }, [models, pending]);

  const change = React.useCallback(
    async (slot: SlotKey, next: AgentModelSelectValue) => {
      const mine = { slot, value: next };
      setPending(mine);
      try {
        await persist(slot, next);
      } catch (error) {
        setPending((current) => (current === mine ? null : current));
        throw error;
      }
    },
    [persist],
  );

  return { value, change };
}
