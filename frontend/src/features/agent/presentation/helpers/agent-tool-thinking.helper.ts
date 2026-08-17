import type { IconName } from "@/lib/icon-context";
import type { Message, Part, Run } from "@/features/chat/interfaces/chat.interfaces";

export type AgentThinkingActionKind =
  | "read"
  | "write"
  | "search"
  | "execute"
  | "manage"
  | "browse"
  | "other";

export interface AgentToolThinkingConfig {
  title: string;
  description: string;
  icon: IconName;
  category: string;
  action: AgentThinkingActionKind;
}

export interface AgentThinkingStepViewModel {
  id: string;
  kind: "reasoning" | "text" | "tool";
  label: string;
  description?: string;
  icon: IconName;
  status: "complete" | "active" | "pending" | "error";
  index: number;
  toolName?: string;
  details?: string[];
}

export interface AgentThinkingSummary {
  total: number;
  reads: number;
  writes: number;
  searches: number;
  executions: number;
  browsing: number;
  management: number;
  other: number;
  errors: number;
  elapsedMs: number;
  isRunning: boolean;
}

/**
 * Ported from the original's AgentToolThinkingHelper, restructured around two
 * real differences in AOS's Go entity (internal/domain/chat/entity.go), not
 * the AI SDK's UIMessage shape the original reads:
 *
 * 1. No per-part `state`. The AI SDK tracks a tool call's progress
 *    (input-streaming → input-available → output-available/-error) as one
 *    evolving part. AOS's Part is a flat struct with a Go-side
 *    PartToolCall/PartToolResult discriminator and no state machine — a tool
 *    invocation is two parts (call, result) paired by toolCallId. Status
 *    here is derived from that pairing: a call with no matching result yet
 *    is "active", a call with a result is "complete". This is not a
 *    simplification of the original's model, it's what a flat, replicated
 *    struct can express instead of an in-memory streaming state machine —
 *    and it does not need a --state field to make the same distinction.
 * 2. Runs live on the message directly (Message.Runs), not on a separate
 *    "agent" message pointing back at a sourceMessageId. Elapsed time reads
 *    the message's own last run.
 *
 * TOOL_CONFIGS keys are AOS's actual agent-facing tool names — verified
 * against the Go source, not assumed from the original: the six built-in
 * dev tools (internal/runtime/toolexec/tools/fs.go) kept the original's
 * names unchanged, and every domain command's tool name is exactly its
 * command key (internal/core/command/descriptor.go: Key() joins Group_Name),
 * the same keys already generated into lib/schema.ts. Original tool names
 * with no AOS equivalent yet (Web*, Browser*, Job control beyond the queue
 * commands, *Skill*, *Template*, *Collection*, *View*, *Instruction*,
 * *Toolset* — none of those domains exist as AOS commands) are not in this
 * map; getToolConfig's fallback names the raw tool instead of mislabeling it.
 */
