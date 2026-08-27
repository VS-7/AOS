import React from "react";
import { useNavigate, useRouter } from "@tanstack/react-router";
import {
  CalendarDays,
  CircleDashed,
  Copy,
  GitBranch,
  Link2,
  Play,
  Square,
  Star,
  CheckIcon,
  MessageSquare,
  Info,
} from "lucide-react";
import { openChatTab } from "@/features/chat/presentation/helpers/open-chat-tab.helper";
import { Button } from "@/components/ui/button";
import { AnimatedEmptyState } from "@/components/ui/animated-empty-state";
import { CircleProgress } from "@/components/ui/circle-progress";
import { Kbd } from "@/components/ui/kbd";
import { MarkdownRenderer } from "@/components/ui/markdown-content";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type {
  TaskWithContext,
  TaskPriority,
} from "@/features/task/interfaces/task.interfaces";
import { toast } from "sonner";
import { TaskHelper } from "@/features/task/presentation/helpers/task.helper";
import { useTasksStatusTransition } from "@/features/task/presentation/hooks/tasks-status-transition.hook";
import { TasksFinishWorkflowDialog } from "@/features/task/presentation/components/dialogs/finish";
import { TaskActionsDropdown } from "@/features/task/presentation/components/dropdowns/task-actions.dropdown";
import { aos } from "@/app/aos";
import { AosResponse } from "@/app/builders/response";
import { TaskAttachments } from "../attachments";
import { TaskComments } from "../comments";
import { MarkdownEditor } from "@/components/ui/markdown-editor";
import type { UseChatResult } from "@/features/chat/presentation/hooks/use-chat";
import { t } from "@/lib/i18n";

interface TaskDetailsMainProps {
  task: TaskWithContext;
  client: typeof aos.client;
  refresh: () => void;
  liveChat?: UseChatResult | null;
}

interface HeaderIconButtonProps {
  children: React.ReactNode;
  label: string;
  shortcut: string;
  onClick?: () => void | Promise<void>;
}

function HeaderIconButton({
  children,
  label,
  shortcut,
  onClick,
}: HeaderIconButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="size-8 rounded-md"
          onClick={onClick}
        >
          {children}
          <span className="sr-only">{label}</span>
        </Button>
      </TooltipTrigger>
      <TooltipContent sideOffset={8} className="flex items-center gap-2">
        <span>{label}</span>
        <Kbd>{shortcut}</Kbd>
      </TooltipContent>
    </Tooltip>
  );
}

