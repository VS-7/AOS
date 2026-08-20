import type { UseChatResult } from "@/features/chat/presentation/hooks/use-chat";
import type { TaskWithContext, TaskPriority } from "@/features/task/interfaces/task.interfaces";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { TaskOverviewTab } from "./components/tabs/overview";
import { TaskExecutionTab } from "./components/tabs/execution";
import { ListChecks, PlayIcon } from "lucide-react";

interface TaskDetailsSidebarProps {
  task: TaskWithContext;
  liveChat?: UseChatResult | null;
  onStatusChange: (status: TaskWithContext["status"]) => void;
  onPriorityChange: (priority: TaskPriority) => void;
  onTypeChange: (type: string) => void;
  onAssigneeChange: (assignee: string | undefined) => void;
  onDueDateChange: (dueAt: string | undefined) => void;
  onProjectChange: (project: string | undefined) => void;
  onGoalChange: (goal: string | undefined) => void;
}

export function TaskDetailsSidebar(props: TaskDetailsSidebarProps) {
  return (
    <SplitPageLayout.DetailTabs defaultValue="overview">
      <SplitPageLayout.DetailTab value="overview" label="Overview" icon={ListChecks}>
        <TaskOverviewTab {...props} />
      </SplitPageLayout.DetailTab>
      <SplitPageLayout.DetailTab value="execution" label="Execution" icon={PlayIcon}>
        <TaskExecutionTab liveChat={props.liveChat} task={props.task} />
      </SplitPageLayout.DetailTab>
    </SplitPageLayout.DetailTabs>
  );
}
