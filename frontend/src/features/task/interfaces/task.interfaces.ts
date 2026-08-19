import { z } from "zod";
import type { ResponseWithCTA } from "@/core/interfaces/response.interfaces";
import type { Chat } from "@/features/chat/interfaces/chat.interfaces";
import { Schema } from "@/core/helpers/schema.helper";

/**
 * Valid statuses a task can transition through in its lifecycle.
 *
 * Lifecycle flow: suggestion → backlog → planning → todo → in_progress → stopped → in_review → finished
 *
 * - **suggestion** — Proactively suggested by an agent. Not yet committed.
 * - **backlog** — Saved for later. Acknowledged but not yet planned.
 * - **planning** — Agent is actively creating a plan and breaking down sub-tasks.
 * - **todo** — Ready to be executed. Plan is complete, waiting for an agent to pick it up.
 * - **in_progress** — Currently being executed by an agent in a sandbox.
 * - **stopped** — An in-progress run was interrupted; see `checkpoint` for where it left off.
 * - **in_review** — Execution complete. Waiting for user approval or feedback.
 * - **finished** — Successfully completed and closed.
 *
 * `stopped` is missing from this file's source extraction (`_extracted/
 * index/`, a server-side snapshot) but real: the Go backend's status enum
 * (`internal/domain/task/entity.go`) has it, and the ported `task`
 * presentation code (`consts/task.ts`, `($id)/components/main/index.tsx`)
 * already reads and renders it. Added here rather than left broken.
 */
export const FractalTaskStatusSchema = z.enum([
  "suggestion",
  "backlog",
  "planning",
  "todo",
  "in_progress",
  "stopped",
  "in_review",
  "finished",
]);

/** `FractalTaskStatusSchema`'s inferred type had no bare alias in this file's source, unlike its sibling `FractalTaskPriority` (see below) — added for the same reason the `stopped` value above was. */
export type FractalTaskStatus = z.infer<typeof FractalTaskStatusSchema>;

/**
 * Priority levels for a task. Determines urgency and execution order.
 *
 * - **urgent** — Must be addressed immediately. Blocks other work.
 * - **high** — Important and should be done soon. High visibility.
 * - **medium** — Normal priority. Default for most tasks.
 * - **low** — Nice to have. Can be deferred.
 * - **no_priority** — Not yet prioritized.
 */
export const FractalTaskPrioritySchema = z.enum([
  "no_priority",
  "urgent",
  "high",
  "medium",
  "low",
]);

/**
 * Supported finish-task Git operations.
 *
 * - **merge_to_main** — Merge the task's worktree branch into main and close it.
 * - **create_branch** — Create a branch from the task's worktree without merging.
 */
export const FractalTaskFinishOperationSchema = z.enum([
  "merge_to_main",
  "create_branch",
]);

/**
 * Valid statuses a todo can have.
 *
 * - **todo** — Ready to be executed. Waiting for an agent to pick it up.
 * - **in_progress** — Currently being executed by the assigned agent.
 * - **in_review** — Execution complete. Waiting for review or approval.
 * - **finished** — Successfully completed.
 */
export const FractalTodoStatusSchema = z.enum([
  "todo",
  "in_progress",
  "in_review",
  "finished",
]);

/**
 * Who authored a comment — used to distinguish user vs agent messages.
 */
export const FractalCommentAuthorTypeSchema = z.enum(["user", "agent"]);

/**
 * Schema for an attachment reference attached to a task or comment.
 * Attachments link external files, URLs, or resources to the task context.
 */
export const FractalAttachmentSchema = z.object({
  uri: z
    .string()
    .describe(
      "URI or file path of the attachment. Can be a local path or a remote URL. Examples: 'file://absolute/path/to/spec.pdf', 'uri://https://example.com/image.png'.",
    ),
  observation: z
    .string()
    .optional()
    .describe(
      "Optional note about the attachment. Use to describe what this attachment contains or why it's relevant.",
    ),
});

/**
 * Schema for a sub-task or step within a larger task's plan.
 * Todos break down a task into ordered, executable steps that an agent can pick up individually.
 */
export const FractalTaskTodoSchema = z.object({
  id: z
    .string()
    .describe(
      "Unique identifier for the todo item. Auto-generated on creation.",
    ),
  taskId: z
    .string()
    .optional()
    .describe(
      "ID of the parent task. Used for file path resolution and scoping.",
    ),
  description: z
    .string()
    .describe(
      "Description of the step or action. Be specific — this is what the assigned agent will execute.",
    ),
  status: FractalTodoStatusSchema.default("todo").describe(
    "Current status of this todo. Follows the lifecycle: todo → in_progress → in_review → finished.",
  ),
  agent: z
    .string()
    .optional()
    .describe(
      "ID of the agent responsible for executing this todo. Must match a registered Fractal agent ID (e.g., 'atlas').",
    ),
  instructions: z
    .string()
    .optional()
    .describe(
      "Prompt or instruction given to the assigned agent. Provides context and behavioral guidance for execution.",
    ),
  output: z
    .any()
    .optional()
    .describe(
      "Output result produced by the agent after execution. Auto-populated when the agent completes the todo.",
    ),
});