export function TaskDetailsMain({
  task,
  client,
  refresh,
  liveChat,
}: TaskDetailsMainProps) {
  const navigate = useNavigate();
  const router = useRouter();
  const finishTransition = useTasksStatusTransition();
  const stopChat = client.chat.stop.useMutation({
    // `chat.stop` is dormant (`command-map.ts`: `"chat.stop": null`) —
    // the facade resolves a dormant call through `onSuccess`, not
    // `onError` (see `lib/aos-facade.ts`'s `call()`: dormant returns an
    // `{ data: undefined, error }` value, it never rejects), and the
    // envelope has no `.message` field the source assumed.
    onSuccess: (result) => {
      if (result?.error) {
        toast.error(result.error.message || "Failed to stop chat");
        return;
      }
      toast.success(t("Chat stopped."));
      router.invalidate();
    },
    onError: (error: any) => {
      toast.error(
        error?.error?.message || error?.message || "Failed to stop chat",
      );
    },
  });
  const todoStats = task.stats.todos;
  const totalTodos =
    todoStats.completed +
    todoStats.in_progress +
    todoStats.in_review +
    todoStats.todo;
  const completionPercentage =
    totalTodos > 0 ? Math.round((todoStats.completed / totalTodos) * 100) : 0;

  const issueLink =
    typeof window === "undefined"
      ? `/tasks/${task.id}`
      : new URL(`/tasks/${task.id}`, window.location.origin).toString();
  const promptText = [
    `Task ${task.id}: ${task.name}`,
    task.summary ? `Summary: ${task.summary}` : undefined,
    task.content ? `Content:\n${task.content}` : undefined,
  ]
    .filter(Boolean)
    .join("\n\n");

  const allTodosCompleted =
    totalTodos > 0 ? todoStats.completed === totalTodos : true;
  const showApproveButton = task.status === "in_review" && allTodosCompleted;
  // Task 9 replaced `chat.interfaces.ts`'s `Message` (this file's original
  // target, mirroring AOS's Go entity directly) with the recovered
  // AOS `ChatMessage` — `runs` now lives at `message.metadata.
  // runs`, matching the source shape this comment used to say AOS didn't
  // have. See `chat.interfaces.ts`'s own doc comment for the full story.
  const isLiveChatRunning = Boolean(
    liveChat?.messages.some((message) =>
      (message.metadata?.runs ?? []).some(
        (run) => run.status === "pending" || run.status === "running",
      ),
    ),
  );
 const showStopButton = isLiveChatRunning;
  const showStartButton =
    !showStopButton &&
    ["suggestion", "backlog", "planning", "todo", "stopped"].includes(task.status);
  const showContinueButton =
   !showStopButton &&
   (task.status === "in_progress" ||
     (task.status === "in_review" && !allTodosCompleted));

  async function copyToClipboard(value: string, label: string) {
    await navigator.clipboard.writeText(value);
    toast.success(`${label} copied`);
  }

  async function handleCopyLink() {
    await copyToClipboard(issueLink, "Issue link");
  }

  async function handleCopyId() {
    await copyToClipboard(task.id, "Task ID");
  }

  async function handleCopyBranch() {
    await copyToClipboard(`${task.type}/${task.slug}`, "Branch name");
  }

  async function handleCopyPrompt() {
    await copyToClipboard(promptText, "Prompt");
  }

  async function handleStartTask() {
    const { error } = await client.task.start.mutate({
      params: { task: task.id },
      body: { delegate: true },
    });

    if (error) {
      // @ts-expect-error - Expected
      const message = error.error?.message || error.message;
      toast.error(message || "Failed to start task");
      return;
    }

    toast.success(`Started ${task.id}`, {
      description: "Moving task to In Progress",
    });

    if (task.chat) {
      openChatTab({ chatId: task.chat, title: task.name });
    }

    router.invalidate();
  }

  async function handleStatusSelect(status: TaskWithContext["status"]) {
    if (status === task.status) {
      return;
    }

    if (status === "finished") {
      if (task.worktree.enabled) {
        finishTransition.open(task, status);
        return;
      }

      const { error } = await client.task.setStatus.mutate({
        params: { task: task.id },
        body: { status },
      });

      if (error) {
        // @ts-expect-error - Expected
        const message = error.error?.message || error.message;
        toast.error(message || "Failed to update task status");
        return;
      }

      toast.success(
        `Moved ${task.id} to ${TaskHelper.getStatus(status).label}`,
      );
      router.invalidate();
      return;
    }

    const { error } = await client.task.setStatus.mutate({
      params: { task: task.id },
      body: { status },
    });

    if (error) {
      toast.error(
        // @ts-expect-error - Expected
        error.error?.message || error.message || "Failed to update task status",
      );
      return;
    }

    toast.success(`Moved ${task.id} to ${TaskHelper.getStatus(status).label}`);
    router.invalidate();
  }

  async function handlePriorityChange(
    priority: TaskWithContext["priority"],
  ) {
    try {
      await client.task.update.mutate({
        params: { task: task.id },
        body: { priority },
      });
      toast.success(t("Priority updated"));
      router.invalidate();
    } catch (error) {
      toast.error(t("Failed to update priority"));
    }
  }

  async function handleAssigneeChange(assignee: string | undefined) {
    try {
      await client.task.update.mutate({
        params: { task: task.id },
        body: { assigned: assignee },
      });
      toast.success(assignee ? "Assigned" : "Unassigned");
      router.invalidate();
    } catch (error) {
      toast.error(t("Failed to update assignee"));
    }
  }

  async function handleTypeChange(type: string) {
    try {
      await client.task.update.mutate({
        params: { task: task.id },
        body: { type },
      });
      toast.success(t("Type updated"));
      router.invalidate();
    } catch (error) {
      toast.error(t("Failed to update type"));
    }
  }

  async function handleDueDateChange(dueAt: string | undefined) {
    try {
      await client.task.update.mutate({
        params: { task: task.id },
        body: { dueAt },
      });
      toast.success(dueAt ? "Due date set" : "Due date removed");
      router.invalidate();
    } catch (error) {
      toast.error(t("Failed to update due date"));
    }
  }

  async function handleDelete() {
    try {
      await client.task.delete.mutate({ params: { task: task.id } });
      toast.success(`Task ${task.id} deleted`);
      navigate({ to: "/tasks" });
    } catch (error) {
      toast.error(t("Failed to delete task"));
    }
  }

  return (
    <>
      <div className="grid h-full grid-rows-[auto_1fr]">
        <SplitPageLayout.ContentHeader>
          <SplitPageLayout.ContentHeaderMain className="items-center">
            <span
              className="inline-flex shrink-0 items-center gap-1 rounded-full border border-border/60 bg-muted/50 px-2 py-1 text-xs font-medium text-muted-foreground"
              aria-label={`${completionPercentage}% of todos completed`}
            >
              <CircleProgress
                progress={completionPercentage}
                size={14}
                strokeWidth={2}
                aria-hidden
                className="text-primary"
              />
              <span>{completionPercentage}%</span>
            </span>
            <SplitPageLayout.ContentTitle>
              {task.id} - {task.name}
            </SplitPageLayout.ContentTitle>
            <TaskActionsDropdown
              task={task}
              onPriorityChange={handlePriorityChange}
              onAssigneeChange={handleAssigneeChange}
              onTypeChange={handleTypeChange}
              onStatusChange={handleStatusSelect}
              onDelete={handleDelete}
              onCopyPrompt={handleCopyPrompt}
            />
          </SplitPageLayout.ContentHeaderMain>

          <SplitPageLayout.ContentHeaderActions>
            <TooltipProvider>
              <HeaderIconButton
                label={t("Copy issue link")}
                shortcut="L"
                onClick={handleCopyLink}
              >
                <Link2 />
              </HeaderIconButton>
              <HeaderIconButton
                label={t("Copy task ID")}
                shortcut="I"
                onClick={handleCopyId}
              >
                <Copy />
              </HeaderIconButton>
              <HeaderIconButton
                label={t("Copy branch name")}
                shortcut="B"
                onClick={handleCopyBranch}
              >
                <GitBranch />
              </HeaderIconButton>
            </TooltipProvider>
            {Boolean(task.chat) && (
              <Button
                variant="outline"
                size="sm"
                className="h-8 gap-2 rounded-full px-3!"
                onClick={() => {
                  if (task.chat) {
                    openChatTab({ chatId: task.chat, title: task.name });
                  }
                }}
              >
                <MessageSquare className="size-3.5" data-icon="inline-start" />
                {t("Open chat")}
              </Button>
            )}
            {showStopButton && (
              <Button
                className="h-8 gap-2 rounded-full px-4!"
                onClick={() => {
                  if (!task.chat) {
                    return;
                  }

                  stopChat.mutate({
                    params: { chat: task.chat },
                    body: {},
                  });
                }}
                variant="outline"
              >
                <Square data-icon="inline-start" />
                {t("Stop")}
              </Button>
            )}
            {showStartButton && (
              <Button
                className="h-8 gap-2 rounded-full px-4!"
                onClick={handleStartTask}
              >
                <Play data-icon="inline-start" />
                {t("Start")}
              </Button>
            )}
            {showContinueButton && (
              <Button
                className="h-8 gap-2 rounded-full px-4!"
                onClick={handleStartTask}
              >
                <Play data-icon="inline-start" />
                {t("Continue")}
              </Button>
            )}
            {showApproveButton && (
              <Button
                size="sm"
                className="h-8 gap-2"
                onClick={() => void handleStatusSelect("finished")}
              >
                <CheckIcon data-icon="inline-start" />
                {t("Aprove and mark as finished")}
              </Button>
            )}
          </SplitPageLayout.ContentHeaderActions>
        </SplitPageLayout.ContentHeader>

      {/*
        The source read `checkpoint.{summary,at,actor.type,execution.
        pendingTodoIds,resume.instructions}` — a shape this backend's
        `task.Checkpoint` (internal/domain/task/entity.go) does not have.
        Adapted to the real fields (`reason`, `stoppedAt`, `pendingTodoIds`,
        no `actor`/`resume`) rather than left referencing ones that don't
        exist; see the port's checkpoint field notes in
        `interfaces/task.interfaces.ts`.
      */}
      {task.status === "stopped" && task.checkpoint && (
        <div className="border-b border-border/60 bg-muted/30 px-6 py-2.5">
          <div className="flex items-start gap-2.5">
            <Info className="mt-0.5 size-3.5 shrink-0 text-warning" />
            <div className="min-w-0">
              <p className="text-xs font-medium text-foreground">
                {task.checkpoint.reason || "Run interrupted"}
              </p>
              <p className="text-[11px] text-muted-foreground">
                {t("Stopped")} {new Date(task.checkpoint.stoppedAt).toLocaleString()}
                {task.checkpoint.pendingTodoIds?.length ? (
                  <> {t("&middot;")} {task.checkpoint.pendingTodoIds.length} {t("pending todos")}</>
                ) : null}
              </p>
            </div>
          </div>
        </div>
      )}

        <SplitPageLayout.ContentBody>
          <div className="container mx-auto flex h-full max-w-3xl flex-col py-6 pb-10">
            {!task.content && (
              <AnimatedEmptyState className="border-none shadow-none py-12">
                <AnimatedEmptyState.Carousel>
                  <div className="flex items-center gap-3">
                    <div className="flex size-8 items-center justify-center rounded-md bg-muted/50">
                      <CircleDashed className="size-3.5 text-muted-foreground" />
                    </div>
                    <div className="flex flex-col gap-0.5">
                      <div className="h-2 w-24 rounded-md bg-muted" />
                      <div className="h-2 w-16 rounded-md bg-muted/50" />
                    </div>
                  </div>
                </AnimatedEmptyState.Carousel>
                <AnimatedEmptyState.Content>
                  <AnimatedEmptyState.Title>
                    {t("No content defined")}
                  </AnimatedEmptyState.Title>
                  <AnimatedEmptyState.Description>
                    {t("This task does not have a detailed description yet.")}
                  </AnimatedEmptyState.Description>
                </AnimatedEmptyState.Content>
              </AnimatedEmptyState>
            )}

            {task.content && (
              <MarkdownEditor value={task.content} onValueChange={() => {}} />
            )}

            <div className="mt-12 space-y-6">
              <TaskAttachments attachments={task.attachments || []} />
              <TaskComments taskId={task.id} />
            </div>
          </div>
        </SplitPageLayout.ContentBody>
      </div>

      <TasksFinishWorkflowDialog
        open={finishTransition.state.open}
        task={finishTransition.state.task}
        onOpenChange={(open) => {
          if (!open) finishTransition.close();
        }}
        onConfirm={async (input) => {
          try {
            if (!finishTransition.state.task) {
              finishTransition.close();
              return;
            }

            const { error } = await client.task.setStatus.mutate({
              params: { task: finishTransition.state.task.id },
              body: input,
            });

            if (error) {
              console.error(error);
              return;
            }

            toast.success(`Finished ${finishTransition.state.task.id}`);
            finishTransition.close();
            refresh();
          } catch (error) {
            toast.error(t("Failed to finish task"), {
              // @ts-expect-error - Expected
              description: error?.error?.message || error?.message || undefined,
            });
          }
        }}
      />
    </>
  );
}
