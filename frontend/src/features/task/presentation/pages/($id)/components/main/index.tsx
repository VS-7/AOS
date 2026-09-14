import React from "react";
import { useRouter } from "@tanstack/react-router";
import {
  ArrowRight,
  CircleDashed,
  Copy,
  GitBranch,
  Link2,
  Pencil,
  Play,
  Square,
  CheckIcon,
  MessageSquare,
  Info,
} from "lucide-react";
import { openChatTab } from "@/features/chat/presentation/helpers/open-chat-tab.helper";
import { Button } from "@/components/ui/button";
import { AnimatedEmptyState } from "@/components/ui/animated-empty-state";
import { CircleProgress } from "@/components/ui/circle-progress";
import { MarkdownRenderer } from "@/components/ui/markdown-content";
import { MarkdownEditor } from "@/components/ui/markdown-editor";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { TaskWithContext } from "@/features/task/interfaces/task.interfaces";
import { toast } from "sonner";
import { TaskHelper } from "@/features/task/presentation/helpers/task.helper";
import { nextStep, type TaskNextStepKind } from "@/features/task/presentation/helpers/task-lifecycle.helper";
import { TaskActionsDropdown } from "@/features/task/presentation/components/dropdowns/task-actions.dropdown";
import type { TaskActions } from "@/features/task/presentation/hooks/task-actions.hook";
import { aos } from "@/app/aos";
import { TaskComments } from "../comments";
import type { UseChatResult } from "@/features/chat/presentation/hooks/use-chat";
import { t } from "@/lib/i18n";
import { errorMessage } from "@/lib/aos-facade";

interface TaskDetailsMainProps {
  task: TaskWithContext;
  actions: TaskActions;
  liveChat?: UseChatResult | null;
}

interface HeaderIconButtonProps {
  children: React.ReactNode;
  label: string;
  onClick?: () => void | Promise<void>;
}

// The tooltips used to advertise L, I and B, and ⌘⇧P on the actions menu;
// none of them was bound, and ⌘⇧P belongs to the project trigger.
function HeaderIconButton({ children, label, onClick }: HeaderIconButtonProps) {
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
      <TooltipContent sideOffset={8}>{label}</TooltipContent>
    </Tooltip>
  );
}

const NEXT_STEP_LABEL: Record<TaskNextStepKind, () => string> = {
  start: () => t("Start"),
  resume: () => t("Resume"),
  continue: () => t("Continue"),
  approve: () => t("Approve and mark as finished"),
  backlog: () => t("Move to Backlog"),
  todo: () => t("Move to Todo"),
};

