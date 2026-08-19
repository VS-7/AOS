import type { IconName } from "@/lib/icon-context";
import type { FractalChatMessage } from "@/features/chat/interfaces/chat.interfaces";

/**
 * @description Action category used to summarize agent tool activity in chat thinking steps.
 *
 * @example
 * const kind: AgentThinkingActionKind = "read";
 */
export type AgentThinkingActionKind =
  | "read"
  | "write"
  | "search"
  | "execute"
  | "manage"
  | "browse"
  | "other";

/**
 * @description Visual and semantic configuration for one Fractal agent tool.
 *
 * @example
 * const config: AgentToolThinkingConfig = {
 *   title: "Read file",
 *   description: "Inspected file contents",
 *   icon: "search",
 *   category: "filesystem",
 *   action: "read",
 * };
 */
export interface AgentToolThinkingConfig {
  /** Human-readable title rendered in the thinking timeline. */
  title: string;
  /** Short description rendered below the step title. */
  description: string;
  /** Icon key supported by the shared icon context. */
  icon: IconName;
  /** Broad tool family used for fallback grouping. */
  category: string;
  /** Summary category used for header counters. */
  action: AgentThinkingActionKind;
}

/**
 * @description View model consumed by the chat message ThinkingSteps renderer.
 *
 * @example
 * const step: AgentThinkingStepViewModel = {
 *   id: "message:0",
 *   kind: "tool",
 *   label: "Read file",
 *   description: "Inspected package.json",
 *   icon: "search",
 *   status: "complete",
 *   index: 0,
 * };
 */
export interface AgentThinkingStepViewModel {
  /** Stable step identifier. */
  id: string;
  /** Source part kind represented by this step. */
  kind: "reasoning" | "text" | "tool";
  /** Main text shown in the step. */
  label: string;
  /** Optional supporting text shown under the label. */
  description?: string;
  /** Icon key supported by ThinkingStep. */
  icon: IconName;
  /** ThinkingStep status. */
  status: "complete" | "active" | "pending" | "error";
  /** Original display order. */
  index: number;
  /** Normalized tool name when the step represents a tool. */
  toolName?: string;
  /** Raw AI SDK state when available. */
  state?: string;
  /** Extra compact details suitable for nested disclosure in the future. */
  details?: string[];
}

/**
 * @description Aggregated agent activity counters and elapsed time for a message.
 *
 * @example
 * const summary: AgentThinkingSummary = {
 *   total: 4,
 *   reads: 2,
 *   writes: 1,
 *   searches: 1,
 *   executions: 0,
 *   browsing: 0,
 *   management: 0,
 *   other: 0,
 *   errors: 0,
 *   approvals: 0,
 *   elapsedMs: 4500,
 *   isRunning: false,
 * };
 */
export interface AgentThinkingSummary {
  /** Total tool steps in the message. */
  total: number;
  /** Count of read/inspection actions. */
  reads: number;
  /** Count of file or state mutation actions. */
  writes: number;
  /** Count of search actions. */
  searches: number;
  /** Count of shell or routine execution actions. */
  executions: number;
  /** Count of browser/web navigation actions. */
  browsing: number;
  /** Count of management CRUD/list/get actions. */
  management: number;
  /** Count of uncategorized tool actions. */
  other: number;
  /** Count of failed tool actions. */
  errors: number;
  /** Count of approval-gated tool actions. */
  approvals: number;
  /** Elapsed processing time in milliseconds. */
  elapsedMs: number;
  /** Whether the message is still being processed. */
  isRunning: boolean;
}

interface AgentMessagePart {
  type: string;
  text?: string;
  state?: string;
  toolName?: string;
  input?: unknown;
  output?: unknown;
  errorText?: string;
}

/**
 * @description Static helper for mapping Fractal agent tool parts into compact thinking UI.
 *
 * Centralizes tool metadata from `tools.md`, AI SDK state normalization, message part
 * splitting, elapsed-time formatting, and summary counters used by chat rendering.
 *
 * @example
 * const split = AgentToolThinkingHelper.splitMessageParts(message);
 * const steps = AgentToolThinkingHelper.toThinkingSteps(message);
 */