export class AgentToolThinkingHelper {
  private static readonly TOOL_CONFIGS: Record<string, AgentToolThinkingConfig> = {
    // Built-in dev tools — unchanged names (internal/runtime/toolexec/tools/fs.go).
    Read: { title: "Read file", description: "Inspected file contents", icon: "search", category: "filesystem", action: "read" },
    Write: { title: "Write file", description: "Wrote a file", icon: "copy", category: "filesystem", action: "write" },
    Edit: { title: "Edit file", description: "Replaced text in a file", icon: "copy", category: "filesystem", action: "write" },
    Glob: { title: "Find files", description: "Matched files by pattern", icon: "search", category: "filesystem", action: "search" },
    Grep: { title: "Search content", description: "Searched inside files", icon: "search", category: "filesystem", action: "search" },
    Bash: { title: "Run command", description: "Ran a program from the allowlist", icon: "settings", category: "execution", action: "execute" },

    // activity
    activity_delete: { title: "Delete log entry", description: "Removed an entry from the log", icon: "mail", category: "activity", action: "write" },
    activity_get: { title: "Read log entry", description: "Read one entry in full", icon: "mail", category: "activity", action: "read" },
    activity_list: { title: "Read inbox", description: "Read the workspace inbox", icon: "mail", category: "activity", action: "read" },
    activity_purge: { title: "Purge log", description: "Dropped entries past the retention window", icon: "mail", category: "activity", action: "write" },
    activity_read: { title: "Mark entry read", description: "Marked one entry as read", icon: "mail", category: "activity", action: "write" },
    "activity_read-all": { title: "Mark inbox read", description: "Marked the whole inbox as read", icon: "mail", category: "activity", action: "write" },

    // agents
    agents_create: { title: "Create agent", description: "Created an agent", icon: "users", category: "agent", action: "write" },
    agents_delete: { title: "Delete agent", description: "Deleted an agent", icon: "users", category: "agent", action: "write" },
    agents_get: { title: "Read agent", description: "Read one agent, instructions included", icon: "users", category: "agent", action: "read" },
    agents_list: { title: "List agents", description: "Listed the agents of the workspace", icon: "users", category: "agent", action: "read" },
    agents_me: { title: "Resolve identity", description: "Resolved its own identity in the workspace", icon: "users", category: "agent", action: "read" },
    agents_update: { title: "Update agent", description: "Changed an agent", icon: "users", category: "agent", action: "write" },

    // approvals
    approvals_decide: { title: "Decide approval", description: "Allowed or refused a waiting tool call", icon: "shield", category: "approval", action: "manage" },
    approvals_list: { title: "List approvals", description: "Listed tool calls waiting for a decision", icon: "shield", category: "approval", action: "read" },

    // chats
    chats_create: { title: "Open conversation", description: "Opened a conversation", icon: "mail", category: "chat", action: "write" },
    chats_get: { title: "Read conversation", description: "Read one conversation", icon: "mail", category: "chat", action: "read" },
    chats_list: { title: "List conversations", description: "Listed conversations", icon: "mail", category: "chat", action: "read" },
    chats_send: { title: "Send message", description: "Wrote to a conversation", icon: "mail", category: "chat", action: "write" },

    // comments
    comments_create: { title: "Add comment", description: "Wrote a comment on a task", icon: "mail", category: "comment", action: "write" },
    comments_delete: { title: "Delete comment", description: "Removed a comment", icon: "mail", category: "comment", action: "write" },
    comments_get: { title: "Read comment", description: "Read one comment", icon: "mail", category: "comment", action: "read" },
    comments_list: { title: "Read discussion", description: "Read a task's discussion", icon: "mail", category: "comment", action: "read" },
    comments_update: { title: "Edit comment", description: "Rewrote a comment", icon: "mail", category: "comment", action: "write" },

    // config
    config_get: { title: "Read config", description: "Read the configuration", icon: "settings", category: "config", action: "read" },
    config_update: { title: "Update config", description: "Changed configuration fields", icon: "settings", category: "config", action: "write" },

    // gateway
    gateway_restart: { title: "Restart daemon", description: "Stopped the daemon and started it again", icon: "settings", category: "gateway", action: "execute" },
    gateway_start: { title: "Start daemon", description: "Started the daemon", icon: "settings", category: "gateway", action: "execute" },
    gateway_status: { title: "Check daemon", description: "Reported whether the daemon is running", icon: "settings", category: "gateway", action: "read" },
    gateway_stop: { title: "Stop daemon", description: "Stopped the daemon", icon: "settings", category: "gateway", action: "execute" },

    // jobs
    jobs_get: { title: "Read job", description: "Read one queued job", icon: "clock", category: "job", action: "read" },
    jobs_list: { title: "List jobs", description: "Listed the queued work", icon: "clock", category: "job", action: "read" },
    jobs_purge: { title: "Purge jobs", description: "Removed finished jobs older than the window", icon: "clock", category: "job", action: "write" },
    jobs_recover: { title: "Recover jobs", description: "Handed back work whose worker stopped reporting", icon: "clock", category: "job", action: "write" },
    jobs_stats: { title: "Queue stats", description: "Read the shape of the queue", icon: "clock", category: "job", action: "read" },

    // memories
    memories_forget: { title: "Forget memory", description: "Deprecated knowledge that no longer holds", icon: "brain", category: "memory", action: "write" },
    memories_graph: { title: "Map memory graph", description: "Mapped the cognitive graph", icon: "brain", category: "memory", action: "read" },
    memories_recall: { title: "Recall memories", description: "Scanned memories", icon: "brain", category: "memory", action: "search" },
    memories_reflect: { title: "Read memory", description: "Read one memory in full", icon: "brain", category: "memory", action: "read" },
    memories_store: { title: "Store memory", description: "Recorded durable knowledge", icon: "brain", category: "memory", action: "write" },

    // routines
    routines_create: { title: "Create routine", description: "Declared a routine", icon: "rotate-ccw", category: "routine", action: "write" },
    routines_delete: { title: "Delete routine", description: "Removed a routine and its runs", icon: "rotate-ccw", category: "routine", action: "write" },
    routines_fire: { title: "Run routine", description: "Ran a routine now", icon: "play", category: "routine", action: "execute" },
    routines_get: { title: "Read routine", description: "Read one routine", icon: "rotate-ccw", category: "routine", action: "read" },
    routines_list: { title: "List routines", description: "Listed the routines", icon: "rotate-ccw", category: "routine", action: "read" },
    routines_rotate: { title: "Rotate webhook token", description: "Minted a new webhook token", icon: "rotate-ccw", category: "routine", action: "write" },
    routines_runs: { title: "Read routine history", description: "Read a routine's audit history", icon: "rotate-ccw", category: "routine", action: "read" },
    routines_update: { title: "Update routine", description: "Changed a routine", icon: "rotate-ccw", category: "routine", action: "write" },

    // tasks
    tasks_branch: { title: "Cut worktree", description: "Cut the isolated checkout a task executes in", icon: "check", category: "task", action: "execute" },
    tasks_create: { title: "Create task", description: "Recorded a unit of work", icon: "check", category: "task", action: "write" },
    tasks_delete: { title: "Delete task", description: "Removed a task and everything under it", icon: "check", category: "task", action: "write" },
    tasks_get: { title: "Read task", description: "Read one task", icon: "check", category: "task", action: "read" },
    tasks_list: { title: "List tasks", description: "Listed the tasks", icon: "check", category: "task", action: "read" },
    "tasks_set-status": { title: "Move task", description: "Moved a task through its lifecycle", icon: "check", category: "task", action: "write" },
    tasks_update: { title: "Update task", description: "Changed what a task says", icon: "check", category: "task", action: "write" },

    // themes
    themes_delete: { title: "Delete theme", description: "Removed an installed theme", icon: "paintbrush", category: "theme", action: "write" },
    themes_get: { title: "Read theme", description: "Read one theme and what it renders to", icon: "paintbrush", category: "theme", action: "read" },
    themes_install: { title: "Install theme", description: "Added a theme", icon: "paintbrush", category: "theme", action: "write" },
    themes_list: { title: "List themes", description: "Listed the themes available", icon: "paintbrush", category: "theme", action: "read" },

    // todos
    todos_create: { title: "Add plan step", description: "Added a step to the plan", icon: "check", category: "todo", action: "write" },
    todos_delete: { title: "Remove plan step", description: "Removed a step from the plan", icon: "check", category: "todo", action: "write" },
    todos_get: { title: "Read plan step", description: "Read one step", icon: "check", category: "todo", action: "read" },
    todos_list: { title: "Read plan", description: "Read a task's plan", icon: "check", category: "todo", action: "read" },
    "todos_set-status": { title: "Move plan step", description: "Moved a step through its lifecycle", icon: "check", category: "todo", action: "write" },
    todos_update: { title: "Update plan step", description: "Changed a step's description", icon: "check", category: "todo", action: "write" },

    // workspace
    workspace_create: { title: "Register workspace", description: "Registered a workspace and laid it out", icon: "globe", category: "workspace", action: "write" },
    workspace_delete: { title: "Unregister workspace", description: "Unregistered a workspace", icon: "globe", category: "workspace", action: "write" },
    workspace_get: { title: "Read workspace", description: "Read one workspace", icon: "globe", category: "workspace", action: "read" },
    workspace_introspect: { title: "Register repository", description: "Registered the repository it is standing in", icon: "globe", category: "workspace", action: "write" },
    workspace_inventory: { title: "Survey workspace", description: "Saw what a workspace holds, by collection", icon: "globe", category: "workspace", action: "read" },
    workspace_list: { title: "List workspaces", description: "Listed the registered workspaces", icon: "globe", category: "workspace", action: "read" },
    workspace_update: { title: "Update workspace", description: "Changed fields of a workspace", icon: "globe", category: "workspace", action: "write" },
  };