/**
 * Schema for a comment on a task.
 * Comments enable collaboration between users and agents within a task context.
 * They support Markdown, threaded replies, and attachments.
 */
export const FractalCommentSchema = z.object({
  id: z
    .string()
    .describe("Unique identifier for the comment. Auto-generated on creation."),
  taskId: z
    .string()
    .optional()
    .describe(
      "ID of the parent task. Used for file path resolution and scoping.",
    ),
  body: z
    .string()
    .describe(
      "Markdown content of the comment. Supports code blocks, lists, mentions (@username), and formatted text.",
    ),
  author: z
    .string()
    .describe(
      "Display name or ID of the author. For agents, use your Fractal agent name (e.g., 'Atlas').",
    ),
  authorType: FractalCommentAuthorTypeSchema.default("user").describe(
    "Whether this comment was written by a 'user' or an 'agent'. Determines how the comment is rendered and attributed.",
  ),
  agentId: z
    .string()
    .optional()
    .describe(
      "If authorType is 'agent', the agent's Fractal identifier (e.g., 'atlas'). Must match a registered agent.",
    ),
  parentId: z
    .string()
    .optional()
    .describe(
      "ID of the parent comment for threaded replies. Use to create conversation threads within a task.",
    ),
  attachments: z
    .array(FractalAttachmentSchema)
    .default([])
    .describe(
      "List of file or URL attachments for this comment. Use to link relevant resources.",
    ),
  createdAt: z
    .string()
    .describe("ISO timestamp when the comment was created. Auto-generated."),
  updatedAt: z
    .string()
    .describe(
      "ISO timestamp when the comment was last updated. Auto-generated.",
    ),
});

/**
 * Schema for a task's worktree configuration.
 * Worktrees create isolated Git branches for task-specific work, preventing conflicts.
 */
export const FractalTaskWorktreeSchema = z.object({
  enabled: z
    .boolean()
    .default(false)
    .describe(
      "Whether to create an isolated Git worktree for this task. When true, a separate branch and directory are created.",
    ),
  base: z
    .string()
    .optional()
    .describe(
      "Base branch to create the worktree from. Defaults to the current branch or 'main'.",
    ),
  branch: z
    .string()
    .optional()
    .describe(
      "Branch name override. Defaults to 'task/{id}' (e.g., 'task/FRA-012').",
    ),
  path: z
    .string()
    .optional()
    .describe(
      "Worktree path override. Defaults to a system-managed location within the workspace.",
    ),
});

/**
 * Primary schema for a Fractal Task record.
 * Tasks are the central unit of work in Fractal. They define what needs to be done,
 * who is responsible, the current status, and all metadata needed for execution.
 */
