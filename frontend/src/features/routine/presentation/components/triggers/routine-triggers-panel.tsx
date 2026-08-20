import * as React from "react";
import { PlusIcon } from "lucide-react";

import { cn } from "@/lib/utils";
import type { ActivityEventDefinition } from "@/features/activity/interfaces/activity.interfaces";
import type { RoutineTriggerFormValue } from "@/features/routine/presentation/consts/routine-triggers";
import { RoutineTriggersHelper } from "@/features/routine/presentation/helpers/routine-triggers.helper";
import { RoutineTriggerAddMenu } from "./routine-trigger-add-menu";
import { ScheduledTriggerRow } from "./scheduled-trigger-row";
import { WebhookTriggerRow } from "./webhook-trigger-row";
import { ActivityTriggerRow } from "./activity-trigger-row";

interface RoutineTriggersPanelProps {
  value: RoutineTriggerFormValue[];
  onChange: (next: RoutineTriggerFormValue[]) => void;
  fireUrl?: string | null;
  activityEvents: ActivityEventDefinition[];
}

export function RoutineTriggersPanel({
  value,
  onChange,
  fireUrl,
  activityEvents,
}: RoutineTriggersPanelProps) {
  const availableTypes = RoutineTriggersHelper.getAvailableTriggerTypes(
    value,
    activityEvents,
  );
  const scheduledCount = RoutineTriggersHelper.countScheduledTriggers(value);
  const canAdd = RoutineTriggersHelper.canAddTriggers(value, activityEvents);

  function handleAdd(trigger: RoutineTriggerFormValue) {
    onChange([...value, trigger]);
  }

  function handleUpdate(index: number, next: RoutineTriggerFormValue) {
    onChange(value.map((item, itemIndex) => (itemIndex === index ? next : item)));
  }

  function handleRemove(index: number) {
    onChange(value.filter((_, itemIndex) => itemIndex !== index));
  }

  return (
    <div className="flex flex-col gap-2">
      <p className="text-sm text-muted-foreground">Triggers</p>

      <div className="overflow-hidden rounded-md border bg-card/30">
        {value.map((trigger, index) => (
          <div
            key={`${trigger.type}-${index}`}
            className={cn(index > 0 && "border-t border-border/60")}
          >
            {trigger.type === "scheduled" ? (
              <ScheduledTriggerRow
                value={trigger}
                onChange={(next) => handleUpdate(index, next)}
                onRemove={() => handleRemove(index)}
              />
            ) : trigger.type === "activity" ? (
              <ActivityTriggerRow
                value={trigger}
                activityEvents={activityEvents}
                onChange={(next) => handleUpdate(index, next)}
                onRemove={() => handleRemove(index)}
              />
            ) : (
              <WebhookTriggerRow
                fireUrl={fireUrl}
                onRemove={() => handleRemove(index)}
              />
            )}
          </div>
        ))}

        {canAdd ? (
          <div className={cn(value.length > 0 && "border-t border-border/60")}>
            <RoutineTriggerAddMenu
              availableTypes={availableTypes}
              activeTriggers={value}
              activityEvents={activityEvents}
              onAdd={handleAdd}
              trigger={
                <button
                  type="button"
                  className="flex w-full items-center gap-2 px-3 py-2.5 text-sm text-muted-foreground transition-colors hover:bg-muted/40 hover:text-foreground"
                >
                  <PlusIcon className="size-4" />
                  <span>Add Trigger</span>
                </button>
              }
            />
          </div>
        ) : null}
      </div>

      {scheduledCount > 1 ? (
        <p className="flex items-center gap-1.5 text-xs text-amber-600 dark:text-amber-500">
          <span aria-hidden>⚠</span>
          Only the first cron trigger will be used
        </p>
      ) : null}
    </div>
  );
}
