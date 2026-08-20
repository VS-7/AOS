import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { useNavigate, useRouter } from "@tanstack/react-router";
import type {
  Goal,
  GoalPriority,
} from "@/features/goal/interfaces/goal.interfaces";
import { GoalHelper } from "../../helpers/goal.helper";

interface GoalsPageSearchSchema {
  query?: string;
  status?: string;
  priority?: string;
  project?: string;
}

interface GoalsContextValue {
  goals: Goal[];
  filteredGoals: Goal[];
  displayedGroupedGoals: Record<Goal["status"], Goal[]>;
  search: GoalsPageSearchSchema;
  searchDraft: string;
  selectedStatuses: Goal["status"][];
  selectedPriorities: GoalPriority[];
  selectedProjects: string[];
  activeFilterCount: number;
  updateSearch: (next: Partial<GoalsPageSearchSchema>) => void;
  handleSearchChange: (value: string) => void;
  handleToggleStatus: (status: Goal["status"]) => void;
  handleTogglePriority: (priority: GoalPriority) => void;
  handleToggleProject: (project: string) => void;
  clearFilters: () => void;
}

const GoalsContext = createContext<GoalsContextValue | null>(null);

export function useGoalsContext() {
  const ctx = useContext(GoalsContext);
  if (!ctx) {
    throw new Error("useGoalsContext must be used within a GoalsProvider");
  }
  return ctx;
}

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

interface GoalsProviderProps {
  children: React.ReactNode;
  goals: Goal[];
  search: GoalsPageSearchSchema;
}

export function GoalsProvider({ children, goals, search }: GoalsProviderProps) {
  const navigate = useNavigate();
  const router = useRouter();
  const [searchDraft, setSearchDraft] = useState(search.query ?? "");

  useEffect(() => {
    setSearchDraft(search.query ?? "");
  }, [search.query]);

  const selectedStatuses = useMemo(
    () => parseMultiValue(search.status) as Goal["status"][],
    [search.status],
  );

  const selectedPriorities = useMemo(
    () => parseMultiValue(search.priority) as GoalPriority[],
    [search.priority],
  );

  const selectedProjects = useMemo(
    () => parseMultiValue(search.project),
    [search.project],
  );

  const filteredGoals = useMemo(() => {
    let result = goals;

    if (selectedStatuses.length > 0) {
      result = result.filter((g) => selectedStatuses.includes(g.status));
    }

    if (selectedPriorities.length > 0) {
      result = result.filter((g) => selectedPriorities.includes(g.priority));
    }

    if (selectedProjects.length > 0) {
      result = result.filter(
        (g) => g.project != null && selectedProjects.includes(g.project),
      );
    }

    if (searchDraft.trim()) {
      const q = searchDraft.toLowerCase();
      result = result.filter(
        (g) =>
          g.title.toLowerCase().includes(q) ||
          g.id.toLowerCase().includes(q) ||
          (g.description?.toLowerCase().includes(q) ?? false),
      );
    }

    return result;
  }, [
    goals,
    selectedStatuses,
    selectedPriorities,
    selectedProjects,
    searchDraft,
  ]);

  const displayedGroupedGoals = useMemo(
    () => GoalHelper.groupByStatus(filteredGoals),
    [filteredGoals],
  );

  const activeFilterCount =
    selectedStatuses.length +
    selectedPriorities.length +
    selectedProjects.length;

  const updateSearch = useCallback(
    (next: Partial<GoalsPageSearchSchema>) => {
      navigate({
        to: "/goals",
        search: (prev: Partial<GoalsPageSearchSchema>) => ({
          ...prev,
          ...next,
        }),
      });
      router.invalidate();
    },
    [navigate, router],
  );

  const handleSearchChange = useCallback(
    (value: string) => {
      setSearchDraft(value);
      updateSearch({ query: value.trim() ? value : undefined });
    },
    [updateSearch],
  );

  const handleToggleStatus = useCallback(
    (status: Goal["status"]) => {
      updateSearch({
        status: serializeMultiValue(
          toggleFilterValue(selectedStatuses, status),
        ),
      });
    },
    [updateSearch, selectedStatuses],
  );

  const handleTogglePriority = useCallback(
    (priority: GoalPriority) => {
      updateSearch({
        priority: serializeMultiValue(
          toggleFilterValue(selectedPriorities, priority),
        ),
      });
    },
    [updateSearch, selectedPriorities],
  );

  const handleToggleProject = useCallback(
    (project: string) => {
      updateSearch({
        project: serializeMultiValue(
          toggleFilterValue(selectedProjects, project),
        ),
      });
    },
    [updateSearch, selectedProjects],
  );

  const clearFilters = useCallback(() => {
    updateSearch({
      status: undefined,
      priority: undefined,
      project: undefined,
      query: undefined,
    });
    setSearchDraft("");
  }, [updateSearch]);

  const value: GoalsContextValue = {
    goals,
    filteredGoals,
    displayedGroupedGoals,
    search,
    searchDraft,
    selectedStatuses,
    selectedPriorities,
    selectedProjects,
    activeFilterCount,
    updateSearch,
    handleSearchChange,
    handleToggleStatus,
    handleTogglePriority,
    handleToggleProject,
    clearFilters,
  };

  return (
    <GoalsContext.Provider value={value}>{children}</GoalsContext.Provider>
  );
}
