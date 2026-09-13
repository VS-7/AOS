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
import type { Run, RunTrigger } from "@/features/routine/interfaces/routine.interfaces";
import {
  RoutineRunHelper,
  type RunOutcome,
} from "@/features/routine/presentation/helpers/routine-run.helper";
import { t } from "@/lib/i18n";

type RunTriggerFilter = RunTrigger;
type RunStatusFilter = RunOutcome;

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
  runs: Run[];
  filters: RoutineRunHistoryFilters;
  routineName?: string;
  onRunNow?: () => void;
  isFiring?: boolean;
}

const TRIGGER_FILTER_OPTIONS: RunTriggerFilter[] = [
  "manual",
  "scheduled",
  "webhook",
  "activity",
];

const STATUS_FILTER_OPTIONS: RunStatusFilter[] = [
  "running",
  "succeeded",
  "failed",
  "skipped",
];

function RunStatusBadge({ status }: { status: Run["status"] }) {
  const outcome = RoutineRunHelper.outcome(status);

  if (outcome === "running") {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
        <DotmSquare4 size={14} className="text-foreground" />
        {RoutineRunHelper.outcomeLabel(outcome)}
      </span>
    );
  }

  if (outcome === "succeeded") {
    return (
      <Badge
        variant="secondary"
        className="border-transparent bg-emerald-500/15 text-emerald-600 dark:text-emerald-400"
      >
        {RoutineRunHelper.outcomeLabel(outcome)}
      </Badge>
    );
  }

  if (outcome === "skipped") {
    return (
      <Badge variant="secondary" className="border-transparent">
        {RoutineRunHelper.outcomeLabel(outcome)}
      </Badge>
    );
  }

  return (
    <Badge variant="destructive" className="border-transparent">
      {status === "timed_out" ? t("Timed out") : RoutineRunHelper.outcomeLabel(outcome)}
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
        <AnimatedEmptyState.Title>{t("No runs yet")}</AnimatedEmptyState.Title>
        <AnimatedEmptyState.Description>
          {t("Fire this routine manually or wait for a trigger to start building history.")}
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
            {isFiring ? t("Running...") : t("Run now")}
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
                placeholder={t("Search runs...")}
                className="px-0"
                onKeyDown={(event) => {
                  if (event.key === "Escape") closeSearch();
                }}
              />
              <InputGroupAddon align="inline-end" className="pr-0">
                <button
                  type="button"
                  onClick={closeSearch}
                  aria-label={t("Close search")}
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
              aria-label={t("Search runs")}
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
            aria-label={t("Filter runs")}
            className={cn(activeFilterCount > 0 && "text-foreground")}
          >
            <ListFilter className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-52">
          <DropdownMenuLabel>{t("Trigger")}</DropdownMenuLabel>
          {TRIGGER_FILTER_OPTIONS.map((option) => (
            <DropdownMenuCheckboxItem
              key={option}
              checked={selectedTriggers.includes(option)}
              onCheckedChange={() => toggleTrigger(option)}
            >
              {RoutineRunHelper.triggerLabel(option)}
            </DropdownMenuCheckboxItem>
          ))}
          <DropdownMenuSeparator />
          <DropdownMenuLabel>{t("Status")}</DropdownMenuLabel>
          {STATUS_FILTER_OPTIONS.map((option) => (
            <DropdownMenuCheckboxItem
              key={option}
              checked={selectedStatuses.includes(option)}
              onCheckedChange={() => toggleStatus(option)}
            >
              {RoutineRunHelper.outcomeLabel(option)}
            </DropdownMenuCheckboxItem>
          ))}
          {activeFilterCount > 0 ? (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={clearFilters}>
                {t("Clear filters")}
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
      label: t("Successful · 24h"),
      value: RoutineRunHelper.countInWindow(runs, "succeeded", dayMs),
    },
    {
      label: t("Failed · 24h"),
      value: RoutineRunHelper.countInWindow(runs, "failed", dayMs),
    },
    {
      label: t("Successful · 7d"),
      value: RoutineRunHelper.countInWindow(runs, "succeeded", weekMs),
    },
    {
      label: t("Failed · 7d"),
      value: RoutineRunHelper.countInWindow(runs, "failed", weekMs),
    },
  ];

  const filteredRuns = runs.filter((run) => {
    if (
      filters.selectedStatuses.length > 0 &&
      !filters.selectedStatuses.includes(RoutineRunHelper.outcome(run.status))
    ) {
      return false;
    }

    if (
      filters.selectedTriggers.length > 0 &&
      !filters.selectedTriggers.includes(run.trigger as RunTrigger)
    ) {
      return false;
    }

    const query = filters.searchQuery.trim().toLowerCase();
    if (!query) return true;

    return [
      run.id,
      RoutineRunHelper.triggerLabel(run.trigger),
      RoutineRunHelper.outcomeLabel(RoutineRunHelper.outcome(run.status)),
      run.error ?? "",
    ].some((text) => text.toLowerCase().includes(query));
  });

  function handleOpenRun(run: Run) {
    // The run's own conversation. A run that never started one — skipped, or
    // refused before a turn — has nothing to open.
    if (!run.chatId) return;
    openChatTab({
      chatId: run.chatId,
      title: routineName
        ? t("{{name}} · run", { name: routineName })
        : t("Run {{id}}", { id: run.id.slice(0, 8) }),
    });
  }

  async function handleCopyRunId(run: Run) {
    try {
      await navigator.clipboard.writeText(run.id);
    } catch {
      // Clipboard access can be refused; nothing else depends on it.
    }
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
          <span>{t("Trigger")}</span>
          <span>{t("Triggered")}</span>
          <span>{t("Status")}</span>
          <span>{t("Duration")}</span>
          <span className="w-8" />
        </div>

        {runs.length === 0 ? (
          <RunHistoryEmptyState onRunNow={onRunNow} isFiring={isFiring} />
        ) : filteredRuns.length === 0 ? (
          <div className="px-3 py-8 text-center text-sm text-muted-foreground">
            {t("No runs match the current search or filters.")}
          </div>
        ) : (
          <div className="divide-y">
            {filteredRuns.map((run) => {
              const outcome = RoutineRunHelper.outcome(run.status);
              const openable = Boolean(run.chatId);
              return (
                // The run's cells are one button and the actions menu sits
                // beside it, not inside: a <button> cannot hold another, and a
                // menu inside a clickable row opened the conversation instead.
                <div
                  key={run.id}
                  className="grid grid-cols-[minmax(0,1.4fr)_minmax(0,1.1fr)_minmax(0,0.9fr)_minmax(0,0.6fr)_auto] items-center gap-x-2 px-3 py-2.5 text-sm transition-colors hover:bg-muted/40"
                >
                  <button
                    type="button"
                    disabled={!openable}
                    onClick={() => handleOpenRun(run)}
                    className="col-span-4 grid grid-cols-subgrid items-center text-left disabled:cursor-default enabled:cursor-pointer"
                  >
                    <span className="flex min-w-0 items-center gap-2">
                      {outcome === "running" ? (
                        <DotmSquare4 size={14} className="shrink-0 text-foreground" />
                      ) : (
                        <Clock className="size-3.5 shrink-0 text-muted-foreground" />
                      )}
                      <span className="truncate">
                        {RoutineRunHelper.triggerLabel(run.trigger)}
                      </span>
                    </span>
                    <span className="truncate text-muted-foreground">
                      {RoutineRunHelper.formatStartedAt(run.startedAt)}
                    </span>
                    <span className="flex items-center">
                      <RunStatusBadge status={run.status} />
                    </span>
                    <span className="tabular-nums text-muted-foreground">
                      {RoutineRunHelper.duration(run)}
                    </span>
                  </button>
                  <span className="flex w-8 items-center justify-end">
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon-sm"
                          className="text-muted-foreground"
                          aria-label={t("Run actions")}
                        >
                          <Ellipsis className="size-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end" className="w-48">
                        <DropdownMenuItem
                          disabled={!openable}
                          onClick={() => handleOpenRun(run)}
                        >
                          {t("Open conversation")}
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => void handleCopyRunId(run)}>
                          {t("Copy run ID")}
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </span>
                  {run.error ? (
                    <p
                      className={cn(
                        "col-span-5 mt-1 line-clamp-2 pl-5.5 text-xs",
                        outcome === "skipped" ? "text-muted-foreground" : "text-destructive",
                      )}
                    >
                      {run.error}
                    </p>
                  ) : null}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
