import * as React from "react";
import { ActivityIcon, PlusIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import type { FractalActivityEventDefinition } from "@/features/activity/interfaces/activity.interfaces";
import { FractalActivityEventHelper } from "@/features/activity/presentation/helpers/activity-event.helper";
import type { FractalRoutineActivityFilter } from "@/features/routine/interfaces/routine.interfaces";
import type { RoutineTriggerFormValue } from "@/features/routine/presentation/consts/routine-triggers";
import { ButtonGroup } from "@/components/ui/button-group";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { Trash2Icon } from "lucide-react";

interface ActivityTriggerRowProps {
  value: Extract<RoutineTriggerFormValue, { type: "activity" }>;
  activityEvents: FractalActivityEventDefinition[];
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
  const eventDefinition = FractalActivityEventHelper.findDefinition(
    activityEvents,
    value.config.namespace,
    value.config.event,
  );


  const filters = value.config.filters ?? [];
  const filterableFields =
    FractalActivityEventHelper.getFilterableFields(eventDefinition);

  const handleAddFilter = () => {
    onChange({
      type: "activity",
      config: {
        ...value.config,
        filters: [...(value.config.filters ?? []), { path: "", operator: "eq", value: "" }],
      },
    });
  };

  const handleUpdateFilter = (
    index: number,
    patch: Partial<FractalRoutineActivityFilter>,
  ) => {
    const current = filters[index];
    if (!current) return;
    const filter: FractalRoutineActivityFilter = { ...current, ...patch };
    onChange({
      type: "activity",
      config: {
        ...value.config,
        filters: value.config.filters?.map((f, i) => i === index ? filter : f) ?? [],
      },
    });
  };

  const handleRemoveFilter = (index: number) => {
    onChange({
      type: "activity",
      config: {
        ...value.config,
        filters: value.config.filters?.filter((_, i) => i !== index) ?? [],
      },
    });
  };

  return (
    <div className="flex flex-col gap-3 px-3 py-3">
      <div className="flex items-center gap-3">
        <div className="flex h-7 shrink-0 items-center justify-center">
          <ActivityIcon className="size-4 text-muted-foreground" />
        </div>

        <div className="min-w-0 flex-1 flex items-center gap-2">
          <span>When</span>
          <p className="text-xs text-muted-foreground">
            {eventDefinition
              ? FractalActivityEventHelper.getDisplayLabel(eventDefinition)
              : "On activity"}
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 w-fit gap-1.5 px-2 text-xs text-muted-foreground"
            onClick={handleAddFilter}
          >
            <PlusIcon className="size-3.5" />
            Add filter
          </Button>

          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="self-center text-muted-foreground hover:text-destructive"
            onClick={onRemove}
          >
            Remove
          </Button>
        </div>
      </div>

      {filters.length > 0 && (
        <div className="flex flex-col border border-border/70 rounded-md bg-card/30 divide-y divide-border/70">
          {filters.map((filter, index) => (
            <div key={`${filter.path}-${index}`} className="flex flex-wrap items-center p-2">
              <Select
                value={filter.path}
                onValueChange={(path) => handleUpdateFilter(index, { path })}
              >
                <SelectTrigger className="!h-7 w-[128px] rounded-md border-border/70 bg-background/70 px-2 text-xs">
                  <SelectValue placeholder="Field" />
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
                    operator: operator as FractalRoutineActivityFilter["operator"],
                  })
                }
              >
                <SelectTrigger className="!h-7 w-[100px] rounded-md border-border/70 bg-background/70 px-2 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="eq">equals</SelectItem>
                  <SelectItem value="neq">not equals</SelectItem>
                  <SelectItem value="contains">contains</SelectItem>
                </SelectContent>
              </Select>

              <Input
                value={filter.value}
                onChange={(event) =>
                  handleUpdateFilter(index, { value: event.target.value })
                }
                placeholder="Value"
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
              </Button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