export const FractalTaskSchema = z.object({
  id: z
    .string()
    .describe(
      "Unique identifier for the task. Auto-generated (e.g., 'FRA-012'). Used in all task operations.",
    ),
  name: z
    .string()
    .describe(
      "Short, descriptive name of the task. Should clearly convey the goal (e.g., 'Fix login bug', 'Add dark mode').",
    ),
  assigned: z
    .string()
    .optional()
    .describe(
      "Person or agent currently responsible for the task. Use Fractal agent IDs (e.g., 'atlas') or user names.",
    ),
  dueAt: z
    .string()
    .optional()
    .describe(
      "ISO timestamp for when the task is due. Used for deadline tracking and prioritization.",
    ),
  slug: z
    .string()
    .describe(
      "URL-friendly version of the name. Auto-generated from the name if not provided (e.g., 'fix-login-bug').",
    ),
  type: z
    .string()
    .describe(
      "Classification of the task. Common values: 'bug', 'feature', 'refactor', 'docs', 'task', 'config'.",
    ),
  priority: FractalTaskPrioritySchema.describe(
    "Priority level of the task. Determines execution order and urgency.",
  ),
  summary: z
    .string()
    .optional()
    .describe(
      "A concise summary of the goal or intent. One or two sentences that capture the 'why' of this task.",
    ),
  content: z
    .string()
    .optional()
    .describe(
      "Detailed markdown description of the task requirements. Includes acceptance criteria, context, and implementation notes.",
    ),
  status: FractalTaskStatusSchema.describe(
    "Current lifecycle status of the task. Follows: suggestion → backlog → planning → todo → in_progress → in_review → finished.",
  ),
  template: z
    .string()
    .optional()
    .describe(
      "ID of the template used to scaffold the task. Templates pre-populate content and structure.",
    ),
  worktree: FractalTaskWorktreeSchema.describe(
    "Worktree configuration for this task. Controls Git isolation behavior.",
  ),
  chat: z
    .string()
    .optional()
    .describe(
      "ID of the chat associated with this task. Links task execution to a conversation context.",
    ),
  attachments: z
    .array(FractalAttachmentSchema)
    .default([])
    .describe(
      "List of file or URL attachments for this task. Use to link specs, screenshots, or references.",
    ),
  project: z
    .string()
    .optional()
    .describe(
      "ID (slug) of the project this task belongs to. Links the task to a project for filtering and organization. Example: 'my-app'.",
    ),
  goal: z
    .string()
    .optional()
    .describe(
      "ID (slug) of the goal this task contributes to. Links the task to a long-term objective for timeline tracking. Example: 'launch-v1'.",
    ),

  /**
   * The four fields below are absent from this file's source extraction
   * (`_extracted/index/`) but real on the Go side: `tasks_list`/`tasks_get`
   * both return `task.View` (`internal/domain/task/service.go`'s `List`/
   * `Get`), which embeds `Task` and adds `Assignee`/`Progress` — so every
   * task the frontend receives already has these at the top level, JSON-
   * flattened by Go's embedded-struct marshaling. Verified by reading
   * `internal/domain/task/entity.go` and `service.go` directly, not
   * guessed.
   */
  dependsOn: z
    .array(z.string())
    .optional()
    .describe(
      "IDs of tasks that must finish before this one starts. Mirrors the Go side's Task.DependsOn exactly.",
    ),
  assignee: z
    .object({
      id: z.string(),
      type: z.enum(["agent", "user", "unknown"]),
      name: z.string().optional(),
      role: z.string().optional(),
      // Go's `ResolvedAssignee` (entity.go) has only {id, type, name,
      // role} — no `image`/`username`. `presentation/helpers/assignee.
      // helper.ts`'s `resolveTaskAssignee` reads both anyway (assuming a
      // richer projection this backend doesn't have); kept optional here
      // so it compiles rather than editing that helper's logic. Both will
      // always be `undefined` against this backend.
      image: z.string().optional(),
      username: z.string().optional(),
    })
    .describe(
      "Read-only resolved-assignee projection (Go's task.View.Assignee) — always present on tasks_list/tasks_get responses, never written directly.",
    ),
  checkpoint: z
    .object({
      chatId: z.string().optional(),
      jobId: z.string().optional(),
      pendingTodoIds: z.array(z.string()).optional(),
      progress: z.object({ completed: z.number(), total: z.number() }),
      stoppedAt: z.string(),
      reason: z.string().optional(),
    })
    .optional()
    .describe(
      "Where an interrupted run stopped (Go's task.Checkpoint), present when status is 'stopped'. Field names match the Go struct's `json` tags exactly — NOT the richer {summary, at, actor, execution, resume} shape `($id)/components/main/index.tsx`'s original checkpoint banner assumed; that render was adapted to these real fields (see the port's hand-edit notes).",
    ),
  /**
   * `dependencies` (plural, resolved task summaries) is what
   * `dependencies-widget/index.tsx` reads for its "already linked" list —
   * distinct from `dependsOn` (id strings, real, above). The Go side has
   * no resolved-dependencies projection, only `DependsOn` and a computed
   * `Blocked []string` (unfinished subset) — neither is an array of full
   * task objects. Typed here only so the widget compiles; it will always
   * be `undefined` against this backend, so that list renders empty. A
   * real fix needs either a Go projection or the widget resolving each id
   * itself against `task.list`, out of scope for this port.
   *
   * Not `z.lazy(() => FractalTaskSchema)`: a self-reference inside this
   * same `z.object({...})` risks `FractalTaskSchema`'s inferred type
   * becoming circular (TS7022). The widget only reads `.id`, `.name`, and
   * `.status` off each entry, so that minimal shape is enough.
   */
  dependencies: z
    .array(z.object({ id: z.string(), name: z.string(), status: FractalTaskStatusSchema }))
    .optional(),
});

/**
 * Schema for creating a new task.
 * All fields are optional except `name` and `slug`, which are required to identify the task.
 */
