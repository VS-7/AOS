import { useState } from "react";
import { Link, useRouter } from "@tanstack/react-router";
import { toast } from "sonner";
import { AlertTriangle, Link2, Plus, X } from "lucide-react";
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
import { t } from "@/lib/i18n";
import type {
  Task,
  TaskWithContext,
} from "@/features/task/interfaces/task.interfaces";

interface DependenciesWidgetProps {
  task: TaskWithContext;
}

/**
 * Sidebar widget that renders the resolved upstream dependencies of a task
 * (populated by the backend `get` projection) and lets the user add or
 * remove them.
 */
export function DependenciesWidget({ task }: DependenciesWidgetProps) {
  const router = useRouter();
  const [pickerOpen, setPickerOpen] = useState(false);
  const [isMutating, setIsMutating] = useState(false);

  const dependencies = task.dependencies ?? [];
  const dependsOnIds = task.dependsOn ?? [];
  const blockedCount = dependencies.filter(
    (dependency) => dependency.status !== "finished",
  ).length;

  const { data: tasksData } = aos.client.task.list.useQuery({
    query: { limit: "200" },
    enabled: pickerOpen,
  });
  const tasks: Task[] =
    (tasksData as { tasks: Task[] } | null | undefined)?.tasks ?? [];

  const currentIds = new Set(dependsOnIds);
  const candidates = tasks.filter(
    (candidate) => candidate.id !== task.id && !currentIds.has(candidate.id),
  );

  async function persistDependsOn(next: string[]) {
    setIsMutating(true);
    const { error } = await aos.client.task.update.mutate({
      params: { task: task.id },
      body: { dependsOn: next },
    });

    if (error) {
      // @ts-expect-error - Expected
      const message = error.error?.message || error.message;
      toast.error(message || "Failed to update dependencies");
      setIsMutating(false);
      return;
    }

    toast.success(t("Dependencies updated"));
    setIsMutating(false);
    router.invalidate();
  }

  async function handleAdd(dependencyId: string) {
    setPickerOpen(false);
    await persistDependsOn([...dependsOnIds, dependencyId]);
  }

  async function handleRemove(dependencyId: string) {
    await persistDependsOn(
      dependsOnIds.filter((id) => id !== dependencyId),
    );
  }

  return (
    <SplitPageLayout.Widget>
      <SplitPageLayout.WidgetHeader>
        <SplitPageLayout.WidgetTitle>{t("Dependencies")}</SplitPageLayout.WidgetTitle>
        <div className="ml-auto flex items-center gap-2">
          {blockedCount > 0 && (
            <span className="flex items-center gap-1 text-xs text-warning">
              <AlertTriangle className="size-3" />
              {blockedCount} unfinished
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
                          onSelect={() => handleAdd(candidate.id)}
                        >
                          <StatusIcon
                            className={cn("size-3.5 shrink-0", status.color)}
                          />
                          <span className="truncate flex-1">
                            {candidate.name}
                          </span>
                          <span className="font-mono text-xs text-muted-foreground">
                            {candidate.id}
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
        {dependencies.length === 0 && (
          <SplitPageLayout.WidgetItem>
            <Link2 className="size-3.5 shrink-0 text-muted-foreground" />
            <span className="text-xs text-muted-foreground">
              {t("No dependencies yet.")}
            </span>
          </SplitPageLayout.WidgetItem>
        )}
        {dependencies.map((dependency) => {
          const status = TaskHelper.getStatus(dependency.status);
          const StatusIcon = status.icon;
          return (
            <SplitPageLayout.WidgetItem
              key={dependency.id}
              className="group pr-2"
            >
              <StatusIcon
                className={cn("size-3.5 shrink-0", status.color)}
              />
              <Link
                to="/tasks/$id"
                params={{ id: dependency.id }}
                className="flex min-w-0 flex-1 flex-col gap-0.5"
              >
                <span className="line-clamp-1 text-xs leading-snug">
                  {dependency.name}
                </span>
                <span className="font-mono text-[10px] leading-none text-muted-foreground">
                  {dependency.id}
                </span>
              </Link>
              <button
                onClick={() => handleRemove(dependency.id)}
                disabled={isMutating}
                className="rounded p-1 text-muted-foreground opacity-0 transition-opacity hover:bg-accent hover:text-foreground group-hover:opacity-100 disabled:opacity-40"
                aria-label={`Remove dependency ${dependency.id}`}
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
