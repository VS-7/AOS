import React, { useMemo } from "react";
import { AnimatePresence } from "framer-motion";
import { useNavigate } from "@tanstack/react-router";
import { Page, PageBody, PageSecondaryHeader } from "@/components/ui/page";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Plus, Search } from "lucide-react";
import { GoalsListSection } from "./components/list/components/goal-list-section.component";
import { GOAL_STATUS_ORDER } from "../../consts/goal";
import { useGoalsContext } from "./context";
import { GoalsFilter } from "./components/header/filter";

const GoalsHeader = React.memo(function GoalsHeader() {
  const { searchDraft, handleSearchChange } = useGoalsContext();
  const navigate = useNavigate();

  return (
    <PageSecondaryHeader className="justify-between px-4 py-2">
      <div className="flex items-center gap-3">
        <h1 className="text-sm font-semibold tracking-tight text-foreground select-none pl-2">
          Goals
        </h1>
        <div className="relative flex items-center h-8 w-44 sm:w-56 rounded-md bg-transparent transition-colors">
          <Search className="absolute left-2.5 size-3.5 text-muted-foreground pointer-events-none" />
          <Input
            type="text"
            value={searchDraft}
            onChange={(event) => handleSearchChange(event.target.value)}
            placeholder="Search goals..."
            className="h-8 pl-8 pr-2.5 py-0 border-0 bg-transparent shadow-none focus-visible:ring-0 focus-visible:ring-offset-0 focus-visible:bg-transparent placeholder:text-muted-foreground/50 text-xs md:text-sm"
          />
        </div>
      </div>

      <div className="flex items-center gap-2">
        <GoalsFilter />

        <Button
          className="h-9"
          onClick={() =>
            void navigate({ to: "/goals/$id", params: { id: "new" } })
          }
        >
          <Plus data-icon="inline-start" />
          Add goal
        </Button>
      </div>
    </PageSecondaryHeader>
  );
});

const GoalsListView = React.memo(function GoalsListView() {
  const { displayedGroupedGoals, selectedStatuses } = useGoalsContext();

  const visibleStatuses = useMemo(
    () =>
      GOAL_STATUS_ORDER.filter(
        (status) =>
          selectedStatuses.length === 0 || selectedStatuses.includes(status),
      ),
    [selectedStatuses],
  );

  return (
    <div className="gap-4 p-4">
      <AnimatePresence mode="popLayout">
        {visibleStatuses.map((status) => (
            <GoalsListSection
              key={status}
              status={status}
              goals={displayedGroupedGoals[status] || []}
            />
          ))}
      </AnimatePresence>
    </div>
  );
});

export function GoalsPageInner() {
  return (
    <Page className="h-full overflow-hidden">
      <GoalsHeader />

      <PageBody className="overflow-y-auto">
        <div className="mx-auto flex w-full flex-col h-full">
          <GoalsListView />
        </div>
      </PageBody>
    </Page>
  );
}
