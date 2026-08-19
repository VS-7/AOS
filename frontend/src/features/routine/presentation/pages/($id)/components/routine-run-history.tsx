import * as React from "react";
import {
  Clock,
  Ellipsis,
  ListFilter,
  PlayIcon,
  Search,
  X,
} from "lucide-react";
import { AnimatePresence, motion } from "motion/react";

import { AnimatedEmptyState } from "@/components/ui/animated-empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DotmSquare4 } from "@/components/ui/dotm-square-4";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from "@/components/ui/input-group";
import { cn } from "@/lib/utils";
import { springs } from "@/lib/springs";
import { openChatTab } from "@/features/chat/presentation/helpers/open-chat-tab.helper";
import type { FractalRun } from "@/features/routine/interfaces/routine.interfaces";

type RunTriggerFilter = NonNullable<FractalRun["trigger"]>;
type RunStatusFilter = FractalRun["status"];

export interface RoutineRunHistoryFilters {
  searchOpen: boolean;
  searchQuery: string;
  selectedTriggers: RunTriggerFilter[];
  selectedStatuses: RunStatusFilter[];
  activeFilterCount: number;
  openSearch: () => void;
  closeSearch: () => void;
  setSearchQuery: (value: string) => void;
  toggleTrigger: (value: RunTriggerFilter) => void;
  toggleStatus: (value: RunStatusFilter) => void;
  clearFilters: () => void;
  searchInputRef: React.RefObject<HTMLInputElement | null>;
}

interface RoutineRunHistoryProps {
  runs: FractalRun[];
  filters: RoutineRunHistoryFilters;
  routineName?: string;
  onRunNow?: () => void;
  isFiring?: boolean;
}

const TRIGGER_FILTER_OPTIONS: Array<{
  value: RunTriggerFilter;
  label: string;
}> = [
  { value: "manual", label: "Manual" },
  { value: "scheduled", label: "Schedule" },
  { value: "webhook", label: "Webhook" },
  { value: "activity", label: "Activity" },
];

const STATUS_FILTER_OPTIONS: Array<{
  value: RunStatusFilter;
  label: string;
}> = [
  { value: "pending", label: "Pending" },
  { value: "running", label: "Running" },
  { value: "completed", label: "Succeeded" },
  { value: "error", label: "Failed" },
];

function formatTriggeredAt(iso: string) {
  return new Date(iso).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

function formatDuration(run: FractalRun) {
  if (!run.finishedAt) return "—";

  const ms =
    new Date(run.finishedAt).getTime() - new Date(run.startedAt).getTime();
  if (!Number.isFinite(ms) || ms < 0) return "—";
  if (ms < 60_000) return "< 1m";

  const minutes = Math.round(ms / 60_000);
  if (minutes < 60) return `${minutes}m`;

  const hours = Math.floor(minutes / 60);
  const rem = minutes % 60;
  return rem === 0 ? `${hours}h` : `${hours}h ${rem}m`;
}

function triggerLabel(run: FractalRun) {
  switch (run.trigger) {
    case "manual":
      return "Manual";
    case "scheduled":
      return "Schedule";
    case "webhook":
      return "Webhook";
    case "activity":
      return "Activity";
    default:
      return "Run";
  }
}

function countRunsInWindow(
  runs: FractalRun[],
  status: FractalRun["status"],
  windowMs: number,
) {
  const cutoff = Date.now() - windowMs;
  return runs.filter(
    (run) =>
      run.status === status &&
      new Date(run.finishedAt ?? run.startedAt).getTime() >= cutoff,
  ).length;
}

function RunStatusBadge({ status }: { status: FractalRun["status"] }) {
  if (status === "running" || status === "pending") {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
        <DotmSquare4 size={14} className="text-foreground" />
        {status === "pending" ? "Pending" : "Running"}
      </span>
    );
  }

  if (status === "completed") {
    return (
      <Badge
        variant="secondary"
        className="border-transparent bg-emerald-500/15 text-emerald-600 dark:text-emerald-400"
      >
        Succeeded
      </Badge>
    );
  }

  return (
    <Badge variant="destructive" className="border-transparent">
      Failed
    </Badge>
  );
}

