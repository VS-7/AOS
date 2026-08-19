import { z } from "zod";

/**
 * PRUNED (beyond Steps 4/5's sanctioned scope — flagged for review, see
 * Task 7 report): the original file also imported 17 backend-only symbols
 * (all workspace-scoped service classes, `FractalCollections`,
 * `FractalConfigService` from a `config` feature that does not exist in
 * this tree, `FractalActivityInstance`, `InferedStore`, `BotsRegistry`).
 * They fed a server-side runtime-wiring block — the type that describes
 * how the backend composes a workspace's services into one runtime object
 * — which the frontend never constructs or consumes. That block
 * (`FractalCollectionsManager`, `FractalWorkspaceDefaultServices`,
 * `WorkspaceServiceInitParams`, `FractalWorkspaceRuntime`,
 * `IWorkspaceRuntimeBindable`, `WorkspaceServiceFactory`,
 * `WorkspaceServiceFactoryMap`, `IWorkspaceService`) was removed below.
 * Entities/schemas the UI actually imports (`FractalWorkspace` and
 * friends) and the self-contained `IWorkspaceCollection` /
 * `FractalWorkspaceTick*` types were kept untouched.
 */

/**
 * Zod schema defining a task type for a Fractal Workspace.
 * @example
 * ```typescript
 * const taskType = FractalWorkspaceTaskTypeSchema.parse({ id: "bug", label: "Bug", color: "#ef4444" });
 * ```
 */
export const FractalWorkspaceTaskTypeSchema = z.object({
  id: z
    .string()
    .describe("Unique identifier for the task type (e.g. 'bug', 'feature')"),
  label: z.string().describe("Human-readable label displayed in the UI"),
  color: z.string().describe("Hex color code representing the task type"),
  description: z
    .string()
    .optional()
    .describe("A brief description of the task type's purpose"),
  instructions: z
    .string()
    .optional()
    .describe("Optional agent instructions specific to this task type"),
});

/**
 * Represents a single task type definition.
 * Inferred from {@link FractalWorkspaceTaskTypeSchema}.
 */
export type FractalWorkspaceTaskType = z.infer<
  typeof FractalWorkspaceTaskTypeSchema
>;

/**
 * Zod schema defining a custom workspace label.
 * @example
 * ```typescript
 * const label = FractalWorkspaceLabelSchema.parse({
 *   id: "ui-ux",
 *   label: "UI/UX",
 *   icon: "Palette",
 *   color: "#ec4899",
 * });
 * ```
 */
export const FractalWorkspaceLabelSchema = z.object({
  id: z.string().describe("Unique identifier for the workspace label"),
  label: z.string().describe("Human-readable label displayed in the UI"),
  icon: z.string().describe("Lucide icon identifier used to render the label"),
  color: z.string().describe("Hex color code representing the label"),
});

/**
 * Represents a single workspace label definition.
 * Inferred from {@link FractalWorkspaceLabelSchema}.
 */
export type FractalWorkspaceLabel = z.infer<typeof FractalWorkspaceLabelSchema>;

/**
 * Zod schema defining the worktree configuration for a Fractal Workspace.
 * @example
 * ```typescript
 * const worktrees = FractalWorkspaceWorktreesSchema.parse(data);
 * ```
 */
export const FractalWorkspaceWorktreesSchema = z.object({
  deleteOldWorktrees: z.boolean(),
  worktreeLimit: z.number().min(1).max(50),
  onCreateScript: z
    .string()
    .optional()
    .describe("Optional Bash script to execute after worktree creation"),
});

/**
 * Zod schema defining the Git configuration for a Fractal Workspace.
 * @example
 * ```typescript
 * const git = FractalWorkspaceGitSchema.parse(data);
 * ```
 */
export const FractalWorkspaceGitSchema = z.object({
  branchPrefix: z.string().optional(),
  forcePush: z.boolean(),
  commitInstructions: z.string().optional(),
  prInstructions: z.string().optional(),
});

/**
 * Zod schema defining the structure of a Fractal Workspace.
 * @example
 * ```typescript
 * const workspace = FractalWorkspaceSchema.parse(data);
 * ```
 */
