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
import { RoutineHelper } from "@/features/routine/presentation/helpers/routine.helper";
import { useRoutinesContext } from "@/features/routine/presentation/pages/(main)/context";
import { aos } from "@/app/aos";
import { t } from "@/lib/i18n";

const TRIGGER_TYPE_OPTIONS = [
  { id: "webhook", get label() { return t("Webhook"); } },
  { id: "scheduled", get label() { return t("Scheduled"); } },
  { id: "activity", get label() { return t("Activity"); } },
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
    if (selectedStatuses.length === 0) return t("Status");
    if (selectedStatuses.length === 1) {
      return t("Status: {{value}}", { value: RoutineHelper.getStatus(selectedStatuses[0]).label });
    }
    return t("Status ({{count}})", { count: selectedStatuses.length });
  };

  const getAgentButtonLabel = () => {
    if (selectedAgents.length === 0) return t("Agent");
    if (selectedAgents.length === 1) {
      return t("Agent: {{value}}", { value: RoutineHelper.getAgentLabel(selectedAgents[0], agents) });
    }
    return t("Agent ({{count}})", { count: selectedAgents.length });
  };

  const getTypeButtonLabel = () => {
    if (selectedTypes.length === 0) return t("Trigger");
    if (selectedTypes.length === 1) {
      const match = TRIGGER_TYPE_OPTIONS.find(
        (option) => option.id === selectedTypes[0],
      );
      return t("Trigger: {{value}}", { value: match?.label ?? selectedTypes[0] });
    }
    return t("Trigger ({{count}})", { count: selectedTypes.length });
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
            const config = RoutineHelper.getStatus(status);
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
            <DropdownMenuItem disabled>{t("No agents available")}</DropdownMenuItem>
          )}
          {agentOptions.map((agentId) => (
            <DropdownMenuCheckboxItem
              key={agentId}
              checked={selectedAgents.includes(agentId)}
              onCheckedChange={() => handleToggleAgent(agentId)}
            >
              <Bot className="mr-2 size-4 text-muted-foreground" />
              {RoutineHelper.getAgentLabel(agentId, agents)}
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
          {t("Clear")}
          <X className="size-3" />
        </Button>
      )}
    </div>
  );
}