  public static getToolConfig(toolName: string): AgentToolThinkingConfig {
    const normalizedName = this.normalizeToolName(toolName);
    return (
      this.TOOL_CONFIGS[normalizedName] ?? {
        title: normalizedName || "Use tool",
        description: "Used an agent tool",
        icon: "settings",
        category: "tool",
        action: "other",
      }
    );
  }

  public static normalizeToolName(tool: string | Pick<Part, "type" | "toolName">): string {
    if (typeof tool !== "string") {
      return tool.toolName ?? tool.type;
    }
    return tool;
  }

  public static isRenderableText(text: string | undefined): boolean {
    const value = text?.trim();
    return Boolean(value && !value.startsWith("[system-reminder]:"));
  }

  /**
   * Splits a message into thinking parts (reasoning + paired tool calls) and
   * the final user-facing text — the last renderable text part.
   */
  public static splitMessageParts(message: Pick<Message, "parts">): {
    thinkingParts: Array<{ part: Part; index: number }>;
    finalTextPart: { part: Part; index: number } | null;
  } {
    const parts = message.parts ?? [];
    const textPartIndexes = parts.flatMap((part, index) =>
      part.type === "text" && this.isRenderableText(part.text) ? [index] : [],
    );
    const finalTextIndex = textPartIndexes.at(-1);

    const thinkingParts = parts.flatMap((part, index) => {
      if (part.type === "reasoning") {
        return this.isRenderableText(part.text) ? [{ part, index }] : [];
      }
      if (part.type === "tool-call") {
        return [{ part, index }];
      }
      if (part.type === "text" && this.isRenderableText(part.text) && index !== finalTextIndex) {
        return [{ part, index }];
      }
      return [];
    });

    return {
      thinkingParts,
      finalTextPart: finalTextIndex === undefined ? null : { part: parts[finalTextIndex]!, index: finalTextIndex },
    };
  }

