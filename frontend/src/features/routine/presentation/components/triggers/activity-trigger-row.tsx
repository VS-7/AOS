import * as React from "react";
import { ActivityIcon, PlusIcon, Trash2Icon } from "lucide-react";

import { Button } from "@/components/ui/button";
import type { ActivityEventDefinition } from "@/features/activity/interfaces/activity.interfaces";
import { ActivityEventHelper } from "@/features/activity/presentation/helpers/activity-event.helper";
import type { RoutineActivityFilter } from "@/features/routine/interfaces/routine.interfaces";
import type { RoutineTriggerFormValue } from "@/features/routine/presentation/consts/routine-triggers";
import { RoutineTriggersHelper } from "@/features/routine/presentation/helpers/routine-triggers.helper";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { t } from "@/lib/i18n";

interface ActivityTriggerRowProps {
  value: Extract<RoutineTriggerFormValue, { type: "activity" }>;
  activityEvents: ActivityEventDefinition[];
  onChange: (
    next: Extract<RoutineTriggerFormValue, { type: "activity" }>,
  ) => void;
  onRemove: () => void;
}

export function ActivityTriggerRow({
  value,
  activityEvents,
  onChange,
  onRemove,
}: ActivityTriggerRowProps) {
  const eventDefinition = ActivityEventHelper.findDefinition(
    activityEvents,
    value.config.namespace,
    value.config.event,
  );

  const filters = value.config.filters ?? [];
  // A filter loaded from a routine written elsewhere may name a key the
  // catalogue does not list; it stays selectable rather than showing blank.
  const filterableFields = Array.from(
    new Set([
      ...ActivityEventHelper.getFilterableFields(eventDefinition),
      ...filters.map((filter) => filter.field).filter(Boolean),
    ]),
  );

  const setFilters = (next: RoutineActivityFilter[]) => {
    onChange({
      type: "activity",
      config: { ...value.config, filters: next },
    });
  };

  const handleAddFilter = () => {
    setFilters([...filters, { field: "", operator: "eq", value: "" }]);
  };

  const handleUpdateFilter = (
    index: number,
    patch: Partial<RoutineActivityFilter>,
  ) => {
    setFilters(filters.map((filter, i) => (i === index ? { ...filter, ...patch } : filter)));
  };

  const handleRemoveFilter = (index: number) => {
    setFilters(filters.filter((_, i) => i !== index));
  };

  return (
    <div className="group flex flex-col gap-3 px-3 py-3">
      <div className="flex items-center gap-3">
        <div className="flex h-7 shrink-0 items-center justify-center">
          <ActivityIcon className="size-4 text-muted-foreground" />
        </div>

        <div className="flex min-w-0 flex-1 items-center gap-2">
          <span className="shrink-0 text-sm">{t("When")}</span>
          <span className="truncate text-sm font-medium">
            {RoutineTriggersHelper.eventTitle(eventDefinition, value.config.namespace, value.config.event)}
          </span>
          {eventDefinition?.description ? (
            <span className="hidden truncate text-xs text-muted-foreground sm:inline">
              {RoutineTriggersHelper.eventDescription(eventDefinition)}
            </span>
          ) : null}
        </div>

        <div className="flex items-center gap-1">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 w-fit gap-1.5 px-2 text-xs text-muted-foreground"
            onClick={handleAddFilter}
          >
            <PlusIcon className="size-3.5" />
            {t("Add filter")}
          </Button>

          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-7 shrink-0 opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100"
            onClick={onRemove}
          >
            <Trash2Icon className="size-3.5" />
            <span className="sr-only">{t("Remove activity trigger")}</span>
          </Button>
        </div>
      </div>

      {filters.length > 0 && (
        <div className="flex flex-col divide-y divide-border/70 rounded-md border border-border/70 bg-card/30">
          {filters.map((filter, index) => (
            <div key={index} className="flex flex-wrap items-center gap-1 p-2">
              <Select
                value={filter.field}
                onValueChange={(field) => handleUpdateFilter(index, { field })}
              >
                <SelectTrigger
                  className="!h-7 w-[128px] rounded-md border-border/70 bg-background/70 px-2 text-xs"
                  aria-label={t("Field")}
                >
                  <SelectValue placeholder={t("Field")} />
                </SelectTrigger>
                <SelectContent>
                  {filterableFields.map((field) => (
                    <SelectItem key={field} value={field}>
                      {field}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>

              <Select
                value={filter.operator}
                onValueChange={(operator) =>
                  handleUpdateFilter(index, {
                    operator: operator as RoutineActivityFilter["operator"],
                  })
                }
              >
                <SelectTrigger
                  className="!h-7 w-[110px] rounded-md border-border/70 bg-background/70 px-2 text-xs"
                  aria-label={t("Operator")}
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="eq">{t("equals")}</SelectItem>
                  <SelectItem value="neq">{t("not equals")}</SelectItem>
                  <SelectItem value="contains">{t("contains")}</SelectItem>
                </SelectContent>
              </Select>

              <Input
                value={filter.value}
                onChange={(event) =>
                  handleUpdateFilter(index, { value: event.target.value })
                }
                placeholder={t("Value")}
                className="h-7 min-w-[120px] flex-1 rounded-md border-border/70 bg-background/70 px-2 !text-xs"
              />

              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-7 shrink-0"
                onClick={() => handleRemoveFilter(index)}
              >
                <Trash2Icon className="size-3.5" />
                <span className="sr-only">{t("Remove filter")}</span>
              </Button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
