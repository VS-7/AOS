import {
  SetPriorityDropdown,
  SetAssigneeDropdown,
} from "@/features/task/presentation/components/dropdowns";
import { SetStatusDropdown } from "@/features/task/presentation/components/dropdowns/set-status.dropdown";
import { SetTypeDropdown } from "@/features/task/presentation/components/dropdowns/set-type.dropdown";
import {
  TaskPriority,
  TaskWithContext,
} from "@/features/task/interfaces/task.interfaces";
import {
  Avatar,
  AvatarAgentFallback,
  AvatarFallback,
  AvatarImage,
} from "@/components/ui/avatar";
import {
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenu,
} from "@/components/ui/dropdown-menu";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import {
  TagIcon,
  User,
  UserIcon,
  CalendarDays,
  FileOutput,
  GitBranch,
  Folder,
  Target,
  ChevronDown,
} from "lucide-react";
import { TodoWidget } from "./components/todo-widget";
import { DependenciesWidget } from "./components/dependencies-widget";
import { aos } from "@/app/aos";
import { TaskHelper } from "@/features/task/presentation/helpers/task.helper";
import { assigneeInitials, resolveTaskAssignee } from "@/features/task/presentation/helpers/assignee.helper";
import { TASK_PRIORITY_CONFIG } from "@/features/task/presentation/consts/task";
import { ProjectSelectorDropdown } from "@/components/ui/project-selector-dropdown";
import { GoalSelectorDropdown } from "@/components/ui/goal-selector-dropdown";
import { DateTimeInput } from "@/components/ui/date-time-input";
import { Icon } from "@/components/ui/icon";
import { ProjectHelper } from "@/features/project/presentation/helpers/project.helper";

interface TaskDetailsSidebarProps {
  task: TaskWithContext;
  onStatusChange: (status: TaskWithContext["status"]) => void;
  onPriorityChange: (priority: TaskPriority) => void;
  onTypeChange: (type: string) => void;
  onAssigneeChange: (assignee: string | undefined) => void;
  onDueDateChange: (dueAt: string | undefined) => void;
  onProjectChange: (project: string | undefined) => void;
  onGoalChange: (goal: string | undefined) => void;
}

