import {
  SetPriorityDropdown,
  SetAssigneeDropdown,
} from "@/features/task/presentation/components/dropdowns";
import { SetStatusDropdown } from "@/features/task/presentation/components/dropdowns/set-status.dropdown";
import { SetTypeDropdown } from "@/features/task/presentation/components/dropdowns/set-type.dropdown";
import type { TaskWithContext } from "@/features/task/interfaces/task.interfaces";
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
import { allowedMoves } from "@/features/task/presentation/helpers/task-lifecycle.helper";
import { TASK_PRIORITY_CONFIG } from "@/features/task/presentation/consts/task";
import { ProjectSelectorDropdown } from "@/components/ui/project-selector-dropdown";
import { GoalSelectorDropdown } from "@/components/ui/goal-selector-dropdown";
import { TaskDueDatePicker } from "@/features/task/presentation/components/due-date/task-due-date";
import { useAssigneeDirectory } from "@/features/task/presentation/hooks/assignee-directory.hook";
import type { TaskActions } from "@/features/task/presentation/hooks/task-actions.hook";
import { Icon } from "@/components/ui/icon";
import { ProjectHelper } from "@/features/project/presentation/helpers/project.helper";
import { t } from "@/lib/i18n";

interface TaskOverviewTabProps {
  task: TaskWithContext;
  actions: TaskActions;
}

export function TaskOverviewTab({ task, actions }: TaskOverviewTabProps) {
  const directory = useAssigneeDirectory();
  const projects = aos.stores.projects.useState((state) => state.items);
  const goals = aos.stores.goals.useState((state) => state.items);
  const workspace = aos.stores.workspace.useState((state) => state.current);
  const taskType = workspace?.tasks?.find((type) => type.id === task.type);

  const statusCfg = TaskHelper.getStatus(task.status);
  const StatusIcon = statusCfg.icon;

  const priorityCfg = TASK_PRIORITY_CONFIG[task.priority];
  const PriorityIcon = priorityCfg.icon;

  const assignee = resolveTaskAssignee(directory, task);
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
              {t("Status")}
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
                  allowed={allowedMoves(task)}
                  onStatusChange={actions.setStatus}
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
              {t("Priority")}
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
                  onPriorityChange={actions.setPriority}
                />
              </DropdownMenuContent>
            </DropdownMenu>
          </SplitPageLayout.WidgetItem>

          {/* Type Dropdown */}
          <SplitPageLayout.WidgetItem>
            <TagIcon className="size-3" />
            <span className="w-16 shrink-0 text-xs text-muted-foreground">
              {t("Type")}
            </span>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button className="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs hover:bg-accent">
                  {taskType?.label || task.type || t("No type")}
                  <ChevronDown className="ml-auto size-3 shrink-0 opacity-60" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start">
                <SetTypeDropdown
                  currentType={task.type}
                  onTypeChange={actions.setType}
                />
              </DropdownMenuContent>
            </DropdownMenu>
          </SplitPageLayout.WidgetItem>

          {/* Assignee Dropdown */}
          <SplitPageLayout.WidgetItem>
            <User className="size-3" />
            <span className="w-16 shrink-0 text-xs text-muted-foreground">
              {t("Assignee")}
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
                  {assignee?.name || t("Unassigned")}
                  <ChevronDown className="ml-auto size-3 shrink-0 opacity-60" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start" className="w-64">
                <SetAssigneeDropdown
                  currentAssignee={task.assigned}
                  onAssigneeChange={actions.setAssignee}
                />
              </DropdownMenuContent>
            </DropdownMenu>
          </SplitPageLayout.WidgetItem>

          {/* Due Date Dropdown */}
          <SplitPageLayout.WidgetItem>
            <CalendarDays className="size-3.5 shrink-0 text-muted-foreground" />
            <span className="w-16 shrink-0 text-xs text-muted-foreground">
              {t("Due Date")}
            </span>
            <TaskDueDatePicker value={task.dueAt} onChange={actions.setDueDate} />
          </SplitPageLayout.WidgetItem>

          <SplitPageLayout.WidgetItem>
            <Folder className="size-3.5 shrink-0 text-muted-foreground" />
            <span className="w-16 shrink-0 text-xs text-muted-foreground">
              {t("Project")}
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
                    {currentProject?.name || t("No project")}
                  </span>
                  <ChevronDown className="ml-auto size-3 shrink-0 opacity-60" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start" className="w-64">
                <ProjectSelectorDropdown
                  currentProject={task.project}
                  onProjectChange={actions.setProject}
                />
              </DropdownMenuContent>
            </DropdownMenu>
          </SplitPageLayout.WidgetItem>

          <SplitPageLayout.WidgetItem>
            <Target className="size-3.5 shrink-0 text-muted-foreground" />
            <span className="w-16 shrink-0 text-xs text-muted-foreground">
              {t("Goal")}
            </span>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button className="flex items-center gap-2 rounded px-1.5 py-0.5 text-xs hover:bg-accent">
                  <span className="line-clamp-1 text-left">
                    {currentGoal?.title || t("No goal")}
                  </span>
                  <ChevronDown className="ml-auto size-3 shrink-0 opacity-60" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start" className="w-64">
                <GoalSelectorDropdown
                  currentGoal={task.goal}
                  onGoalChange={actions.setGoal}
                />
              </DropdownMenuContent>
            </DropdownMenu>
          </SplitPageLayout.WidgetItem>

          {task.template && (
            <SplitPageLayout.WidgetItem>
              <FileOutput className="size-3.5 shrink-0 text-muted-foreground" />
              <span className="w-16 shrink-0 text-xs text-muted-foreground">
                {t("Template")}
              </span>
              <span className="truncate text-xs">{task.template}</span>
            </SplitPageLayout.WidgetItem>
          )}
          {task.worktree.enabled && (
            <SplitPageLayout.WidgetItem>
              <GitBranch className="size-3.5 shrink-0 text-muted-foreground" />
              <span className="w-16 shrink-0 text-xs text-muted-foreground">
                {t("Worktree")}
              </span>
              {/* The branch the daemon cuts, not a made-up "/task/<id>", and a
                  path only once the checkout exists. */}
              <span className="flex min-w-0 flex-col text-xs">
                <span className="truncate" title={TaskHelper.branchName(task, workspace?.git?.branchPrefix)}>
                  {TaskHelper.branchName(task, workspace?.git?.branchPrefix)}
                </span>
                <code className="truncate text-muted-foreground" title={task.worktree.path}>
                  {task.worktree.path || t("Not created yet")}
                </code>
              </span>
            </SplitPageLayout.WidgetItem>
          )}
          {task.worktree.base && (
            <SplitPageLayout.WidgetItem>
              <GitBranch className="size-3.5 shrink-0 text-muted-foreground" />
              <span className="w-16 shrink-0 text-xs text-muted-foreground">
                {t("Base")}
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
            <SplitPageLayout.WidgetTitle>{t("Summary")}</SplitPageLayout.WidgetTitle>
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

      <DependenciesWidget task={task} actions={actions} />

      <TodoWidget taskId={task.id} />
    </>
  );
}
