import {
  FormControl,
  FormField,
  FormItem,
  FormMessage,
} from "@/components/ui/form";
import type { Control, FieldValues } from "react-hook-form";

import type { ActivityEventDefinition } from "@/features/activity/interfaces/activity.interfaces";
import { RoutineTriggersPanel } from "./routine-triggers-panel";

interface RoutineTriggersFieldProps<TFieldValues extends FieldValues> {
  control: Control<TFieldValues>;
  fireUrl?: string | null;
  activityEvents: ActivityEventDefinition[];
}

export function RoutineTriggersField<TFieldValues extends FieldValues>({
  control,
  fireUrl,
  activityEvents,
}: RoutineTriggersFieldProps<TFieldValues>) {
  return (
    <FormField
      control={control}
      name={"triggers" as never}
      render={({ field }) => (
        <FormItem className="border-0 p-0">
          <FormControl>
            <RoutineTriggersPanel
              value={field.value ?? []}
              onChange={field.onChange}
              fireUrl={fireUrl}
              activityEvents={activityEvents}
            />
          </FormControl>
          <FormMessage />
        </FormItem>
      )}
    />
  );
}
