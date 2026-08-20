import { z } from "zod";
import type { ResponseWithCTA } from "@/core/interfaces/response.interfaces";

/**
 * Valid statuses for a routine.
 *
 * - **enabled** — Routine is active and can be triggered.
 * - **disabled** — Routine is inactive and will not respond to triggers.
 * - **paused** — Routine is temporarily suspended.
 */
export const RoutineStatusSchema = z.enum([
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
export const RunStatusSchema = z.enum([
  "pending",
  "running",
  "completed",
  "error",
]);

/**
 * Schema for a webhook trigger configuration.
 */
export const RoutineWebhookTriggerSchema = z.object({
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
export const RoutineScheduledTriggerSchema = z.object({
  type: z.literal("scheduled"),
  config: z.object({
    cron: z.string().describe("Cron expression defining the schedule."),
  }),
});

/**
 * Schema for an activity-event trigger configuration. Absent from this
 * file's source alongside webhook/scheduled — reconstructed from
 * `presentation/consts/routine-triggers.ts`'s own `"activity"` variant of
 * `RoutineTriggerFormValue`, which this schema must match for
 * `RoutineTriggerTypeId` (`Routine["triggers"][number]["type"]`) to
 * include `"activity"` at all — see that file's `ROUTINE_TRIGGER_TYPE_ORDER`
 * and `presentation/components/triggers/activity-trigger-row.tsx`.
 */
export const RoutineActivityTriggerSchema = z.object({
  type: z.literal("activity"),
  config: z.object({
    namespace: z.string().describe("Activity namespace to listen on."),
    event: z.string().describe("Activity event name within the namespace."),
    filters: z.array(
      z.object({
        path: z.string(),
        operator: z.enum(["eq", "neq", "contains"]),
        value: z.string(),
      }),
    ).optional(),
  }),
});

/**
 * Discriminated union for routine triggers.
 * Supports webhook, scheduled, and activity trigger types.
 */
export const RoutineTriggerSchema = z.discriminatedUnion("type", [
  RoutineWebhookTriggerSchema,
  RoutineScheduledTriggerSchema,
  RoutineActivityTriggerSchema,
]);

/**
 * Primary schema for a AOS Routine record.
 * A Routine is a saved agent configuration (prompt + triggers) that runs automatically.
 */
export const RoutineSchema = z.object({
  id: z
    .string()
    .describe("Unique identifier (slug) auto-generated from the routine name."),
  name: z.string().describe("Human-readable name of the routine."),
  agent: z.string().describe("Agent slug (owner) that executes this routine."),
  content: z
    .string()
    .describe("Markdown body content used as the system prompt for execution."),
  triggers: z
    .array(RoutineTriggerSchema)
    .default([])
    .describe("List of triggers that can activate this routine."),
  status: RoutineStatusSchema.describe(
    "Current operational status of the routine.",
  ),
  createdAt: z.string().describe("ISO timestamp when the routine was created."),
  updatedAt: z
    .string()
    .describe("ISO timestamp when the routine was last updated."),
});

/**
 * Primary schema for a AOS Run record.
 * A Run represents a single execution of a routine.
 */
export const RunSchema = z.object({
  id: z.string().describe("Unique identifier for the run (UUID)."),
  routine: z.string().describe("Parent routine ID."),
  agent: z.string().describe("Agent slug that executed this run."),
  // Absent from this file's source — reconstructed from
  // `presentation/pages/($id)/components/routine-run-history.tsx`'s
  // `triggerLabel` switch, this field's only consumer.
  trigger: z.enum(["manual", "scheduled", "webhook", "activity"]).optional(),
  status: RunStatusSchema.describe(
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
export const RoutineCreateSchema = z.object({
  name: z.string().describe("Human-readable name of the routine."),
  agent: z.string().describe("Agent slug (owner) that executes this routine."),
  prompt: z
    .string()
    .describe("Markdown body content used as the system prompt for execution."),
  triggers: z
    .array(RoutineTriggerSchema)
    .default([])
    .describe("List of triggers that can activate this routine."),
  status: RoutineStatusSchema.default("enabled").describe(
    "Initial operational status.",
  ),
});

/**
 * Schema for updating an existing routine.
 * All fields are optional — only provide the fields you want to change.
 */
export const RoutineUpdateSchema = z.object({
  name: z.string().optional(),
  agent: z.string().optional(),
  prompt: z.string().optional(),
  triggers: z.array(RoutineTriggerSchema).optional(),
  status: RoutineStatusSchema.optional(),
});

/**
 * Schema for querying the routine list.
 */
export const RoutineListQuerySchema = z.object({
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
export const RoutineWebhookFireSchema = z.object({
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
export const RoutineScheduledFireSchema = z.object({
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
export const RoutineFireSchema = z.discriminatedUnion("type", [
  RoutineWebhookFireSchema,
  RoutineScheduledFireSchema,
]);

/**
 * Schema for creating a new run.
 */
export const RunCreateSchema = z.object({
  routine: z.string().describe("Parent routine ID."),
  agent: z.string().describe("Agent slug that executes this run."),
  status: RunStatusSchema.default("pending"),
  input: z.string().default(""),
  chat: z.string().optional(),
  output: z.string().optional(),
  startedAt: z.string(),
  finishedAt: z.string().optional(),
});

/**
 * TypeScript type for a Routine record.
 */
export type Routine = z.infer<typeof RoutineSchema>;

/**
 * TypeScript type for a Run record.
 */
export type Run = z.infer<typeof RunSchema>;

/**
 * Input type for creating a routine.
 */
export type RoutineCreateInput = z.infer<
  typeof RoutineCreateSchema
>;

/**
 * Input type for updating a routine.
 */
export type RoutineUpdateInput = z.infer<
  typeof RoutineUpdateSchema
>;

/**
 * Input type for querying routines.
 */
export type RoutineListQueryInput = z.infer<
  typeof RoutineListQuerySchema
>;

/**
 * Reserved routine agent targets — not resolved to a real agent record.
 * Absent from this file's source; reconstructed from
 * `presentation/helpers/routine.helper.ts`'s own `RESERVED_AGENTS` const
 * (`["orchestrator", "all"] as const satisfies readonly
 * RoutineReservedAgent[]`), its only consumer.
 */
export type RoutineReservedAgent = "orchestrator" | "all";

/**
 * One filter clause on an `"activity"` routine trigger — narrows which
 * activity events fire the routine. Absent from this file's source (the
 * trigger schemas above have no `filters` field yet); reconstructed from
 * `presentation/components/triggers/activity-trigger-row.tsx`'s filter
 * editor (the `operator` union matches its `<Select>` options exactly) and
 * `presentation/consts/routine-triggers.ts`'s `filters?:
 * RoutineActivityFilter[]` field.
 */
export interface RoutineActivityFilter {
  path: string;
  operator: "eq" | "neq" | "contains";
  value: string;
}

/**
 * Input type for firing a routine.
 */
export type RoutineFireInput = z.infer<typeof RoutineFireSchema>;

/**
 * Input type for creating a run.
 */
export type RunCreateInput = z.infer<typeof RunCreateSchema>;

/**
 * Routine record enriched with its runs.
 */
export type RoutineWithRuns = Routine & {
  runs: Run[];
  fireUrl: string | null;
};

/**
 * Input parameters for stale running routine-run recovery.
 */
export interface RoutineRecoverStaleRunsInput {
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
export interface RoutineProcessScheduledInput {
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
export interface RoutineAutomationSkippedEntry {
  /** Routine or run identifier. */
  id: string;
  /** Human-readable skip reason. */
  reason: string;
}

/**
 * Normalized failed routine entry.
 */
export interface RoutineAutomationFailedEntry {
  /** Routine or run identifier. */
  id: string;
  /** Human-readable failure reason. */
  reason: string;
}

/**
 * Normalized started routine entry.
 */
export interface RoutineAutomationStartedEntry {
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
export interface RoutineAutomationRecoveredEntry {
  /** Run identifier. */
  id: string;
  /** Recovery action applied to this run. */
  action: "marked_error";
}

/**
 * Result payload for stale run recovery.
 */
export interface RoutineRecoverStaleRunsResult {
  /** Effective workspace id processed. */
  workspace: string;
  /** Effective ISO timestamp used during evaluation. */
  now: string;
  /** Number of scanned running runs. */
  scanned: number;
  /** Runs recovered in this pass. */
  recovered: RoutineAutomationRecoveredEntry[];
  /** Runs skipped and reasons. */
  skipped: RoutineAutomationSkippedEntry[];
  /** Runs failed and reasons. */
  failed: RoutineAutomationFailedEntry[];
}

/**
 * Result payload for scheduled routine processing.
 */
export interface RoutineProcessScheduledResult {
  /** Effective workspace id processed. */
  workspace: string;
  /** Effective ISO timestamp used during evaluation. */
  now: string;
  /** Number of scanned enabled routines. */
  scanned: number;
  /** Routines started in this pass (max one initially). */
  started: RoutineAutomationStartedEntry[];
  /** Routines skipped and reasons. */
  skipped: RoutineAutomationSkippedEntry[];
  /** Routines failed and reasons. */
  failed: RoutineAutomationFailedEntry[];
}

/**
 * @interface IRoutineService
 * @description Defines the contract for the RoutineService managing AOS Routines and Runs.
 * All methods return `ResponseWithCTA`-wrapped data to provide rich CLI call-to-action hints.
 */
export interface IRoutineService {
  /**
   * Retrieves a list of routines with optional filtering.
   */
  list(
    params: RoutineListQueryInput,
  ): Promise<ResponseWithCTA<{ routines: Routine[] }>>;

  /**
   * Retrieves a single routine by its ID, hydrated with runs.
   */
  getById(params: {
    id: string;
  }): Promise<ResponseWithCTA<{ routine: RoutineWithRuns | null }>>;

  /**
   * Creates a new routine record.
   */
  create(
    params: RoutineCreateInput,
  ): Promise<ResponseWithCTA<Routine>>;

  /**
   * Updates an existing routine with partial data.
   */
  update(params: {
    id: string;
    data: RoutineUpdateInput;
  }): Promise<ResponseWithCTA<Routine>>;

  /**
   * Permanently removes a routine and cleans up its runs.
   */
  delete(params: { id: string }): Promise<ResponseWithCTA>;

  /**
   * Fires a routine via webhook, creating a run and dispatching to the agent.
   */
  fire(params: {
    id: string;
    input: RoutineFireInput;
  }): Promise<ResponseWithCTA<{ run: Run; chat?: { id: string } }>>;

  /**
   * Recovers stale routine runs currently marked as running.
   */
  recoverStaleRuns(
    params: RoutineRecoverStaleRunsInput,
  ): Promise<ResponseWithCTA<RoutineRecoverStaleRunsResult>>;

  /**
   * Processes enabled scheduled routines and starts at most one due routine.
   */
  processScheduled(
    params: RoutineProcessScheduledInput,
  ): Promise<ResponseWithCTA<RoutineProcessScheduledResult>>;
}
