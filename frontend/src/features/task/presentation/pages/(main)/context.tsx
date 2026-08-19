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
import type {
  FractalTask,
  FractalTaskPriority,
} from "@/features/task/interfaces/task.interfaces";
import { TaskHelper } from "../../helpers/task.helper";
import { TASK_STATUS_ORDER } from "../../consts/task";
import { useTasksStatusTransition } from "../../hooks/tasks-status-transition.hook";
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
  activeTask: FractalTask | null;
  overContainerId: FractalTask["status"] | null;
  isDragActive: boolean;
  activeDropStatus: FractalTask["status"] | null;
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
  tasks: FractalTask[];
  filteredTasks: FractalTask[];
  displayedGroupedTasks: Record<FractalTask["status"], FractalTask[]>;
  statusCountByType: Record<FractalTask["status"], number>;
  taskTypes: string[];

  // Search state
  search: TasksPageSearchSchema;
  searchDraft: string;

  // Selected filters
  selectedStatuses: FractalTask["status"][];
  selectedPriorities: FractalTaskPriority[];
  selectedTypes: string[];
  selectedProjects: string[];
  selectedGoals: string[];

  // UI state
  activeFilterCount: number;
  hasStatusShortcut: "all" | FractalTask["status"];
  currentView: "list" | "kanban";

  // Actions (stable refs)
  updateSearch: (next: Partial<TasksPageSearchSchema>) => void;
  handleViewChange: (view: "list" | "kanban") => void;
  handleSearchChange: (value: string) => void;
  handleStatusShortcut: (status: "all" | FractalTask["status"]) => void;
  handleToggleStatus: (status: FractalTask["status"]) => void;
  handleTogglePriority: (priority: FractalTaskPriority) => void;
  handleToggleType: (type: string) => void;
  handleToggleProject: (project: string) => void;
  handleToggleGoal: (goal: string) => void;
  clearFilters: () => void;
  setSearchDraft: (value: string) => void;

  // Finish dialog
  finishTransition: ReturnType<typeof useTasksStatusTransition>;
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
  tasks: FractalTask[];
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
    FractalTask["status"] | null
  >(null);
  const finishTransition = useTasksStatusTransition();

  // Keep a stable ref to tasks for drag handlers so they don't stale-close
  const tasksRef = useRef(tasks);
  tasksRef.current = tasks;

  useEffect(() => {
    setSearchDraft(search.query ?? "");
  }, [search.query]);

  // --- Parsed filters (memoized) ---

  const selectedStatuses = useMemo(
    () => parseMultiValue(search.status) as FractalTask["status"][],
    [search.status],
  );
  const selectedPriorities = useMemo(
    () => parseMultiValue(search.priority) as FractalTaskPriority[],
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
    () =>
      Array.from(new Set(tasks.map((task) => task.type))).sort((a, b) =>
        a.localeCompare(b),
      ),
    [tasks],
  );

  const statusCountByType = useMemo(() => {
    return TASK_STATUS_ORDER.reduce(
      (acc, status) => {
        acc[status] = tasks.filter((task) => task.status === status).length;
        return acc;
      },
      {} as Record<FractalTask["status"], number>,
    );
  }, [tasks]);

  const filteredTasks = useMemo(() => {
    if (selectedStatuses.length === 0) return tasks;
    return tasks.filter((task) => selectedStatuses.includes(task.status));
  }, [tasks, selectedStatuses]);

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

  const hasStatusShortcut: "all" | FractalTask["status"] =
    selectedStatuses.length === 1 ? selectedStatuses[0] : "all";

  const currentView = search.view === "kanban" ? "kanban" : "list";
  const isDragActive = activeTaskId !== null;

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
    (status: "all" | FractalTask["status"]) => {
      updateSearch({
        status: status === "all" ? undefined : serializeMultiValue([status]),
      });
    },
    [updateSearch],
  );

  const handleToggleStatus = useCallback(
    (status: FractalTask["status"]) => {
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
    (priority: FractalTaskPriority) => {
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
    (id: string | null): FractalTask["status"] | null => {
      if (!id) return null;
      if (TASK_STATUS_ORDER.includes(id as FractalTask["status"])) {
        return id as FractalTask["status"];
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
      const resolved = resolveStatusFromId(overId);
      if (resolved) setOverContainerId(resolved);
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

      if (resolvedOverStatus === "finished") {
        finishTransition.open(task, resolvedOverStatus);
        return;
      }

      const { error } = await client.task.setStatus.mutate({
        params: { id: activeId },
        body: { status: resolvedOverStatus },
      });

      if (!error) router.invalidate();
    },
    [resolveStatusFromId, client, router, finishTransition],
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
      finishTransition,
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
      finishTransition,
    ],
  );

  const dragContextValue = useMemo<DragContextValue>(
    () => ({
      activeTaskId,
      activeTask,
      overContainerId,
      isDragActive,
      activeDropStatus: overContainerId,
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
