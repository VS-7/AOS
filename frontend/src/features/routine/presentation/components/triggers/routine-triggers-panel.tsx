import * as React from "react";
import { PlusIcon } from "lucide-react";

import { cn } from "@/lib/utils";
import type { ActivityEventDefinition } from "@/features/activity/interfaces/activity.interfaces";
import type { RoutineTriggerFormValue } from "@/features/routine/presentation/consts/routine-triggers";
import { RoutineTriggersHelper } from "@/features/routine/presentation/helpers/routine-triggers.helper";
import { RoutineTriggerAddMenu } from "./routine-trigger-add-menu";
import { ScheduledTriggerRow } from "./scheduled-trigger-row";
import { WebhookTriggerRow, type WebhookTriggerState } from "./webhook-trigger-row";
import { ActivityTriggerRow } from "./activity-trigger-row";
import { t } from "@/lib/i18n";

/** What the daemon said about the schedule as it is saved. */
export interface SavedSchedule {
  cron?: string;
  nextRun?: string;
  /** Warnings already in the interface's language. */
  warnings: string[];
}

interface RoutineTriggersPanelProps {
  value: RoutineTriggerFormValue[];
  onChange: (next: RoutineTriggerFormValue[]) => void;
  webhook?: WebhookTriggerState;
  saved?: SavedSchedule;
  activityEvents: ActivityEventDefinition[];
  /** Validation messages for each trigger, by index. */
  errors?: string[][];
}

export function RoutineTriggersPanel({
  value,
  onChange,
  webhook,
  saved,
  activityEvents,
  errors = [],
}: RoutineTriggersPanelProps) {
  const availableTypes = RoutineTriggersHelper.getAvailableTriggerTypes(
    value,
    activityEvents,
  );
  const scheduledCount = RoutineTriggersHelper.countScheduledTriggers(value);
  const canAdd = RoutineTriggersHelper.canAddTriggers(value, activityEvents);
  const firstCron = value.find((trigger) => trigger.type === "scheduled");
  // The daemon's answers describe the saved schedule; once the cron on screen
  // differs they would describe something that is no longer being edited.
  const scheduleIsSaved =
    firstCron?.type === "scheduled" && saved?.cron === firstCron.config.cron;

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
      <p className="text-sm text-muted-foreground">{t("Triggers")}</p>

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
                savedNextRun={
                  scheduleIsSaved && trigger === firstCron ? saved?.nextRun : undefined
                }
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
                webhook={webhook}
                onRemove={() => handleRemove(index)}
              />
            )}
            {(errors[index] ?? []).map((message) => (
              <p
                key={message}
                data-slot="form-message"
                className="px-3 pb-2 text-xs text-destructive"
              >
                {t(message)}
              </p>
            ))}
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
                  <span>{t("Add Trigger")}</span>
                </button>
              }
            />
          </div>
        ) : null}
      </div>

      {scheduledCount > 1 ? (
        <p className="flex items-center gap-1.5 text-xs text-amber-600 dark:text-amber-500">
          <span aria-hidden>⚠</span>
          {t("Only the first cron trigger will be used")}
        </p>
      ) : null}

      {scheduleIsSaved
        ? (saved?.warnings ?? []).map((warning) => (
            <p
              key={warning}
              className="flex items-start gap-1.5 text-xs text-amber-600 dark:text-amber-500"
            >
              <span aria-hidden>⚠</span>
              {warning}
            </p>
          ))
        : null}
    </div>
  );
}
