import React, {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  useCallback,
  useRef,
  startTransition,
} from "react";
import { useNavigate, useRouter } from "@tanstack/react-router";
import { toast } from "sonner";
import { aos } from "@/app/aos";
import { errorMessage } from "@/lib/aos-facade";
import { t } from "@/lib/i18n";
import type {
  Task,
  TaskPriority,
} from "@/features/task/interfaces/task.interfaces";
import { TaskHelper } from "../../helpers/task.helper";
import { TASK_STATUS_ORDER } from "../../consts/task";
import { canMoveTo } from "../../helpers/task-lifecycle.helper";
import { filterTasks, filterableTypes } from "../../helpers/task-filter.helper";
import type {
  DragStartEvent,
  DragEndEvent,
  DragOverEvent,
} from "@dnd-kit/core";

interface TasksPageSearchSchema {
  view?: "list" | "kanban";
  query?: string;
  status?: string;
  priority?: string;
  type?: string;
  project?: string;
  goal?: string;
}

// --- Drag Context (separated to avoid re-rendering non-drag consumers) ---

interface DragContextValue {
  activeTaskId: string | null;
  activeTask: Task | null;
  overContainerId: Task["status"] | null;
  isDragActive: boolean;
  activeDropStatus: Task["status"] | null;
  /** Whether the task being dragged can be dropped on this status. True when nothing is dragged. */
  acceptsDrop: (status: Task["status"]) => boolean;
  handleDragStart: (event: DragStartEvent) => void;
  handleDragOver: (event: DragOverEvent) => void;
  handleDragEnd: (event: DragEndEvent) => void;
  handleDragCancel: () => void;
}

const DragContext = createContext<DragContextValue | null>(null);

export function useDragContext() {
  const ctx = useContext(DragContext);
  if (!ctx) {
    throw new Error("useDragContext must be used within a TasksProvider");
  }
  return ctx;
}

// --- Tasks Context (data + filters + UI) ---

interface TasksContextValue {
  // Data
  tasks: Task[];
  filteredTasks: Task[];
  displayedGroupedTasks: Record<Task["status"], Task[]>;
  statusCountByType: Record<Task["status"], number>;
  taskTypes: string[];

  // Search state
  search: TasksPageSearchSchema;
  searchDraft: string;

  // Selected filters
  selectedStatuses: Task["status"][];
  selectedPriorities: TaskPriority[];
  selectedTypes: string[];
  selectedProjects: string[];
  selectedGoals: string[];

  // UI state
  activeFilterCount: number;
  hasStatusShortcut: "all" | Task["status"];
  currentView: "list" | "kanban";

  /** Whether a search or a filter is narrowing the list. */
  isNarrowed: boolean;

  // Actions (stable refs)
  updateSearch: (next: Partial<TasksPageSearchSchema>) => void;
  handleViewChange: (view: "list" | "kanban") => void;
  handleSearchChange: (value: string) => void;
  handleStatusShortcut: (status: "all" | Task["status"]) => void;
  handleToggleStatus: (status: Task["status"]) => void;
  handleTogglePriority: (priority: TaskPriority) => void;
  handleToggleType: (type: string) => void;
  handleToggleProject: (project: string) => void;
  handleToggleGoal: (goal: string) => void;
  clearFilters: () => void;
  setSearchDraft: (value: string) => void;
}

const TasksContext = createContext<TasksContextValue | null>(null);

export function useTasksContext() {
  const ctx = useContext(TasksContext);
  if (!ctx) {
    throw new Error("useTasksContext must be used within a TasksProvider");
  }
  return ctx;
}

// --- Utility functions (module-level, no re-creation) ---

