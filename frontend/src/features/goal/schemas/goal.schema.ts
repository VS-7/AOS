import { z } from "zod";
import { Schema } from "@/core/helpers/schema.helper";

// ============================================================================
// Enums
// Domain enum schemas for Goal
// ============================================================================

/**
 * Valid statuses a goal can have throughout its lifecycle.
 *
 * @example
 * ```typescript
 * "active"
 * ```
 */
export const GoalStatusSchema = z
  .enum(["active", "achieved", "abandoned", "paused"])
  .describe(
    "Goal lifecycle status. Use active for in-progress goals, achieved when done, abandoned when cancelled, paused when set aside. Example: \"active\".",
  );

/**
 * Priority levels for a goal. Determines urgency and execution order.
 *
 * @example
 * ```typescript
 * "high"
 * ```
 */
export const GoalPrioritySchema = z
  .enum(["no_priority", "urgent", "high", "medium", "low"])
  .describe(
    "Goal priority. Use urgent for blockers, high for soon, medium default, low deferrable, no_priority unset. Example: \"high\".",
  );

// ============================================================================
// Entity
// Full persisted / domain shape — master blueprint
// ============================================================================

/**
 * Complete AOS goal record — master shape for persistence and DTOs.
 *
 * Maps to `.aos/goals/{id}/GOAL.md` (and skill-bound goal files).
 *
 * @example
 * ```typescript
 * {
 *   id: "GOAL-001",
 *   slug: "launch-v1",
 *   title: "Launch V1",
 *   description: "Ship the first stable release of AOS.",
 *   content: "# Launch V1",
 *   priority: "high",
 *   project: "website-redesign",
 *   deadline: "2026-01-01T00:00:00.000Z",
 *   status: "active",
 *   createdAt: "2025-06-14T12:00:00.000Z",
 *   updatedAt: "2025-06-14T12:00:00.000Z"
 * }
 * ```
 */
export const GoalSchema = Schema.object({
  id: z
    .string()
    .describe(
      "Auto-generated incremental ID for the goal. Example: \"GOAL-001\".",
    ),
  slug: z
    .string()
    .describe(
      "URL-friendly slug auto-generated from the title. Example: \"launch-v1\".",
    ),
  title: z
    .string()
    .describe("Human-readable title of the goal. Example: \"Launch V1\"."),
  description: z
    .string()
    .optional()
    .describe(
      "Concise one-line description of the goal. Example: \"Ship the first stable release.\".",
    ),
  content: z
    .string()
    .optional()
    .describe(
      "Markdown body stored below YAML frontmatter. Example: \"# Launch V1\\n\\nDetails...\".",
    ),
  priority: GoalPrioritySchema.optional()
    .default("no_priority")
    .describe(
      "Priority level of the goal. Defaults to no_priority. Example: \"high\".",
    ),
  project: z
    .string()
    .optional()
    .describe(
      "Optional project ID this goal belongs to. Example: \"website-redesign\".",
    ),
  deadline: z
    .string()
    .optional()
    .describe(
      "ISO-8601 deadline timestamp as a string. Example: \"2026-01-01T00:00:00.000Z\".",
    ),
  status: GoalStatusSchema.describe(
    "Current lifecycle status of the goal. Example: \"active\".",
  ),
  createdAt: z
    .string()
    .describe(
      "ISO-8601 created timestamp as a string. Example: \"2025-06-14T12:00:00.000Z\".",
    ),
  updatedAt: z
    .string()
    .describe(
      "ISO-8601 updated timestamp as a string. Example: \"2025-06-14T12:00:00.000Z\".",
    ),
});

/**
 * Goal with associated task IDs (service enrichment, not persisted).
 *
 * @example
 * ```typescript
 * {
 *   id: "GOAL-001",
 *   slug: "launch-v1",
 *   title: "Launch V1",
 *   status: "active",
 *   priority: "high",
 *   createdAt: "2025-06-14T12:00:00.000Z",
 *   updatedAt: "2025-06-14T12:00:00.000Z",
 *   tasks: ["FRA-001", "FRA-002"]
 * }
 * ```
 */