  /** Converts a message's reasoning/tool parts into ThinkingStep view models. */
  public static toThinkingSteps(message: Pick<Message, "id" | "parts">): AgentThinkingStepViewModel[] {
    const { thinkingParts } = this.splitMessageParts(message);
    const parts = message.parts ?? [];

    return thinkingParts.map(({ part, index }, displayIndex) => {
      if (part.type === "tool-call") {
        const result = part.toolCallId
          ? parts.find((p) => p.type === "tool-result" && p.toolCallId === part.toolCallId)
          : undefined;
        return this.toToolStep(message.id, part, result, index, displayIndex);
      }

      if (part.type === "reasoning") {
        return {
          id: `${message.id}:reasoning:${index}`,
          kind: "reasoning",
          label: "Reasoning",
          description: this.compactText(part.text ?? ""),
          icon: "brain",
          status: "complete",
          index: displayIndex,
        };
      }

      return {
        id: `${message.id}:text:${index}`,
        kind: "text",
        label: "Drafted response",
        description: this.compactText(part.text ?? ""),
        icon: "dot",
        status: "complete",
        index: displayIndex,
      };
    });
  }

  public static getSummary(message: Message, nowMs = Date.now()): AgentThinkingSummary {
    const summary: AgentThinkingSummary = {
      total: 0,
      reads: 0,
      writes: 0,
      searches: 0,
      executions: 0,
      browsing: 0,
      management: 0,
      other: 0,
      errors: 0,
      elapsedMs: this.getElapsedMs(message, nowMs),
      isRunning: this.isRunning(message),
    };

    for (const part of message.parts ?? []) {
      if (part.type !== "tool-call") continue;

      const config = this.getToolConfig(this.normalizeToolName(part));
      summary.total += 1;

      if (config.action === "read") summary.reads += 1;
      else if (config.action === "write") summary.writes += 1;
      else if (config.action === "search") summary.searches += 1;
      else if (config.action === "execute") summary.executions += 1;
      else if (config.action === "browse") summary.browsing += 1;
      else if (config.action === "manage") summary.management += 1;
      else summary.other += 1;
    }

    const lastRun = (message.runs ?? []).at(-1);
    if (lastRun?.status === "error") summary.errors += 1;

    return summary;
  }

