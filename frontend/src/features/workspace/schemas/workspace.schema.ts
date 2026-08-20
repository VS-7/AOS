import { z } from "zod";
import { Schema } from "@/core/helpers/schema.helper";

// ============================================================================
// Enums
// Domain enum schemas for Workspace
// ============================================================================

/**
 * Role of a workspace member — gates privileged actions and member management.
 *
 * @example
 * ```typescript
 * "owner"
 * ```
 */
export const WorkspaceMemberRoleSchema = z
  .enum(["owner", "member"])
  .describe(
    'Member role. owner manages members and settings; member has read access. Example: "owner".',
  );

/**
 * Communication tone of the initial orchestrator agent.
 *
 * @example
 * ```typescript
 * "professional"
 * ```
 */
export const WorkspaceOrchestratorToneSchema = z
  .enum(["efficient", "friendly", "professional", "candid"])
  .describe(
    'Orchestrator communication tone. Example: "professional".',
  );

/**
 * Response depth style of the initial orchestrator agent.
 *
 * @example
 * ```typescript
 * "balanced"
 * ```
 */
export const WorkspaceOrchestratorStyleSchema = z
  .enum(["concise", "balanced", "detailed"])
  .describe(
    'Orchestrator response style. Example: "balanced".',
  );

// ============================================================================
// Nested Objects
// Extracted object schemas composed into the entity
// ============================================================================

/**
 * Custom-domain routing entry — maps one FQDN to a workspace artifact.
 *
 * @example
 * ```typescript
 * {
 *   artifactId: "550e8400-e29b-41d4-a716-446655440000",
 *   workspaceId: "project-alpha",
 *   createdAt: "2026-07-17T12:00:00.000Z"
 * }
 * ```
 */
export const WorkspaceDomainSchema = Schema.object({
  artifactId: z
    .string()
    .describe(
      'Artifact this domain resolves to. Example: "550e8400-e29b-41d4-a716-446655440000".',
    ),
  workspaceId: z
    .string()
    .describe('Workspace the artifact belongs to. Example: "project-alpha".'),
  createdAt: z
    .string()
    .describe(
      'ISO timestamp when the domain was registered. Example: "2026-07-17T12:00:00.000Z".',
    ),
});

/**
 * Task type definition — the taxonomy used across agents, tasks, and views.
 *
 * @example
 * ```typescript
 * {
 *   id: "bug",
 *   label: "Bug",
 *   color: "#ef4444",
 *   description: "A software bug",
 *   instructions: "Reproduce and isolate before fixing."
 * }
 * ```
 */
export const WorkspaceTaskTypeSchema = Schema.object({
  id: z
    .string()
    .describe(
      'Unique identifier for the task type (e.g. "bug", "feature"). Example: "bug".',
    ),
  label: z
    .string()
    .describe('Human-readable label displayed in the UI. Example: "Bug".'),
  color: z
    .string()
    .describe(
      'Hex color code representing the task type. Example: "#ef4444".',
    ),
  description: z
    .string()
    .optional()
    .describe(
      'A brief description of the task type purpose. Example: "A software bug".',
    ),
  instructions: z
    .string()
    .optional()
    .describe(
      'Optional agent instructions specific to this task type. Example: "Reproduce and isolate before fixing.".',
    ),
});

/**
 * Custom workspace label — used to categorize workspace resources.
 *
 * @example
 * ```typescript
 * {
 *   id: "ui-ux",
 *   label: "UI/UX",
 *   icon: "Palette",
 *   color: "#ec4899"
 * }
 * ```
 */
export const WorkspaceLabelSchema = Schema.object({
  id: z
    .string()
    .describe('Unique identifier for the workspace label. Example: "ui-ux".'),
  label: z
    .string()
    .describe('Human-readable label displayed in the UI. Example: "UI/UX".'),
  icon: z
    .string()
    .describe(
      'Lucide icon identifier used to render the label. Example: "Palette".',
    ),
  color: z
    .string()
    .describe('Hex color code representing the label. Example: "#ec4899".'),
});

/**
 * Workspace membership entry — user, role, and join timestamp.
 *
 * @example
 * ```typescript
 * {
 *   userId: "550e8400-e29b-41d4-a716-446655440000",
 *   role: "owner",
 *   addedAt: "2026-07-17T12:00:00.000Z"
 * }
 * ```
 */