function RunHistoryEmptyState({
  onRunNow,
  isFiring,
}: {
  onRunNow?: () => void;
  isFiring?: boolean;
}) {
  return (
    <AnimatedEmptyState className="rounded-none border-0 shadow-none md:min-h-[280px]">
      <AnimatedEmptyState.Carousel>
        <div className="flex w-full items-center gap-3">
          <div className="flex size-8 shrink-0 items-center justify-center rounded-md bg-muted/60">
            <Clock className="size-3.5 text-muted-foreground" />
          </div>
          <div className="min-w-0 flex-1 space-y-1.5">
            <div className="h-2 w-20 rounded-md bg-muted" />
            <div className="h-2 w-28 rounded-md bg-muted/50" />
          </div>
          <div className="h-5 w-14 rounded-full bg-emerald-500/15" />
        </div>
      </AnimatedEmptyState.Carousel>
      <AnimatedEmptyState.Content>
        <AnimatedEmptyState.Title>No runs yet</AnimatedEmptyState.Title>
        <AnimatedEmptyState.Description>
          Fire this routine manually or wait for a trigger to start building
          history.
        </AnimatedEmptyState.Description>
      </AnimatedEmptyState.Content>
      {onRunNow ? (
        <AnimatedEmptyState.Actions>
          <AnimatedEmptyState.Action
            type="button"
            size="sm"
            disabled={isFiring}
            onClick={onRunNow}
          >
            <PlayIcon data-icon="inline-start" />
            {isFiring ? "Running..." : "Run now"}
          </AnimatedEmptyState.Action>
        </AnimatedEmptyState.Actions>
      ) : null}
    </AnimatedEmptyState>
  );
}

/**
 * Shared filter/search state for the Run History tab and its trailing toolbar.
 */
export function useRoutineRunHistoryFilters(): RoutineRunHistoryFilters {
  const searchInputRef = React.useRef<HTMLInputElement>(null);
  const [searchOpen, setSearchOpen] = React.useState(false);
  const [searchQuery, setSearchQuery] = React.useState("");
  const [selectedTriggers, setSelectedTriggers] = React.useState<
    RunTriggerFilter[]
  >([]);
  const [selectedStatuses, setSelectedStatuses] = React.useState<
    RunStatusFilter[]
  >([]);

  React.useEffect(() => {
    if (!searchOpen) return;
    searchInputRef.current?.focus();
  }, [searchOpen]);

  function openSearch() {
    setSearchOpen(true);
  }

  function closeSearch() {
    setSearchOpen(false);
    setSearchQuery("");
  }

  function toggleTrigger(value: RunTriggerFilter) {
    setSelectedTriggers((current) =>
      current.includes(value)
        ? current.filter((item) => item !== value)
        : [...current, value],
    );
  }

  function toggleStatus(value: RunStatusFilter) {
    setSelectedStatuses((current) =>
      current.includes(value)
        ? current.filter((item) => item !== value)
        : [...current, value],
    );
  }

  function clearFilters() {
    setSelectedTriggers([]);
    setSelectedStatuses([]);
  }

  return {
    searchOpen,
    searchQuery,
    selectedTriggers,
    selectedStatuses,
    activeFilterCount: selectedTriggers.length + selectedStatuses.length,
    openSearch,
    closeSearch,
    setSearchQuery,
    toggleTrigger,
    toggleStatus,
    clearFilters,
    searchInputRef,
  };
}

/**
 * Trailing actions for the Run History tab row (search collapse + filter).
 */
