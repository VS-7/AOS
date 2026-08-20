import * as React from "react";
import { ChevronRightIcon } from "lucide-react";

import { Input } from "@/components/ui/input";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { ActivityEventDefinition } from "@/features/activity/interfaces/activity.interfaces";
import { ActivityEventHelper } from "@/features/activity/presentation/helpers/activity-event.helper";
import {
  ROUTINE_SCHEDULED_PRESET_OPTIONS,
  ROUTINE_TRIGGER_TYPE_ORDER,
  ROUTINE_TRIGGER_TYPE_REGISTRY,
  type RoutineScheduledPresetId,
  type RoutineTriggerFormValue,
  type RoutineTriggerTypeId,
} from "@/features/routine/presentation/consts/routine-triggers";
import { RoutineTriggersHelper } from "@/features/routine/presentation/helpers/routine-triggers.helper";

interface RoutineTriggerAddMenuProps {
  availableTypes: RoutineTriggerTypeId[];
  activeTriggers: RoutineTriggerFormValue[];
  activityEvents: ActivityEventDefinition[];
  onAdd: (trigger: RoutineTriggerFormValue) => void;
  trigger: React.ReactNode;
}

export function RoutineTriggerAddMenu({
  availableTypes,
  activeTriggers,
  activityEvents,
  onAdd,
  trigger,
}: RoutineTriggerAddMenuProps) {
  const [query, setQuery] = React.useState("");
  const normalizedQuery = query.trim().toLowerCase();

  const visibleTypes = ROUTINE_TRIGGER_TYPE_ORDER.filter((type) =>
    availableTypes.includes(type),
  ).filter((type) => {
    if (!normalizedQuery) return true;

    const definition = ROUTINE_TRIGGER_TYPE_REGISTRY[type];
    const haystack = [
      definition.label,
      definition.addLabel,
      definition.description,
      ...definition.searchableTerms,
    ]
      .join(" ")
      .toLowerCase();

    return haystack.includes(normalizedQuery);
  });

  const availableActivityEvents =
    RoutineTriggersHelper.getAvailableActivityEvents(
      activeTriggers,
      activityEvents,
    );

  const filteredActivityEvents = availableActivityEvents.filter((item) => {
    if (!normalizedQuery) return true;

    const haystack = [
      item.namespace,
      item.event,
      ActivityEventHelper.getDisplayLabel(item),
      item.description,
      item.title,
    ]
      .join(" ")
      .toLowerCase();

    return haystack.includes(normalizedQuery);
  });

  function handleAddScheduled(preset: RoutineScheduledPresetId) {
    onAdd(ROUTINE_TRIGGER_TYPE_REGISTRY.scheduled.createDefault(preset));
    setQuery("");
  }

  function handleAddWebhook() {
    onAdd(ROUTINE_TRIGGER_TYPE_REGISTRY.webhook.createDefault());
    setQuery("");
  }

  function handleAddActivity(namespace: string, event: string) {
    onAdd({
      type: "activity",
      config: {
        namespace,
        event,
        filters: [],
      },
    });
    setQuery("");
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>{trigger}</DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-80 p-0">
        <div className="p-2">
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search triggers..."
            className="h-6 border-0 bg-transparent px-2 shadow-none focus-visible:ring-0"
          />
        </div>

        <div className="max-h-72 overflow-y-auto p-1.5 shadow-sm">
          {visibleTypes.length === 0 && filteredActivityEvents.length === 0 ? (
            <p className="px-2 py-1.5 text-xs text-muted-foreground">
              No triggers available
            </p>
          ) : null}

          {visibleTypes.map((type) => {
            const definition = ROUTINE_TRIGGER_TYPE_REGISTRY[type];
            const Icon = definition.icon;

            if (type === "scheduled") {
              return (
                <DropdownMenuSub key={type}>
                  <DropdownMenuSubTrigger className="gap-2">
                    <Icon className="size-4 text-muted-foreground" />
                    <span>{definition.addLabel}</span>
                  </DropdownMenuSubTrigger>
                  <DropdownMenuSubContent className="w-44">
                    {ROUTINE_SCHEDULED_PRESET_OPTIONS.map((preset) => (
                      <DropdownMenuItem
                        key={preset.id}
                        onClick={() => handleAddScheduled(preset.id)}
                      >
                        {preset.label}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuSubContent>
                </DropdownMenuSub>
              );
            }

            if (type === "activity") {
              return (
                <DropdownMenuSub key={type}>
                  <DropdownMenuSubTrigger className="gap-2">
                    <Icon className="size-4 text-muted-foreground" />
                    <span>{definition.addLabel}</span>
                  </DropdownMenuSubTrigger>
                  <DropdownMenuSubContent className="w-72">
                    {filteredActivityEvents.length === 0 ? (
                      <p className="px-2 py-1.5 text-xs text-muted-foreground">
                        No activity events available
                      </p>
                    ) : (
                      filteredActivityEvents.map((item) => (
                        <DropdownMenuItem
                          key={`${item.namespace}.${item.event}`}
                          className=""
                          onClick={() =>
                            handleAddActivity(item.namespace, item.event)
                          }
                        >
                          <span className="text-sm font-medium line-clamp-1">
                            {ActivityEventHelper.getDisplayLabel(item)}
                          </span>
                        </DropdownMenuItem>
                      ))
                    )}
                  </DropdownMenuSubContent>
                </DropdownMenuSub>
              );
            }

            return (
              <DropdownMenuItem
                key={type}
                className="gap-2"
                onClick={handleAddWebhook}
              >
                <Icon className="size-4 text-muted-foreground" />
                <span>{definition.addLabel}</span>
                <ChevronRightIcon className="ml-auto size-3.5 opacity-0" />
              </DropdownMenuItem>
            );
          })}
        </div>
      </DropdownMenuContent >
    </DropdownMenu >
  );
}