export const GoalWithContextSchema = GoalSchema.extend({
  tasks: z
    .array(z.string())
    .default([])
    .describe(
      "Task IDs associated with this goal. Example: [\"FRA-001\",\"FRA-002\"].",
    ),
});

// ============================================================================
// Create
// Input derived from entity (omits server-owned fields)
// ============================================================================

/**
 * Create-goal input — derived from {@link GoalSchema}.
 *
 * Omits server-owned `id` / `slug` / timestamps. Domain SSOT for
 * procedure/service/CLI/tool/forms.
 *
 * @example
 * ```typescript
 * {
 *   title: "Launch V1",
 *   description: "Ship the first stable release.",
 *   priority: "high",
 *   deadline: "2026-01-01T00:00:00.000Z"
 * }
 * ```
 */
export const GoalCreateInputSchema = GoalSchema.omit({
  id: true,
  slug: true,
  createdAt: true,
  updatedAt: true,
}).extend({
  priority: GoalPrioritySchema.default("no_priority").describe(
    "Priority level. Defaults to no_priority. Example: \"high\".",
  ),
  status: GoalStatusSchema.default("active").describe(
    "Initial lifecycle status. Defaults to active. Example: \"active\".",
  ),
});

// ============================================================================
// List
// Input = filters only (no entity field overlap → Schema.object is OK)
// ============================================================================

/**
 * List-goals input — query filters for procedure/service/CLI/tool/forms.
 *
 * @example
 * ```typescript
 * {
 *   query: "launch",
 *   status: ["active"],
 *   project: "website-redesign",
 *   limit: "10"
 * }
 * ```
 */
export const GoalListInputSchema = Schema.object({
  query: z
    .string()
    .optional()
    .describe(
      "Full-text search across goal id and title. Example: \"launch\".",
    ),
  status: z
    .array(GoalStatusSchema)
    .optional()
    .describe(
      "Filter by one or more statuses. Example: [\"active\"] or [\"active\",\"achieved\"].",
    ),
  project: z
    .string()
    .optional()
    .describe(
      "Filter goals by project ID. Example: \"website-redesign\".",
    ),
  limit: z
    .string()
    .optional()
    .describe("Maximum number of goals to return. Example: \"10\"."),
  offset: z
    .string()
    .optional()
    .describe("Number of goals to skip for pagination. Example: \"0\"."),
});

// ============================================================================
// Get
// Path-only Input — domain identifier `goal`
// ============================================================================

/**
 * Get-goal input — assembled from route/CLI params (`goal`), never bare `id`.
 *
 * @example
 * ```typescript
 * {
 *   goal: "GOAL-001"
 * }
 * ```
 */
export const GoalGetInputSchema = Schema.object({
  goal: z
    .string()
    .describe(
      "Unique goal identifier (e.g. GOAL-001) to retrieve. Example: \"GOAL-001\".",
    ),
});

// ============================================================================
// Update
// Input derived from entity (partial patch)
// ============================================================================

/**
 * Update-goal input — derived from {@link GoalSchema} (partial).
 *
 * @example
 * ```typescript
 * {
 *   title: "Launch V1 - Updated",
 *   status: "achieved"
 * }
 * ```
 */
export const GoalUpdateInputSchema = GoalSchema.omit({
  id: true,
  createdAt: true,
  updatedAt: true,
}).partial();

// ============================================================================
// Delete
// Path-only Input — same shape as Get
// ============================================================================

/**
 * Delete-goal input — same shape as {@link GoalGetInputSchema}.
 *
 * @example
 * ```typescript
 * {
 *   goal: "GOAL-001"
 * }
 * ```
 */
export const GoalDeleteInputSchema = GoalGetInputSchema;
