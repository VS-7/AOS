import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { Bot, CircleDot, ChevronDown, Zap, X } from "lucide-react";
import {
  ROUTINE_STATUS_ORDER,
} from "@/features/routine/presentation/consts/routine";
import { FractalRoutineHelper } from "@/features/routine/presentation/helpers/routine.helper";
import { useRoutinesContext } from "@/features/routine/presentation/pages/(main)/context";
import { aos } from "@/app/aos";

const TRIGGER_TYPE_OPTIONS = [
  { id: "webhook", label: "Webhook" },
  { id: "scheduled", label: "Scheduled" },
  { id: "activity", label: "Activity" },
] as const;

export function RoutinesFilter() {
  const {
    selectedStatuses,
    selectedAgents,
    selectedTypes,
    agentOptions,
    activeFilterCount,
    handleToggleStatus,
    handleToggleAgent,
    handleToggleType,
    clearFilters,
  } = useRoutinesContext();

  const agents = aos.stores.agent.useState((state) => state.items);

  const getStatusButtonLabel = () => {
    if (selectedStatuses.length === 0) return "Status";
    if (selectedStatuses.length === 1) {
      return `Status: ${FractalRoutineHelper.getStatus(selectedStatuses[0]).label}`;
    }
    return `Status (${selectedStatuses.length})`;
  };

  const getAgentButtonLabel = () => {
    if (selectedAgents.length === 0) return "Agent";
    if (selectedAgents.length === 1) {
      return `Agent: ${FractalRoutineHelper.getAgentLabel(selectedAgents[0], agents)}`;
    }
    return `Agent (${selectedAgents.length})`;
  };

  const getTypeButtonLabel = () => {
    if (selectedTypes.length === 0) return "Trigger";
    if (selectedTypes.length === 1) {
      const match = TRIGGER_TYPE_OPTIONS.find(
        (option) => option.id === selectedTypes[0],
      );
      return `Trigger: ${match?.label ?? selectedTypes[0]}`;
    }
    return `Trigger (${selectedTypes.length})`;
  };

  return (
    <div className="flex items-center gap-1.5 flex-wrap">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="sm"
            className={cn(
              "h-8 text-xs font-normal border border-dashed border-border/60 gap-1.5 px-2.5",
              selectedStatuses.length > 0
                ? "bg-secondary text-secondary-foreground border-solid border-border font-medium"
                : "text-muted-foreground hover:bg-secondary/40 hover:text-foreground",
            )}
          >
            <CircleDot className="size-3.5" />
            {getStatusButtonLabel()}
            <ChevronDown className="size-3 opacity-50 ml-0.5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-48">
          {ROUTINE_STATUS_ORDER.map((status) => {
            const config = FractalRoutineHelper.getStatus(status);
            const Icon = config.icon;
            return (
              <DropdownMenuCheckboxItem
                key={status}
                checked={selectedStatuses.includes(status)}
                onCheckedChange={() => handleToggleStatus(status)}
              >
                <Icon className={`mr-2 size-4 ${config.color}`} />
                {config.label}
              </DropdownMenuCheckboxItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="sm"
            className={cn(
              "h-8 text-xs font-normal border border-dashed border-border/60 gap-1.5 px-2.5",
              selectedAgents.length > 0
                ? "bg-secondary text-secondary-foreground border-solid border-border font-medium"
                : "text-muted-foreground hover:bg-secondary/40 hover:text-foreground",
            )}
          >
            <Bot className="size-3.5" />
            {getAgentButtonLabel()}
            <ChevronDown className="size-3 opacity-50 ml-0.5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-56">
          {agentOptions.length === 0 && (
            <DropdownMenuItem disabled>No agents available</DropdownMenuItem>
          )}
          {agentOptions.map((agentId) => (
            <DropdownMenuCheckboxItem
              key={agentId}
              checked={selectedAgents.includes(agentId)}
              onCheckedChange={() => handleToggleAgent(agentId)}
            >
              <Bot className="mr-2 size-4 text-muted-foreground" />
              {FractalRoutineHelper.getAgentLabel(agentId, agents)}
            </DropdownMenuCheckboxItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="sm"
            className={cn(
              "h-8 text-xs font-normal border border-dashed border-border/60 gap-1.5 px-2.5",
              selectedTypes.length > 0
                ? "bg-secondary text-secondary-foreground border-solid border-border font-medium"
                : "text-muted-foreground hover:bg-secondary/40 hover:text-foreground",
            )}
          >
            <Zap className="size-3.5" />
            {getTypeButtonLabel()}
            <ChevronDown className="size-3 opacity-50 ml-0.5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-48">
          {TRIGGER_TYPE_OPTIONS.map((option) => (
            <DropdownMenuCheckboxItem
              key={option.id}
              checked={selectedTypes.includes(option.id)}
              onCheckedChange={() => handleToggleType(option.id)}
            >
              <Zap className="mr-2 size-4 text-muted-foreground" />
              {option.label}
            </DropdownMenuCheckboxItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      {activeFilterCount > 0 && (
        <Button
          variant="ghost"
          size="sm"
          onClick={clearFilters}
          className="h-8 text-xs text-muted-foreground hover:text-foreground px-2 gap-1"
        >
          Clear
          <X className="size-3" />
        </Button>
      )}
    </div>
  );
}