export const FractalTaskCreateSchema = z.object({
  name: z
    .string()
    .describe(
      "Short, descriptive name of the task. Should clearly convey the goal (e.g., 'Fix login bug', 'Add dark mode').",
    ),
  assigned: z
    .string()
    .optional()
    .describe(
      "Person or agent currently responsible for the task. Use Fractal agent IDs (e.g., 'atlas') or user names.",
    ),
  dueAt: z
    .string()
    .optional()
    .describe(
      "ISO timestamp for when the task is due. Used for deadline tracking and prioritization.",
    ),
  slug: z
    .string()
    .describe(
      "URL-friendly version of the name. Used as a stable identifier in URLs and CLI (e.g., 'fix-login-bug').",
    ),
  type: z
    .string()
    .default("task")
    .describe(
      "Classification of the task. Common values: 'bug', 'feature', 'refactor', 'docs', 'task', 'config'. Defaults to 'task'.",
    ),
  priority: FractalTaskPrioritySchema.default("no_priority").describe(
    "Priority level of the task. Determines execution order and urgency. Defaults to 'no_priority'.",
  ),
  summary: z
    .string()
    .optional()
    .describe(
      "A concise summary of the goal or intent. One or two sentences that capture the 'why' of this task.",
    ),
  content: z
    .string()
    .optional()
    .describe(
      "Detailed markdown description of the task requirements. Includes acceptance criteria, context, and implementation notes.",
    ),
  status: FractalTaskStatusSchema.default("backlog").describe(
    "Initial lifecycle status. Defaults to 'backlog'. Use 'suggestion' for proactively suggested tasks.",
  ),
  template: z
    .string()
    .optional()
    .describe(
      "ID of the template used to scaffold the task. Templates pre-populate content and structure.",
    ),
  worktree: FractalTaskWorktreeSchema.default({ enabled: false }).describe(
    "Worktree configuration for this task. Defaults to disabled.",
  ),
  chat: z
    .string()
    .optional()
    .describe(
      "ID of the chat associated with this task. Links task execution to a conversation context.",
    ),
  attachments: z
    .array(FractalAttachmentSchema)
    .default([])
    .describe(
      "List of file or URL attachments for this task. Use to link specs, screenshots, or references.",
    ),
  data: z
    .any()
    .optional()
    .describe(
      "Object data for rendering the template with necessary variables. Only used when a template is specified.",
    ),
  project: z
    .string()
    .optional()
    .describe("ID (slug) of the project this task belongs to."),
  goal: z
    .string()
    .optional()
    .describe("ID (slug) of the goal this task contributes to."),
});

/**
 * Schema for partially updating an existing task.
 * All fields are optional — only provide the fields you want to change.
 * Use the `transition` command to update status, NOT this schema.
 */
export const FractalTaskUpdateSchema = z
  .object({
    name: z
      .string()
      .describe(
        "Short, descriptive name of the task. Should clearly convey the goal.",
      ),
    assigned: z
      .string()
      .describe(
        "Person or agent currently responsible for the task. Use Fractal agent IDs (e.g., 'atlas') or user names.",
      ),
    dueAt: z
      .string()
      .describe(
        "ISO timestamp for when the task is due. Used for deadline tracking and prioritization.",
      ),
    type: z
      .string()
      .describe(
        "Classification of the task. Common values: 'bug', 'feature', 'refactor', 'docs', 'task', 'config'.",
      ),
    priority: FractalTaskPrioritySchema.describe(
      "Priority level of the task. Determines execution order and urgency.",
    ),
    summary: z
      .string()
      .describe(
        "A concise summary of the goal or intent. One or two sentences that capture the 'why'.",
      ),
    content: z
      .string()
      .describe(
        "Detailed markdown description of the task requirements. Includes acceptance criteria and implementation notes.",
      ),
    status: FractalTaskStatusSchema.describe(
      "Current lifecycle status. Prefer using the `transition` command instead of updating directly.",
    ),
    template: z
      .string()
      .describe(
        "ID of the template used to scaffold the task. Templates pre-populate content and structure.",
      ),
    worktree: FractalTaskWorktreeSchema.describe(
      "Worktree configuration for this task. Controls Git isolation behavior.",
    ),
    chat: z
      .string()
      .describe(
        "ID of the chat associated with this task. Links task execution to a conversation context.",
      ),
    attachments: z
      .array(FractalAttachmentSchema)
      .describe(
        "List of file or URL attachments for this task. Use to link specs, screenshots, or references.",
      ),
    data: z
      .any()
      .describe(
        "Object data for rendering the template with necessary variables. Only used when a template is specified.",
      ),
    project: z
      .string()
      .optional()
      .describe("ID (slug) of the project this task belongs to."),
    goal: z
      .string()
      .optional()
      .describe("ID (slug) of the goal this task contributes to."),
  })
  .partial();

/**
 * Schema for filtering task lists.
 * All fields are optional — combine them to narrow down results.
 */
