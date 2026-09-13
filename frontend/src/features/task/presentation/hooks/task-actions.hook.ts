import { toast } from "sonner";
import { aos } from "@/app/aos";
import { useAlert } from "@/components/ui/alert-provider";
import { errorMessage } from "@/lib/aos-facade";
import { t } from "@/lib/i18n";
import type { Task, TaskPriority, TaskStatus } from "@/features/task/interfaces/task.interfaces";
import { TASK_PRIORITY_CONFIG } from "@/features/task/presentation/consts/task";
import { TaskHelper } from "@/features/task/presentation/helpers/task.helper";
import { resolveAssignee } from "@/features/task/presentation/helpers/assignee.helper";
import { canMoveTo } from "@/features/task/presentation/helpers/task-lifecycle.helper";
import { useAssigneeDirectory } from "./assignee-directory.hook";

interface TaskActionsOptions {
  /** Re-reads whatever shows the task. Runs after every change the daemon accepted. */
  onChanged: () => void;
  /** Where to go once the task is gone. Defaults to `onChanged`. */
  onDeleted?: () => void;
}

/**
 * Every change a person can make to a task from a row, a card, the task page's
 * header or its sidebar.
 *
 * Those four places each carried their own copy of these handlers, and the
 * copies had drifted into the same defects in different spellings: a cleared
 * assignee, project, goal or due date was sent as `undefined`, which JSON
 * drops, so `tasks_update` read "leave it" and the toast said "Unassigned"
 * over a task that kept its owner; a status the lifecycle refuses was sent
 * anyway; Delete removed the task, its plan, its discussion and its runs on
 * one click; and every toast named the task by its UUID. One copy now.
 */
export function useTaskActions(task: Task, { onChanged, onDeleted }: TaskActionsOptions) {
  const { confirm } = useAlert();
  const directory = useAssigneeDirectory();

  async function update(body: Record<string, unknown>, success: string, failure: string): Promise<boolean> {
    try {
      await aos.client.task.update.mutateOrThrow({ params: { task: task.id }, body });
      toast.success(success);
      onChanged();
      return true;
    } catch (error) {
      toast.error(failure, { description: errorMessage(error) });
      return false;
    }
  }

  async function copy(value: string, success: string) {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(success);
    } catch (error) {
      toast.error(t("Could not copy to the clipboard"), { description: errorMessage(error) });
    }
  }

  return {
    setPriority: (priority: TaskPriority) =>
      update(
        { priority },
        t("Priority updated to {{priority}}", { priority: TASK_PRIORITY_CONFIG[priority].label }),
        t("Failed to update priority"),
      ),

    setType: (type: string) => update({ type }, t("Type updated"), t("Failed to update type")),

    // "" is how tasks_update is told to clear a field: a missing key means
    // "leave it as it is" (internal/domain/task/schema.go's UpdateInput).
    setAssignee: (assignee: string | undefined) =>
      update(
        { assigned: assignee ?? "" },
        assignee
          ? t("Assigned to {{name}}", { name: resolveAssignee(directory, assignee)?.name || assignee })
          : t("Unassigned"),
        t("Failed to update assignee"),
      ),

    setDueDate: (dueAt: string | undefined) =>
      update(
        { dueAt: dueAt ?? "" },
        dueAt ? t("Due date set") : t("Due date removed"),
        t("Failed to update due date"),
      ),

    setProject: (project: string | undefined) =>
      update(
        { project: project ?? "" },
        project ? t("Project updated") : t("Project removed"),
        t("Failed to update project"),
      ),

    setGoal: (goal: string | undefined) =>
      update({ goal: goal ?? "" }, goal ? t("Goal updated") : t("Goal removed"), t("Failed to update goal")),

    setDependencies: (dependsOn: string[]) =>
      update({ dependsOn }, t("Dependencies updated"), t("Failed to update dependencies")),

    setContent: (content: string) =>
      update({ content }, t("Description saved"), t("Failed to save the description")),

    /**
     * Moves the task. A move the lifecycle does not have is not sent: the
     * pickers already leave it out, and a stale screen that still offers one
     * says why instead of asking the daemon to refuse it.
     */
    async setStatus(status: TaskStatus): Promise<boolean> {
      if (status === task.status) return true;
      const to = TaskHelper.getStatus(status).label;
      if (!canMoveTo(task, status)) {
        toast.error(t("Failed to update status"), {
          description: t("A task in {{from}} cannot move to {{to}}.", {
            from: TaskHelper.getStatus(task.status).label,
            to,
          }),
        });
        return false;
      }
      try {
        await aos.client.task.setStatus.mutateOrThrow({ params: { task: task.id }, body: { status } });
        toast.success(t("Moved to {{status}}", { status: to }));
        onChanged();
        return true;
      } catch (error) {
        toast.error(t("Failed to update status"), { description: errorMessage(error) });
        return false;
      }
    },

    async remove(): Promise<boolean> {
      const confirmed = await confirm({
        title: t("Delete this task?"),
        description: t(
          "\"{{name}}\" is removed with its todos, comments and runs. This cannot be undone.",
          { name: task.name },
        ),
        confirmText: t("Delete task"),
        cancelText: t("Cancel"),
        variant: "destructive",
      });
      if (!confirmed) return false;
      try {
        await aos.client.task.delete.mutateOrThrow({ params: { task: task.id } });
        toast.success(t("Task deleted: {{name}}", { name: task.name }));
        (onDeleted ?? onChanged)();
        return true;
      } catch (error) {
        toast.error(t("Failed to delete task"), { description: errorMessage(error) });
        return false;
      }
    },

    /** Cuts the task's isolated checkout through tasks_branch. */
    async createWorktree(): Promise<boolean> {
      try {
        const tree = (await aos.client.task.branch.mutateOrThrow({ params: { task: task.id } })) as
          | { path?: string }
          | undefined;
        toast.success(t("Worktree ready"), { description: tree?.path });
        onChanged();
        return true;
      } catch (error) {
        toast.error(t("Could not create the worktree"), { description: errorMessage(error) });
        return false;
      }
    },

    copyIdentifier: () => copy(task.id, t("Task ID copied")),

    copyPath: (path: string) => copy(path, t("Worktree path copied")),

    copyPrompt: () =>
      copy(
        [
          `Task ${task.id}: ${task.name}`,
          task.summary ? `Summary: ${task.summary}` : undefined,
          task.content ? `Content:\n${task.content}` : undefined,
        ]
          .filter(Boolean)
          .join("\n\n"),
        t("Prompt copied"),
      ),
  };
}

export type TaskActions = ReturnType<typeof useTaskActions>;