export class AgentToolThinkingHelper {
  private static readonly TOOL_CONFIGS: Record<string, AgentToolThinkingConfig> = {
    Read: {
      title: "Read file",
      description: "Inspected file contents",
      icon: "search",
      category: "filesystem",
      action: "read",
    },
    Write: {
      title: "Write file",
      description: "Created or replaced file content",
      icon: "copy",
      category: "filesystem",
      action: "write",
    },
    Edit: {
      title: "Edit file",
      description: "Changed existing file content",
      icon: "copy",
      category: "filesystem",
      action: "write",
    },
    Glob: {
      title: "Find files",
      description: "Matched files by path pattern",
      icon: "search",
      category: "filesystem",
      action: "search",
    },
    Grep: {
      title: "Search content",
      description: "Searched inside files",
      icon: "search",
      category: "filesystem",
      action: "search",
    },
    Bash: {
      title: "Run command",
      description: "Executed a shell command",
      icon: "settings",
      category: "execution",
      action: "execute",
    },
    JobList: {
      title: "List jobs",
      description: "Checked background jobs",
      icon: "clock",
      category: "jobs",
      action: "manage",
    },
    JobOutput: {
      title: "Read job output",
      description: "Inspected background job output",
      icon: "clock",
      category: "jobs",
      action: "read",
    },
    JobWait: {
      title: "Wait for job",
      description: "Waited for a background job",
      icon: "clock",
      category: "jobs",
      action: "execute",
    },
    JobStop: {
      title: "Stop job",
      description: "Stopped a background job",
      icon: "clock",
      category: "jobs",
      action: "write",
    },
    QueryTask: { title: "Query tasks", description: "Listed or read task records", icon: "check", category: "task", action: "read" },
    MutateTask: { title: "Mutate task", description: "Created, updated, transitioned, started, stopped, or deleted a task", icon: "check", category: "task", action: "write" },
    ManageTodo: { title: "Manage todos", description: "Listed, read, created, updated, transitioned, or deleted task todos", icon: "check", category: "todo", action: "write" },
    ManageComment: { title: "Manage comments", description: "Listed, read, added, updated, or deleted task comments", icon: "mail", category: "comment", action: "write" },
    ListAgents: { title: "List agents", description: "Checked available agents", icon: "users", category: "agent", action: "read" },
    GetAgent: { title: "Read agent", description: "Loaded agent details", icon: "users", category: "agent", action: "read" },
    CreateAgent: { title: "Create agent", description: "Created an agent", icon: "users", category: "agent", action: "write" },
    UpdateAgent: { title: "Update agent", description: "Updated an agent", icon: "users", category: "agent", action: "write" },
    DeleteAgent: { title: "Delete agent", description: "Deleted an agent", icon: "users", category: "agent", action: "write" },
    ListMemories: { title: "List memories", description: "Checked stored memories", icon: "brain", category: "memory", action: "read" },
    GetMemory: { title: "Read memory", description: "Loaded memory details", icon: "brain", category: "memory", action: "read" },
    CreateMemory: { title: "Create memory", description: "Stored a new memory", icon: "brain", category: "memory", action: "write" },
    ForgotMemory: { title: "Forget memory", description: "Deprecated a memory", icon: "brain", category: "memory", action: "write" },
    ListSkills: { title: "List skills", description: "Checked available skills", icon: "rocket", category: "skill", action: "read" },
    GetSkill: { title: "Read skill", description: "Loaded skill details", icon: "rocket", category: "skill", action: "read" },
    CreateSkill: { title: "Create skill", description: "Created a skill", icon: "rocket", category: "skill", action: "write" },
    UpdateSkill: { title: "Update skill", description: "Updated a skill", icon: "rocket", category: "skill", action: "write" },
    DeleteSkill: { title: "Delete skill", description: "Deleted a skill", icon: "rocket", category: "skill", action: "write" },
    DiscoverySkill: { title: "Discover skills", description: "Searched installable skills", icon: "rocket", category: "skill", action: "search" },
    InstallSkill: { title: "Install skill", description: "Installed a skill", icon: "rocket", category: "skill", action: "write" },
    ListInstructions: { title: "List instructions", description: "Checked workspace instructions", icon: "lightbulb", category: "instruction", action: "read" },
    GetInstruction: { title: "Read instruction", description: "Loaded instruction details", icon: "lightbulb", category: "instruction", action: "read" },
    CreateInstruction: { title: "Create instruction", description: "Created an instruction", icon: "lightbulb", category: "instruction", action: "write" },
    UpdateInstruction: { title: "Update instruction", description: "Updated an instruction", icon: "lightbulb", category: "instruction", action: "write" },
    DeleteInstruction: { title: "Delete instruction", description: "Deleted an instruction", icon: "lightbulb", category: "instruction", action: "write" },
    ListTemplates: { title: "List templates", description: "Checked templates", icon: "square-library", category: "template", action: "read" },
    GetTemplate: { title: "Read template", description: "Loaded template details", icon: "square-library", category: "template", action: "read" },
    CreateTemplate: { title: "Create template", description: "Created a template", icon: "square-library", category: "template", action: "write" },
    UpdateTemplate: { title: "Update template", description: "Updated a template", icon: "square-library", category: "template", action: "write" },
    DeleteTemplate: { title: "Delete template", description: "Deleted a template", icon: "square-library", category: "template", action: "write" },
    RenderTemplate: { title: "Render template", description: "Rendered template content", icon: "square-library", category: "template", action: "execute" },
    ListCollections: { title: "List collections", description: "Checked collections", icon: "square-library", category: "collection", action: "read" },
    GetCollection: { title: "Read collection", description: "Loaded collection details", icon: "square-library", category: "collection", action: "read" },
    CreateCollection: { title: "Create collection", description: "Created a collection", icon: "square-library", category: "collection", action: "write" },
    DeleteCollection: { title: "Delete collection", description: "Deleted a collection", icon: "square-library", category: "collection", action: "write" },
    ListCollectionRecords: { title: "List records", description: "Checked collection records", icon: "square-library", category: "record", action: "read" },
    GetCollectionRecord: { title: "Read record", description: "Loaded collection record", icon: "square-library", category: "record", action: "read" },
    CreateCollectionRecord: { title: "Create record", description: "Created collection record", icon: "square-library", category: "record", action: "write" },
    UpdateCollectionRecord: { title: "Update record", description: "Updated collection record", icon: "square-library", category: "record", action: "write" },
    DeleteCollectionRecord: { title: "Delete record", description: "Deleted collection record", icon: "square-library", category: "record", action: "write" },
    ListViews: { title: "List views", description: "Checked workspace views", icon: "rectangle-horizontal", category: "view", action: "read" },
    GetView: { title: "Read view", description: "Loaded view details", icon: "rectangle-horizontal", category: "view", action: "read" },
    CreateView: { title: "Create view", description: "Created a view", icon: "rectangle-horizontal", category: "view", action: "write" },
    RenderView: { title: "Render view", description: "Rendered a view", icon: "rectangle-horizontal", category: "view", action: "execute" },
    ListViewComponents: { title: "List view components", description: "Checked view components", icon: "rectangle-horizontal", category: "view", action: "read" },
    ListRoutines: { title: "List routines", description: "Checked routines", icon: "rotate-ccw", category: "routine", action: "read" },
    GetRoutine: { title: "Read routine", description: "Loaded routine details", icon: "rotate-ccw", category: "routine", action: "read" },
    CreateRoutine: { title: "Create routine", description: "Created a routine", icon: "rotate-ccw", category: "routine", action: "write" },
    UpdateRoutine: { title: "Update routine", description: "Updated a routine", icon: "rotate-ccw", category: "routine", action: "write" },
    DeleteRoutine: { title: "Delete routine", description: "Deleted a routine", icon: "rotate-ccw", category: "routine", action: "write" },
    FireRoutine: { title: "Run routine", description: "Fired a routine", icon: "play", category: "routine", action: "execute" },
    GetWorkspace: { title: "Read workspace", description: "Loaded workspace details", icon: "settings", category: "workspace", action: "read" },
    UpdateWorkspace: { title: "Update workspace", description: "Updated workspace settings", icon: "settings", category: "workspace", action: "write" },
    GetConfig: { title: "Read config", description: "Loaded configuration", icon: "settings", category: "config", action: "read" },
    UpdateConfig: { title: "Update config", description: "Updated configuration", icon: "settings", category: "config", action: "write" },
    ListToolsets: { title: "List toolsets", description: "Checked toolsets", icon: "settings", category: "toolset", action: "read" },
    GetToolset: { title: "Read toolset", description: "Loaded toolset details", icon: "settings", category: "toolset", action: "read" },
    CallToolset: { title: "Call toolset", description: "Called an external toolset", icon: "settings", category: "toolset", action: "execute" },
    WebSearch: { title: "Search web", description: "Searched web sources", icon: "globe", category: "web", action: "search" },
    WebFetch: { title: "Fetch webpage", description: "Read webpage content", icon: "globe", category: "web", action: "browse" },
    View: { title: "View file", description: "Inspected rich file content", icon: "image", category: "filesystem", action: "read" },
    BrowserOpen: { title: "Open browser", description: "Opened a browser session", icon: "globe", category: "browser", action: "browse" },
    BrowserList: { title: "List browser tabs", description: "Checked browser sessions", icon: "globe", category: "browser", action: "read" },
    BrowserNavigate: { title: "Navigate browser", description: "Navigated a browser page", icon: "globe", category: "browser", action: "browse" },
    BrowserScreenshot: { title: "Capture screenshot", description: "Captured browser pixels", icon: "image", category: "browser", action: "read" },
    BrowserEvaluate: { title: "Evaluate page", description: "Ran page JavaScript", icon: "settings", category: "browser", action: "execute" },
    BrowserClick: { title: "Click page", description: "Clicked a browser target", icon: "globe", category: "browser", action: "browse" },
    BrowserType: { title: "Type on page", description: "Typed into a browser target", icon: "globe", category: "browser", action: "write" },
    BrowserPress: { title: "Press key", description: "Pressed a browser key", icon: "globe", category: "browser", action: "browse" },
    BrowserScroll: { title: "Scroll page", description: "Scrolled a browser page", icon: "globe", category: "browser", action: "browse" },
    BrowserResize: { title: "Resize browser", description: "Changed browser viewport", icon: "globe", category: "browser", action: "write" },
    BrowserClose: { title: "Close browser", description: "Closed a browser session", icon: "globe", category: "browser", action: "write" },
  };