export const FractalTaskListQuerySchema = Schema.object({
  query: z
    .string()
    .optional()
    .describe(
      "Full-text search across task id, name, summary, content, and type. Use to find tasks by keyword.",
    ),
  status: z
    .custom<z.infer<typeof FractalTaskStatusSchema>[]>((value) =>
      Array.isArray(value) ? value : String(value)?.split(",") || [],
    )
    .optional()
    .describe(
      "Filter by one or more task statuses. Comma-separated. Values: suggestion, backlog, planning, todo, in_progress, in_review, finished.",
    ),
  priority: z
    .custom<z.infer<typeof FractalTaskPrioritySchema>[]>((value) =>
      Array.isArray(value) ? value : String(value)?.split(",") || [],
    )
    .optional()
    .describe(
      "Filter by one or more priority levels. Comma-separated. Values: urgent, high, medium, low, no_priority.",
    ),
  type: z
    .custom<string[]>((value) =>
      Array.isArray(value) ? value : String(value)?.split(",") || [],
    )
    .optional()
    .describe(
      "Filter by one or more task types. Comma-separated. Common values: bug, feature, refactor, docs, task, config.",
    ),
  project: z
    .custom<string[]>((value) =>
      Array.isArray(value) ? value : String(value)?.split(",") || [],
    )
    .optional()
    .describe("Filter by one or more project IDs (slugs). Comma-separated."),
  goal: z
    .custom<string[]>((value) =>
      Array.isArray(value) ? value : String(value)?.split(",") || [],
    )
    .optional()
    .describe("Filter by one or more goal IDs (slugs). Comma-separated."),
  limit: z
    .string()
    .optional()
    .describe("Maximum number of tasks to return. Use for pagination."),
  offset: z
    .string()
    .optional()
    .describe("Number of tasks to skip. Use with limit for pagination."),
});

/**
 * Schema for finish-task workflow options.
 * Controls what Git operation happens when a task transitions to 'finished'.
 */
export const FractalTaskFinishOptionsSchema = z.object({
  operation: FractalTaskFinishOperationSchema.describe(
    "Git operation to execute when transitioning to finished. 'merge_to_main' merges the branch; 'create_branch' keeps it separate.",
  ),
  openPullRequest: z
    .boolean()
    .default(false)
    .describe(
      "Reserved flag for future GitHub pull request automation. Currently unused.",
    ),
});

/**
 * Schema for transitioning a task to a new status.
 * Use this to move a task through its lifecycle instead of updating status directly.
 */
export const FractalTaskTransitionSchema = z.object({
  status: FractalTaskStatusSchema.describe(
    "The new status to transition to. Must follow the lifecycle: suggestion → backlog → planning → todo → in_progress → in_review → finished.",
  ),
  delegate: z
    .boolean()
    .default(true)
    .describe(
      "When transitioning to 'in_progress', controls whether Fractal should delegate execution into a new or resumed chat session. Use false to return the full execution context for direct execution without starting chat execution. Defaults to true for backward compatibility.",
    ),
  completion: FractalTaskFinishOptionsSchema.optional().describe(
    "Git operation to execute when transitioning to 'finished'. Only applies when status is 'finished'.",
  ),
});

/**
 * Schema for starting a task execution flow.
 */
export const FractalTaskStartSchema = z.object({
  delegate: z
    .boolean()
    .default(true)
    .describe(
      "When starting a task, controls whether Fractal should create or resume a background execution chat. Use false to return the full execution context for direct execution without spawning a chat. Defaults to true for backward compatibility.",
    ),
});

/**
 * Schema for creating a new comment.
 */
/**
 * Schema for creating a new comment.
 * Omits auto-generated fields (id, createdAt, updatedAt).
 */
export const FractalCommentCreateSchema = FractalCommentSchema.omit({
  id: true,
  createdAt: true,
  updatedAt: true,
});

/**
 * Schema for partially updating a comment.
 * Only body and attachments can be updated after creation.
 */
export const FractalCommentUpdateSchema = z.object({
  body: z
    .string()
    .optional()
    .describe(
      "Updated markdown content of the comment. Supports code blocks, lists, mentions (@username), and formatted text.",
    ),
  attachments: z
    .array(FractalAttachmentSchema)
    .optional()
    .describe("Updated list of file or URL attachments for this comment."),
});

/**
 * Schema for the todo completion counters returned with a task context.
 *
 * @description Aggregated status counts for all todos associated with a task.
 * @property completed - Number of todos finished and counted as completed.
 * @property in_progress - Number of todos currently in progress.
 * @property in_review - Number of todos waiting for review.
 * @property todo - Number of todos still waiting to be started.
 */
export const FractalTaskTodoStatsSchema = z.object({
  completed: z
    .number()
    .int()
    .nonnegative()
    .describe("Number of todos finished and counted as completed."),
  in_progress: z
    .number()
    .int()
    .nonnegative()
    .describe("Number of todos currently in progress."),
  in_review: z
    .number()
    .int()
    .nonnegative()
    .describe("Number of todos waiting for review."),
  todo: z
    .number()
    .int()
    .nonnegative()
    .describe("Number of todos still waiting to be started."),
});

