import type { UseChatResult } from "@/features/chat/presentation/hooks/use-chat";
import type { TaskWithContext } from "@/features/task/interfaces/task.interfaces";
import type { TaskActions } from "@/features/task/presentation/hooks/task-actions.hook";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { TaskOverviewTab } from "./components/tabs/overview";
import { TaskExecutionTab } from "./components/tabs/execution";
import { ListChecks, PlayIcon } from "lucide-react";
import { t } from "@/lib/i18n";

interface TaskDetailsSidebarProps {
  task: TaskWithContext;
  actions: TaskActions;
  liveChat?: UseChatResult | null;
}

export function TaskDetailsSidebar({ task, actions, liveChat }: TaskDetailsSidebarProps) {
  return (
    <SplitPageLayout.DetailTabs defaultValue="overview">
      <SplitPageLayout.DetailTab value="overview" label={t("Overview")} icon={ListChecks}>
        <TaskOverviewTab task={task} actions={actions} />
      </SplitPageLayout.DetailTab>
      <SplitPageLayout.DetailTab value="execution" label={t("Execution")} icon={PlayIcon}>
        <TaskExecutionTab liveChat={liveChat} task={task} />
      </SplitPageLayout.DetailTab>
    </SplitPageLayout.DetailTabs>
  );
}