export function TaskDetailsMain({ task, actions, liveChat }: TaskDetailsMainProps) {
  const router = useRouter();
  const branchPrefix = aos.stores.workspace.useState((state) => state.current?.git?.branchPrefix);
  const stopChat = aos.client.chat.stop.useMutation({
    // `chats_stop` answers `{stopped, message}`: asking to stop a chat with
    // nothing running is not a refusal, but it is not "Chat stopped." either.
    onSuccess: (result) => {
      if (!result?.data?.stopped) {
        toast.info(result?.data?.message ?? t("No active run was found to stop."));
        return;
      }
      toast.success(t("Chat stopped."));
      router.invalidate();
    },
    onError: (error) => {
      toast.error(t("Failed to stop chat"), { description: errorMessage(error) });
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
  const planSettled = totalTodos > 0 ? todoStats.completed === totalTodos : true;

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
  const step = isLiveChatRunning ? null : nextStep(task, planSettled);

  async function copyToClipboard(value: string, success: string) {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(success);
    } catch (error) {
      toast.error(t("Could not copy to the clipboard"), { description: errorMessage(error) });
    }
  }

  async function handleNextStep() {
    if (!step) return;
    const moved = await actions.setStatus(step.status);
    if (moved && step.status === "in_progress" && task.chat) {
      openChatTab({ chatId: task.chat, title: task.name });
    }
  }

  const issueLink =
    typeof window === "undefined"
      ? `/tasks/${task.id}`
      : new URL(`/tasks/${task.id}`, window.location.origin).toString();

  return (
    // A column of three: header, the stopped banner when there is one, and
    // the body that scrolls. It was a two-row grid holding three children, so
    // the banner took the scrolling row, the body fell into an implicit one
    // and never scrolled, and the implicit column grew to the header's width
    // and pushed its buttons under the detail panel.
    <div className="flex h-full min-h-0 min-w-0 flex-col">
      <SplitPageLayout.ContentHeader className="shrink-0">
        <SplitPageLayout.ContentHeaderMain className="items-center">
          <span
            className="inline-flex shrink-0 items-center gap-1 rounded-full border border-border/60 bg-muted/50 px-2 py-1 text-xs font-medium text-muted-foreground"
            aria-label={t("{{percent}}% of todos completed", { percent: completionPercentage })}
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
          <span className="shrink-0 font-mono text-xs text-muted-foreground" title={task.id}>
            {TaskHelper.shortId(task.id)}
          </span>
          <SplitPageLayout.ContentTitle className="min-w-0">
            {task.name}
          </SplitPageLayout.ContentTitle>
          <TaskActionsDropdown task={task} actions={actions} />
        </SplitPageLayout.ContentHeaderMain>

        <SplitPageLayout.ContentHeaderActions>
          <TooltipProvider>
            <HeaderIconButton
              label={t("Copy issue link")}
              onClick={() => copyToClipboard(issueLink, t("Issue link copied"))}
            >
              <Link2 />
            </HeaderIconButton>
            <HeaderIconButton label={t("Copy task ID")} onClick={actions.copyIdentifier}>
              <Copy />
            </HeaderIconButton>
            <HeaderIconButton
              label={t("Copy branch name")}
              onClick={() =>
                copyToClipboard(TaskHelper.branchName(task, branchPrefix), t("Branch name copied"))
              }
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
          {isLiveChatRunning && (
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
          {step && (
            <Button
              size="sm"
              variant={step.kind === "backlog" || step.kind === "todo" ? "outline" : "default"}
              className="h-8 gap-2 rounded-full px-4!"
              onClick={() => void handleNextStep()}
            >
              {step.kind === "approve" ? (
                <CheckIcon data-icon="inline-start" />
              ) : step.kind === "backlog" || step.kind === "todo" ? (
                <ArrowRight data-icon="inline-start" />
              ) : (
                <Play data-icon="inline-start" />
              )}
              {NEXT_STEP_LABEL[step.kind]()}
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
        <div className="shrink-0 border-b border-border/60 bg-muted/30 px-6 py-2.5">
          <div className="flex min-w-0 items-start gap-2.5">
            <Info className="mt-0.5 size-3.5 shrink-0 text-warning" />
            <div className="min-w-0">
              <p className="break-words text-xs font-medium text-foreground">
                {task.checkpoint.reason || t("Run interrupted")}
              </p>
              <p className="text-[11px] text-muted-foreground">
                {t("Stopped")} {TaskHelper.formatDate(task.checkpoint.stoppedAt)}
                {task.checkpoint.pendingTodoIds?.length
                  ? ` · ${t("{{count}} pending todos", { count: task.checkpoint.pendingTodoIds.length })}`
                  : null}
              </p>
            </div>
          </div>
        </div>
      )}

      <SplitPageLayout.ContentBody>
        <div className="container mx-auto flex max-w-3xl flex-col py-6 pb-10">
          <TaskDescription task={task} actions={actions} />

          <div className="mt-12 space-y-6">
            <TaskComments taskId={task.id} />
          </div>
        </div>
      </SplitPageLayout.ContentBody>
    </div>
  );
}

/**
 * The task's description, read by default and edited on request.
 *
 * It used to be an editable rich-text editor wired to a no-op: typing worked
 * on screen and was gone on reload, with nothing to say it was never saved.
 * Editing is now an explicit mode that saves through tasks_update, so an
 * agent writing the same description meanwhile is not overwritten by a page
 * that merely had it open.
 */
function TaskDescription({ task, actions }: { task: TaskWithContext; actions: TaskActions }) {
  const [editing, setEditing] = React.useState(false);
  const [draft, setDraft] = React.useState(task.content ?? "");
  const [saving, setSaving] = React.useState(false);

  function startEditing() {
    setDraft(task.content ?? "");
    setEditing(true);
  }

  async function save() {
    setSaving(true);
    const saved = await actions.setContent(draft);
    setSaving(false);
    if (saved) setEditing(false);
  }

  if (editing) {
    return (
      <div className="flex flex-col gap-3">
        <MarkdownEditor
          value={draft}
          onValueChange={setDraft}
          placeholder={t("Describe the work, what done looks like, and anything an agent needs to know.")}
        />
        <div className="flex items-center justify-end gap-2">
          <Button variant="ghost" size="sm" onClick={() => setEditing(false)} disabled={saving}>
            {t("Cancel")}
          </Button>
          <Button size="sm" onClick={() => void save()} disabled={saving || draft === (task.content ?? "")}>
            {saving ? t("Saving...") : t("Save")}
          </Button>
        </div>
      </div>
    );
  }

  if (!task.content) {
    return (
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
          <Button variant="outline" size="sm" className="mt-3" onClick={startEditing}>
            <Pencil data-icon="inline-start" />
            {t("Write a description")}
          </Button>
        </AnimatedEmptyState.Content>
      </AnimatedEmptyState>
    );
  }

  return (
    <div className="flex flex-col gap-1">
      <div className="flex justify-end">
        <Button
          variant="ghost"
          size="sm"
          className="h-7 gap-1.5 px-2 text-xs text-muted-foreground"
          onClick={startEditing}
        >
          <Pencil className="size-3.5" />
          {t("Edit")}
        </Button>
      </div>
      <MarkdownRenderer content={task.content} />
    </div>
  );
}