export const FractalWorkspaceSchema = z.object({
  id: z.string().describe("The generated ID for the workspace"),
  name: z.string().describe("The name of the workspace"),
  path: z.string().describe("The absolute path of the workspace"),
  logo: z.string().optional().describe("Optional logo URL"),
  color: z.string().optional().describe("Optional color hex"),
  tasks: z
    .array(FractalWorkspaceTaskTypeSchema)
    .describe("Task type definitions for this workspace"),
  labels: z
    .array(FractalWorkspaceLabelSchema)
    .describe("Custom label definitions for this workspace"),
  worktrees: FractalWorkspaceWorktreesSchema,
  git: FractalWorkspaceGitSchema,
  archived: z
    .boolean()
    .default(false)
    .describe("Whether the workspace is archived"),
  createdAt: z.string().describe("ISO date string"),
  updatedAt: z.string().describe("ISO date string"),
});

/**
 * Represents a Fractal Workspace object.
 * Inferred from {@link FractalWorkspaceSchema}.
 */
export type FractalWorkspace = z.infer<typeof FractalWorkspaceSchema>;
export type FractalWorkspacePathHelper = (
  scope: "global" | "workspace",
  ...paths: string[]
) => string;

/**
 * Zod schema for creating a new Fractal Workspace.
 * @example
 * ```typescript
 * const createData = FractalWorkspaceCreateSchema.parse({ name: "My Workspace", path: "/path/to/workspace" });
 * ```
 */
export const FractalWorkspaceCreateSchema = z.object({
  name: z.string().describe("The name of the workspace"),
  path: z.string().optional().describe("The absolute path of the workspace"),
  logo: z.string().optional().describe("Optional logo URL"),
  color: z.string().optional().describe("Optional color hex"),
  orchestrator: z
    .object({
      name: z.string(),
      tone: z.enum(["efficient", "friendly", "professional", "candid"]),
      style: z.enum(["concise", "balanced", "detailed"]),
      autonomy: z.number().min(0).max(1),
    })
    .optional()
    .describe("Initial orchestrator agent configuration"),
});

/**
 * Input parameters for creating a workspace.
 * Inferred from {@link FractalWorkspaceCreateSchema}.
 */
export type FractalWorkspaceCreate = z.infer<
  typeof FractalWorkspaceCreateSchema
>;

/**
 * Zod schema for updating an existing Fractal Workspace.
 * @example
 * ```typescript
 * const updateData = FractalWorkspaceUpdateSchema.parse({ name: "New Name" });
 * ```
 */
export const FractalWorkspaceUpdateSchema = FractalWorkspaceSchema.omit({
  id: true,
  createdAt: true,
  updatedAt: true,
  archived: true,
}).partial();

/**
 * Input parameters for updating a workspace.
 * Inferred from {@link FractalWorkspaceUpdateSchema}.
 */
export type FractalWorkspaceUpdate = z.infer<
  typeof FractalWorkspaceUpdateSchema
>;

/**
 * Input parameters for a multi-tenant workspace automation tick.
 *
 * @description
 * Represents one scheduler execution pass that fans out automation jobs
 * across all workspaces.
 *
 * @example
 * ```typescript
 * const input: FractalWorkspaceTickInput = {
 *   runId: "job_123",
 *   now: new Date().toISOString(),
 * };
 * ```
 */
export interface FractalWorkspaceTickInput {
  /**
   * @description Identifier of the scheduler/queue run that triggered this tick.
   */
  runId: string;
  /**
   * @description Optional ISO timestamp used as the reference time for deterministic tests and replays.
   */
  now?: string;
}

/**
 * Single dispatched automation job metadata emitted by workspace tick.
 *
 * @description
 * Captures the queue/job identity and the resulting dispatched job id
 * whenever available from `@igniter-js/jobs` runtime.
 */
export interface FractalWorkspaceTickDispatchEntry {
  /**
   * @description Target workspace receiving this automation dispatch.
   */
  workspace: string;
  /**
   * @description Queue name used for dispatch.
   */
  queue: "task" | "routine";
  /**
   * @description Job name dispatched inside the queue.
   */
  job:
    | "recover-stale"
    | "process-due"
    | "recover-stale-runs"
    | "process-scheduled";
  /**
   * @description Enqueued job id returned by the jobs runtime.
   */
  dispatchedJobId?: string;
}