/**
 * Schema for task-level statistics returned by the task detail endpoint.
 *
 * @description Aggregated task metrics used by the detail page and progress UI.
 * @property todos - Todo status counters grouped by lifecycle state.
 */
export const FractalTaskStatsSchema = z.object({
  todos: FractalTaskTodoStatsSchema.describe(
    "Todo status counters grouped by lifecycle state.",
  ),
});

export const FractalTaskWithContextSchema = FractalTaskSchema.extend({
  stats: FractalTaskStatsSchema.describe(
    "Aggregated task metrics returned by the task detail endpoint.",
  ),
  todos: z.array(FractalTaskTodoSchema).default([]),
  comments: z.array(FractalCommentSchema).default([]),
});

// ─── Task Types ──────────────────────────────────────────────────────────────

export type FractalTask = z.infer<typeof FractalTaskSchema>;
/** No standalone export in this file's source either — derived the same way as `FractalTask` itself, from the `assignee` field just added to `FractalTaskSchema`. */
export type FractalTaskAssignee = NonNullable<FractalTask["assignee"]>;
export type FractalTaskTodoStats = z.infer<typeof FractalTaskTodoStatsSchema>;
export type FractalTaskStats = z.infer<typeof FractalTaskStatsSchema>;
export type FractalTaskWithContext = z.infer<
  typeof FractalTaskWithContextSchema
>;
export type FractalTaskCreateInput = z.infer<typeof FractalTaskCreateSchema>;
export type FractalTaskUpdateInput = z.infer<typeof FractalTaskUpdateSchema>;
export type FractalTaskListQueryInput = z.infer<
  typeof FractalTaskListQuerySchema
>;
export type FractalTaskTransitionInput = z.infer<
  typeof FractalTaskTransitionSchema
>;
export type FractalTaskFinishOperation = z.infer<
  typeof FractalTaskFinishOperationSchema
>;
export type FractalTaskPriority = z.infer<typeof FractalTaskPrioritySchema>;
export type FractalTaskWorktree = z.infer<typeof FractalTaskWorktreeSchema>;
export type FractalTaskStartInput = z.input<typeof FractalTaskStartSchema>;

/**
 * Result returned when a task execution is delegated to a background chat.
 */
export interface FractalTaskDelegatedExecutionResult {
  mode: "delegated";
  task: FractalTask;
  chat: Chat;
  instruction: string;
}

/**
 * Result returned when the caller-agent receives the full execution context.
 */
export interface FractalTaskContextExecutionResult {
  mode: "context";
  task: FractalTaskWithContext;
  stats: FractalTaskStats;
  prompt: string;
  instruction: string;
}

/**
 * Unified execution result for task start and in-progress transitions.
 */
export type FractalTaskExecutionResult =
  | FractalTaskDelegatedExecutionResult
  | FractalTaskContextExecutionResult;

/**
 * Result returned by task transitions that still expose the updated task.
 */
export type FractalTaskTransitionResult =
  | { task: FractalTask }
  | FractalTaskExecutionResult;

export interface FractalTaskGetParams {
  id: string;
}

export interface FractalTaskUpdateParams {
  id: string;
  data: FractalTaskUpdateInput;
}

export interface FractalTaskDeleteParams {
  id: string;
}

/**
 * Input parameters for transitioning a task.
 *
 * @description Carries the target task identifier, the desired status, the
 * optional delegation toggle for `in_progress`, and the optional completion
 * operation used when finishing a task.
 */
export interface FractalTaskTransitionParams {
  /** The unique identifier of the task being transitioned. */
  id: string;
  /** The new lifecycle status to apply. */
  status: z.infer<typeof FractalTaskStatusSchema>;
  /**
   * Whether `in_progress` should spawn or resume execution chat.
   * Defaults to `true` when omitted by the schema layer.
   */
  delegate?: boolean;
  /** Git completion operation used when transitioning to `finished`. */
  completion?: z.infer<typeof FractalTaskFinishOptionsSchema>;
}

export interface FractalTaskStartParams {
  id: string;
  delegate?: boolean;
}

export interface FractalTaskBranchParams {
  id: string;
}

export interface FractalTaskContinueParams {
  id: string;
  chatId?: string;
}

/**
 * Input parameters for stale in-progress task recovery.
 */
export interface FractalTaskRecoverStaleInput {
  /** Workspace identifier used for explicit automation scoping. */
  workspace: string;
  /** Optional scheduler run id responsible for this pass. */
  runId?: string;
  /** Optional ISO timestamp used as deterministic reference time. */
  now?: string;
}