function parseMultiValue(value?: string): string[] {
  if (!value) return [];
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function serializeMultiValue(values: string[]): string | undefined {
  if (!values.length) return undefined;
  return values.join(",");
}

function toggleFilterValue(values: string[], value: string): string[] {
  if (values.includes(value)) {
    return values.filter((item) => item !== value);
  }
  return [...values, value];
}

// --- Provider ---

interface TasksProviderProps {
  children: React.ReactNode;
  tasks: Task[];
  search: TasksPageSearchSchema;
  client: any;
  route: any;
}

export function TasksProvider({
  children,
  tasks,
  search,
  client,
  route,
}: TasksProviderProps) {
  const navigate = useNavigate();
  const router = useRouter();
  const [searchDraft, setSearchDraft] = useState(search.query ?? "");
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [overContainerId, setOverContainerId] = useState<
    Task["status"] | null
  >(null);
  const workspaceTypes = aos.stores.workspace.useState(
    (state) => state.current?.tasks,
  );

  // Keep a stable ref to tasks for drag handlers so they don't stale-close
  const tasksRef = useRef(tasks);
  tasksRef.current = tasks;

  useEffect(() => {
    setSearchDraft(search.query ?? "");
  }, [search.query]);

  // --- Parsed filters (memoized) ---

  const selectedStatuses = useMemo(
    () => parseMultiValue(search.status) as Task["status"][],
    [search.status],
  );
  const selectedPriorities = useMemo(
    () => parseMultiValue(search.priority) as TaskPriority[],
    [search.priority],
  );
  const selectedTypes = useMemo(
    () => parseMultiValue(search.type),
    [search.type],
  );
  const selectedProjects = useMemo(
    () => parseMultiValue(search.project),
    [search.project],
  );
  const selectedGoals = useMemo(
    () => parseMultiValue(search.goal),
    [search.goal],
  );

  // --- Derived data (memoized) ---

  const taskTypes = useMemo(
    () => filterableTypes(workspaceTypes ?? [], tasks),
    [workspaceTypes, tasks],
  );

  // Priority, type, project and goal are applied here, over everything the
  // loader read — see `filterTasks` for why they are no longer sent.
  const matchingTasks = useMemo(
    () =>
      filterTasks(tasks, {
        priorities: selectedPriorities,
        types: selectedTypes,
        projects: selectedProjects,
        goals: selectedGoals,
      }),
    [tasks, selectedPriorities, selectedTypes, selectedProjects, selectedGoals],
  );

  const statusCountByType = useMemo(() => {
    return TASK_STATUS_ORDER.reduce(
      (acc, status) => {
        acc[status] = matchingTasks.filter((task) => task.status === status).length;
        return acc;
      },
      {} as Record<Task["status"], number>,
    );
  }, [matchingTasks]);

  const filteredTasks = useMemo(() => {
    if (selectedStatuses.length === 0) return matchingTasks;
    return matchingTasks.filter((task) => selectedStatuses.includes(task.status));
  }, [matchingTasks, selectedStatuses]);

  const displayedGroupedTasks = useMemo(
    () => TaskHelper.groupByStatus(filteredTasks),
    [filteredTasks],
  );

  const activeTask = useMemo(() => {
    if (!activeTaskId) return null;
    return tasks.find((t) => t.id === activeTaskId) ?? null;
  }, [tasks, activeTaskId]);

  // --- Scalar derived values ---

  const activeFilterCount =
    selectedStatuses.length +
    selectedPriorities.length +
    selectedTypes.length +
    selectedProjects.length +
    selectedGoals.length;

  const hasStatusShortcut: "all" | Task["status"] =
    selectedStatuses.length === 1 ? selectedStatuses[0] : "all";

  const currentView = search.view === "kanban" ? "kanban" : "list";
  const isDragActive = activeTaskId !== null;
  const isNarrowed = activeFilterCount > 0 || Boolean(search.query?.trim());

  const acceptsDrop = useCallback(
    (status: Task["status"]) => !activeTask || activeTask.status === status || canMoveTo(activeTask, status),
    [activeTask],
  );

  // --- Stable action callbacks ---

  const updateSearch = useCallback(
    (next: Partial<TasksPageSearchSchema>) => {
      startTransition(() => {
        navigate({
          to: "/tasks",
          search: (prev: Partial<TasksPageSearchSchema>) => ({
            ...prev,
            ...next,
          }),
        });
        router.invalidate();
      });
    },
    [navigate, router],
  );

  const handleViewChange = useCallback(
    (view: "list" | "kanban") => {
      updateSearch({ view: view === "list" ? undefined : view });
    },
    [updateSearch],
  );

  const handleSearchChange = useCallback(
    (value: string) => {
      setSearchDraft(value);
      updateSearch({ query: value.trim() ? value : undefined });
    },
    [updateSearch],
  );

  const handleStatusShortcut = useCallback(
    (status: "all" | Task["status"]) => {
      updateSearch({
        status: status === "all" ? undefined : serializeMultiValue([status]),
      });
    },
    [updateSearch],
  );

  const handleToggleStatus = useCallback(
    (status: Task["status"]) => {
      updateSearch({
        status: serializeMultiValue(
          toggleFilterValue(
            parseMultiValue(search.status),
            status,
          ),
        ),
      });
    },
    [updateSearch, search.status],
  );

  const handleTogglePriority = useCallback(
    (priority: TaskPriority) => {
      updateSearch({
        priority: serializeMultiValue(
          toggleFilterValue(
            parseMultiValue(search.priority),
            priority,
          ),
        ),
      });
    },
    [updateSearch, search.priority],
  );

  const handleToggleType = useCallback(
    (type: string) => {
      updateSearch({
        type: serializeMultiValue(
          toggleFilterValue(parseMultiValue(search.type), type),
        ),
      });
    },
    [updateSearch, search.type],
  );

  const handleToggleProject = useCallback(
    (project: string) => {
      updateSearch({
        project: serializeMultiValue(
          toggleFilterValue(
            parseMultiValue(search.project),
            project,
          ),
        ),
      });
    },
    [updateSearch, search.project],
  );

  const handleToggleGoal = useCallback(
    (goal: string) => {
      updateSearch({
        goal: serializeMultiValue(
          toggleFilterValue(parseMultiValue(search.goal), goal),
        ),
      });
    },
    [updateSearch, search.goal],
  );

  const clearFilters = useCallback(() => {
    updateSearch({
      status: undefined,
      priority: undefined,
      type: undefined,
      project: undefined,
      goal: undefined,
    });
  }, [updateSearch]);

  // --- Drag handlers (use ref for tasks to avoid stale closures) ---

  const resolveStatusFromId = useCallback(
    (id: string | null): Task["status"] | null => {
      if (!id) return null;
      if (TASK_STATUS_ORDER.includes(id as Task["status"])) {
        return id as Task["status"];
      }
      const task = tasksRef.current.find((t) => t.id === id);
      return task?.status ?? null;
    },
    [],
  );

  const handleDragStart = useCallback(
    (event: DragStartEvent) => {
      const id = event.active.id as string;
      setActiveTaskId(id);
      const currentStatus = resolveStatusFromId(id);
      if (currentStatus) setOverContainerId(currentStatus);
    },
    [resolveStatusFromId],
  );

  const handleDragOver = useCallback(
    (event: DragOverEvent) => {
      const overId = (event.over?.id as string | null) ?? null;
      // Clear, rather than keep the last column that resolved: a column the
      // lifecycle does not reach is a disabled droppable, and `over` is null
      // over it. Holding the previous value there left the column the card
      // had crossed lit as the drop target while the pointer sat on one that
      // will refuse it, and releasing did nothing.
      setOverContainerId(resolveStatusFromId(overId));
    },
    [resolveStatusFromId],
  );

  const handleDragEnd = useCallback(
    async (event: DragEndEvent) => {
      const activeId = event.active.id as string;
      const overId = (event.over?.id as string | null) ?? null;
      const resolvedOverStatus = resolveStatusFromId(overId);

      setActiveTaskId(null);
      setOverContainerId(null);

      if (!resolvedOverStatus || !activeId) return;

      const task = tasksRef.current.find((t) => t.id === activeId);
      if (!task || task.status === resolvedOverStatus) return;

      const to = TaskHelper.getStatus(resolvedOverStatus).label;

      // A column the lifecycle does not reach from here takes no drop (see
      // `acceptsDrop`), so this is the drop that raced a change of status.
      // Said, rather than sent for the daemon to refuse.
      if (!canMoveTo(task, resolvedOverStatus)) {
        toast.error(t("Failed to update status"), {
          description: t("A task in {{from}} cannot move to {{to}}.", {
            from: TaskHelper.getStatus(task.status).label,
            to,
          }),
        });
        return;
      }

      const { error } = await client.task.setStatus.mutate({
        params: { task: activeId },
        body: { status: resolvedOverStatus },
      });

      // The card snaps back to its column on a refusal; without this the
      // drop just looked like it had not taken.
      if (error) {
        toast.error(t("Failed to update status"), { description: errorMessage(error) });
        return;
      }
      toast.success(t("Moved to {{status}}", { status: to }));
      router.invalidate();
    },
    [resolveStatusFromId, client, router],
  );

  const handleDragCancel = useCallback(() => {
    setActiveTaskId(null);
    setOverContainerId(null);
  }, []);

  // --- Build stable context values ---

  const tasksContextValue = useMemo<TasksContextValue>(
    () => ({
      tasks,
      filteredTasks,
      displayedGroupedTasks,
      statusCountByType,
      taskTypes,
      search,
      searchDraft,
      setSearchDraft,
      selectedStatuses,
      selectedPriorities,
      selectedTypes,
      selectedProjects,
      selectedGoals,
      activeFilterCount,
      hasStatusShortcut,
      currentView,
      updateSearch,
      handleViewChange,
      handleSearchChange,
      handleStatusShortcut,
      handleToggleStatus,
      handleTogglePriority,
      handleToggleType,
      handleToggleProject,
      handleToggleGoal,
      clearFilters,
      isNarrowed,
    }),
    [
      tasks,
      filteredTasks,
      displayedGroupedTasks,
      statusCountByType,
      taskTypes,
      search,
      searchDraft,
      selectedStatuses,
      selectedPriorities,
      selectedTypes,
      selectedProjects,
      selectedGoals,
      activeFilterCount,
      hasStatusShortcut,
      currentView,
      updateSearch,
      handleViewChange,
      handleSearchChange,
      handleStatusShortcut,
      handleToggleStatus,
      handleTogglePriority,
      handleToggleType,
      handleToggleProject,
      handleToggleGoal,
      clearFilters,
      isNarrowed,
    ],
  );

  const dragContextValue = useMemo<DragContextValue>(
    () => ({
      activeTaskId,
      activeTask,
      overContainerId,
      isDragActive,
      activeDropStatus: overContainerId,
      acceptsDrop,
      handleDragStart,
      handleDragOver,
      handleDragEnd,
      handleDragCancel,
    }),
    [
      activeTaskId,
      activeTask,
      overContainerId,
      isDragActive,
      acceptsDrop,
      handleDragStart,
      handleDragOver,
      handleDragEnd,
      handleDragCancel,
    ],
  );

  return (
    <TasksContext.Provider value={tasksContextValue}>
      <DragContext.Provider value={dragContextValue}>
        {children}
      </DragContext.Provider>
    </TasksContext.Provider>
  );
}
