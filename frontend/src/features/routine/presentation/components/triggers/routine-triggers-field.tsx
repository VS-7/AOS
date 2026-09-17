import {
  FormControl,
  FormField,
  FormItem,
  FormMessage,
} from "@/components/ui/form";
import type { Control, FieldValues } from "react-hook-form";

import type { ActivityEventDefinition } from "@/features/activity/interfaces/activity.interfaces";
import type { WebhookTriggerState } from "./webhook-trigger-row";
import { RoutineTriggersPanel, type SavedSchedule } from "./routine-triggers-panel";

interface RoutineTriggersFieldProps<TFieldValues extends FieldValues> {
  control: Control<TFieldValues>;
  webhook?: WebhookTriggerState;
  saved?: SavedSchedule;
  activityEvents: ActivityEventDefinition[];
}

/**
 * Every message a nested validation error holds, depth first.
 *
 * The field rendered one FormMessage for the root `triggers` error, and a
 * trigger's own errors — a filter with no field, a cron cleared — are nested
 * under its index with no root message. Save then did nothing and said
 * nothing.
 */
export function collectErrorMessages(node: unknown): string[] {
  if (!node || typeof node !== "object") return [];
  const out: string[] = [];
  const { message } = node as { message?: unknown };
  if (typeof message === "string" && message) out.push(message);
  for (const [key, child] of Object.entries(node)) {
    if (key === "message" || key === "ref" || key === "type") continue;
    out.push(...collectErrorMessages(child));
  }
  return Array.from(new Set(out));
}

export function RoutineTriggersField<TFieldValues extends FieldValues>({
  control,
  webhook,
  saved,
  activityEvents,
}: RoutineTriggersFieldProps<TFieldValues>) {
  return (
    <FormField
      control={control}
      name={"triggers" as never}
      render={({ field, fieldState }) => {
        const nested = Array.isArray(fieldState.error)
          ? (fieldState.error as unknown[]).map((error) => collectErrorMessages(error))
          : [];
        return (
          <FormItem className="border-0 p-0">
            <FormControl>
              <RoutineTriggersPanel
                value={field.value ?? []}
                onChange={field.onChange}
                webhook={webhook}
                saved={saved}
                activityEvents={activityEvents}
                errors={nested}
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        );
      }}
    />
  );
}