/**
 * Input parameters for due todo task processing.
 */
export interface FractalTaskProcessDueInput {
  /** Workspace identifier used for explicit automation scoping. */
  workspace: string;
  /** Optional scheduler run id responsible for this pass. */
  runId?: string;
  /** Optional ISO timestamp used as deterministic reference time. */
  now?: string;
}

/**
 * Normalized entry describing a skipped task during automation.
 */
export interface FractalTaskAutomationSkippedEntry {
  /** Task identifier. */
  id: string;
  /** Human-readable skip reason. */
  reason: string;
}

/**
 * Normalized entry describing a failed task during automation.
 */
export interface FractalTaskAutomationFailedEntry {
  /** Task identifier. */
  id: string;
  /** Human-readable failure reason. */
  reason: string;
}

/**
 * Normalized entry describing a started task during automation.
 */
export interface FractalTaskAutomationStartedEntry {
  /** Task identifier. */
  id: string;
  /** Optional chat id created/reused during start. */
  chatId?: string;
}

/**
 * Normalized entry describing a recovered task during stale pass.
 */
export interface FractalTaskAutomationRecoveredEntry {
  /** Task identifier. */
  id: string;
  /** Recovery action applied to this task. */
  action: "continued" | "moved_to_in_review" | "restarted";
  /** Optional chat id touched by the recovery action. */
  chatId?: string;
}

/**
 * Result payload for stale in-progress task recovery.
 */
export interface FractalTaskRecoverStaleResult {
  /** Effective workspace id processed. */
  workspace: string;
  /** Effective ISO timestamp used during evaluation. */
  now: string;
  /** Number of scanned in-progress tasks. */
  scanned: number;
  /** Tasks recovered in this pass. */
  recovered: FractalTaskAutomationRecoveredEntry[];
  /** Tasks skipped and reasons. */
  skipped: FractalTaskAutomationSkippedEntry[];
  /** Tasks failed and reasons. */
  failed: FractalTaskAutomationFailedEntry[];
}

/**
 * Result payload for due todo task processing.
 */
export interface FractalTaskProcessDueResult {
  /** Effective workspace id processed. */
  workspace: string;
  /** Effective ISO timestamp used during evaluation. */
  now: string;
  /** Number of scanned todo tasks. */
  scanned: number;
  /** Tasks started in this pass (max one initially). */
  started: FractalTaskAutomationStartedEntry[];
  /** Tasks skipped and reasons. */
  skipped: FractalTaskAutomationSkippedEntry[];
  /** Tasks failed and reasons. */
  failed: FractalTaskAutomationFailedEntry[];
}

// ─── Todo Types ───────────────────────────────────────────────────────────────

export type FractalTodo = z.infer<typeof FractalTaskTodoSchema>;
export type FractalTodoCreateInput = Omit<FractalTodo, "id">;
export type FractalTodoUpdateInput = Partial<FractalTodoCreateInput>;

export interface FractalTodoGetParams {
  id: string;
}

export interface FractalTodoUpdateParams {
  id: string;
  data: FractalTodoUpdateInput;
}

export interface FractalTodoDeleteParams {
  id: string;
}

export interface FractalTodoTransitionParams {
  id: string;
  status: z.infer<typeof FractalTodoStatusSchema>;
}

// ─── Comment Types ────────────────────────────────────────────────────────────

export type FractalComment = z.infer<typeof FractalCommentSchema>;
export type FractalCommentCreateInput = z.infer<
  typeof FractalCommentCreateSchema
>;
export type FractalCommentUpdateInput = z.infer<
  typeof FractalCommentUpdateSchema
>;
export type FractalAttachment = z.infer<typeof FractalAttachmentSchema>;

export interface FractalCommentGetParams {
  id: string;
}

export interface FractalCommentUpdateParams {
  id: string;
  data: FractalCommentUpdateInput;
}

export interface FractalCommentDeleteParams {
  id: string;
}

// ─── Service Interfaces ───────────────────────────────────────────────────────

/**
 * @interface IFractalTaskService
 * @description Contract for the TaskService managing the task lifecycle and persistence.
 */
