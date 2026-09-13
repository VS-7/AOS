import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { AlertTriangle, CircleHelp, Link2, Plus, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { aos } from "@/app/aos";
import { TaskHelper } from "@/features/task/presentation/helpers/task.helper";
import type { TaskActions } from "@/features/task/presentation/hooks/task-actions.hook";
import { t } from "@/lib/i18n";
import type {
  Task,
  TaskWithContext,
} from "@/features/task/interfaces/task.interfaces";

interface DependenciesWidgetProps {
  task: TaskWithContext;
  actions: TaskActions;
}

/**
 * The tasks this one waits on, and the picker that adds more.
 *
 * It rendered `task.dependencies`, a list of resolved summaries the daemon
 * has never sent: a dependency was saved, the widget still said "No
 * dependencies yet.", and the picker then hid that task, so it could be
 * neither seen nor removed. What the daemon does send is `dependsOn` (the
 * ids) and `blocked` (the ones not finished); names and statuses come from
 * the task list this widget already reads for its picker.
 */
export function DependenciesWidget({ task, actions }: DependenciesWidgetProps) {
  const [pickerOpen, setPickerOpen] = useState(false);
  const [isMutating, setIsMutating] = useState(false);

  const dependsOnIds = task.dependsOn ?? [];
  const blocked = new Set(task.blocked ?? []);

  const { data: tasksData } = aos.client.task.list.useQuery({
    query: { limit: "200" },
  });
  const tasks: Task[] =
    (tasksData as { tasks: Task[] } | null | undefined)?.tasks ?? [];
  const byId = new Map(tasks.map((candidate) => [candidate.id, candidate]));

  const currentIds = new Set(dependsOnIds);
  const candidates = tasks.filter(
    (candidate) => candidate.id !== task.id && !currentIds.has(candidate.id),
  );

  async function persistDependsOn(next: string[]) {
    setIsMutating(true);
    await actions.setDependencies(next);
    setIsMutating(false);
  }

  return (
    <SplitPageLayout.Widget>
      <SplitPageLayout.WidgetHeader>
        <SplitPageLayout.WidgetTitle>{t("Dependencies")}</SplitPageLayout.WidgetTitle>
        <div className="ml-auto flex items-center gap-2">
          {blocked.size > 0 && (
            <span className="flex items-center gap-1 text-xs text-warning">
              <AlertTriangle className="size-3" />
              {t("{{count}} unfinished", { count: blocked.size })}
            </span>
          )}
          <Popover open={pickerOpen} onOpenChange={setPickerOpen}>
            <PopoverTrigger asChild>
              <Button
                size="icon"
                variant="secondary"
                className="rounded-full"
                disabled={isMutating}
                aria-label={t("Add dependency")}
              >
                <Plus />
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-64 p-0" align="end" side="left">
              <Command>
                <CommandInput placeholder={t("Search tasks...")} />
                <CommandList>
                  <CommandEmpty>{t("No eligible tasks.")}</CommandEmpty>
                  <CommandGroup>
                    {candidates.map((candidate) => {
                      const status = TaskHelper.getStatus(candidate.status);
                      const StatusIcon = status.icon;
                      return (
                        <CommandItem
                          key={candidate.id}
                          value={`${candidate.id} ${candidate.name}`}
                          onSelect={() => {
                            setPickerOpen(false);
                            void persistDependsOn([...dependsOnIds, candidate.id]);
                          }}
                        >
                          <StatusIcon
                            className={cn("size-3.5 shrink-0", status.color)}
                          />
                          <span className="truncate flex-1">
                            {candidate.name}
                          </span>
                          <span className="font-mono text-xs text-muted-foreground">
                            {TaskHelper.shortId(candidate.id)}
                          </span>
                        </CommandItem>
                      );
                    })}
                  </CommandGroup>
                </CommandList>
              </Command>
            </PopoverContent>
          </Popover>
        </div>
      </SplitPageLayout.WidgetHeader>
      <SplitPageLayout.WidgetContent>
        {dependsOnIds.length === 0 && (
          <SplitPageLayout.WidgetItem>
            <Link2 className="size-3.5 shrink-0 text-muted-foreground" />
            <span className="text-xs text-muted-foreground">
              {t("No dependencies yet.")}
            </span>
          </SplitPageLayout.WidgetItem>
        )}
        {dependsOnIds.map((id) => {
          const dependency = byId.get(id);
          const status = dependency ? TaskHelper.getStatus(dependency.status) : null;
          const StatusIcon = status?.icon ?? CircleHelp;
          return (
            <SplitPageLayout.WidgetItem key={id} className="group pr-2">
              <StatusIcon
                className={cn("size-3.5 shrink-0", status?.color ?? "text-muted-foreground")}
              />
              <Link
                to="/tasks/$id"
                params={{ id }}
                className="flex min-w-0 flex-1 flex-col gap-0.5"
              >
                <span className="line-clamp-1 text-xs leading-snug">
                  {dependency?.name ?? (tasksData ? t("A task that no longer exists") : TaskHelper.shortId(id))}
                </span>
                <span className="font-mono text-[10px] leading-none text-muted-foreground">
                  {TaskHelper.shortId(id)}
                  {blocked.has(id) ? ` · ${t("unfinished")}` : ""}
                </span>
              </Link>
              <button
                onClick={() => void persistDependsOn(dependsOnIds.filter((other) => other !== id))}
                disabled={isMutating}
                className="rounded p-1 text-muted-foreground opacity-0 transition-opacity hover:bg-accent hover:text-foreground focus-visible:opacity-100 group-hover:opacity-100 disabled:opacity-40"
                aria-label={t("Remove dependency {{name}}", { name: dependency?.name ?? id })}
              >
                <X className="size-3" />
              </button>
            </SplitPageLayout.WidgetItem>
          );
        })}
      </SplitPageLayout.WidgetContent>
    </SplitPageLayout.Widget>
  );
}
