import * as React from "react";
import { ClockIcon, Trash2Icon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import type { RoutineTriggerFormValue } from "@/features/routine/presentation/consts/routine-triggers";
import { t } from "@/lib/i18n";
import {
  ROUTINE_WEEKDAY_OPTIONS,
  RoutineTriggersHelper,
} from "@/features/routine/presentation/helpers/routine-triggers.helper";

const TIME_OPTIONS = Array.from({ length: 48 }, (_, index) => {
  const hour = Math.floor(index / 2)
    .toString()
    .padStart(2, "0");
  const minute = index % 2 === 0 ? "00" : "30";
  return `${hour}:${minute}`;
});

interface ScheduledTriggerRowProps {
  value: Extract<RoutineTriggerFormValue, { type: "scheduled" }>;
  onChange: (
    next: Extract<RoutineTriggerFormValue, { type: "scheduled" }>,
  ) => void;
  onRemove: () => void;
}

export function ScheduledTriggerRow({
  value,
  onChange,
  onRemove,
}: ScheduledTriggerRowProps) {
  const nextRunLabel = RoutineTriggersHelper.getNextRunLabel(value.config.cron);

  function updateConfig(
    patch: Partial<Extract<RoutineTriggerFormValue, { type: "scheduled" }>["config"]>,
  ) {
    const nextConfig = {
      ...value.config,
      ...patch,
    };

    const cron =
      nextConfig.preset === "custom"
        ? nextConfig.cron
        : RoutineTriggersHelper.buildCronFromScheduledConfig(nextConfig);

    onChange({
      type: "scheduled",
      config: {
        ...nextConfig,
        cron,
      },
    });
  }

  return (
    <div className="group flex gap-2 px-3 py-2.5">
      <div className="flex h-7 w-4 shrink-0 items-center justify-center">
        <ClockIcon className="size-4 text-muted-foreground" />
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex min-h-7 flex-wrap items-center gap-x-1.5 gap-y-1 text-sm">
          {value.config.preset === "hourly" ? (
            <span>{t("Every hour")}</span>
          ) : null}

          {value.config.preset === "daily" ? (
            <>
              <span>{t("Every day at")}</span>
              <InlineSelect
                value={value.config.time}
                options={TIME_OPTIONS.map((time) => ({ value: time, label: time }))}
                onValueChange={(time) => updateConfig({ time })}
              />
            </>
          ) : null}

          {value.config.preset === "weekly" ? (
            <>
              <span>{t("Every week on")}</span>
              <InlineSelect
                value={value.config.day}
                options={ROUTINE_WEEKDAY_OPTIONS.map((option) => ({
                  value: option.value,
                  label: option.label,
                }))}
                onValueChange={(day) => updateConfig({ day })}
              />
              <span>at</span>
              <InlineSelect
                value={value.config.time}
                options={TIME_OPTIONS.map((time) => ({ value: time, label: time }))}
                onValueChange={(time) => updateConfig({ time })}
              />
            </>
          ) : null}

          {value.config.preset === "custom" ? (
            <>
              <span>{t("Custom schedule")}</span>
              <Input
                value={value.config.cron}
                onChange={(event) =>
                  updateConfig({ cron: event.target.value, preset: "custom" })
                }
                className="h-7 w-[9.5rem] rounded-md border-border/70 bg-background/70 px-2 text-xs"
                placeholder="0 9 * * *"
              />
            </>
          ) : null}

          {nextRunLabel ? (
            <span className="text-xs text-muted-foreground">{nextRunLabel}</span>
          ) : null}
        </div>
      </div>

      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="size-7 shrink-0 self-center opacity-0 transition-opacity group-hover:opacity-100"
        onClick={onRemove}
      >
        <Trash2Icon className="size-3.5" />
        <span className="sr-only">{t("Remove scheduled trigger")}</span>
      </Button>
    </div>
  );
}

interface InlineSelectProps {
  value: string;
  options: Array<{ value: string; label: string }>;
  onValueChange: (value: string) => void;
}

function InlineSelect({ value, options, onValueChange }: InlineSelectProps) {
  return (
    <Select value={value} onValueChange={onValueChange}>
      <SelectTrigger
        size="sm"
        className={cn(
          "h-7 min-w-[4.5rem] border-border/70 bg-background/70 px-2 text-xs shadow-none",
        )}
      >
        <SelectValue />
      </SelectTrigger>
      <SelectContent align="start">
        {options.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