export function TaskOverviewTab({
  task,
  onAssigneeChange,
  onDueDateChange,
  onPriorityChange,
  onStatusChange,
  onTypeChange,
  onProjectChange,
  onGoalChange,
}: TaskDetailsSidebarProps) {
  const directory = aos.stores.workspace.useState((state) => state.directory);
  const self = aos.stores.auth.useState((state) => state.user);
  const projects = aos.stores.projects.useState((state) => state.items);
  const goals = aos.stores.goals.useState((state) => state.items);

  const statusCfg = TaskHelper.getStatus(task.status);
  const StatusIcon = statusCfg.icon;

  const priorityCfg = TASK_PRIORITY_CONFIG[task.priority];
  const PriorityIcon = priorityCfg.icon;

  const assignee = resolveTaskAssignee(
    { ...directory, self },
    { assigned: task.assigned, assignee: task.assignee },
  );
  const currentProject = projects.find(
    (project) => project.id === task.project,
  );
  const currentGoal = goals.find((goal) => goal.id === task.goal);

  return (
    <>
      <SplitPageLayout.Widget>
        <SplitPageLayout.WidgetContent>
          {/* Status Dropdown */}
          <SplitPageLayout.WidgetItem>
            <StatusIcon className={`size-3.5 shrink-0 ${statusCfg.color}`} />
            <span className="w-16 shrink-0 text-xs text-muted-foreground">
              Status
            </span>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button className="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs hover:bg-accent">
                  {statusCfg.label}
                  <ChevronDown className="ml-auto size-3 shrink-0 opacity-60" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start">
                <SetStatusDropdown
                  currentStatus={task.status}
                  onStatusChange={(s) =>
                    onStatusChange(s as TaskWithContext["status"])
                  }
                />
              </DropdownMenuContent>
            </DropdownMenu>
          </SplitPageLayout.WidgetItem>

          {/* Priority Dropdown */}
          <SplitPageLayout.WidgetItem>
            <PriorityIcon
              className={`size-3.5 shrink-0 ${priorityCfg.colorClass}`}
            />
            <span className="w-16 shrink-0 text-xs text-muted-foreground">
              Priority
            </span>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button className="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs hover:bg-accent">
                  {priorityCfg.label}
                  <ChevronDown className="ml-auto size-3 shrink-0 opacity-60" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start">
                <SetPriorityDropdown
                  currentPriority={task.priority}
                  onPriorityChange={onPriorityChange}
                />
              </DropdownMenuContent>
            </DropdownMenu>
          </SplitPageLayout.WidgetItem>

          {/* Type Dropdown */}
          <SplitPageLayout.WidgetItem>
            <TagIcon className="size-3" />
            <span className="w-16 shrink-0 text-xs text-muted-foreground">
              Type
            </span>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button className="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs hover:bg-accent capitalize">
                  {task.type}
                  <ChevronDown className="ml-auto size-3 shrink-0 opacity-60" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start">
                <SetTypeDropdown
                  currentType={task.type}
                  onTypeChange={onTypeChange}
                />
              </DropdownMenuContent>
            </DropdownMenu>
          </SplitPageLayout.WidgetItem>

          {/* Assignee Dropdown */}
          <SplitPageLayout.WidgetItem>
            <User className="size-3" />
            <span className="w-16 shrink-0 text-xs text-muted-foreground">
              Assignee
            </span>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button className="flex items-center gap-2 rounded px-1.5 py-0.5 text-xs hover:bg-accent">
                  {assignee?.type === "agent" ? (
                    <Avatar className="size-3.5">
                      <AvatarAgentFallback
                        name={assignee.name.toLocaleLowerCase()}
                      />
                    </Avatar>
                  ) : assignee ? (
                    <Avatar className="size-3.5">
                      {assignee.image ? (
                        <AvatarImage src={assignee.image} alt={assignee.name} />
                      ) : (
                        <AvatarFallback>{assigneeInitials(assignee.name)}</AvatarFallback>
                      )}
                    </Avatar>
                  ) : (
                    <Avatar className="size-3.5">
                      <AvatarFallback>
                        <UserIcon className="size-3 text-muted-foreground" />
                      </AvatarFallback>
                    </Avatar>
                  )}
                  {assignee?.name || "Unassigned"}
                  <ChevronDown className="ml-auto size-3 shrink-0 opacity-60" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start" className="w-64">
                <SetAssigneeDropdown
                  currentAssignee={task.assigned}
                  onAssigneeChange={onAssigneeChange}
                />
              </DropdownMenuContent>
            </DropdownMenu>
          </SplitPageLayout.WidgetItem>

          {/* Due Date Dropdown */}
          <SplitPageLayout.WidgetItem>
            <CalendarDays className="size-3.5 shrink-0 text-muted-foreground" />
            <span className="w-16 shrink-0 text-xs text-muted-foreground">
              Due Date
            </span>
            <DateTimeInput
              value={task.dueAt}
              onValueChange={onDueDateChange}
              showTime
              variant="ghost"
              size="sm"
              className="h-auto w-auto min-w-0 flex-1 justify-start rounded px-1.5 py-0.5 text-xs"
            />
          </SplitPageLayout.WidgetItem>

          <SplitPageLayout.WidgetItem>
            <Folder className="size-3.5 shrink-0 text-muted-foreground" />
            <span className="w-16 shrink-0 text-xs text-muted-foreground">
              Project
            </span>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button className="flex items-center gap-2 rounded px-1.5 py-0.5 text-xs hover:bg-accent">
                  <Icon
                    value={ProjectHelper.getIcon(currentProject?.icon)}
                    fallback="Folder"
                    className="size-3.5 shrink-0 text-muted-foreground"
                  />
                  <span className="line-clamp-1 text-left">
                    {currentProject?.name || "No project"}
                  </span>
                  <ChevronDown className="ml-auto size-3 shrink-0 opacity-60" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start" className="w-64">
                <ProjectSelectorDropdown
                  currentProject={task.project}
                  onProjectChange={onProjectChange}
                />
              </DropdownMenuContent>
            </DropdownMenu>
          </SplitPageLayout.WidgetItem>

          <SplitPageLayout.WidgetItem>
            <Target className="size-3.5 shrink-0 text-muted-foreground" />
            <span className="w-16 shrink-0 text-xs text-muted-foreground">
              Goal
            </span>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button className="flex items-center gap-2 rounded px-1.5 py-0.5 text-xs hover:bg-accent">
                  <span className="line-clamp-1 text-left">
                    {currentGoal?.title || "No goal"}
                  </span>
                  <ChevronDown className="ml-auto size-3 shrink-0 opacity-60" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start" className="w-64">
                <GoalSelectorDropdown
                  currentGoal={task.goal}
                  onGoalChange={onGoalChange}
                />
              </DropdownMenuContent>
            </DropdownMenu>
          </SplitPageLayout.WidgetItem>

          {task.template && (
            <SplitPageLayout.WidgetItem>
              <FileOutput className="size-3.5 shrink-0 text-muted-foreground" />
              <span className="w-16 shrink-0 text-xs text-muted-foreground">
                Template
              </span>
              <span className="truncate text-xs">{task.template}</span>
            </SplitPageLayout.WidgetItem>
          )}
          {task.worktree.enabled && (
            <SplitPageLayout.WidgetItem>
              <GitBranch className="size-3.5 shrink-0 text-muted-foreground" />
              <span className="w-16 shrink-0 text-xs text-muted-foreground">
                Worktree
              </span>
              <span className="text-xs">
                {task.worktree.branch
                  ? `/${task.worktree.branch}`
                  : `/task/${task.id}`}
                {task.worktree.path && (
                  <code className="ml-1 text-xs text-muted-foreground">
                    → {task.worktree.path}
                  </code>
                )}
              </span>
            </SplitPageLayout.WidgetItem>
          )}
          {task.worktree.base && (
            <SplitPageLayout.WidgetItem>
              <GitBranch className="size-3.5 shrink-0 text-muted-foreground" />
              <span className="w-16 shrink-0 text-xs text-muted-foreground">
                Base
              </span>
              <code className="text-xs text-muted-foreground">
                {task.worktree.base}
              </code>
            </SplitPageLayout.WidgetItem>
          )}
        </SplitPageLayout.WidgetContent>
      </SplitPageLayout.Widget>

      {task.summary && (
        <SplitPageLayout.Widget>
          <SplitPageLayout.WidgetHeader>
            <SplitPageLayout.WidgetTitle>Summary</SplitPageLayout.WidgetTitle>
          </SplitPageLayout.WidgetHeader>
          <SplitPageLayout.WidgetContent>
            <SplitPageLayout.WidgetItem className="items-start">
              <p className="text-xs leading-relaxed text-muted-foreground">
                {task.summary}
              </p>
            </SplitPageLayout.WidgetItem>
          </SplitPageLayout.WidgetContent>
        </SplitPageLayout.Widget>
      )}

      <DependenciesWidget task={task} />

      <TodoWidget taskId={task.id} />
    </>
  );
}
