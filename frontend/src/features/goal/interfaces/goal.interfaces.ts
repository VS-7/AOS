import { z } from "zod";
import type { ResponseWithCTA } from "@/core/interfaces/response.interfaces";
import { Schema } from "@/core/helpers/schema.helper";

/**
 * Valid statuses a goal can have throughout its lifecycle.
 *
 * - **active** — Currently being pursued. The default status for new goals.
 * - **achieved** — Successfully completed. The goal's outcome has been met.
 * - **abandoned** — No longer pursued. The goal was intentionally deprioritized or cancelled.
 */
export const GoalStatusSchema = z.enum([
  "active",
  "achieved",
  "abandoned",
]);

/**
 * Priority levels for a goal. Determines urgency and execution order.
 *
 * - **urgent** — Must be addressed immediately. Blocks other work.
 * - **high** — Important and should be done soon. High visibility.
 * - **medium** — Normal priority. Default for most goals.
 * - **low** — Nice to have. Can be deferred.
 * - **no_priority** — Not yet prioritized.
 */
export const GoalPrioritySchema = z.enum([
  "no_priority",
  "urgent",
  "high",
  "medium",
  "low",
]);

/**
 * Schema for a AOS Goal record.
 *
 * Goals represent high-level objectives that can be associated with tasks.
 * They are stored as Markdown files with YAML frontmatter in `.aos/goals/{id}/GOAL.md`.
 *
 * @property id - Auto-generated incremental ID (e.g., `GOAL-001`).
 * @property slug - URL-friendly version of the title, auto-generated.
 * @property title - Human-readable title of the goal.
 * @property description - Concise description of the goal.
 * @property content - Markdown body describing the goal in detail. Stored as file content below YAML frontmatter.
 * @property priority - Priority level. Defaults to `no_priority`.
 * @property project - Optional project ID this goal belongs to. Enables filtering goals by project.
 * @property deadline - ISO timestamp for when the goal should be achieved.
 * @property status - Current lifecycle status. Defaults to `active`.
 * @property createdAt - ISO timestamp. Auto-generated on creation.
 * @property updatedAt - ISO timestamp. Auto-generated on creation, updated on modification.
 *
 * @example
 * ```typescript
 * const goal: Goal = {
 *   id: "GOAL-001",
 *   slug: "launch-v1",
 *   title: "Launch V1",
 *   description: "Ship the first stable release of AOS.",
 *   content: "# Launch V1\n\nDetailed markdown description...",
 *   priority: "high",
 *   project: "website-redesign",
 *   deadline: "2026-01-01T00:00:00.000Z",
 *   status: "active",
 *   createdAt: "2025-06-14T12:00:00.000Z",
 *   updatedAt: "2025-06-14T12:00:00.000Z",
 * };
 * ```
 */
export const GoalSchema = z.object({
  id: z.string().describe("Auto-generated incremental ID (e.g., 'GOAL-001')."),
  slug: z
    .string()
    .describe(
      "URL-friendly version of the title, auto-generated from the title via Slug helper.",
    ),
  title: z.string().describe("Human-readable title of the goal."),
  description: z
    .string()
    .optional()
    .describe("Concise description of the goal."),
  content: z
    .string()
    .optional()
    .describe(
      "Markdown body describing the goal in detail. Stored as the file content below the YAML frontmatter.",
    ),
  priority: GoalPrioritySchema.optional()
    .default("no_priority")
    .describe(
      "Priority level of the goal. Determines urgency. Defaults to 'no_priority'.",
    ),
  project: z
    .string()
    .optional()
    .describe(
      "Project ID this goal belongs to. Enables filtering goals by project.",
    ),
  deadline: z
    .string()
    .optional()
    .describe("ISO timestamp for when the goal should be achieved."),
  status: GoalStatusSchema.describe(
    "Current lifecycle status of the goal. Defaults to 'active'.",
  ),
  createdAt: z.string().describe("ISO timestamp when the goal was created."),
  updatedAt: z
    .string()
    .describe("ISO timestamp when the goal was last updated."),
});

