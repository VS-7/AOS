import { z } from "zod";
import type { ResponseWithCTA } from "@/core/interfaces/response.interfaces";

/**
 * Valid statuses for a routine.
 *
 * - **enabled** — Routine is active and can be triggered.
 * - **disabled** — Routine is inactive and will not respond to triggers.
 * - **paused** — Routine is temporarily suspended.
 */
export const FractalRoutineStatusSchema = z.enum([
  "enabled",
  "disabled",
  "paused",
]);

/**
 * Valid statuses for a run.
 *
 * - **pending** — Run has been created but not yet started.
 * - **running** — Agent is currently executing the routine.
 * - **completed** — Run finished successfully.
 * - **error** — Run failed during execution.
 */
export const FractalRunStatusSchema = z.enum([
  "pending",
  "running",
  "completed",
  "error",
]);

/**
 * Schema for a webhook trigger configuration.
 */
export const FractalRoutineWebhookTriggerSchema = z.object({
  type: z.literal("webhook"),
  config: z.object({
    token: z
      .string()
      .describe("Auto-generated unique token for webhook validation."),
  }),
});

/**
 * Schema for a scheduled (cron) trigger configuration.
 */
export const FractalRoutineScheduledTriggerSchema = z.object({
  type: z.literal("scheduled"),
  config: z.object({
    cron: z.string().describe("Cron expression defining the schedule."),
  }),
});

/**
 * Discriminated union for routine triggers.
 * Supports webhook and scheduled trigger types.
 */
export const FractalRoutineTriggerSchema = z.discriminatedUnion("type", [
  FractalRoutineWebhookTriggerSchema,
  FractalRoutineScheduledTriggerSchema,
]);

/**
 * Primary schema for a Fractal Routine record.
 * A Routine is a saved agent configuration (prompt + triggers) that runs automatically.
 */
export const FractalRoutineSchema = z.object({
  id: z
    .string()
    .describe("Unique identifier (slug) auto-generated from the routine name."),
  name: z.string().describe("Human-readable name of the routine."),
  agent: z.string().describe("Agent slug (owner) that executes this routine."),
  content: z
    .string()
    .describe("Markdown body content used as the system prompt for execution."),
  triggers: z
    .array(FractalRoutineTriggerSchema)
    .default([])
    .describe("List of triggers that can activate this routine."),
  status: FractalRoutineStatusSchema.describe(
    "Current operational status of the routine.",
  ),
  createdAt: z.string().describe("ISO timestamp when the routine was created."),
  updatedAt: z
    .string()
    .describe("ISO timestamp when the routine was last updated."),
});

/**
 * Primary schema for a Fractal Run record.
 * A Run represents a single execution of a routine.
 */
export const FractalRunSchema = z.object({
  id: z.string().describe("Unique identifier for the run (UUID)."),
  routine: z.string().describe("Parent routine ID."),
  agent: z.string().describe("Agent slug that executed this run."),
  status: FractalRunStatusSchema.describe(
    "Current execution status of the run.",
  ),
  input: z
    .string()
    .default("")
    .describe("Webhook payload or empty string for scheduled runs."),
  chat: z
    .string()
    .optional()
    .describe("Chat session ID created for execution."),
  output: z
    .string()
    .optional()
    .describe("Summary result or output from the execution."),
  startedAt: z.string().describe("ISO timestamp when the run started."),
  finishedAt: z
    .string()
    .optional()
    .describe("ISO timestamp when the run completed or errored."),
});

/**
 * Schema for creating a new routine.
 */
export const FractalRoutineCreateSchema = z.object({
  name: z.string().describe("Human-readable name of the routine."),
  agent: z.string().describe("Agent slug (owner) that executes this routine."),
  prompt: z
    .string()
    .describe("Markdown body content used as the system prompt for execution."),
  triggers: z
    .array(FractalRoutineTriggerSchema)
    .default([])
    .describe("List of triggers that can activate this routine."),
  status: FractalRoutineStatusSchema.default("enabled").describe(
    "Initial operational status.",
  ),
});

/**
 * Schema for updating an existing routine.
 * All fields are optional — only provide the fields you want to change.
 */
