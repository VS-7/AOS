import React, { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Switch } from "@/components/ui/switch";
import type {
  Task,
  TaskTransitionInput,
} from "@/features/task/interfaces/task.interfaces";
import { CheckCircle2, GitBranch, GitMerge } from "lucide-react";

interface TasksFinishWorkflowDialogProps {
  open: boolean;
  task: Task | null;
  onOpenChange: (open: boolean) => void;
  onConfirm: (input: TaskTransitionInput) => void | Promise<void>;
}

type FinishOperation = NonNullable<
  TaskTransitionInput["completion"]
>["operation"];

const OPTIONS: Array<{
  operation: FinishOperation;
  title: string;
  description: string;
  icon: React.ComponentType<{ className?: string }>;
}> = [
  {
    operation: "merge_to_main",
    title: "Merge worktree to main",
    description:
      "Finalize the task by merging the current worktree directly into main.",
    icon: GitMerge,
  },
  {
    operation: "create_branch",
    title: "Create branch from worktree",
    description:
      "Keep the work isolated and create a dedicated branch from the current worktree.",
    icon: GitBranch,
  },
];

export function TasksFinishWorkflowDialog({
  open,
  task,
  onOpenChange,
  onConfirm,
}: TasksFinishWorkflowDialogProps) {
  const [operation, setOperation] = useState<FinishOperation>("merge_to_main");

  useEffect(() => {
    if (open) {
      setOperation("merge_to_main");
    }
  }, [open]);

  async function handleConfirm() {
    const input: TaskTransitionInput = {
      status: "finished",
      delegate: true,
      completion: {
        operation,
        openPullRequest: false,
      },
    };

    await onConfirm(input);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Finish task</DialogTitle>
          <DialogDescription>
            Choose how <strong>{task?.id}</strong> should be finalized before it
            moves to finished.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-3">
          {OPTIONS.map((option) => {
            const Icon = option.icon;
            const selected = operation === option.operation;

            return (
              <button
                key={option.operation}
                type="button"
                onClick={() => setOperation(option.operation)}
                className={`flex items-start gap-3 rounded-xl border px-4 py-4 text-left transition-colors ${
                  selected
                    ? "border-primary bg-primary/5"
                    : "border-border bg-card hover:bg-accent/20"
                }`}
              >
                <div className="mt-0.5 rounded-lg border bg-background p-2">
                  <Icon className="size-4" />
                </div>
                <div className="min-w-0 flex-1 space-y-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium">{option.title}</span>
                    {selected && (
                      <Badge variant="secondary">
                        <CheckCircle2 className="size-3" />
                        Selected
                      </Badge>
                    )}
                  </div>
                  <p className="text-sm text-muted-foreground">
                    {option.description}
                  </p>
                </div>
              </button>
            );
          })}
        </div>

        <div className="rounded-xl border bg-muted/20 px-4 py-3">
          <div className="flex items-center justify-between gap-4">
            <div className="space-y-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium">
                  Automatically open PR on GitHub
                </span>
                <Badge variant="outline">Coming Soon</Badge>
              </div>
              <p className="text-sm text-muted-foreground">
                The switch is visible now to establish the workflow, but it
                remains disabled until the GitHub automation is wired.
              </p>
            </div>
            <Switch checked={false} disabled />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={() => void handleConfirm()}>Continue</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
