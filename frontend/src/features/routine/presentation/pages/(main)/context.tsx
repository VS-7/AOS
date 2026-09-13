import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { useNavigate } from "@tanstack/react-router";
import type { ActivityEventDefinition } from "@/features/activity/interfaces/activity.interfaces";
import type {
  Routine,
  RoutineStatus,
} from "@/features/routine/interfaces/routine.interfaces";
import { ROUTINE_STATUS_ORDER } from "@/features/routine/presentation/consts/routine";
import { RoutineHelper } from "@/features/routine/presentation/helpers/routine.helper";

interface RoutinesPageSearchSchema {
  query?: string;
  status?: string;
  agent?: string;
  type?: string;
}

interface RoutinesContextValue {
  routines: Routine[];
  activityEvents: ActivityEventDefinition[];
  filteredRoutines: Routine[];
  displayedGroupedRoutines: Record<RoutineStatus, Routine[]>;
  search: RoutinesPageSearchSchema;
  searchDraft: string;
  selectedStatuses: RoutineStatus[];
  selectedAgents: string[];
  selectedTypes: string[];
  agentOptions: string[];
  activeFilterCount: number;
  updateSearch: (next: Partial<RoutinesPageSearchSchema>) => void;
  handleSearchChange: (value: string) => void;
  handleToggleStatus: (status: RoutineStatus) => void;
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
  routines: Routine[];
  activityEvents: ActivityEventDefinition[];
  search: RoutinesPageSearchSchema;
}

/** How long typing pauses before the search is written into the URL. */
const SEARCH_URL_DELAY_MS = 300;

export function RoutinesProvider({
  children,
  routines,
  activityEvents,
  search,
}: RoutinesProviderProps) {
  const navigate = useNavigate();
  const [searchDraft, setSearchDraft] = useState(search.query ?? "");
  // What this page last wrote into the URL. The box follows the URL only when
  // something else changed it (a link, Back), never when the URL is catching
  // up with typing — that would put back an older value over a newer one.
  const writtenQuery = React.useRef(search.query ?? "");

  useEffect(() => {
    const incoming = search.query ?? "";
    if (incoming === writtenQuery.current) return;
    writtenQuery.current = incoming;
    setSearchDraft(incoming);
  }, [search.query]);

  // Only statuses Go has: a stale `?status=paused` link filters nothing out
  // rather than emptying the page.
  const selectedStatuses = useMemo(
    () =>
      parseMultiValue(search.status).filter((status): status is RoutineStatus =>
        (ROUTINE_STATUS_ORDER as string[]).includes(status),
      ),
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

    // Name and id, what the rows show. The list does not load prompts
    // (`routines_list` leaves `content` out), and reading one threw on every
    // keystroke that matched no name — which remounted the list and took the
    // focus out of the search box.
    const q = searchDraft.trim().toLowerCase();
    if (q) {
      result = result.filter(
        (routine) =>
          routine.name.toLowerCase().includes(q) ||
          routine.id.toLowerCase().includes(q),
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
    () => RoutineHelper.groupByStatus(filteredRoutines),
    [filteredRoutines],
  );

  const activeFilterCount =
    selectedStatuses.length + selectedAgents.length + selectedTypes.length;

  // The URL only remembers the filters; the list is already loaded, so
  // nothing is re-fetched when they change.
  const updateSearch = useCallback(
    (next: Partial<RoutinesPageSearchSchema>) => {
      void navigate({
        to: "/routines",
        replace: true,
        search: (prev: Partial<RoutinesPageSearchSchema>) => ({
          ...prev,
          ...next,
        }),
      });
    },
    [navigate],
  );

  const searchTimer = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => () => {
    if (searchTimer.current) clearTimeout(searchTimer.current);
  }, []);

  const handleSearchChange = useCallback(
    (value: string) => {
      setSearchDraft(value);
      if (searchTimer.current) clearTimeout(searchTimer.current);
      searchTimer.current = setTimeout(() => {
        writtenQuery.current = value.trim() ? value : "";
        updateSearch({ query: value.trim() ? value : undefined });
      }, SEARCH_URL_DELAY_MS);
    },
    [updateSearch],
  );

  const handleToggleStatus = useCallback(
    (status: RoutineStatus) => {
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
    if (searchTimer.current) clearTimeout(searchTimer.current);
    writtenQuery.current = "";
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
    activityEvents,
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