/**
 * Per-workspace tick execution summary.
 */
export interface FractalWorkspaceTickWorkspaceSummary {
  /**
   * @description Target workspace identifier.
   */
  workspace: string;
  /**
   * @description Number of dispatched automation jobs.
   */
  dispatched: number;
  /**
   * @description Dispatches that failed for this workspace.
   */
  failed: Array<{
    job: FractalWorkspaceTickDispatchEntry["job"];
    reason: string;
  }>;
}

/**
 * Output payload for workspace automation tick.
 *
 * @description
 * Exposes deterministic observability for scheduler orchestration, including
 * processed workspaces and recovery-first dispatch ordering results.
 */
export interface FractalWorkspaceTickResult {
  /**
   * @description Scheduler run id associated with this tick.
   */
  runId: string;
  /**
   * @description Effective ISO timestamp used during this tick.
   */
  now: string;
  /**
   * @description Number of workspaces scanned.
   */
  scanned: number;
  /**
   * @description Dispatches successfully enqueued during this tick.
   */
  dispatched: FractalWorkspaceTickDispatchEntry[];
  /**
   * @description Dispatch attempts that failed and their reasons.
   */
  failed: Array<
    FractalWorkspaceTickDispatchEntry & {
      reason: string;
    }
  >;
  /**
   * @description Per-workspace summary to aid review and logs.
   */
  workspaces: FractalWorkspaceTickWorkspaceSummary[];
}

/**
 * PRUNED: `IWorkspaceService` (the backend service contract — `create`,
 * `update`, `delete`, `get`, `resolve`, `list`, `tick`) depended on the
 * removed `FractalWorkspaceRuntime`/`FractalWorkspaceDefaultServices`
 * types and is not implemented or called by the frontend. See the prune
 * note above the import block.
 */

/**
 * Defines the contract for the Workspace Collection.
 */
export interface IWorkspaceCollection {
  /**
   * Retrieves a workspace record from the file system.
   *
   * @param id - The unique identifier of the workspace.
   * @returns The parsed workspace record, or null if not found.
   *
   * @example
   * ```typescript
   * const record = await collection.get("my-workspace");
   * ```
   */
  get(id: string): Promise<FractalWorkspace | null>;

  /**
   * Lists all workspace records available in the system.
   *
   * @returns An array of workspace records.
   *
   * @example
   * ```typescript
   * const records = await collection.list();
   * ```
   */
  list(): Promise<FractalWorkspace[]>;

  /**
   * Saves a workspace record to the file system.
   *
   * @param workspace - The workspace object to save.
   * @returns A promise that resolves when the save is complete.
   *
   * @example
   * ```typescript
   * await collection.save(workspaceObj);
   * ```
   */
  save(workspace: FractalWorkspace): Promise<void>;

  /**
   * Permanently deletes a workspace directory from the global file system.
   *
   * @param id - The unique identifier of the workspace to delete.
   * @returns A promise that resolves when the directory is removed.
   *
   * @example
   * ```typescript
   * await collection.deleteWorkspace("my-workspace");
   * ```
   */
  deleteWorkspace(id: string): Promise<void>;
}

export const DEFAULT_WORSKSPACE_GIT_OPTIONS: FractalWorkspace["git"] = {
  branchPrefix: "fractal",
  forcePush: false,
  commitInstructions: "",
  prInstructions: "",
};

export const DEFAULT_WORSKSPACE_WORKTREE_OPTIONS: FractalWorkspace["worktrees"] =
  {
    deleteOldWorktrees: true,
    worktreeLimit: 15,
    onCreateScript: "",
  };

export const DEFAULT_WORKSPACE_TASK_TYPES: FractalWorkspace["tasks"] = [
  { id: "feature", label: "Feature", color: "#6366f1" },
  { id: "bug", label: "Bug", color: "#ef4444" },
  { id: "refactor", label: "Refactor", color: "#f59e0b" },
  { id: "docs", label: "Docs", color: "#10b981" },
  { id: "config", label: "Config", color: "#64748b" },
];

/**
 * Default custom labels configured for a workspace.
 * New workspaces start with no custom labels until the user defines them.
 */
export const DEFAULT_WORKSPACE_LABELS: FractalWorkspace["labels"] = [];
