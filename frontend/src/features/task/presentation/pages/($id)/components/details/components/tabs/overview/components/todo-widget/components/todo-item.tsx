import React, { useRef, useState } from "react";
import { Trash2 } from "lucide-react";
import { toast } from "sonner";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useAlert } from "@/components/ui/alert-provider";
import { aos } from "@/app/aos";
import type { Todo } from "@/features/task/interfaces/todo.interfaces";
import { TaskHelper } from "@/features/task/presentation/helpers/task.helper";
import { TODO_STATUS_ORDER, TODO_TRANSITIONS } from "@/features/task/presentation/consts/todo";
import { errorMessage } from "@/lib/aos-facade";
import { t } from "@/lib/i18n";

interface TodoItemProps {
  taskId: string;
  todo: Todo;
  onOpen: () => void;
  onChanged: () => void;
}

/**
 * One step of the plan: its status, which can be moved, its title, which opens
 * the step, and a way to remove it.
 *
 * The widget used to list steps and nothing else. There was no way to finish,
 * block or skip one, or to delete one, although todos_set-status and
 * todos_delete are both mapped; and hovering a blocked step showed "Blocked"
 * without the evidence recorded for why, because it read a field (`output`)
 * the daemon has never had.
 */
export function TodoItem({ taskId, todo, onOpen, onChanged }: TodoItemProps) {
  const [open, setOpen] = useState(false);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { confirm } = useAlert();

  const statusCfg = TaskHelper.getTodoStatus(todo.status);
  const StatusIcon = statusCfg.icon;
  const statusColor = statusCfg.color;
  const isFinished = todo.status === "finished";
  const allowed = TODO_TRANSITIONS[todo.status] ?? [];

  function handleMouseEnter() {
    if (closeTimer.current) clearTimeout(closeTimer.current);
    setOpen(true);
  }

  function handleMouseLeave() {
    closeTimer.current = setTimeout(() => setOpen(false), 200);
  }

  async function setStatus(status: Todo["status"]) {
    try {
      const answer = (await aos.client.todo.setStatus.mutateOrThrow({
        params: { taskId, id: todo.id },
        body: { status },
      })) as { warning?: string } | undefined;
      toast.success(t("Step moved to {{status}}", { status: TaskHelper.getTodoStatus(status).label }), {
        // Finishing a step without evidence is allowed and advised against;
        // the daemon says so, and the person should read it.
        description: answer?.warning,
      });
      onChanged();
    } catch (error) {
      toast.error(t("Failed to update the step"), { description: errorMessage(error) });
    }
  }

  async function remove() {
    const confirmed = await confirm({
      title: t("Delete this step?"),
      description: t("\"{{title}}\" is removed from the plan.", { title: todo.title }),
      confirmText: t("Delete"),
      cancelText: t("Cancel"),
      variant: "destructive",
    });
    if (!confirmed) return;
    try {
      await aos.client.todo.delete.mutateOrThrow({ params: { taskId, id: todo.id } });
      toast.success(t("Step deleted"));
      onChanged();
    } catch (error) {
      toast.error(t("Failed to delete the step"), { description: errorMessage(error) });
    }
  }

  return (
    <SplitPageLayout.WidgetItem className="group relative gap-1.5 py-2 pl-2 pr-2">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            className="flex shrink-0 items-center justify-center rounded p-1 hover:bg-accent"
          >
            <StatusIcon className={`size-3.5 ${statusColor}`} />
            <span className="sr-only">{t("Step status: {{status}}", { status: statusCfg.label })}</span>
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          {TODO_STATUS_ORDER.map((status) => {
            const cfg = TaskHelper.getTodoStatus(status);
            const Icon = cfg.icon;
            const current = status === todo.status;
            return (
              <DropdownMenuItem
                key={status}
                disabled={!current && !allowed.includes(status)}
                onClick={() => {
                  if (!current) void setStatus(status);
                }}
                className="flex items-center gap-2"
              >
                <Icon className={`size-4 ${cfg.color}`} />
                <span>{cfg.label}</span>
                {current && <span className="ml-auto text-xs text-muted-foreground">✓</span>}
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>

      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <button
            type="button"
            className="min-w-0 flex-1 text-left"
            onClick={onOpen}
            onMouseEnter={handleMouseEnter}
            onMouseLeave={handleMouseLeave}
          >
            {/* Go's `Todo` field is `title`, not `description` — see
                `interfaces/task.interfaces.ts`'s `TaskTodoSchema`. */}
            <span className={`block text-xs leading-snug line-clamp-1 ${isFinished ? "line-through text-muted-foreground" : ""}`}>
              {todo.title}
            </span>
          </button>
        </PopoverTrigger>
        <PopoverContent
          side="left"
          align="start"
          className="w-72 p-3 space-y-2.5"
          onMouseEnter={handleMouseEnter}
          onMouseLeave={handleMouseLeave}
          onOpenAutoFocus={(event) => event.preventDefault()}
        >
          <p className="text-xs font-medium leading-snug">{todo.title}</p>
          <div className="flex items-center gap-2">
            <StatusIcon className={`size-3 shrink-0 ${statusColor}`} />
            <span className="text-sm text-muted-foreground">{statusCfg.label}</span>
          </div>
          {/* Go's field is `content`, not `instructions` — same schema note. */}
          {todo.content && (
            <div className="space-y-1">
              <span className="text-sm text-muted-foreground">{t("Notes")}</span>
              <p className="line-clamp-4 whitespace-pre-wrap text-xs">{todo.content}</p>
            </div>
          )}
          {todo.evidence && (
            <div className="space-y-1">
              <span className="text-sm text-muted-foreground">{t("Evidence")}</span>
              <p className="max-h-32 overflow-auto whitespace-pre-wrap rounded bg-muted/40 p-2 text-xs">
                {todo.evidence}
              </p>
            </div>
          )}
        </PopoverContent>
      </Popover>

      <button
        type="button"
        onClick={() => void remove()}
        className="shrink-0 rounded p-1 text-muted-foreground opacity-0 transition-opacity hover:bg-accent hover:text-foreground focus-visible:opacity-100 group-hover:opacity-100"
      >
        <Trash2 className="size-3" />
        <span className="sr-only">{t("Delete step")}</span>
      </button>
    </SplitPageLayout.WidgetItem>
  );
}