  private static readonly ACTIVE_STATES = new Set([
    "input-streaming",
    "input-available",
    "approval-requested",
    "streaming",
  ]);

  private static readonly ERROR_STATES = new Set(["output-error", "output-denied"]);

  /**
   * @description Returns the mapped display configuration for a tool.
   * @param toolName - AI SDK or Fractal tool name.
   * @returns Tool display configuration with a fallback for unknown tools.
   *
   * @example
   * AgentToolThinkingHelper.getToolConfig("Read").title;
   */
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

  /**
   * @description Normalizes a tool name from either a raw name or AI SDK part type.
   * @param tool - Raw tool name, part type, or tool-like message part.
   * @returns Canonical tool name without the `tool-` prefix.
   *
   * @example
   * AgentToolThinkingHelper.normalizeToolName("tool-Read"); // "Read"
   */
  public static normalizeToolName(tool: string | Pick<AgentMessagePart, "type" | "toolName">): string {
    if (typeof tool !== "string") {
      if (tool.type === "dynamic-tool" && tool.toolName) {
        return tool.toolName;
      }

      return this.normalizeToolName(tool.toolName ?? tool.type);
    }

    if (tool.startsWith("tool-")) {
      return tool.slice("tool-".length);
    }

    return tool === "dynamic-tool" ? "Tool" : tool;
  }

