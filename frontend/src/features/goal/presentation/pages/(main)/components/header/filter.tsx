import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { CircleDot, ChevronDown, Flag, X } from "lucide-react";
import {
  GOAL_PRIORITY_CONFIG,
  goalPriorityConfig,
  GOAL_PRIORITY_ORDER,
  GOAL_STATUS_ORDER,
} from "@/features/goal/presentation/consts/goal";
import type { GoalPriority } from "@/features/goal/interfaces/goal.interfaces";
import { GoalHelper } from "@/features/goal/presentation/helpers/goal.helper";
import { useGoalsContext } from "@/features/goal/presentation/pages/(main)/context";
import { aos } from "@/app/aos";
import { Icon } from "@/components/ui/icon";
import { ProjectHelper } from "@/features/project/presentation/helpers/project.helper";
import { useTranslation } from "@/lib/i18n";

/**
 * These menus pick several values, and a checkbox item closes its menu on
 * select by default — so picking a second project meant reopening it.
 */
const keepOpen = (event: Event) => event.preventDefault();

export function GoalsFilter() {
  // Subscribing re-renders the button labels when the language changes.
  const { t } = useTranslation();
  const {
    selectedStatuses,
    selectedPriorities,
    selectedProjects,
    activeFilterCount,
    handleToggleStatus,
    handleTogglePriority,
    handleToggleProject,
    clearFilters,
  } = useGoalsContext();

  const projects = aos.stores.projects.useState((state) => state.items);

  const getStatusButtonLabel = () => {
    if (selectedStatuses.length === 0) return t("Status");
    if (selectedStatuses.length === 1) {
      return t("Status: {{value}}", { value: GoalHelper.getStatus(selectedStatuses[0]).label });
    }
    return t("Status ({{count}})", { count: selectedStatuses.length });
  };

  const getPriorityButtonLabel = () => {
    if (selectedPriorities.length === 0) return t("Priority");
    if (selectedPriorities.length === 1) {
      return t("Priority: {{value}}", { value: goalPriorityConfig(selectedPriorities[0]).label });
    }
    return t("Priority ({{count}})", { count: selectedPriorities.length });
  };

  const getProjectButtonLabel = () => {
    if (selectedProjects.length === 0) return t("Project");
    if (selectedProjects.length === 1) {
      const proj = projects.find((p) => p.id === selectedProjects[0]);
      return t("Project: {{value}}", { value: proj ? proj.name : selectedProjects[0] });
    }
    return t("Project ({{count}})", { count: selectedProjects.length });
  };

  const selectedProjectIcon =
    selectedProjects.length === 1
      ? ProjectHelper.getIcon(
          projects.find((p) => p.id === selectedProjects[0])?.icon,
        )
      : "Folder";

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
          {GOAL_STATUS_ORDER.map((status) => {
            const config = GoalHelper.getStatus(status);
            const Icon = config.icon;
            return (
              <DropdownMenuCheckboxItem
                key={status}
                checked={selectedStatuses.includes(status)}
                onCheckedChange={() => handleToggleStatus(status)}
                onSelect={keepOpen}
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
              selectedPriorities.length > 0
                ? "bg-secondary text-secondary-foreground border-solid border-border font-medium"
                : "text-muted-foreground hover:bg-secondary/40 hover:text-foreground",
            )}
          >
            <Flag className="size-3.5" />
            {getPriorityButtonLabel()}
            <ChevronDown className="size-3 opacity-50 ml-0.5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-48">
          {GOAL_PRIORITY_ORDER.map((priority) => {
            const config = GOAL_PRIORITY_CONFIG[priority];
            const PriorityIcon = config.icon;
            return (
              <DropdownMenuCheckboxItem
                key={priority}
                checked={selectedPriorities.includes(priority)}
                onCheckedChange={() => handleTogglePriority(priority)}
                onSelect={keepOpen}
              >
                <PriorityIcon className={`mr-2 size-4 ${config.colorClass}`} />
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
              selectedProjects.length > 0
                ? "bg-secondary text-secondary-foreground border-solid border-border font-medium"
                : "text-muted-foreground hover:bg-secondary/40 hover:text-foreground",
            )}
          >
            <Icon
              value={selectedProjectIcon}
              fallback="Folder"
              className="size-3.5 text-muted-foreground"
            />
            {getProjectButtonLabel()}
            <ChevronDown className="size-3 opacity-50 ml-0.5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-56">
          {projects.length === 0 && (
            <DropdownMenuItem disabled>{t("No projects available")}</DropdownMenuItem>
          )}
          {projects.map((project) => (
            <DropdownMenuCheckboxItem
              key={project.id}
              checked={selectedProjects.includes(project.id)}
              onCheckedChange={() => handleToggleProject(project.id)}
              onSelect={keepOpen}
            >
              <Icon
                value={ProjectHelper.getIcon(project.icon)}
                fallback="Folder"
                className="mr-2 size-4 text-muted-foreground"
              />
              {project.name}
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