export const FractalRoutineUpdateSchema = z.object({
  name: z.string().optional(),
  agent: z.string().optional(),
  prompt: z.string().optional(),
  triggers: z.array(FractalRoutineTriggerSchema).optional(),
  status: FractalRoutineStatusSchema.optional(),
});

/**
 * Schema for querying the routine list.
 */
export const FractalRoutineListQuerySchema = z.object({
  agent: z.string().optional().describe("Filter routines by agent slug."),
  status: z
    .string()
    .optional()
    .describe("Filter routines by status (enabled, disabled, paused)."),
  type: z
    .string()
    .optional()
    .describe("Filter routines by trigger type (webhook, scheduled)."),
  query: z
    .string()
    .optional()
    .describe("Full-text search query across name and prompt."),
  limit: z
    .string()
    .optional()
    .describe("Maximum number of routines to return."),
  offset: z
    .string()
    .optional()
    .describe("Number of routines to skip for pagination."),
});

/**
 * Schema for firing a routine via webhook.
 */
export const FractalRoutineWebhookFireSchema = z.object({
  type: z
    .literal("webhook")
    .describe(
      "Webhook-triggered routine execution. Validates the configured token before firing.",
    ),
  token: z.string().describe("Webhook token for validation."),
  input: z
    .string()
    .optional()
    .describe("Optional payload to pass to the routine execution."),
});

/**
 * Schema for firing a scheduled routine execution.
 */
export const FractalRoutineScheduledFireSchema = z.object({
  type: z
    .literal("scheduled")
    .describe("Cron-triggered routine execution. Used by the scheduler path."),
  input: z
    .string()
    .optional()
    .describe(
      "Optional payload or scheduler metadata to pass to the routine execution.",
    ),
});

/**
 * Schema for firing a routine.
 */
export const FractalRoutineFireSchema = z.discriminatedUnion("type", [
  FractalRoutineWebhookFireSchema,
  FractalRoutineScheduledFireSchema,
]);

/**
 * Schema for creating a new run.
 */
export const FractalRunCreateSchema = z.object({
  routine: z.string().describe("Parent routine ID."),
  agent: z.string().describe("Agent slug that executes this run."),
  status: FractalRunStatusSchema.default("pending"),
  input: z.string().default(""),
  chat: z.string().optional(),
  output: z.string().optional(),
  startedAt: z.string(),
  finishedAt: z.string().optional(),
});

/**
 * TypeScript type for a Routine record.
 */
export type FractalRoutine = z.infer<typeof FractalRoutineSchema>;

/**
 * TypeScript type for a Run record.
 */
export type FractalRun = z.infer<typeof FractalRunSchema>;

/**
 * Input type for creating a routine.
 */
export type FractalRoutineCreateInput = z.infer<
  typeof FractalRoutineCreateSchema
>;

/**
 * Input type for updating a routine.
 */
export type FractalRoutineUpdateInput = z.infer<
  typeof FractalRoutineUpdateSchema
>;

/**
 * Input type for querying routines.
 */
export type FractalRoutineListQueryInput = z.infer<
  typeof FractalRoutineListQuerySchema
>;

/**
 * Input type for firing a routine.
 */
export type FractalRoutineFireInput = z.infer<typeof FractalRoutineFireSchema>;

/**
 * Input type for creating a run.
 */
export type FractalRunCreateInput = z.infer<typeof FractalRunCreateSchema>;

/**
 * Routine record enriched with its runs.
 */
export type FractalRoutineWithRuns = FractalRoutine & {
  runs: FractalRun[];
  fireUrl: string | null;
};

/**
 * Input parameters for stale running routine-run recovery.
 */
export interface FractalRoutineRecoverStaleRunsInput {
  /** Workspace identifier used for explicit automation scoping. */
  workspace: string;
  /** Optional scheduler run id responsible for this pass. */
  runId?: string;
  /** Optional ISO timestamp used as deterministic reference time. */
  now?: string;
}

/**
 * Input parameters for scheduled routine processing.
 */
export interface FractalRoutineProcessScheduledInput {
  /** Workspace identifier used for explicit automation scoping. */
  workspace: string;
  /** Optional scheduler run id responsible for this pass. */
  runId?: string;
  /** Optional ISO timestamp used as deterministic reference time. */
  now?: string;
}