  /**
   * @description Checks whether a text part should be visible to users or thinking UI.
   * @param text - Raw text part content.
   * @returns True when the text is non-empty and not a system reminder.
   *
   * @example
   * AgentToolThinkingHelper.isRenderableText("hello"); // true
   */
  public static isRenderableText(text: string | undefined): boolean {
    const value = text?.trim();
    return Boolean(value && !value.startsWith("[system-reminder]:"));
  }

  /**
   * @description Splits a message into thinking parts and final user-facing text.
   * @param message - Chat message to split.
   * @returns Thinking parts plus the last renderable text part, if present.
   *
   * @example
   * const { thinkingParts, finalTextPart } = AgentToolThinkingHelper.splitMessageParts(message);
   */
  public static splitMessageParts(message: Pick<FractalChatMessage, "parts">): {
    thinkingParts: Array<{ part: AgentMessagePart; index: number }>;
    finalTextPart: { part: AgentMessagePart; index: number } | null;
  } {
    const parts = (message.parts ?? []) as AgentMessagePart[];
    const textPartIndexes = parts.flatMap((part, index) =>
      part.type === "text" && this.isRenderableText(part.text) ? [index] : [],
    );
    const finalTextIndex = textPartIndexes.at(-1);

    const thinkingParts = parts.flatMap((part, index) => {
      if (part.type === "reasoning") {
        return this.isRenderableText(part.text) ? [{ part, index }] : [];
      }

      if (this.isToolPart(part)) {
        return [{ part, index }];
      }

      if (
        part.type === "text" &&
        this.isRenderableText(part.text) &&
        index !== finalTextIndex
      ) {
        return [{ part, index }];
      }

      return [];
    });

    return {
      thinkingParts,
      finalTextPart:
        finalTextIndex === undefined
          ? null
          : { part: parts[finalTextIndex], index: finalTextIndex },
    };
  }

