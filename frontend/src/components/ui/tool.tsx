"use client";

import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { cn } from "@/lib/utils";
import {
  CheckCircleIcon,
  ChevronDownIcon,
  XCircleIcon,
} from "lucide-react";
import type { ComponentProps, ReactNode } from "react";
import { motion, AnimatePresence } from "motion/react";
import { CodeBlock } from "./code-block";
import { DotmSquare4 } from "./dotm-square-4";
import { Shimmer } from "./shimmer";
import { AgentToolThinkingHelper } from "@/features/agent/presentation/helpers/agent-tool-thinking.helper";

export type ToolProps = ComponentProps<typeof Collapsible>;

export const Tool = ({ className, ...props }: ToolProps) => (
  <Collapsible
    className={cn("group not-prose mb-4 w-full transition-all duration-500", className)}
    {...props}
  />
);

/**
 * Three states, not the original's seven. The AI SDK tracks a tool call's
 * approval/streaming lifecycle inline as part state; AOS's Part struct only
 * ever carries two facts on the wire — a tool-call part, and (once it
 * exists) its paired tool-result part — because approval is a separate
 * channel (ADR-0007's ApprovalModal), not encoded here. "error" is a
 * heuristic: the runtime (agentloop/loop.go) doesn't persist a distinct
 * error flag onto the Part, only an Output shaped like {"error": "..."} on
 * failure — detected in isErrorOutput below.
 */
export type ToolState = "running" | "done" | "error";

const statusLabels: Record<ToolState, string> = {
  running: "Running",
  done: "Completed",
  error: "Error",
};

const statusVisuals: Record<ToolState, { icon: ReactNode; tone: string }> = {
  running: {
    icon: <DotmSquare4 size={12} dotSize={1.5} speed={1.35} />,
    tone: "text-foreground/70",
  },
  done: {
    icon: <CheckCircleIcon className="size-3.5" />,
    tone: "text-emerald-500/90",
  },
  error: {
    icon: <XCircleIcon className="size-3.5" />,
    tone: "text-destructive",
  },
};

/** Whether a tool-result's output looks like the runtime's error encoding. */
export function isErrorOutput(output: unknown): boolean {
  if (!output || typeof output !== "object" || Array.isArray(output)) return false;
  const keys = Object.keys(output as Record<string, unknown>);
  return keys.length > 0 && keys.every((k) => k === "error") && typeof (output as Record<string, unknown>).error === "string";
}

function ToolStatusIndicator({ state, label }: { state: ToolState; label?: string }) {
  const visual = statusVisuals[state];

  return (
    <div className="flex shrink-0 items-center pl-1" aria-live="polite">
      <AnimatePresence mode="wait" initial={false}>
        <motion.span
          key={state}
          initial={{ opacity: 0, y: 3, scale: 0.96 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
          exit={{ opacity: 0, y: -3, scale: 0.96 }}
          transition={{ duration: 0.2, ease: [0.22, 1, 0.36, 1] }}
          className={cn(
            "inline-flex items-center gap-1.5 text-[11px] font-medium tracking-tight whitespace-nowrap",
            visual.tone,
          )}
        >
          {visual.icon}
          <span>{label ?? statusLabels[state]}</span>
        </motion.span>
      </AnimatePresence>
    </div>
  );
}

export interface ToolHeaderProps {
  toolName: string;
  state: ToolState;
  reasoning?: string;
  title?: string;
  className?: string;
}

export const ToolHeader = ({ className, title, reasoning, toolName, state }: ToolHeaderProps) => {
  const config = AgentToolThinkingHelper.getToolConfig(toolName);
  const isRunning = state === "running";
  const displayTitle = title ?? config.title;

  return (
    <CollapsibleTrigger
      className={cn(
        "flex w-full items-center justify-between gap-3 p-1.5 px-0 transition-all duration-500 group/trigger",
        className,
      )}
    >
      <div className="flex items-center gap-2 min-w-0 flex-1">
        <div className="flex items-center justify-start gap-2 min-w-0 flex-1 border-l border-border/20">
          <AnimatePresence mode="wait">
            <motion.span
              key={isRunning ? "running" : "idle"}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.6 }}
              className="text-sm font-medium text-foreground/50 shrink-0"
            >
              {isRunning ? <Shimmer className="text-foreground/70">{displayTitle}</Shimmer> : displayTitle}
            </motion.span>
          </AnimatePresence>

          {reasoning && (
            <AnimatePresence mode="wait">
              <motion.span
                key={`${state}-${reasoning}`}
                initial={{ opacity: 0, x: -2 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: 2 }}
                transition={{ duration: 0.8, ease: "easeInOut" }}
                className="flex justify-start text-sm text-muted-foreground/40 truncate font-normal flex-1"
              >
                {reasoning}
              </motion.span>
            </AnimatePresence>
          )}
        </div>
      </div>

      <ToolStatusIndicator state={state} />
      <ChevronDownIcon className="size-3.5 shrink-0 text-muted-foreground/40 transition-transform group-data-[state=open]/trigger:rotate-180" />
    </CollapsibleTrigger>
  );
};

export type ToolContentProps = ComponentProps<typeof CollapsibleContent>;

export const ToolContent = ({ className, children, ...props }: ToolContentProps) => (
  <CollapsibleContent className={cn("overflow-hidden", className)} {...props}>
    <motion.div
      initial={{ opacity: 0, y: -4 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -4 }}
      transition={{ duration: 0.5, ease: "easeOut" }}
      className="space-y-4 p-2 text-popover-foreground outline-none"
    >
      {children}
    </motion.div>
  </CollapsibleContent>
);

export type ToolInputProps = ComponentProps<"div"> & { input: unknown };

export const ToolInput = ({ className, input, ...props }: ToolInputProps) => (
  <div className={cn("space-y-1.5 overflow-hidden", className)} {...props}>
    <h4 className="font-bold text-[9px] text-muted-foreground/30 uppercase tracking-widest pl-1">
      Parameters
    </h4>
    <div className="rounded-md bg-muted/20 border border-border/10 overflow-hidden">
      <CodeBlock code={JSON.stringify(input, null, 2)} language="json" />
    </div>
  </div>
);

export type ToolOutputProps = ComponentProps<"div"> & { output: unknown };

export const ToolOutput = ({ className, output, ...props }: ToolOutputProps) => {
  if (output === undefined || output === null) return null;

  const errored = isErrorOutput(output);
  const errorText = errored ? (output as { error: string }).error : undefined;

  let Output = <div>{String(output)}</div>;
  if (typeof output === "object") {
    Output = <CodeBlock code={JSON.stringify(output, null, 2)} language="json" />;
  } else if (typeof output === "string") {
    Output = <CodeBlock code={output} language="json" />;
  }

  return (
    <div className={cn("space-y-1.5", className)} {...props}>
      <h4 className="font-bold text-[9px] text-muted-foreground/30 uppercase tracking-widest pl-1">
        {errored ? "Error" : "Result"}
      </h4>
      <div
        className={cn(
          "overflow-x-auto rounded-md text-xs [&_table]:w-full border border-border/10",
          errored ? "bg-destructive/5 text-destructive border-destructive/10" : "bg-muted/20 text-foreground",
        )}
      >
        {errorText && <div className="p-2 font-medium text-sm">{errorText}</div>}
        {Output}
      </div>
    </div>
  );
};