export const WorkspaceMemberSchema = Schema.object({
  userId: z
    .string()
    .uuid()
    .describe(
      'User UUID of the member. Example: "550e8400-e29b-41d4-a716-446655440000".',
    ),
  role: WorkspaceMemberRoleSchema,
  addedAt: z
    .string()
    .describe(
      'ISO timestamp when the member was added. Example: "2026-07-17T12:00:00.000Z".',
    ),
});

/**
 * Git worktree policy — cleanup and creation behavior for task isolation.
 *
 * @example
 * ```typescript
 * {
 *   deleteOldWorktrees: true,
 *   worktreeLimit: 15,
 *   onCreateScript: "bun install"
 * }
 * ```
 */
export const WorkspaceWorktreesSchema = Schema.object({
  deleteOldWorktrees: z
    .boolean()
    .describe("Delete stale worktrees automatically. Example: true."),
  worktreeLimit: z
    .number()
    .min(1)
    .max(50)
    .describe("Maximum concurrent worktrees. Example: 15."),
  onCreateScript: z
    .string()
    .optional()
    .describe(
      'Optional Bash script executed after worktree creation. Example: "bun install".',
    ),
});

/**
 * Git policy — branch prefix, push behavior, and commit/PR instructions.
 *
 * @example
 * ```typescript
 * {
 *   branchPrefix: "aos",
 *   forcePush: false,
 *   commitInstructions: "Conventional commits",
 *   prInstructions: "Link the task id"
 * }
 * ```
 */
export const WorkspaceGitSchema = Schema.object({
  branchPrefix: z
    .string()
    .optional()
    .describe(
      'Branch name prefix for generated branches. Example: "aos".',
    ),
  forcePush: z
    .boolean()
    .describe("Allow force-push to shared branches. Example: false."),
  commitInstructions: z
    .string()
    .optional()
    .describe(
      'Guidelines injected into commit prompts. Example: "Use conventional commits".',
    ),
  prInstructions: z
    .string()
    .optional()
    .describe(
      'Guidelines injected into pull request prompts. Example: "Link the task id".',
    ),
});

/**
 * Initial orchestrator agent configuration for workspace creation.
 *
 * @example
 * ```typescript
 * {
 *   name: "Atlas",
 *   tone: "professional",
 *   style: "balanced",
 *   autonomy: 0.8
 * }
 * ```
 */
export const WorkspaceOrchestratorSchema = Schema.object({
  name: z.string().describe('Orchestrator display name. Example: "Atlas".'),
  tone: WorkspaceOrchestratorToneSchema,
  style: WorkspaceOrchestratorStyleSchema,
  autonomy: z
    .number()
    .min(0)
    .max(1)
    .describe("Autonomy level from 0 to 1. Example: 0.8."),
});

// ============================================================================
// Entity
// Full persisted / domain shape — master blueprint
// ============================================================================

/**
 * Complete AOS workspace — master shape for persistence and derived DTOs.
 *
 * @example
 * ```typescript
 * {
 *   id: "project-alpha",
 *   name: "Project Alpha",
 *   path: "/path/to/project",
 *   logo: "https://example.com/logo.png",
 *   color: "#6366f1",
 *   tasks: [{ id: "bug", label: "Bug", color: "#ef4444" }],
 *   labels: [],
 *   members: [{ userId: "550e8400-e29b-41d4-a716-446655440000", role: "owner", addedAt: "2026-07-17T12:00:00.000Z" }],
 *   worktrees: { deleteOldWorktrees: true, worktreeLimit: 15 },
 *   git: { branchPrefix: "aos", forcePush: false },
 *   archived: false,
 *   domains: {},
 *   createdAt: "2026-07-17T12:00:00.000Z",
 *   updatedAt: "2026-07-17T12:00:00.000Z"
 * }
 * ```
 */
