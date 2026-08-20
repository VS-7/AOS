import { z } from "zod";
import type { WorkspaceMemberSchema } from "@/features/workspace/schemas/workspace.schema";

/**
 * `WorkspaceMember` — absent from `index/`, present in
 * `_extracted/v401/server/src/features/workspace/interfaces/workspace.
 * interfaces.ts` as `z.infer<typeof WorkspaceMemberSchema>`. The
 * schema itself already lives in this feature's own `schemas/workspace.
 * schema.ts` (a pristine `v401/web` copy), so only the inferred type is
 * added here, per the task brief's explicit note on this gap.
 */
export type WorkspaceMember = z.infer<typeof WorkspaceMemberSchema>;

/**
 * PRUNED (beyond Steps 4/5's sanctioned scope — flagged for review, see
 * Task 7 report): the original file also imported 17 backend-only symbols
 * (all workspace-scoped service classes, `Collections`,
 * `ConfigService` from a `config` feature that does not exist in
 * this tree, `ActivityInstance`, `InferedStore`, `BotsRegistry`).
 * They fed a server-side runtime-wiring block — the type that describes
 * how the backend composes a workspace's services into one runtime object
 * — which the frontend never constructs or consumes. That block
 * (`CollectionsManager`, `WorkspaceDefaultServices`,
 * `WorkspaceServiceInitParams`, `WorkspaceRuntime`,
 * `IWorkspaceRuntimeBindable`, `WorkspaceServiceFactory`,
 * `WorkspaceServiceFactoryMap`, `IWorkspaceService`) was removed below.
 * Entities/schemas the UI actually imports (`Workspace` and
 * friends) and the self-contained `IWorkspaceCollection` /
 * `WorkspaceTick*` types were kept untouched.
 */

/**
 * Zod schema defining a task type for a AOS Workspace.
 * @example
 * ```typescript
 * const taskType = WorkspaceTaskTypeSchema.parse({ id: "bug", label: "Bug", color: "#ef4444" });
 * ```
 */