/**
 * Schema for creating a new goal.
 *
 * Omits auto-generated fields (`id`, `slug`, `createdAt`, `updatedAt`).
 * The `id` is auto-generated incrementally (e.g., `GOAL-001`).
 * The `slug` is generated from the title via `Slug.generate(title)`.
 * The `status` defaults to `active` if not provided.
 * The `priority` defaults to `no_priority` if not provided.
 *
 * @example
 * ```typescript
 * const input: GoalCreateInput = {
 *   title: "Launch V1",
 *   description: "Ship the first stable release.",
 *   priority: "high",
 *   deadline: "2026-01-01T00:00:00.000Z",
 * };
 * ```
 */
export const GoalCreateSchema = z.object({
  title: z.string().describe("Human-readable title of the goal."),
  description: z
    .string()
    .optional()
    .describe("Concise description of the goal."),
  content: z
    .string()
    .optional()
    .describe(
      "Markdown body describing the goal in detail. Stored as the file content below the YAML frontmatter.",
    ),
  priority: GoalPrioritySchema.default("no_priority").describe(
    "Priority level of the goal. Determines urgency. Defaults to 'no_priority'.",
  ),
  project: z
    .string()
    .optional()
    .describe(
      "Project ID this goal belongs to. Enables filtering goals by project.",
    ),
  deadline: z
    .string()
    .optional()
    .describe("ISO timestamp for when the goal should be achieved."),
  status: GoalStatusSchema.default("active").describe(
    "Current lifecycle status of the goal. Defaults to 'active'.",
  ),
});

/**
 * Schema for partially updating an existing goal.
 *
 * All fields are optional except the identifier used for routing.
 *
 * @example
 * ```typescript
 * const input: GoalUpdateInput = {
 *   title: "Launch V1 - Updated",
 *   status: "achieved",
 * };
 * ```
 */
export const GoalUpdateSchema = GoalSchema.omit({
  id: true,
  createdAt: true,
  updatedAt: true,
}).partial();

/**
 * Schema for filtering goal lists.
 * All fields are optional — combine them to narrow down results.
 *
 * @example
 * ```typescript
 * const query: GoalListQueryInput = {
 *   query: "launch",
 *   status: ["active"],
 *   project: "website-redesign",
 *   limit: "10",
 * };
 * ```
 */
export const GoalListQuerySchema = Schema.object({
  query: z
    .string()
    .optional()
    .describe(
      "Full-text search across goal id and title. Use to find goals by keyword.",
    ),
  status: z
    .custom<z.infer<typeof GoalStatusSchema>[]>((value) =>
      Array.isArray(value) ? value : String(value)?.split(",") || [],
    )
    .optional()
    .describe(
      "Filter by one or more goal statuses. Comma-separated. Values: active, achieved, abandoned.",
    ),
  project: z
    .string()
    .optional()
    .describe("Filter goals by the project ID they belong to."),
  limit: z
    .string()
    .optional()
    .describe("Maximum number of goals to return. Use for pagination."),
  offset: z
    .string()
    .optional()
    .describe("Number of goals to skip. Use with limit for pagination."),
});

/**
 * Schema for a goal with associated context.
 *
 * Extends the base goal schema with task association data.
 * The `tasks` field is populated by querying all tasks where `goal === id`.
 *
 * @property tasks - List of task IDs associated with this goal.
 *
 * @example
 * ```typescript
 * const goalWithContext: GoalWithContext = {
 *   ...goalData,
 *   tasks: ["FRA-001", "FRA-002"],
 * };
 * ```
 */
export const GoalWithContextSchema = GoalSchema.extend({
  tasks: z
    .array(z.string())
    .default([])
    .describe(
      "List of task IDs associated with this goal, populated by querying tasks where goal === id.",
    ),
});