/**
 * Normalized skipped routine entry.
 */
export interface FractalRoutineAutomationSkippedEntry {
  /** Routine or run identifier. */
  id: string;
  /** Human-readable skip reason. */
  reason: string;
}

/**
 * Normalized failed routine entry.
 */
export interface FractalRoutineAutomationFailedEntry {
  /** Routine or run identifier. */
  id: string;
  /** Human-readable failure reason. */
  reason: string;
}

/**
 * Normalized started routine entry.
 */
export interface FractalRoutineAutomationStartedEntry {
  /** Routine identifier. */
  id: string;
  /** Run identifier started in this pass. */
  runId: string;
  /** Optional chat id associated with the run. */
  chatId?: string;
}

/**
 * Normalized recovered run entry.
 */
export interface FractalRoutineAutomationRecoveredEntry {
  /** Run identifier. */
  id: string;
  /** Recovery action applied to this run. */
  action: "marked_error";
}

/**
 * Result payload for stale run recovery.
 */
export interface FractalRoutineRecoverStaleRunsResult {
  /** Effective workspace id processed. */
  workspace: string;
  /** Effective ISO timestamp used during evaluation. */
  now: string;
  /** Number of scanned running runs. */
  scanned: number;
  /** Runs recovered in this pass. */
  recovered: FractalRoutineAutomationRecoveredEntry[];
  /** Runs skipped and reasons. */
  skipped: FractalRoutineAutomationSkippedEntry[];
  /** Runs failed and reasons. */
  failed: FractalRoutineAutomationFailedEntry[];
}

/**
 * Result payload for scheduled routine processing.
 */
export interface FractalRoutineProcessScheduledResult {
  /** Effective workspace id processed. */
  workspace: string;
  /** Effective ISO timestamp used during evaluation. */
  now: string;
  /** Number of scanned enabled routines. */
  scanned: number;
  /** Routines started in this pass (max one initially). */
  started: FractalRoutineAutomationStartedEntry[];
  /** Routines skipped and reasons. */
  skipped: FractalRoutineAutomationSkippedEntry[];
  /** Routines failed and reasons. */
  failed: FractalRoutineAutomationFailedEntry[];
}

/**
 * @interface IFractalRoutineService
 * @description Defines the contract for the RoutineService managing Fractal Routines and Runs.
 * All methods return `ResponseWithCTA`-wrapped data to provide rich CLI call-to-action hints.
 */
export interface IFractalRoutineService {
  /**
   * Retrieves a list of routines with optional filtering.
   */
  list(
    params: FractalRoutineListQueryInput,
  ): Promise<ResponseWithCTA<{ routines: FractalRoutine[] }>>;

  /**
   * Retrieves a single routine by its ID, hydrated with runs.
   */
  getById(params: {
    id: string;
  }): Promise<ResponseWithCTA<{ routine: FractalRoutineWithRuns | null }>>;

  /**
   * Creates a new routine record.
   */
  create(
    params: FractalRoutineCreateInput,
  ): Promise<ResponseWithCTA<FractalRoutine>>;

  /**
   * Updates an existing routine with partial data.
   */
  update(params: {
    id: string;
    data: FractalRoutineUpdateInput;
  }): Promise<ResponseWithCTA<FractalRoutine>>;

  /**
   * Permanently removes a routine and cleans up its runs.
   */
  delete(params: { id: string }): Promise<ResponseWithCTA>;

  /**
   * Fires a routine via webhook, creating a run and dispatching to the agent.
   */
  fire(params: {
    id: string;
    input: FractalRoutineFireInput;
  }): Promise<ResponseWithCTA<{ run: FractalRun; chat?: { id: string } }>>;

  /**
   * Recovers stale routine runs currently marked as running.
   */
  recoverStaleRuns(
    params: FractalRoutineRecoverStaleRunsInput,
  ): Promise<ResponseWithCTA<FractalRoutineRecoverStaleRunsResult>>;

  /**
   * Processes enabled scheduled routines and starts at most one due routine.
   */
  processScheduled(
    params: FractalRoutineProcessScheduledInput,
  ): Promise<ResponseWithCTA<FractalRoutineProcessScheduledResult>>;
}
