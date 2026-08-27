import React, { useMemo } from "react";
import { AnimatePresence } from "framer-motion";
import { Plus, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Page, PageBody, PageSecondaryHeader } from "@/components/ui/page";
import { ROUTINE_STATUS_ORDER } from "@/features/routine/presentation/consts/routine";
import { RoutineListSection } from "./components/list/components/routine-list-section.component";
import { RoutinesFilter } from "./components/header/filter";
import { useRoutinesContext } from "./context";
import { useRouter } from "@tanstack/react-router";
import { t } from "@/lib/i18n";

const RoutinesHeader = React.memo(function RoutinesHeader() {
  const { searchDraft, handleSearchChange } = useRoutinesContext();
  const router = useRouter();

  return (
    <PageSecondaryHeader className="justify-between px-4 py-2">
      <div className="flex items-center gap-3">
        <h1 className="text-sm font-semibold tracking-tight text-foreground select-none pl-2">
          {t("Routines")}
        </h1>
        <div className="relative flex items-center h-8 w-44 sm:w-56 rounded-md bg-transparent transition-colors">
          <Search className="absolute left-2.5 size-3.5 text-muted-foreground pointer-events-none" />
          <Input
            type="text"
            value={searchDraft}
            onChange={(event) => handleSearchChange(event.target.value)}
            placeholder={t("Search routines...")}
            className="h-8 pl-8 pr-2.5 py-0 border-0 bg-transparent shadow-none focus-visible:ring-0 focus-visible:ring-offset-0 focus-visible:bg-transparent placeholder:text-muted-foreground/50 text-xs md:text-sm"
          />
        </div>
      </div>

      <div className="flex items-center gap-2">
        <RoutinesFilter />

        <Button
          className="h-9"
          onClick={() => router.navigate({ to: "/routines/new" })}
        >
          <Plus data-icon="inline-start" />
          {t("Add routine")}
        </Button>
      </div>
    </PageSecondaryHeader>
  );
});

const RoutinesListView = React.memo(function RoutinesListView() {
  const { displayedGroupedRoutines, selectedStatuses } = useRoutinesContext();

  const visibleStatuses = useMemo(
    () =>
      ROUTINE_STATUS_ORDER.filter(
        (status) =>
          selectedStatuses.length === 0 || selectedStatuses.includes(status),
      ),
    [selectedStatuses],
  );

  return (
    <div className="gap-4 p-4">
      <AnimatePresence mode="popLayout">
        {visibleStatuses.map((status) => (
            <RoutineListSection
              key={status}
              status={status}
              routines={displayedGroupedRoutines[status] || []}
            />
          ))}
      </AnimatePresence>
    </div>
  );
});

export function RoutinesPageInner() {
  return (
    <Page className="h-full overflow-hidden">
      <RoutinesHeader />

      <PageBody className="overflow-y-auto">
        <div className="mx-auto flex w-full flex-col h-full">
          <RoutinesListView />
        </div>
      </PageBody>
    </Page>
  );
}
