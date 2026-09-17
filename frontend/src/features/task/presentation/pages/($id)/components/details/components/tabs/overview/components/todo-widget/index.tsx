import { useState } from "react";
import { useRouter } from "@tanstack/react-router";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { Flag, Plus } from "lucide-react";
import { aos } from "@/app/aos";
import { TodoItem } from "./components/todo-item";
import { Button } from "@/components/ui/button";
import { TodoDialogUpsert } from "./components/todo-dialog-upsert";
import type { Todo } from "@/features/task/interfaces/todo.interfaces";
import { t } from "@/lib/i18n";

interface TodoWidgetProps {
  taskId: string;
}

export function TodoWidget({ taskId }: TodoWidgetProps) {
  const router = useRouter();
  const [editing, setEditing] = useState<Todo | null>(null);
  const { data: todosData, refetch } = aos.client.todo.list.useQuery({
    enabled: !!taskId,
    params: {
      taskId,
    },
  });

  const todos: Todo[] =
    (todosData as { todos: Todo[] } | null | undefined)?.todos || [];
  const progress = (todosData as { progress?: { completed: number; total: number } } | null | undefined)?.progress;
  // Skipped steps count as done, the way the review guard counts them.
  const settledCount = progress?.completed ?? todos.filter((todo) => todo.status === "finished" || todo.status === "skipped").length;
  const progressPct = todos.length > 0 ? Math.round((settledCount / todos.length) * 100) : 0;

  // The task's own progress (the header's percentage, the review button) is
  // read from the task, so a step that moved re-reads both.
  const refresh = () => {
    void refetch();
    void router.invalidate();
  };

  return (
    <SplitPageLayout.Widget>
      <SplitPageLayout.WidgetHeader>
        <SplitPageLayout.WidgetTitle>{t("Todos")}</SplitPageLayout.WidgetTitle>
        <div className="ml-auto gap-2 flex items-center">
          {todos.length > 0 && (
            <span className="text-xs text-muted-foreground">{progressPct}%</span>
          )}
          <TodoDialogUpsert taskId={taskId} onCreated={refresh}>
            <Button size="icon" variant="secondary" className="rounded-full" aria-label={t("Add todo")}>
              <Plus />
            </Button>
          </TodoDialogUpsert>
        </div>
      </SplitPageLayout.WidgetHeader>
      <SplitPageLayout.WidgetContent>
        <SplitPageLayout.WidgetItem>
          <Flag className="size-3.5 shrink-0 text-muted-foreground" />
          <span className="text-xs text-muted-foreground">
            {t("{{done}} / {{total}} todos finished", { done: settledCount, total: todos.length })}
          </span>
        </SplitPageLayout.WidgetItem>
        {todos.length === 0 && (
          <SplitPageLayout.WidgetItem>
            <span className="text-xs text-muted-foreground">{t("No todos yet.")}</span>
          </SplitPageLayout.WidgetItem>
        )}
        {todos.map((todo) => (
          <TodoItem
            key={todo.id}
            taskId={taskId}
            todo={todo}
            onOpen={() => setEditing(todo)}
            onChanged={refresh}
          />
        ))}
      </SplitPageLayout.WidgetContent>

      {/* One dialog for whichever step was opened, rather than one mounted per
          row with the row as its trigger — which is also what left the list
          without keys on its children. */}
      <TodoDialogUpsert
        taskId={taskId}
        todo={editing ?? undefined}
        open={editing !== null}
        onOpenChange={(next) => {
          if (!next) setEditing(null);
        }}
        onCreated={refresh}
      />
    </SplitPageLayout.Widget>
  );
}