  public static getHeaderLabel(message: Message, nowMs = Date.now()): string {
    const summary = this.getSummary(message, nowMs);
    const prefix = summary.isRunning ? "Working for" : "Finished in";
    const counters = this.formatSummaryCounters(summary);
    return [`${prefix} ${this.formatElapsed(summary.elapsedMs)}`, counters].filter(Boolean).join(" • ");
  }

  public static formatElapsed(elapsedMs: number): string {
    const totalSeconds = Math.max(0, Math.floor(elapsedMs / 1000));
    const days = Math.floor(totalSeconds / 86400);
    const hours = Math.floor((totalSeconds % 86400) / 3600);
    const minutes = Math.floor((totalSeconds % 3600) / 60);
    const seconds = totalSeconds % 60;

    if (days > 0) return `${days}d ${hours}h`;
    if (hours > 0) return `${hours}h ${minutes}m`;
    if (minutes > 0) return `${minutes}m ${seconds}s`;
    return `${seconds}s`;
  }

  public static isRunning(message: Pick<Message, "runs">): boolean {
    return (message.runs ?? []).some((run) => run.status === "pending" || run.status === "running");
  }

  private static toToolStep(
    messageId: string,
    call: Part,
    result: Part | undefined,
    index: number,
    displayIndex: number,
  ): AgentThinkingStepViewModel {
    const toolName = this.normalizeToolName(call);
    const config = this.getToolConfig(toolName);
    const status: AgentThinkingStepViewModel["status"] = result ? "complete" : "active";

    return {
      id: `${messageId}:tool:${index}`,
      kind: "tool",
      label: config.title,
      description: this.describeToolPart(config, call),
      icon: config.icon,
      status,
      index: displayIndex,
      toolName,
      details: this.getToolDetails(call),
    };
  }

  private static describeToolPart(config: AgentToolThinkingConfig, part: Part): string {
    const reasoning = this.getReasoning(part.input);
    return reasoning ? this.compactText(reasoning) : config.description;
  }

  private static getToolDetails(part: Part): string[] {
    const details: string[] = [];
    if (part.input && typeof part.input === "object") {
      for (const key of ["file_path", "pattern", "query", "command", "task", "agent", "url"]) {
        const value = (part.input as Record<string, unknown>)[key];
        if (typeof value === "string" && value.trim()) {
          details.push(`${key}: ${this.compactText(value, 96)}`);
        }
      }
    }
    return details;
  }

  private static getReasoning(input: unknown): string | null {
    if (!input || typeof input !== "object") return null;
    const reasoning = (input as Record<string, unknown>)["_reasoning"];
    return typeof reasoning === "string" && reasoning.trim() ? reasoning : null;
  }

  private static compactText(text: string, maxLength = 160): string {
    const compacted = text.replace(/\s+/g, " ").trim();
    if (compacted.length <= maxLength) return compacted;
    return `${compacted.slice(0, maxLength - 1).trimEnd()}…`;
  }

  private static getElapsedMs(message: Pick<Message, "createdAt" | "runs">, nowMs: number): number {
    const run: Run | undefined = (message.runs ?? []).at(-1);
    const startedAt = this.parseDate(run?.startedAt) ?? this.parseDate(message.createdAt);
    if (!startedAt) return 0;

    if (run?.status === "pending" || run?.status === "running") {
      return Math.max(0, nowMs - startedAt);
    }

    const endMs = this.parseDate(run?.completedAt) ?? nowMs;
    return Math.max(0, endMs - startedAt);
  }

  private static parseDate(value: string | undefined): number | null {
    if (!value) return null;
    const timestamp = new Date(value).getTime();
    return Number.isNaN(timestamp) ? null : timestamp;
  }

  private static formatSummaryCounters(summary: AgentThinkingSummary): string {
    const parts = [
      this.pluralize(summary.reads, "read"),
      this.pluralize(summary.writes, "write"),
      this.pluralize(summary.searches, "search"),
      this.pluralize(summary.executions, "run"),
      this.pluralize(summary.browsing, "browse"),
      this.pluralize(summary.errors, "error"),
    ].filter(Boolean);
    return parts.join(" • ");
  }

  private static pluralize(count: number, singular: string): string | null {
    if (count <= 0) return null;
    if (count === 1) return `${count} ${singular}`;
    if (/[sxz]$|[cs]h$/.test(singular)) return `${count} ${singular}es`;
    return `${count} ${singular}s`;
  }
}
