import React, { useMemo } from "react";
import { Page, PageBody, PageSecondaryHeader } from "@/components/ui/page";
import { Button } from "@/components/ui/button";
import { Plus, Search } from "lucide-react";
import { aos } from "@/app/aos";
import { TasksListSection } from "./components/list/components/task-list-section.component";
import { TaskKanbanColumn } from "./components/kanban/components/task-kanban-column.component";
import { TaskKanbanCard } from "./components/kanban/components/task-kanban-card.component";
import { TaskListRow } from "./components/list/components/task-list-row.component";
import { toast } from "sonner";
import { TasksFinishWorkflowDialog } from "../../components/dialogs/finish";
import { cn } from "@/lib/utils";
import { TASK_STATUS_ORDER } from "@/features/task/presentation/consts/task";
import { useTasksContext, useDragContext } from "./context";
import { TasksFilter } from "./components/header/filter";
import { TasksViewToggle } from "./components/header/view-toggle";
import { useRouter } from "@tanstack/react-router";
import { Input } from "@/components/ui/input";
import { t } from "@/lib/i18n";
import {
  DndContext,
  DragOverlay,
  useSensor,
  useSensors,
  PointerSensor,
} from "@dnd-kit/core";

// --- Memoized sub-trees to prevent unnecessary re-renders ---

const TasksHeader = React.memo(function TasksHeader() {
  const { searchDraft, handleSearchChange } = useTasksContext();

  return (
    <PageSecondaryHeader className="justify-between px-4 py-2">
      <div className="flex items-center gap-3">
        <h1 className="text-sm font-semibold tracking-tight text-foreground select-none pl-2">
          {t("Tasks")}
        </h1>
        <div className="relative flex items-center h-8 w-44 sm:w-56 rounded-md bg-transparent transition-colors">
          <Search className="absolute left-2.5 size-3.5 text-muted-foreground pointer-events-none" />
          <Input
            type="text"
            value={searchDraft}
            onChange={(event) => handleSearchChange(event.target.value)}
            placeholder={t("Search tasks...")}
            className="h-8 pl-8 pr-2.5 py-0 border-0 bg-transparent shadow-none focus-visible:ring-0 focus-visible:ring-offset-0 focus-visible:bg-transparent placeholder:text-muted-foreground/50 text-xs md:text-sm"
          />
        </div>
      </div>

      <div className="flex items-center gap-2">
        <TasksFilter />

        <Button
          className="h-9"
          // `aos.triggers` is statically typed as a union with the
          // no-trigger fallback (`AosTriggerAPIFallback`, which has no
          // `.dispatch`) regardless of whether `.withTriggers(...)` was
          // actually called — `aos.tsx` does call it, so this is real at
          // runtime; the cast just narrows past a static type the builder
          // can't currently express.
          onClick={() => (aos.triggers as { dispatch: (id: string) => void }).dispatch("tasks.new")}
        >
          <Plus data-icon="inline-start" />
          {t("Add task")}
        </Button>

        <TasksViewToggle />
      </div>
    </PageSecondaryHeader>
  );
});

const TasksListView = React.memo(function TasksListView() {
  const { displayedGroupedTasks, selectedStatuses } = useTasksContext();

  const visibleStatuses = useMemo(
    () =>
      TASK_STATUS_ORDER.filter(
        (status) =>
          selectedStatuses.length === 0 || selectedStatuses.includes(status),
      ),
    [selectedStatuses],
  );

  return (
    <div className="gap-4 p-4">
      {visibleStatuses.map((status) => (
        <TasksListSection
          key={status}
          status={status}
          tasks={displayedGroupedTasks[status] || []}
        />
      ))}
    </div>
  );
});

const TasksKanbanView = React.memo(function TasksKanbanView() {
  const { displayedGroupedTasks, selectedStatuses } = useTasksContext();
  const { activeTaskId, isDragActive, activeDropStatus } = useDragContext();

  const visibleStatuses = useMemo(
    () =>
      TASK_STATUS_ORDER.filter(
        (status) =>
          selectedStatuses.length === 0 || selectedStatuses.includes(status),
      ),
    [selectedStatuses],
  );

  return (
    <div className="h-full overflow-x-auto overflow-y-hidden">
      <div className="grid h-full min-w-max auto-cols-[20rem] grid-flow-col divide-x">
        {visibleStatuses.map((status) => (
          <TaskKanbanColumn
            key={status}
            status={status}
            tasks={displayedGroupedTasks[status] || []}
            draggedTaskId={activeTaskId}
            isDragActive={isDragActive}
            isActiveDropTarget={activeDropStatus === status}
          />
        ))}
      </div>
    </div>
  );
});

const TasksDragOverlay = React.memo(function TasksDragOverlay({
  currentView,
}: {
  currentView: "list" | "kanban";
}) {
  const { activeTask } = useDragContext();

  return (
    <DragOverlay dropAnimation={null}>
      {activeTask ? (
        currentView === "kanban" ? (
          <TaskKanbanCard
            task={activeTask}
            isDragging={false}
            isDragOverlay={true}
          />
        ) : (
          <TaskListRow task={activeTask} isDragOverlay={true} />
        )
      ) : null}
    </DragOverlay>
  );
});

export function TasksPageInner() {
  const { currentView, finishTransition } = useTasksContext();
  const {
    handleDragStart,
    handleDragOver,
    handleDragEnd,
    handleDragCancel,
  } = useDragContext();

  const router = useRouter();

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 8,
      },
    }),
  );

  return (
    <>
      <Page className="h-full overflow-hidden">
        <TasksHeader />

        <PageBody
          className={cn(
            currentView === "kanban" ? "overflow-hidden" : "overflow-y-auto",
          )}
        >
          <DndContext
            sensors={sensors}
            onDragStart={handleDragStart}
            onDragOver={handleDragOver}
            onDragEnd={handleDragEnd}
            onDragCancel={handleDragCancel}
          >
            <div className={cn("mx-auto flex h-full w-full flex-col")}>
              {currentView === "list" && <TasksListView />}
              {currentView === "kanban" && <TasksKanbanView />}
            </div>

            <TasksDragOverlay currentView={currentView} />
          </DndContext>
        </PageBody>
      </Page>

      <TasksFinishWorkflowDialog
        open={finishTransition.state.open}
        task={finishTransition.state.task}
        onOpenChange={(open) => {
          if (!open) finishTransition.close();
        }}
        onConfirm={async (input) => {
          if (!finishTransition.state.task) {
            finishTransition.close();
            return;
          }

          const { error } = await aos.client.task.setStatus.mutate({
            params: { task: finishTransition.state.task.id },
            body: input,
          });

          if (error) {
            toast.error(
              error instanceof Error ? error.message : "Failed to finish task",
            );
            return;
          }

          toast.success(`Finished ${finishTransition.state.task.id}`);
          finishTransition.close();
          router.invalidate();
        }}
      />
    </>
  );
}