export interface IFractalTaskService {
  list(
    params?: FractalTaskListQueryInput,
  ): Promise<ResponseWithCTA<{ tasks: FractalTask[] }>>;
  getById(
    params: FractalTaskGetParams,
  ): Promise<
    ResponseWithCTA<{ task: FractalTaskWithContext; stats: FractalTaskStats }>
  >;
  /**
   * Builds a reusable execution prompt from the current task record and its todos.
   *
   * @param taskId - The unique identifier of the task whose execution prompt should be generated.
   * @returns A markdown prompt containing the task name, summary, detailed content, and current todo list.
   * @throws {TaskError} with code `FRACTAL_TASK_NOT_FOUND` if the task does not exist.
   *
   * @example
   * ```typescript
   * const prompt = await taskService.generateExecutionPrompt("FRA-008");
   * ```
   */
  generateExecutionPrompt(taskId: string): Promise<string>;
  /**
   * Builds a continuation prompt for an already-running task execution loop.
   *
   * @param taskId - The unique identifier of the task to continue.
   * @returns A markdown prompt focused on finishing remaining todos and advancing task state.
   */
  generateContinuationPrompt(taskId: string): Promise<string>;
  /**
   * Recovers stale task automations currently marked as in-progress.
   */
  recoverStale(
    params: FractalTaskRecoverStaleInput,
  ): Promise<ResponseWithCTA<FractalTaskRecoverStaleResult>>;
  /**
   * Processes due todo tasks and starts at most one per pass.
   */
  processDue(
    params: FractalTaskProcessDueInput,
  ): Promise<ResponseWithCTA<FractalTaskProcessDueResult>>;
  /**
   * Creates a new chat session preloaded with the generated task prompt,
   * or resumes the existing task chat when the task is already in progress.
   *
   * @param params - Contains the unique identifier of the task that should seed the new chat session.
   * @returns The created or resumed {@link Chat} record and the generated markdown prompt used as the first message when applicable.
   * @throws {TaskError} with code `FRACTAL_TASK_NOT_FOUND` if the task does not exist.
   *
   * @example
   * ```typescript
   * const result = await taskService.start({ id: "FRA-008" });
   * ```
   */
  start(
    params: FractalTaskStartParams,
  ): Promise<ResponseWithCTA<FractalTaskExecutionResult>>;
  /**
   * Resolves the git branch associated with a task worktree.
   *
   * @param params - Contains the unique identifier of the task whose branch should be resolved.
   * @returns The active branch name and the resolved worktree path when it exists.
   * @throws {TaskError} with code `FRACTAL_TASK_NOT_FOUND` if the task does not exist.
   *
   * @example
   * ```typescript
   * const result = await taskService.branch({ id: "FRA-008" });
   * ```
   */
  branch(
    params: FractalTaskBranchParams,
  ): Promise<ResponseWithCTA<{ branch: string; worktree: string | null }>>;
  continue(params: FractalTaskContinueParams): Promise<
    ResponseWithCTA<{
      continued: boolean;
      pendingTodos: string[];
      messageId?: string;
    }>
  >;
  create(
    params: FractalTaskCreateInput,
  ): Promise<ResponseWithCTA<{ task: FractalTask }>>;
  update(
    params: FractalTaskUpdateParams,
  ): Promise<ResponseWithCTA<{ task: FractalTask }>>;
  delete(params: FractalTaskDeleteParams): Promise<ResponseWithCTA>;
  todos(taskId: string): IFractalTodoService;
  comments(taskId: string): IFractalCommentService;
  setStatus(
    params: FractalTaskTransitionParams,
  ): Promise<ResponseWithCTA<FractalTaskTransitionResult>>;
}

/**
 * @interface IFractalTodoService
 * @description Contract for the TodoService managing todos nested inside a task context.
 */
export interface IFractalTodoService {
  list(): Promise<ResponseWithCTA<{ todos: FractalTodo[] }>>;
  getById(
    params: FractalTodoGetParams,
  ): Promise<ResponseWithCTA<{ todo: FractalTodo }>>;
  create(
    params: FractalTodoCreateInput,
  ): Promise<ResponseWithCTA<{ todo: FractalTodo }>>;
  update(
    params: FractalTodoUpdateParams,
  ): Promise<ResponseWithCTA<{ todo: FractalTodo }>>;
  delete(params: FractalTodoDeleteParams): Promise<ResponseWithCTA>;
  setStatus(
    params: FractalTodoTransitionParams,
  ): Promise<ResponseWithCTA<{ todo: FractalTodo }>>;
}

/**
 * @interface IFractalCommentService
 * @description Contract for the CommentService managing comments nested inside a task context.
 * Agents can create comments but cannot edit or delete user comments.
 */
export interface IFractalCommentService {
  list(): Promise<ResponseWithCTA<{ comments: FractalComment[] }>>;
  getById(
    params: FractalCommentGetParams,
  ): Promise<ResponseWithCTA<{ comment: FractalComment }>>;
  create(
    params: FractalCommentCreateInput,
  ): Promise<ResponseWithCTA<{ comment: FractalComment }>>;
  update(
    params: FractalCommentUpdateParams,
  ): Promise<ResponseWithCTA<{ comment: FractalComment }>>;
  delete(params: FractalCommentDeleteParams): Promise<ResponseWithCTA>;
}