  /**
   * @description Converts all intermediate agent parts into ThinkingStep view models.
   * @param message - Chat message containing AI SDK parts.
   * @returns Ordered thinking step models.
   *
   * @example
   * const steps = AgentToolThinkingHelper.toThinkingSteps(message);
   */
  public static toThinkingSteps(message: Pick<FractalChatMessage, "id" | "parts">): AgentThinkingStepViewModel[] {
    const { thinkingParts } = this.splitMessageParts(message);

    return thinkingParts.map(({ part, index }, displayIndex) => {
      if (this.isToolPart(part)) {
        return this.toToolStep(message.id, part, index, displayIndex);
      }

      if (part.type === "reasoning") {
        return {
          id: `${message.id}:reasoning:${index}`,
          kind: "reasoning",
          label: "Reasoning",
          description: this.compactText(part.text ?? ""),
          icon: "brain",
          status: this.toStepStatus(part.state),
          index: displayIndex,
          state: part.state,
        };
      }

      return {
        id: `${message.id}:text:${index}`,
        kind: "text",
        label: "Drafted response",
        description: this.compactText(part.text ?? ""),
        icon: "dot",
        status: this.toStepStatus(part.state),
        index: displayIndex,
        state: part.state,
      };
    });
  }

  /**
   * @description Builds summary counters and elapsed state for an agent message.
   * @param message - Message to summarize.
   * @param nowMs - Current timestamp used for live running duration.
   * @returns Aggregated thinking summary.
   *
   * @example
   * const summary = AgentToolThinkingHelper.getSummary(message, Date.now());
   */
  public static getSummary(
    message: FractalChatMessage,
    nowMs = Date.now(),
    sourceMessage?: FractalChatMessage,
  ): AgentThinkingSummary {
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
      approvals: 0,
      elapsedMs: this.getElapsedMs(message, nowMs, sourceMessage),
      isRunning: this.isRunning(message),
    };