export function RoutineRunHistoryToolbar({
  filters,
}: {
  filters: RoutineRunHistoryFilters;
}) {
  const {
    searchOpen,
    searchQuery,
    selectedTriggers,
    selectedStatuses,
    activeFilterCount,
    openSearch,
    closeSearch,
    setSearchQuery,
    toggleTrigger,
    toggleStatus,
    clearFilters,
    searchInputRef,
  } = filters;

  return (
    <div className="flex shrink-0 items-center justify-end gap-0.5">
      <AnimatePresence initial={false} mode="popLayout">
        {searchOpen ? (
          <motion.div
            key="search-field"
            initial={{ width: 32, opacity: 0 }}
            animate={{ width: 220, opacity: 1 }}
            exit={{ width: 32, opacity: 0 }}
            transition={springs.fast}
            className="overflow-hidden"
          >
            <InputGroup className="h-8 border-0 bg-transparent shadow-none ring-0 has-[[data-slot=input-group-control]:focus-visible]:bg-transparent has-[[data-slot=input-group-control]:focus-visible]:ring-0">
              <InputGroupAddon align="inline-start" className="pl-0">
                <Search className="size-4" />
              </InputGroupAddon>
              <InputGroupInput
                ref={searchInputRef}
                value={searchQuery}
                onChange={(event) => setSearchQuery(event.target.value)}
                placeholder="Search runs..."
                className="px-0"
                onKeyDown={(event) => {
                  if (event.key === "Escape") closeSearch();
                }}
              />
              <InputGroupAddon align="inline-end" className="pr-0">
                <button
                  type="button"
                  onClick={closeSearch}
                  aria-label="Close search"
                  className="flex size-5 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                >
                  <X className="size-3.5" />
                </button>
              </InputGroupAddon>
            </InputGroup>
          </motion.div>
        ) : (
          <motion.div
            key="search-button"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.08 }}
          >
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label="Search runs"
              onClick={openSearch}
            >
              <Search className="size-4" />
            </Button>
          </motion.div>
        )}
      </AnimatePresence>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label="Filter runs"
            className={cn(activeFilterCount > 0 && "text-foreground")}
          >
            <ListFilter className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-52">
          <DropdownMenuLabel>Trigger</DropdownMenuLabel>
          {TRIGGER_FILTER_OPTIONS.map((option) => (
            <DropdownMenuCheckboxItem
              key={option.value}
              checked={selectedTriggers.includes(option.value)}
              onCheckedChange={() => toggleTrigger(option.value)}
            >
              {option.label}
            </DropdownMenuCheckboxItem>
          ))}
          <DropdownMenuSeparator />
          <DropdownMenuLabel>Status</DropdownMenuLabel>
          {STATUS_FILTER_OPTIONS.map((option) => (
            <DropdownMenuCheckboxItem
              key={option.value}
              checked={selectedStatuses.includes(option.value)}
              onCheckedChange={() => toggleStatus(option.value)}
            >
              {option.label}
            </DropdownMenuCheckboxItem>
          ))}
          {activeFilterCount > 0 ? (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={clearFilters}>
                Clear filters
              </DropdownMenuItem>
            </>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

export function RoutineRunHistory({
  runs,
  filters,
  routineName,
  onRunNow,
  isFiring,
}: RoutineRunHistoryProps) {
  const dayMs = 24 * 60 * 60 * 1000;
  const weekMs = 7 * dayMs;

  const stats = [
    {
      label: "Successful · 24h",
      value: countRunsInWindow(runs, "completed", dayMs),
    },
    {
      label: "Failed · 24h",
      value: countRunsInWindow(runs, "error", dayMs),
    },
    {
      label: "Successful · 7d",
      value: countRunsInWindow(runs, "completed", weekMs),
    },
    {
      label: "Failed · 7d",
      value: countRunsInWindow(runs, "error", weekMs),
    },
  ];

  const filteredRuns = runs.filter((run) => {
    if (
      filters.selectedStatuses.length > 0 &&
      !filters.selectedStatuses.includes(run.status)
    ) {
      return false;
    }

    if (filters.selectedTriggers.length > 0) {
      if (!run.trigger || !filters.selectedTriggers.includes(run.trigger)) {
        return false;
      }
    }

    const query = filters.searchQuery.trim().toLowerCase();
    if (!query) return true;

    return (
      run.id.toLowerCase().includes(query) ||
      triggerLabel(run).toLowerCase().includes(query) ||
      run.status.toLowerCase().includes(query)
    );
  });

  function handleOpenRun(run: FractalRun) {
    openChatTab({
      chatId: run.id,
      title: routineName ? `${routineName} · run` : `Run ${run.id.slice(0, 8)}`,
    });
  }

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        {stats.map((stat) => (
          <div
            key={stat.label}
            className="rounded-lg border bg-muted/30 px-3 py-2.5"
          >
            <div className="text-[11px] text-muted-foreground">{stat.label}</div>
            <div className="mt-1 text-2xl font-semibold tracking-tight tabular-nums">
              {stat.value}
            </div>
          </div>
        ))}
      </div>

      <div className="overflow-hidden rounded-lg border">
        <div className="grid grid-cols-[minmax(0,1.4fr)_minmax(0,1.1fr)_minmax(0,0.9fr)_minmax(0,0.6fr)_auto] gap-2 border-b bg-muted/20 px-3 py-2 text-[11px] font-medium text-muted-foreground">
          <span>Trigger</span>
          <span>Triggered</span>
          <span>Status</span>
          <span>Duration</span>
          <span className="w-8" />
        </div>

        {runs.length === 0 ? (
          <RunHistoryEmptyState onRunNow={onRunNow} isFiring={isFiring} />
        ) : filteredRuns.length === 0 ? (
          <div className="px-3 py-8 text-center text-sm text-muted-foreground">
            No runs match the current search or filters.
          </div>
        ) : (
          <div className="divide-y">
            {filteredRuns.map((run) => (
              <button
                key={run.id}
                type="button"
                onClick={() => handleOpenRun(run)}
                className={cn(
                  "grid w-full grid-cols-[minmax(0,1.4fr)_minmax(0,1.1fr)_minmax(0,0.9fr)_minmax(0,0.6fr)_auto] gap-2 px-3 py-2.5 text-left text-sm transition-colors",
                  "hover:bg-muted/40",
                )}
              >
                <span className="flex min-w-0 items-center gap-2">
                  {run.status === "running" || run.status === "pending" ? (
                    <DotmSquare4
                      size={14}
                      className="shrink-0 text-foreground"
                    />
                  ) : (
                    <Clock className="size-3.5 shrink-0 text-muted-foreground" />
                  )}
                  <span className="truncate">{triggerLabel(run)}</span>
                </span>
                <span className="truncate text-muted-foreground">
                  {formatTriggeredAt(run.startedAt)}
                </span>
                <span className="flex items-center">
                  <RunStatusBadge status={run.status} />
                </span>
                <span className="tabular-nums text-muted-foreground">
                  {formatDuration(run)}
                </span>
                <span
                  className="flex w-8 items-center justify-end"
                  onClick={(event) => event.stopPropagation()}
                >
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className="text-muted-foreground"
                    aria-label="Run actions"
                  >
                    <Ellipsis className="size-4" />
                  </Button>
                </span>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