export const WorkspaceTaskTypeSchema = z.object({
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
 * Inferred from {@link WorkspaceTaskTypeSchema}.
 */
export type WorkspaceTaskType = z.infer<
  typeof WorkspaceTaskTypeSchema
>;

/**
 * Zod schema defining a custom workspace label.
 * @example
 * ```typescript
 * const label = WorkspaceLabelSchema.parse({
 *   id: "ui-ux",
 *   label: "UI/UX",
 *   icon: "Palette",
 *   color: "#ec4899",
 * });
 * ```
 */
export const WorkspaceLabelSchema = z.object({
  id: z.string().describe("Unique identifier for the workspace label"),
  label: z.string().describe("Human-readable label displayed in the UI"),
  icon: z.string().describe("Lucide icon identifier used to render the label"),
  color: z.string().describe("Hex color code representing the label"),
});

/**
 * Represents a single workspace label definition.
 * Inferred from {@link WorkspaceLabelSchema}.
 */
export type WorkspaceLabel = z.infer<typeof WorkspaceLabelSchema>;

/**
 * Zod schema defining the worktree configuration for a AOS Workspace.
 * @example
 * ```typescript
 * const worktrees = WorkspaceWorktreesSchema.parse(data);
 * ```
 */
export const WorkspaceWorktreesSchema = z.object({
  deleteOldWorktrees: z.boolean(),
  worktreeLimit: z.number().min(1).max(50),
  onCreateScript: z
    .string()
    .optional()
    .describe("Optional Bash script to execute after worktree creation"),
});

/**
 * Zod schema defining the Git configuration for a AOS Workspace.
 * @example
 * ```typescript
 * const git = WorkspaceGitSchema.parse(data);
 * ```
 */
export const WorkspaceGitSchema = z.object({
  branchPrefix: z.string().optional(),
  forcePush: z.boolean(),
  commitInstructions: z.string().optional(),
  prInstructions: z.string().optional(),
});

/**
 * Zod schema defining the structure of a AOS Workspace.
 * @example
 * ```typescript
 * const workspace = WorkspaceSchema.parse(data);
 * ```
 */
export const WorkspaceSchema = z.object({
  id: z.string().describe("The generated ID for the workspace"),
  name: z.string().describe("The name of the workspace"),
  path: z.string().describe("The absolute path of the workspace"),
  logo: z.string().optional().describe("Optional logo URL"),
  color: z.string().optional().describe("Optional color hex"),
  tasks: z
    .array(WorkspaceTaskTypeSchema)
    .describe("Task type definitions for this workspace"),
  labels: z
    .array(WorkspaceLabelSchema)
    .describe("Custom label definitions for this workspace"),
  worktrees: WorkspaceWorktreesSchema,
  git: WorkspaceGitSchema,
  archived: z
    .boolean()
    .default(false)
    .describe("Whether the workspace is archived"),
  createdAt: z.string().describe("ISO date string"),
  updatedAt: z.string().describe("ISO date string"),
});

/**
 * Represents a AOS Workspace object.
 * Inferred from {@link WorkspaceSchema}.
 */
export type Workspace = z.infer<typeof WorkspaceSchema>;
export type WorkspacePathHelper = (
  scope: "global" | "workspace",
  ...paths: string[]
) => string;

/**
 * Zod schema for creating a new AOS Workspace.
 * @example
 * ```typescript
 * const createData = WorkspaceCreateSchema.parse({ name: "My Workspace", path: "/path/to/workspace" });
 * ```
 */
export const WorkspaceCreateSchema = z.object({
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
 * Inferred from {@link WorkspaceCreateSchema}.
 */
export type WorkspaceCreate = z.infer<
  typeof WorkspaceCreateSchema
>;

/**
 * Zod schema for updating an existing AOS Workspace.
 * @example
 * ```typescript
 * const updateData = WorkspaceUpdateSchema.parse({ name: "New Name" });
 * ```
 */
export const WorkspaceUpdateSchema = WorkspaceSchema.omit({
  id: true,
  createdAt: true,
  updatedAt: true,
  archived: true,
}).partial();

/**
 * Input parameters for updating a workspace.
 * Inferred from {@link WorkspaceUpdateSchema}.
 */
export type WorkspaceUpdate = z.infer<
  typeof WorkspaceUpdateSchema
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
 * const input: WorkspaceTickInput = {
 *   runId: "job_123",
 *   now: new Date().toISOString(),
 * };
 * ```
 */
export interface WorkspaceTickInput {
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
 * whenever available from `@aos-js/jobs` runtime.
 */
export interface WorkspaceTickDispatchEntry {
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
export interface WorkspaceTickWorkspaceSummary {
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
    job: WorkspaceTickDispatchEntry["job"];
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
export interface WorkspaceTickResult {
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
  dispatched: WorkspaceTickDispatchEntry[];
  /**
   * @description Dispatch attempts that failed and their reasons.
   */
  failed: Array<
    WorkspaceTickDispatchEntry & {
      reason: string;
    }
  >;
  /**
   * @description Per-workspace summary to aid review and logs.
   */
  workspaces: WorkspaceTickWorkspaceSummary[];
}

/**
 * PRUNED: `IWorkspaceService` (the backend service contract — `create`,
 * `update`, `delete`, `get`, `resolve`, `list`, `tick`) depended on the
 * removed `WorkspaceRuntime`/`WorkspaceDefaultServices`
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
  get(id: string): Promise<Workspace | null>;

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
  list(): Promise<Workspace[]>;

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
  save(workspace: Workspace): Promise<void>;

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

export const DEFAULT_WORSKSPACE_GIT_OPTIONS: Workspace["git"] = {
  branchPrefix: "aos",
  forcePush: false,
  commitInstructions: "",
  prInstructions: "",
};

export const DEFAULT_WORSKSPACE_WORKTREE_OPTIONS: Workspace["worktrees"] =
  {
    deleteOldWorktrees: true,
    worktreeLimit: 15,
    onCreateScript: "",
  };

export const DEFAULT_WORKSPACE_TASK_TYPES: Workspace["tasks"] = [
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
export const DEFAULT_WORKSPACE_LABELS: Workspace["labels"] = [];
