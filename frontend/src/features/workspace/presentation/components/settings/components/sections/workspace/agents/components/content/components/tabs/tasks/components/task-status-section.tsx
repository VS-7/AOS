import { AnimatePresence, motion } from "framer-motion";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleIcon,
  CollapsibleTitle,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import type { Task } from "@/features/task/interfaces/task.interfaces";
import { TaskHelper } from "@/features/task/presentation/helpers/task.helper";
import { TaskListRow } from "@/features/task/presentation/pages/(main)/components/list/components/task-list-row.component";
import { useEffect, useState } from "react";

interface AgentTaskStatusSectionProps {
  status: Task["status"];
  tasks: Task[];
}

const itemVariants = {
  hidden: { opacity: 0, y: 6 },
  visible: { opacity: 1, y: 0 },
  exit: { opacity: 0, y: -6, transition: { duration: 0.15 } },
} as const;

export function AgentTaskStatusSection({ status, tasks }: AgentTaskStatusSectionProps) {
  const config = TaskHelper.getStatus(status);
  const Icon = config.icon;
  const isEmpty = tasks.length === 0;
  const [isOpen, setIsOpen] = useState(!isEmpty);

  useEffect(() => {
    setIsOpen(!isEmpty);
  }, [isEmpty, status]);

  return (
    <motion.section layout className="flex flex-col gap-1 not-first:mt-4">
      <Collapsible open={isOpen} onOpenChange={setIsOpen}>
        <CollapsibleTrigger asChild>
          <header className="group/collapsible-trigger flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 hover:bg-accent/50">
            <CollapsibleIcon>
              <Icon className={`size-4 ${config.color}`} />
            </CollapsibleIcon>
            <CollapsibleTitle className="flex items-center gap-2 normal-case tracking-normal text-sm font-medium text-foreground">
              {config.label}
            </CollapsibleTitle>
            <span className="h-5 px-1.5 text-xs text-muted-foreground">{tasks.length}</span>
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
                  className="flex h-11 items-center px-3 text-xs text-muted-foreground"
                >
                  No tasks in {config.label.toLowerCase()}
                </motion.div>
              ) : (
                tasks.map((task) => <TaskListRow key={task.id} task={task} />)
              )}
            </AnimatePresence>
          </div>
        </CollapsibleContent>
      </Collapsible>
    </motion.section>
  );
}
