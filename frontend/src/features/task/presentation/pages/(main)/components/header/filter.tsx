import {
  DropdownMenuCheckboxItem,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  CircleDot,
  Flag,
  Tags,
  Target,
  X,
  ChevronDown,
} from "lucide-react";
import {
  TASK_STATUS_ORDER,
  TASK_PRIORITY_CONFIG,
} from "../../../../consts/task";
import type { FractalTaskPriority } from "@/features/task/interfaces/task.interfaces";
import { TaskHelper } from "../../../../helpers/task.helper";
import { useTasksContext } from "@/features/task/presentation/pages/(main)/context";
import { aos } from "@/app/aos";
import { Icon } from "@/components/ui/icon";
import { ProjectHelper } from "@/features/project/presentation/helpers/project.helper";

export function TasksFilter() {
  const {
    selectedStatuses,
    selectedPriorities,
    selectedTypes,
    selectedProjects,
    selectedGoals,
    taskTypes,
    activeFilterCount,
    handleToggleStatus,
    handleTogglePriority,
    handleToggleType,
    handleToggleProject,
    handleToggleGoal,
    clearFilters,
  } = useTasksContext();

  const projects = aos.stores.projects.useState((state) => state.items);
  const goals = aos.stores.goals.useState((state) => state.items);
  const currentWorkspace = aos.stores.workspace.useState(
    (state) => state.current,
  );

  const getStatusButtonLabel = () => {
    if (selectedStatuses.length === 0) return "Status";
    if (selectedStatuses.length === 1) {
      return `Status: ${TaskHelper.getStatus(selectedStatuses[0]).label}`;
    }
    return `Status (${selectedStatuses.length})`;
  };

  const getPriorityButtonLabel = () => {
    if (selectedPriorities.length === 0) return "Priority";
    if (selectedPriorities.length === 1) {
      return `Priority: ${TASK_PRIORITY_CONFIG[selectedPriorities[0]].label}`;
    }
    return `Priority (${selectedPriorities.length})`;
  };

  // C5 of the final review ("honest empty state" policy, R26):
  // `command-map.ts`'s `task.list` entry can only send Go's `ListInput` one
  // scalar `type`/`project`/`goal` each — a genuine wire limitation, not a
  // UI bug — so checking a second box here doesn't broaden what the server
  // returns, only what shows in the button. Before this, that was silent:
  // the button read "(3)" as if all three were filtering. Disclosed here,
  // not fixed at the wire — there's no server capability yet to widen into.
  const ONLY_FIRST_APPLIES = " (only the first applies)";

  const getTypeButtonLabel = () => {
    if (selectedTypes.length === 0) return "Type";
    if (selectedTypes.length === 1) {
      return `Type: ${selectedTypes[0]}`;
    }
    return `Type (${selectedTypes.length})${ONLY_FIRST_APPLIES}`;
  };

  const getProjectButtonLabel = () => {
    if (selectedProjects.length === 0) return "Project";
    if (selectedProjects.length === 1) {
      const proj = projects.find((p) => p.id === selectedProjects[0]);
      return `Project: ${proj ? proj.name : selectedProjects[0]}`;
    }
    return `Project (${selectedProjects.length})${ONLY_FIRST_APPLIES}`;
  };

  const selectedProjectIcon =
    selectedProjects.length === 1
      ? ProjectHelper.getIcon(
          projects.find((p) => p.id === selectedProjects[0])?.icon,
        )
      : "Folder";

  const getGoalButtonLabel = () => {
    if (selectedGoals.length === 0) return "Goal";
    if (selectedGoals.length === 1) {
      const goal = goals.find((g) => g.id === selectedGoals[0]);
      return `Goal: ${goal ? goal.title : selectedGoals[0]}`;
    }
    return `Goal (${selectedGoals.length})${ONLY_FIRST_APPLIES}`;
  };

  const activeTypeConfig =
    selectedTypes.length === 1
      ? currentWorkspace?.tasks?.find((t) => t.id === selectedTypes[0])
      : null;

  return (
    <div className="flex items-center gap-1.5 flex-wrap">
      {/* Status Filter */}
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
          {TASK_STATUS_ORDER.map((status) => {
            const config = TaskHelper.getStatus(status);
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

      {/* Priority Filter */}
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
          {Object.entries(TASK_PRIORITY_CONFIG).map(([priority, config]) => {
            const Icon = config.icon;
            return (
              <DropdownMenuCheckboxItem
                key={priority}
                checked={selectedPriorities.includes(
                  priority as FractalTaskPriority,
                )}
                onCheckedChange={() =>
                  handleTogglePriority(priority as FractalTaskPriority)
                }
              >
                <Icon className={`mr-2 size-4 ${config.colorClass}`} />
                {config.label}
              </DropdownMenuCheckboxItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>

      {/* Type Filter */}
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
            <Icon
              value={(activeTypeConfig as any)?.icon}
              fallback={selectedTypes.length > 0 ? "Tag" : "Tags"}
              className="size-3.5"
              style={
                activeTypeConfig?.color
                  ? { color: activeTypeConfig.color }
                  : undefined
              }
            />
            {getTypeButtonLabel()}
            <ChevronDown
              className="size-3 opacity-50 ml-0.5"
              style={
                activeTypeConfig?.color
                  ? { color: activeTypeConfig.color }
                  : undefined
              }
            />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-48">
          {taskTypes.length === 0 && (
            <DropdownMenuItem disabled>No types available</DropdownMenuItem>
          )}
          {taskTypes.map((type) => {
            const workspaceType = currentWorkspace?.tasks?.find(
              (t) => t.id === type,
            );
            return (
              <DropdownMenuCheckboxItem
                key={type}
                checked={selectedTypes.includes(type)}
                onCheckedChange={() => handleToggleType(type)}
              >
                <div className="block size-2 border rounded-full" style={{ borderColor: workspaceType?.color }} />
                <span
                >
                  {workspaceType?.label || type}
                </span>
              </DropdownMenuCheckboxItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>

      {/* Project Filter */}
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
            <DropdownMenuItem disabled>No projects available</DropdownMenuItem>
          )}
          {projects.map((project) => (
            <DropdownMenuCheckboxItem
              key={project.id}
              checked={selectedProjects.includes(project.id)}
              onCheckedChange={() => handleToggleProject(project.id)}
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

      {/* Goal Filter */}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="sm"
            className={cn(
              "h-8 text-xs font-normal border border-dashed border-border/60 gap-1.5 px-2.5",
              selectedGoals.length > 0
                ? "bg-secondary text-secondary-foreground border-solid border-border font-medium"
                : "text-muted-foreground hover:bg-secondary/40 hover:text-foreground",
            )}
          >
            <Target className="size-3.5" />
            {getGoalButtonLabel()}
            <ChevronDown className="size-3 opacity-50 ml-0.5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-56">
          {goals.length === 0 && (
            <DropdownMenuItem disabled>No goals available</DropdownMenuItem>
          )}
          {goals.map((goal) => (
            <DropdownMenuCheckboxItem
              key={goal.id}
              checked={selectedGoals.includes(goal.id)}
              onCheckedChange={() => handleToggleGoal(goal.id)}
            >
              <Target className="mr-2 size-4 text-muted-foreground" />
              {goal.title}
            </DropdownMenuCheckboxItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      {/* Clear Filters Button */}
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