    for (const part of (message.parts ?? []) as AgentMessagePart[]) {
      if (!this.isToolPart(part)) {
        continue;
      }

      const config = this.getToolConfig(this.normalizeToolName(part));
      summary.total += 1;

      if (config.action === "read") summary.reads += 1;
      else if (config.action === "write") summary.writes += 1;
      else if (config.action === "search") summary.searches += 1;
      else if (config.action === "execute") summary.executions += 1;
      else if (config.action === "browse") summary.browsing += 1;
      else if (config.action === "manage") summary.management += 1;
      else summary.other += 1;

      if (part.state && this.ERROR_STATES.has(part.state)) {
        summary.errors += 1;
      }

      if (part.state === "approval-requested") {
        summary.approvals += 1;
      }
    }

    return summary;
  }

  /**
   * @description Formats the interactive ThinkingSteps header label.
   * @param message - Agent message being summarized.
   * @param nowMs - Current timestamp used for live running duration.
   * @returns Human-readable header label.
   *
   * @example
   * AgentToolThinkingHelper.getHeaderLabel(message, Date.now());
   */
  public static getHeaderLabel(
    message: FractalChatMessage,
    nowMs = Date.now(),
    sourceMessage?: FractalChatMessage,
  ): string {
    const summary = this.getSummary(message, nowMs, sourceMessage);
    const prefix = summary.isRunning ? "Working for" : "Finished in";
    const counters = this.formatSummaryCounters(summary);

    return [`${prefix} ${this.formatElapsed(summary.elapsedMs)}`, counters]
      .filter(Boolean)
      .join(" • ");
  }

  /**
   * @description Formats elapsed milliseconds for compact chat UI.
   * @param elapsedMs - Elapsed duration in milliseconds.
   * @returns Compact duration string.
   *
   * @example
   * AgentToolThinkingHelper.formatElapsed(72000); // "1m 12s"
   */
  public static formatElapsed(elapsedMs: number): string {
    const totalSeconds = Math.max(0, Math.floor(elapsedMs / 1000));
    const days = Math.floor(totalSeconds / 86400);
    const hours = Math.floor((totalSeconds % 86400) / 3600);
    const minutes = Math.floor((totalSeconds % 3600) / 60);
    const seconds = totalSeconds % 60;

    if (days > 0) {
      return `${days}d ${hours}h`;
    }

    if (hours > 0) {
      return `${hours}h ${minutes}m`;
    }

    if (minutes > 0) {
      return `${minutes}m ${seconds}s`;
    }

    return `${seconds}s`;
  }

  /**
   * @description Determines whether message processing is still running.
   * @param message - Chat message with processing metadata.
   * @returns True for streaming or running agent messages.
   *
   * @example
   * AgentToolThinkingHelper.isRunning(message);
   */
  public static isRunning(message: FractalChatMessage): boolean {
    if (message.metadata?.type === "agent") {
      return message.metadata.execution?.status === "running";
    }

    return (message.metadata?.runs ?? []).some(
      (run) => run.status === "pending" || run.status === "running",
    );
  }

  private static toToolStep(
    messageId: string,
    part: AgentMessagePart,
    index: number,
    displayIndex: number,
  ): AgentThinkingStepViewModel {
    const toolName = this.normalizeToolName(part);
    const config = this.getToolConfig(toolName);
    const status = this.toStepStatus(part.state);
    const label = this.getStateAwareLabel(config, part.state);

    return {
      id: `${messageId}:tool:${index}`,
      kind: "tool",
      label,
      description: this.describeToolPart(config, part),
      icon: config.icon,
      status,
      index: displayIndex,
      toolName,
      state: part.state,
      details: this.getToolDetails(part),
    };
  }

  private static getStateAwareLabel(config: AgentToolThinkingConfig, state?: string): string {
    if (state === "output-error") return `${config.title} failed`;
    if (state === "output-denied") return `${config.title} denied`;
    if (state === "approval-requested") return `Confirm ${config.title.toLowerCase()}`;
    return config.title;
  }

  private static describeToolPart(config: AgentToolThinkingConfig, part: AgentMessagePart): string {
    const reasoning = this.getReasoning(part.input);
    if (reasoning) {
      return this.compactText(reasoning);
    }

    if (part.errorText) {
      return this.compactText(part.errorText);
    }

    return config.description;
  }

  private static getToolDetails(part: AgentMessagePart): string[] {
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
    if (!input || typeof input !== "object") {
      return null;
    }

    const reasoning = (input as Record<string, unknown>)._reasoning;
    return typeof reasoning === "string" && reasoning.trim() ? reasoning : null;
  }

  private static compactText(text: string, maxLength = 160): string {
    const compacted = text.replace(/\s+/g, " ").trim();
    if (compacted.length <= maxLength) {
      return compacted;
    }

    return `${compacted.slice(0, maxLength - 1).trimEnd()}…`;
  }

  private static toStepStatus(state?: string): AgentThinkingStepViewModel["status"] {
    if (state && this.ERROR_STATES.has(state)) {
      return "error";
    }

    if (state && this.ACTIVE_STATES.has(state)) {
      return "active";
    }

    return "complete";
  }

  private static isToolPart(part: AgentMessagePart): boolean {
    return part.type === "dynamic-tool" || part.type.startsWith("tool-");
  }

  private static getElapsedMs(
    message: FractalChatMessage,
    nowMs: number,
    sourceMessage?: FractalChatMessage,
  ): number {
    const sourceCreatedAt = this.parseDate(sourceMessage?.metadata?.createdAt);
    const sourceUpdatedAt = this.parseDate(sourceMessage?.metadata?.updatedAt);
    const messageCreatedAt = this.parseDate(message.metadata?.createdAt);
    const messageUpdatedAt = this.parseDate(message.metadata?.updatedAt);
    const execution =
      message.metadata?.type === "agent" ? message.metadata.execution : undefined;
    const sourceRun =
      sourceMessage?.metadata?.runs?.find(
        (run) => run.jobId === execution?.jobId,
      ) ?? undefined;
    const sourceProcessingStartedAt = this.parseDate(sourceRun?.startedAt);
    const sourceCompletedAt = this.parseDate(sourceRun?.completedAt);
    const messageProcessingStartedAt = this.parseDate(execution?.startedAt);
    const messageCompletedAt = this.parseDate(execution?.completedAt);

    if (this.isRunning(message)) {
      const startedAt = sourceCreatedAt ?? messageCreatedAt ?? sourceProcessingStartedAt ?? messageProcessingStartedAt;

      if (!startedAt) {
        return 0;
      }

      return Math.max(0, nowMs - startedAt);
    }

    const startedAt =
      sourceProcessingStartedAt ??
      messageProcessingStartedAt ??
      sourceCreatedAt ??
      messageCreatedAt;
    const endMs =
      sourceUpdatedAt ??
      sourceCompletedAt ??
      messageUpdatedAt ??
      messageCompletedAt ??
      nowMs;

    if (!startedAt) {
      return 0;
    }

    return Math.max(0, endMs - startedAt);
  }

  private static parseDate(value: string | undefined): number | null {
    if (!value) {
      return null;
    }

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
    if (count <= 0) {
      return null;
    }

    if (count === 1) {
      return `${count} ${singular}`;
    }

    if (
      singular.endsWith("s") ||
      singular.endsWith("x") ||
      singular.endsWith("z") ||
      singular.endsWith("ch") ||
      singular.endsWith("sh")
    ) {
      return `${count} ${singular}es`;
    }

    return `${count} ${singular}s`;
  }
}
