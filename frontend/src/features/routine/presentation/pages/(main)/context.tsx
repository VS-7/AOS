import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { useNavigate, useRouter } from "@tanstack/react-router";
import type { FractalRoutine } from "@/features/routine/interfaces/routine.interfaces";
import { FractalRoutineHelper } from "@/features/routine/presentation/helpers/routine.helper";

interface RoutinesPageSearchSchema {
  query?: string;
  status?: string;
  agent?: string;
  type?: string;
}

interface RoutinesContextValue {
  routines: FractalRoutine[];
  filteredRoutines: FractalRoutine[];
  displayedGroupedRoutines: Record<FractalRoutine["status"], FractalRoutine[]>;
  search: RoutinesPageSearchSchema;
  searchDraft: string;
  selectedStatuses: FractalRoutine["status"][];
  selectedAgents: string[];
  selectedTypes: string[];
  agentOptions: string[];
  activeFilterCount: number;
  updateSearch: (next: Partial<RoutinesPageSearchSchema>) => void;
  handleSearchChange: (value: string) => void;
  handleToggleStatus: (status: FractalRoutine["status"]) => void;
  handleToggleAgent: (agent: string) => void;
  handleToggleType: (type: string) => void;
  clearFilters: () => void;
}

const RoutinesContext = createContext<RoutinesContextValue | null>(null);

export function useRoutinesContext() {
  const ctx = useContext(RoutinesContext);
  if (!ctx) {
    throw new Error(
      "useRoutinesContext must be used within a RoutinesProvider",
    );
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

interface RoutinesProviderProps {
  children: React.ReactNode;
  routines: FractalRoutine[];
  search: RoutinesPageSearchSchema;
}

export function RoutinesProvider({
  children,
  routines,
  search,
}: RoutinesProviderProps) {
  const navigate = useNavigate();
  const router = useRouter();
  const [searchDraft, setSearchDraft] = useState(search.query ?? "");

  useEffect(() => {
    setSearchDraft(search.query ?? "");
  }, [search.query]);

  const selectedStatuses = useMemo(
    () => parseMultiValue(search.status) as FractalRoutine["status"][],
    [search.status],
  );

  const selectedAgents = useMemo(
    () => parseMultiValue(search.agent),
    [search.agent],
  );

  const selectedTypes = useMemo(
    () => parseMultiValue(search.type),
    [search.type],
  );

  const agentOptions = useMemo(
    () =>
      Array.from(new Set(routines.map((routine) => routine.agent))).sort(
        (a, b) => a.localeCompare(b),
      ),
    [routines],
  );

  const filteredRoutines = useMemo(() => {
    let result = routines;

    if (selectedStatuses.length > 0) {
      result = result.filter((routine) =>
        selectedStatuses.includes(routine.status),
      );
    }

    if (selectedAgents.length > 0) {
      result = result.filter((routine) =>
        selectedAgents.includes(routine.agent),
      );
    }

    if (selectedTypes.length > 0) {
      result = result.filter((routine) =>
        routine.triggers.some((trigger) =>
          selectedTypes.includes(trigger.type),
        ),
      );
    }

    if (searchDraft.trim()) {
      const q = searchDraft.toLowerCase();
      result = result.filter(
        (routine) =>
          routine.name.toLowerCase().includes(q) ||
          routine.id.toLowerCase().includes(q) ||
          routine.content.toLowerCase().includes(q),
      );
    }

    return result;
  }, [
    routines,
    selectedStatuses,
    selectedAgents,
    selectedTypes,
    searchDraft,
  ]);

  const displayedGroupedRoutines = useMemo(
    () => FractalRoutineHelper.groupByStatus(filteredRoutines),
    [filteredRoutines],
  );

  const activeFilterCount =
    selectedStatuses.length + selectedAgents.length + selectedTypes.length;

  const updateSearch = useCallback(
    (next: Partial<RoutinesPageSearchSchema>) => {
      navigate({
        to: "/routines",
        search: (prev: Partial<RoutinesPageSearchSchema>) => ({
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
    (status: FractalRoutine["status"]) => {
      updateSearch({
        status: serializeMultiValue(
          toggleFilterValue(selectedStatuses, status),
        ),
      });
    },
    [updateSearch, selectedStatuses],
  );

  const handleToggleAgent = useCallback(
    (agent: string) => {
      updateSearch({
        agent: serializeMultiValue(toggleFilterValue(selectedAgents, agent)),
      });
    },
    [updateSearch, selectedAgents],
  );

  const handleToggleType = useCallback(
    (type: string) => {
      updateSearch({
        type: serializeMultiValue(toggleFilterValue(selectedTypes, type)),
      });
    },
    [updateSearch, selectedTypes],
  );

  const clearFilters = useCallback(() => {
    updateSearch({
      status: undefined,
      agent: undefined,
      type: undefined,
      query: undefined,
    });
    setSearchDraft("");
  }, [updateSearch]);

  const value: RoutinesContextValue = {
    routines,
    filteredRoutines,
    displayedGroupedRoutines,
    search,
    searchDraft,
    selectedStatuses,
    selectedAgents,
    selectedTypes,
    agentOptions,
    activeFilterCount,
    updateSearch,
    handleSearchChange,
    handleToggleStatus,
    handleToggleAgent,
    handleToggleType,
    clearFilters,
  };

  return (
    <RoutinesContext.Provider value={value}>
      {children}
    </RoutinesContext.Provider>
  );
}
