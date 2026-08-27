import React from "react";
import { AnimatePresence, motion } from "framer-motion";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleIcon,
  CollapsibleTitle,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { RoutineHelper } from "@/features/routine/presentation/helpers/routine.helper";
import type { Routine } from "@/features/routine/interfaces/routine.interfaces";
import { RoutineListRow } from "./routine-list-row.component";
import { t } from "@/lib/i18n";

const itemVariants = {
  hidden: { opacity: 0, y: 6 },
  visible: { opacity: 1, y: 0 },
  exit: { opacity: 0, y: -6, transition: { duration: 0.15 } },
} as const;

const sectionVariants = {
  hidden: { opacity: 0, y: 12 },
  visible: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.25, ease: "easeOut" as const },
  },
  exit: {
    opacity: 0,
    y: -8,
    transition: { duration: 0.2, ease: "easeIn" as const },
  },
};

interface RoutineListSectionProps {
  status: Routine["status"];
  routines: Routine[];
}

export function RoutineListSection({ status, routines }: RoutineListSectionProps) {
  const config = RoutineHelper.getStatus(status);
  const Icon = config.icon;
  const isEmpty = routines.length === 0;
  const [isOpen, setIsOpen] = React.useState(!isEmpty);

  React.useEffect(() => {
    setIsOpen(!isEmpty);
  }, [isEmpty, status]);

  return (
    <motion.section
      layout
      variants={sectionVariants}
      initial="hidden"
      animate="visible"
      exit="exit"
      className="flex flex-col gap-1 not-first:mt-4"
    >
      <Collapsible open={isOpen} onOpenChange={setIsOpen}>
        <CollapsibleTrigger asChild>
          <header className="group/collapsible-trigger flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 hover:bg-accent/50">
            <CollapsibleIcon>
              <Icon className={`size-4 ${config.color}`} />
            </CollapsibleIcon>
            <CollapsibleTitle className="flex items-center gap-2 normal-case tracking-normal text-sm font-medium text-foreground">
              {config.label}
            </CollapsibleTitle>
            <span className="h-5 px-1.5 text-xs text-muted-foreground">{routines.length}</span>
          </header>
        </CollapsibleTrigger>

        <CollapsibleContent>
          <div className="flex flex-col rounded-md border shadow-inner bg-muted divide-y overflow-hidden">
            <AnimatePresence mode="popLayout">
              {isEmpty ? (
                <motion.div
                  key={`${status}-empty`}
                  layout
                  variants={itemVariants}
                  initial="hidden"
                  animate="visible"
                  exit="exit"
                  transition={{ duration: 0.2 }}
                  className="flex h-11 items-center justify-center px-3 text-xs text-muted-foreground"
                >
                  {t("No routines in")} {config.label.toLowerCase()}
                </motion.div>
              ) : (
                routines.map((routine) => <RoutineListRow key={routine.id} routine={routine} />)
              )}
            </AnimatePresence>
          </div>
        </CollapsibleContent>
      </Collapsible>
    </motion.section>
  );
}
