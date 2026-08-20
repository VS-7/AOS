import React from "react";
import { AnimatePresence, motion } from "framer-motion";
import {
  Collapsible,
  CollapsibleTrigger,
  CollapsibleContent,
  CollapsibleTitle,
  CollapsibleIcon,
} from "@/components/ui/collapsible";
import type { Goal } from "@/features/goal/interfaces/goal.interfaces";
import { GoalHelper } from "@/features/goal/presentation/helpers/goal.helper";
import { GOAL_STATUS_CONFIG } from "@/features/goal/presentation/consts/goal";
import { GoalListRow } from "./goal-list-row.component";

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

interface GoalsListSectionProps {
  status: Goal["status"];
  goals: Goal[];
}

export function GoalsListSection({ status, goals }: GoalsListSectionProps) {
  const config = GoalHelper.getStatus(status);
  const Icon = config.icon;
  const isEmpty = goals.length === 0;
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
            <span className="h-5 px-1.5 text-xs text-muted-foreground">
              {goals.length}
            </span>
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
                  No goals in {config.label.toLowerCase()}
                </motion.div>
              ) : (
                goals.map((goal) => (
                  <GoalListRow key={goal.id} goal={goal} />
                ))
              )}
            </AnimatePresence>
          </div>
        </CollapsibleContent>
      </Collapsible>
    </motion.section>
  );
}