export const WorkspaceSchema = Schema.object({
  id: z
    .string()
    .describe('The generated ID for the workspace. Example: "project-alpha".'),
  name: z
    .string()
    .describe('The name of the workspace. Example: "Project Alpha".'),
  path: z
    .string()
    .describe(
      'The absolute path of the workspace. Example: "/path/to/project".',
    ),
  logo: z
    .string()
    .optional()
    .describe(
      'Optional logo URL. Example: "https://example.com/logo.png".',
    ),
  color: z
    .string()
    .optional()
    .describe('Optional color hex. Example: "#6366f1".'),
  tasks: z
    .array(WorkspaceTaskTypeSchema)
    .describe(
      'Task type definitions for this workspace. Example: [{ "id": "bug", "label": "Bug", "color": "#ef4444" }].',
    ),
  labels: z
    .array(WorkspaceLabelSchema)
    .describe(
      'Custom label definitions for this workspace. Example: [{ "id": "ui-ux", "label": "UI/UX", "icon": "Palette", "color": "#ec4899" }].',
    ),
  members: z
    .array(WorkspaceMemberSchema)
    .default([])
    .describe(
      'Workspace membership entries. Example: [{ "userId": "550e8400-e29b-41d4-a716-446655440000", "role": "owner", "addedAt": "2026-07-17T12:00:00.000Z" }].',
    ),
  worktrees: WorkspaceWorktreesSchema,
  git: WorkspaceGitSchema,
  archived: z
    .boolean()
    .default(false)
    .describe("Whether the workspace is archived. Example: false."),
  domains: z
    .record(z.string(), WorkspaceDomainSchema)
    .default({})
    .describe(
      'Map of custom domain → artifact routing entries; key is the FQDN. Example: { "workspace.example.com": { "artifactId": "550e8400-e29b-41d4-a716-446655440000", "workspaceId": "project-alpha", "createdAt": "2026-07-17T12:00:00.000Z" } }.',
    ),
  createdAt: z
    .string()
    .describe('ISO date string. Example: "2026-07-17T12:00:00.000Z".'),
  updatedAt: z
    .string()
    .describe('ISO date string. Example: "2026-07-17T12:00:00.000Z".'),
});

// ============================================================================
// Create
// Input derived from entity (omits server-owned fields)
// ============================================================================

/**
 * Create-workspace input — branding fields plus an optional orchestrator seed.
 *
 * @example
 * ```typescript
 * {
 *   name: "Project Alpha",
 *   path: "/path/to/project",
 *   logo: "https://example.com/logo.png",
 *   color: "#6366f1",
 *   orchestrator: { name: "Atlas", tone: "professional", style: "balanced", autonomy: 0.8 }
 * }
 * ```
 */
export const WorkspaceCreateInputSchema = WorkspaceSchema.omit({
  id: true,
  tasks: true,
  labels: true,
  members: true,
  worktrees: true,
  git: true,
  archived: true,
  domains: true,
  createdAt: true,
  updatedAt: true,
}).extend({
  path: z
    .string()
    .optional()
    .describe(
      'The absolute path of the workspace; omit to skip scaffolding. Example: "/path/to/project".',
    ),
  orchestrator: WorkspaceOrchestratorSchema.optional().describe(
    'Initial orchestrator agent configuration. Example: { "name": "Atlas", "tone": "professional", "style": "balanced", "autonomy": 0.8 }.',
  ),
});

// ============================================================================
// Get
// Input — optional workspace identifier (ID or slug)
// ============================================================================

/**
 * Get-workspace input — workspace ID or slug to retrieve.
 *
 * When omitted, defaults to the current workspace from the `AOS_WORKSPACE_ID`
 * environment variable.
 *
 * @example
 * ```typescript
 * {
 *   workspace: "project-alpha"
 * }
 * ```
 */
export const WorkspaceGetInputSchema = Schema.object({
  workspace: z
    .string()
    .optional()
    .describe(
      'Workspace ID or slug to retrieve; when omitted, defaults to the current workspace from the AOS_WORKSPACE_ID environment variable. Example: "project-alpha".',
    ),
});

// ============================================================================
// Update
// Input derived from entity (all fields optional)
// ============================================================================

/**
 * Update-workspace input — partial patch of any non-server-owned field.
 *
 * @example
 * ```typescript
 * {
 *   name: "Renamed Workspace",
 *   color: "#00ff00"
 * }
 * ```
 */
export const WorkspaceUpdateInputSchema = WorkspaceSchema.omit({
  id: true,
  createdAt: true,
  updatedAt: true,
  archived: true,
}).partial();

// ============================================================================
// Add Member
// Body-only input derived from the member nested schema
// ============================================================================

/**
 * Add-member input — user and role for a new membership entry.
 *
 * @example
 * ```typescript
 * {
 *   userId: "550e8400-e29b-41d4-a716-446655440000",
 *   role: "member"
 * }
 * ```
 */
export const WorkspaceMemberAddInputSchema =
  WorkspaceMemberSchema.pick({
    userId: true,
    role: true,
  });

// ============================================================================
// Update Member
// Body-only input derived from the member nested schema
// ============================================================================

/**
 * Update-member input — new role for an existing membership entry.
 *
 * @example
 * ```typescript
 * {
 *   role: "owner"
 * }
 * ```
 */
export const WorkspaceMemberUpdateInputSchema =
  WorkspaceMemberSchema.pick({
    role: true,
  });