// ─── Goal Types ──────────────────────────────────────────────────────────────

export type Goal = z.infer<typeof GoalSchema>;
export type GoalStatus = z.infer<typeof GoalStatusSchema>;
export type GoalPriority = z.infer<typeof GoalPrioritySchema>;
export type GoalCreateInput = z.infer<typeof GoalCreateSchema>;
export type GoalUpdateInput = z.infer<typeof GoalUpdateSchema>;
export type GoalListQueryInput = z.infer<
  typeof GoalListQuerySchema
>;
export type GoalWithContext = z.infer<
  typeof GoalWithContextSchema
>;

export interface GoalGetParams {
  /** The unique identifier (slug) of the goal to retrieve. */
  id: string;
}

export interface GoalDeleteParams {
  /** The unique identifier (slug) of the goal to delete. */
  id: string;
}

export interface GoalUpdateParams {
  /** The unique identifier (slug) of the goal to update. */
  id: string;
  /** Partial data to merge into the existing goal record. */
  data: GoalUpdateInput;
}

/**
 * @interface IGoalService
 * @description Contract for the GoalService managing the goal lifecycle and persistence.
 * Provides full CRUD operations with CTA-enhanced responses for CLI integration.
 */
export interface IGoalService {
  /**
   * Lists goals with optional filtering and full-text search.
   *
   * @param params - Optional query parameters for filtering, search, and pagination.
   * @returns A CTA-wrapped list of goals sorted by updatedAt descending.
   *
   * @example
   * ```typescript
   * const result = await goalService.list({ status: ["active"], project: "website-redesign", limit: "10" });
   * ```
   */
  list(
    params?: GoalListQueryInput,
  ): Promise<ResponseWithCTA<{ goals: Goal[] }>>;

  /**
   * Retrieves a single goal by its unique identifier, including associated task IDs.
   *
   * @param params - Contains the goal ID to look up.
   * @returns The goal with its associated task IDs.
   * @throws {GoalError} with code `AOS_GOAL_NOT_FOUND` if the goal does not exist.
   *
   * @example
   * ```typescript
   * const result = await goalService.getById({ id: "launch-v1" });
   * ```
   */
  getById(
    params: GoalGetParams,
  ): Promise<ResponseWithCTA<{ goal: GoalWithContext }>>;

  /**
   * Creates a new goal. The ID is auto-generated from the title via Slug.
   *
   * @param params - The creation payload (title is required).
   * @returns The newly created goal record.
   * @throws {GoalError} with code `AOS_GOAL_PERSISTENCE_ERROR` if creation fails.
   *
   * @example
   * ```typescript
   * const result = await goalService.create({ title: "Launch V1" });
   * ```
   */
  create(
    params: GoalCreateInput,
  ): Promise<ResponseWithCTA<{ goal: Goal }>>;

  /**
   * Updates an existing goal with partial data.
   *
   * @param params - Contains the goal ID and partial data to merge.
   * @returns The updated goal record.
   * @throws {GoalError} with code `AOS_GOAL_NOT_FOUND` if the goal does not exist.
   * @throws {GoalError} with code `AOS_GOAL_PERSISTENCE_ERROR` if update fails.
   *
   * @example
   * ```typescript
   * const result = await goalService.update({ id: "launch-v1", data: { status: "achieved" } });
   * ```
   */
  update(
    params: GoalUpdateParams,
  ): Promise<ResponseWithCTA<{ goal: Goal }>>;

  /**
   * Permanently deletes a goal by its ID.
   *
   * @param params - Contains the goal ID to delete.
   * @returns A confirmation CTA response.
   * @throws {GoalError} with code `AOS_GOAL_NOT_FOUND` if the goal does not exist.
   *
   * @example
   * ```typescript
   * await goalService.delete({ id: "launch-v1" });
   * ```
   */
  delete(params: GoalDeleteParams): Promise<ResponseWithCTA>;
}
