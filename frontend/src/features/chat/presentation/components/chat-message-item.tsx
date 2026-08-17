import * as React from "react";
import {
  Avatar,
  AvatarFallback,
  AvatarAgentFallback,
} from "@/components/ui/avatar";
import {
  Message,
  MessageActions,
  MessageAction,
  MessageContent,
} from "@/components/ui/message";
import { cn } from "@/lib/utils";
import {
  AudioLines,
  CheckIcon,
  CopyIcon,
  FileCode2,
  FileImage,
  FileVideo,
  Link2,
} from "lucide-react";
import type { Agent } from "@/features/agent/interfaces/agent.interfaces";
import { AgentToolThinkingHelper } from "@/features/agent/presentation/helpers/agent-tool-thinking.helper";
import type { AgentThinkingSummary } from "@/features/agent/presentation/helpers/agent-tool-thinking.helper";
import type { Chat, Message as ChatMessage, Part } from "@/features/chat/interfaces/chat.interfaces";
import { ToolBlock } from "@/features/chat/presentation/components/message/tool-part";
import { ChatThreadHelper } from "@/features/chat/presentation/helpers/chat-thread.helper";
import { ChatInlineMarkupHelper } from "@/features/chat/presentation/helpers/chat-inline-markup.helper";
import { toast } from "sonner";
import { VirtualMarkdownRenderer } from "@/components/ui/markdown-content";
import {
  ThinkingStep,
  ThinkingStepDetails,
  ThinkingSteps,
  ThinkingStepsContent,
  ThinkingStepsHeader,
} from "@/components/ui/thinking-steps";
import { Accordion, AccordionItem } from "@/components/ui/accordion";
import { SlidingNumber } from "@/components/ui/sliding-number";
import { motion, useReducedMotion } from "motion/react";

function pluralizeThinkingLabel(count: number, singular: string): string {
  if (count === 1) return singular;
  if (/[sxz]$|[cs]h$/.test(singular)) return `${singular}es`;
  return `${singular}s`;
}

