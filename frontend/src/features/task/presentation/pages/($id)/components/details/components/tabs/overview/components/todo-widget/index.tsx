import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { Flag, Plus } from "lucide-react";
import { aos } from "@/app/aos";
import { TodoItem } from "./components/todo-item";
import { Button } from "@/components/ui/button";
import { TodoDialogUpsert } from "./components/todo-dialog-upsert";
import type { FractalTodo } from "@/features/task/interfaces/todo.interfaces";

interface TodoWidgetProps {
  taskId: string;
}

export function TodoWidget({ taskId }: TodoWidgetProps) {
  const { data: todosData, refetch } = aos.client.todo.list.useQuery({
    enabled: !!taskId,
    params: {
      taskId,
    },
  });

  const todos: FractalTodo[] =
    (todosData as { todos: FractalTodo[] } | null | undefined)?.todos || [];
  const finishedCount = todos.filter((todo) => todo.status === "finished").length;
  const progressPct = todos.length > 0 ? Math.round((finishedCount / todos.length) * 100) : 0;

  return (
    <SplitPageLayout.Widget>
      <SplitPageLayout.WidgetHeader>
        <SplitPageLayout.WidgetTitle>Todos</SplitPageLayout.WidgetTitle>
        <div className="ml-auto gap-2 flex items-center">
          {todos.length > 0 && (
            <span className="text-xs text-muted-foreground">{progressPct}%</span>
          )}
          <TodoDialogUpsert taskId={taskId} onCreated={() => refetch()}>
            <Button size="icon" variant="secondary" className="rounded-full">
              <Plus />
            </Button>
          </TodoDialogUpsert>
        </div>
      </SplitPageLayout.WidgetHeader>
      <SplitPageLayout.WidgetContent>
        <SplitPageLayout.WidgetItem>
          <Flag className="size-3.5 shrink-0 text-muted-foreground" />
          <span className="text-xs text-muted-foreground">
            {finishedCount} / {todos.length} todos finished
          </span>
        </SplitPageLayout.WidgetItem>
        {todos.length === 0 && (
          <SplitPageLayout.WidgetItem>
            <span className="text-xs text-muted-foreground">No todos yet.</span>
          </SplitPageLayout.WidgetItem>
        )}
        {todos.map((todo) => (
          <TodoDialogUpsert taskId={taskId} todo={todo} onCreated={() => refetch()}>
            <TodoItem key={todo.id} todo={todo} />
          </TodoDialogUpsert>
        ))}
      </SplitPageLayout.WidgetContent>
    </SplitPageLayout.Widget>
  );
}