function getDurationParts(elapsedMs: number): Array<{ label: string; value: number }> {
  const totalSeconds = Math.max(0, Math.floor(elapsedMs / 1000));
  const days = Math.floor(totalSeconds / 86400);
  const hours = Math.floor((totalSeconds % 86400) / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (days > 0) return [{ label: "d", value: days }, { label: "h", value: hours }];
  if (hours > 0) return [{ label: "h", value: hours }, { label: "m", value: minutes }];
  if (minutes > 0) return [{ label: "m", value: minutes }, { label: "s", value: seconds }];
  return [{ label: "s", value: seconds }];
}

function AgentThinkingHeader({ summary }: { summary: AgentThinkingSummary }) {
  const durationParts = getDurationParts(summary.elapsedMs);
  const counters = [
    { count: summary.reads, label: "read" },
    { count: summary.writes, label: "write" },
    { count: summary.searches, label: "search" },
    { count: summary.executions, label: "run" },
    { count: summary.browsing, label: "browse" },
    { count: summary.errors, label: "error" },
  ].filter((item) => item.count > 0);
  const prefix = summary.isRunning ? "Working for" : "Worked for";

  return (
    <span className="flex flex-wrap items-center gap-x-1.5 gap-y-0.5">
      <span>{prefix}</span>
      <span className="inline-flex items-center gap-1 tabular-nums text-foreground/80">
        {durationParts.map((part) => (
          <span key={part.label} className="inline-flex items-baseline">
            <SlidingNumber value={part.value} />
            <span>{part.label}</span>
          </span>
        ))}
      </span>
      {counters.map((item) => (
        <React.Fragment key={item.label}>
          <span className="text-muted-foreground/50">•</span>
          <span className="inline-flex items-center gap-1">
            <SlidingNumber value={item.count} />
            <span>{pluralizeThinkingLabel(item.count, item.label)}</span>
          </span>
        </React.Fragment>
      ))}
    </span>
  );
}

/**
 * Grouped by emoji, display-only. AOS's Message carries Reactions
 * (internal/domain/chat/entity.go) but no command sets one yet — no
 * chats_react equivalent exists — so there's nothing to add or toggle here.
 * What renders is honest: whatever reactions the data actually has, with no
 * affordance implying a capability that isn't there.
 */
interface GroupedReaction {
  emoji: string;
  count: number;
  actors: string[];
}

function groupMessageReactions(reactions: ChatMessage["reactions"]): GroupedReaction[] {
  const grouped = new Map<string, GroupedReaction>();
  for (const reaction of reactions ?? []) {
    const current = grouped.get(reaction.value);
    if (current) {
      current.count += 1;
      current.actors.push(reaction.actor);
      continue;
    }
    grouped.set(reaction.value, { emoji: reaction.value, count: 1, actors: [reaction.actor] });
  }
  return [...grouped.values()];
}

export interface ChatMessageItemProps {
  agents: Agent[];
  chat: Chat;
  message: ChatMessage;
  previousMessage?: ChatMessage;
  userName: string;
  selfUserId?: string;
  /** True for messages that were already there on initial load — skips the mount animation. */
  animateIn?: boolean;
}

export function ChatMessageItem({
  agents,
  chat,
  message,
  previousMessage,
  userName,
  selfUserId,
  animateIn = true,
}: ChatMessageItemProps) {
  const [copied, setCopied] = React.useState(false);
  const copyTimeoutRef = React.useRef<number | null>(null);
  const isUserMessage = message.role === "user";
  const isAgentMessage = message.role === "assistant" || message.author?.type === "agent";
  const participant = ChatThreadHelper.resolveParticipant({ agents, chat, message, userName, selfUserId });

  const parts = message.parts ?? [];
  const texts = parts
    .filter((part) => part.type === "text")
    .map((part) => part.text?.trim() ?? "")
    .filter((text) => text.length > 0 && !text.startsWith("[system-reminder]:"));

  const agentThinkingSteps = React.useMemo(() => AgentToolThinkingHelper.toThinkingSteps(message), [message]);
  const { finalTextPart } = React.useMemo(() => AgentToolThinkingHelper.splitMessageParts(message), [message]);
  const isAgentProcessing = isAgentMessage && AgentToolThinkingHelper.isRunning(message);
  const agentThinkingSummary = React.useMemo(
    () => AgentToolThinkingHelper.getSummary(message),
    [message],
  );
  const shouldOpenThinkingSteps = isAgentProcessing || agentThinkingSteps.some((step) => step.status === "error");
  const hasRenderablePart =
    agentThinkingSteps.length > 0 || texts.length > 0 || parts.some((p) => p.type === "file");

  const timestamp = ChatThreadHelper.getMessageTimestamp(message);
  const previousTimestamp = previousMessage ? ChatThreadHelper.getMessageTimestamp(previousMessage) : null;
  const previousParticipant = previousMessage
    ? ChatThreadHelper.resolveParticipant({ agents, chat, message: previousMessage, userName, selfUserId })
    : null;

  const isGroupedWithPrevious = ChatThreadHelper.isGroupedWithNeighbor({
    participantId: participant.id,
    neighborParticipantId: previousParticipant?.id,
    timestamp,
    neighborTimestamp: previousTimestamp,
  });

  const isCompact = isGroupedWithPrevious;
  const reactions = React.useMemo(() => groupMessageReactions(message.reactions), [message.reactions]);
  const showActionsRow = reactions.length > 0;
  const reduceMotion = useReducedMotion();
  const shouldAnimateIn = animateIn && !reduceMotion;

  React.useEffect(() => {
    return () => {
      if (copyTimeoutRef.current) window.clearTimeout(copyTimeoutRef.current);
    };
  }, []);

  const renderedThinkingSteps = React.useMemo(
    () =>
      agentThinkingSteps.map((step, index) => {
        const hasDetails = Boolean(step.details && step.details.length > 0);
        return hasDetails ? (
          <Accordion key={step.id} type="single" collapsible className="w-full max-w-full">
            <AccordionItem value={`details-${step.id}`} className="[&>.absolute]:hidden">
              <ThinkingStep
                icon={step.icon}
                label={step.label}
                description={step.description}
                descriptionFade={Boolean(step.description && hasDetails)}
                trailing={<ThinkingStepDetails summary="Details" mode="trigger" />}
                status={step.status}
                index={index}
                isLast={index === agentThinkingSteps.length - 1}
              >
                <ThinkingStepDetails summary="Details" details={step.details} mode="content" className="pt-1" />
              </ThinkingStep>
            </AccordionItem>
          </Accordion>
        ) : (
          <ThinkingStep
            key={step.id}
            icon={step.icon}
            label={step.label}
            description={step.description}
            status={step.status}
            index={index}
            isLast={index === agentThinkingSteps.length - 1}
          />
        );
      }),
    [agentThinkingSteps],
  );

  if (!hasRenderablePart) return null;

  async function handleCopy() {
    const fullText = texts.join("\n\n");
    if (!fullText) return;
    try {
      await navigator.clipboard.writeText(fullText);
      setCopied(true);
      toast.success("Message copied");
      if (copyTimeoutRef.current) window.clearTimeout(copyTimeoutRef.current);
      copyTimeoutRef.current = window.setTimeout(() => setCopied(false), 1600);
    } catch {
      toast.error("Could not copy this message");
    }
  }

  function renderTextPart(text: string, key: string) {
    return (
      <article key={key} className="my-0">
        <VirtualMarkdownRenderer content={ChatInlineMarkupHelper.toDisplayMarkdown(text)} />
      </article>
    );
  }

  function renderFilePart(part: Part, key: string) {
    const mediaType = part.mediaType || "application/octet-stream";
    const isAudio = mediaType.startsWith("audio/");
    const isImage = mediaType.startsWith("image/");
    const isVideo = mediaType.startsWith("video/");
    const Icon = isAudio ? AudioLines : isImage ? FileImage : isVideo ? FileVideo : FileCode2;
    const filename = part.uri?.split("/").pop() || "Attachment";

    return (
      <div key={key} className="overflow-hidden rounded-2xl border border-border/70 bg-background/80 shadow-xs">
        {isAudio ? (
          <div className="flex flex-col gap-3 p-3">
            <div className="flex items-center gap-3">
              <div className="flex size-9 items-center justify-center rounded-md bg-primary/10 text-primary">
                <Icon className="size-4" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-foreground">{filename}</p>
                <p className="text-xs text-muted-foreground">{mediaType}</p>
              </div>
            </div>
            <audio controls className="w-full" src={part.uri} />
          </div>
        ) : (
          <a
            className="flex items-center gap-3 p-3 transition-colors hover:bg-muted/40"
            download={filename}
            href={part.uri}
            rel="noreferrer"
            target="_blank"
          >
            <div className="flex size-9 items-center justify-center rounded-md bg-muted text-muted-foreground">
              <Icon className="size-4" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-foreground">{filename}</p>
              <p className="text-xs text-muted-foreground">{mediaType}</p>
            </div>
            <Link2 className="size-4 text-muted-foreground" />
          </a>
        )}
      </div>
    );
  }

  function renderAgentMessageParts() {
    return (
      <>
        {agentThinkingSteps.length > 0 ? (
          <ThinkingSteps className="my-1 w-full max-w-full -ml-3" defaultOpen={shouldOpenThinkingSteps}>
            <ThinkingStepsHeader className="py-1 text-xs text-muted-foreground hover:text-foreground">
              <AgentThinkingHeader summary={agentThinkingSummary} />
            </ThinkingStepsHeader>
            <ThinkingStepsContent className="pt-1">{renderedThinkingSteps}</ThinkingStepsContent>
          </ThinkingSteps>
        ) : null}

        {finalTextPart ? renderTextPart(finalTextPart.part.text?.trim() ?? "", `${message.id}-final-text-${finalTextPart.index}`) : null}

        {parts.map((part, index) => (part.type === "file" ? renderFilePart(part, `${message.id}-file-${index}`) : null))}
      </>
    );
  }

  return (
    <motion.div
      className="w-full"
      initial={shouldAnimateIn ? { opacity: 0, y: 6, filter: "blur(3px)" } : false}
      animate={{ opacity: 1, y: 0, filter: "blur(0px)" }}
      transition={{ duration: 0.3, ease: [0.22, 1, 0.36, 1] }}
    >
      <Message className="relative w-full gap-0" from={message.role === "tool" ? "assistant" : message.role}>
        <div
          className={cn(
            "grid grid-cols-[1.25rem_minmax(0,1fr)] gap-x-3 px-6",
            isCompact ? "pt-0.5 pb-1" : isUserMessage ? "pt-2.5 pb-2" : "pt-3 pb-2",
          )}
        >
          <div className="flex w-5 shrink-0 justify-center">
            {isCompact ? (
              <span className="pt-0.5 text-[10px] leading-none text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100">
                {timestamp ? ChatThreadHelper.formatMessageTime(timestamp) : ""}
              </span>
            ) : (
              <Avatar className="size-5 rounded-md">
                {participant.kind === "agent" ? (
                  <AvatarAgentFallback size={20} name={participant.id} />
                ) : (
                  <AvatarFallback className="text-[10px] font-semibold bg-secondary text-foreground">
                    {ChatThreadHelper.getInitials(participant.label)}
                  </AvatarFallback>
                )}
              </Avatar>
            )}
          </div>

          <div className="relative min-w-0">
            {!isCompact ? (
              <div className="mb-1 flex items-baseline gap-2">
                <span className="truncate text-xs font-medium text-foreground">{participant.label}</span>
                <span className="text-[11px] text-muted-foreground">
                  {timestamp ? ChatThreadHelper.formatMessageTime(timestamp) : ""}
                </span>
              </div>
            ) : null}

            <MessageContent
              className={cn(
                "w-full max-w-none overflow-visible bg-transparent px-0 py-0 gap-1",
                "group-[.is-user]:ml-0 group-[.is-user]:w-full group-[.is-user]:rounded-none group-[.is-user]:bg-transparent group-[.is-user]:px-0 group-[.is-user]:py-0",
              )}
            >
              <div className="flex flex-col gap-1">
                {isAgentMessage
                  ? renderAgentMessageParts()
                  : parts.map((part, index) => {
                      if (part.type === "text") {
                        const text = part.text?.trim() ?? "";
                        if (!text || text.startsWith("[system-reminder]:")) return null;
                        return renderTextPart(text, `${message.id}-text-${index}`);
                      }
                      if (part.type === "tool-call") {
                        const result = part.toolCallId
                          ? parts.find((p) => p.type === "tool-result" && p.toolCallId === part.toolCallId)
                          : undefined;
                        return <ToolBlock key={`${message.id}-tool-${index}`} call={part} result={result} />;
                      }
                      if (part.type === "file") {
                        return renderFilePart(part, `${message.id}-file-${index}`);
                      }
                      return null;
                    })}
              </div>
            </MessageContent>

            <div className="mt-1 flex items-center gap-1">
              {showActionsRow ? (
                <MessageActions className="flex-wrap gap-0.5 h-8 px-2 bg-card/50 rounded-sm">
                  {reactions.map((reaction) => (
                    <span
                      key={reaction.emoji}
                      className="inline-flex h-5 items-center gap-0.5 rounded-sm px-1 text-muted-foreground"
                      title={reaction.actors.join(", ")}
                    >
                      <span className="text-[11px] leading-none">{reaction.emoji}</span>
                      <span className="text-[10px] font-medium tabular-nums">{reaction.count}</span>
                    </span>
                  ))}
                </MessageActions>
              ) : null}

              <div
                className={cn(
                  "flex w-fit items-center gap-1",
                  "opacity-0 pointer-events-none",
                  "transition-[opacity,transform] duration-200 ease-[cubic-bezier(0.22,1,0.36,1)]",
                  !reduceMotion && "translate-y-0.5",
                  "group-hover:opacity-100 group-hover:pointer-events-auto",
                  !reduceMotion && "group-hover:translate-y-0",
                  "focus-within:opacity-100 focus-within:pointer-events-auto",
                  !reduceMotion && "focus-within:translate-y-0",
                  "max-md:opacity-100 max-md:pointer-events-auto",
                  !reduceMotion && "max-md:translate-y-0",
                )}
              >
                <MessageActions className="ml-0.5 flex items-center gap-0 h-8 px-2 bg-card/50 rounded-sm">
                  <MessageAction
                    className="size-5 rounded-sm text-muted-foreground hover:bg-muted/50 hover:text-foreground"
                    disabled={texts.length === 0}
                    label={copied ? "Copied" : "Copy message"}
                    onClick={handleCopy}
                    tooltip={copied ? "Copied" : "Copy message"}
                    variant="ghost"
                  >
                    {copied ? <CheckIcon className="size-3" /> : <CopyIcon className="size-3" />}
                  </MessageAction>
                </MessageActions>
              </div>
            </div>
          </div>
        </div>
      </Message>
    </motion.div>
  );
}
