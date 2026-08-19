import { z } from "zod";
import { ToolsetSchema } from "@/features/toolset/interfaces/toolset.interfaces";
import { RuleSchema } from "@/core/interfaces/rule.interfaces";
import { Schema } from "@/core/helpers/schema.helper";
import type { ResponseWithCTA } from "@/core/interfaces/response.interfaces";

/**
 * SkillResourceSchema: Defines an external resource used by the skill.
 * @description Resources provide access to documentation, endpoints, or data files.
 * @example
 * ```typescript
 * const resource = {
 *   uri: "https://api.example.com/v1/users",
 *   mimeType: "application/json",
 *   description: "User API endpoint",
 *   always: true,
 *   rules: [accessRule]
 * };
 * ```
 */
export const SkillResourceSchema = z.object({
  uri: z
    .string()
    .describe(
      "Uniform Resource Identifier for the resource. Example: https://api.example.com/data"
    ),
  mimeType: z
    .string()
    .describe(
      "MIME type indicating the resource format. Example: application/json"
    ),
  description: z
    .string()
    .optional()
    .describe(
      "Purpose and details about the resource. Example: API endpoint for user data retrieval"
    ),
  always: z
    .boolean()
    .optional()
    .describe(
      "Whether the resource is always available and required. Example: true"
    ),
  rules: z
    .array(RuleSchema)
    .optional()
    .describe(
      "Access and usage rules for this resource. Example: [authRule, rateRule]"
    ),
});

/**
 * SkillMetadataSchema: Aggregates all sub-schemas defining the skill's behavior.
 * @description Metadata contains rules, toolsets, and resources for the skill.
 * @example
 * ```typescript
 * const metadata = {
 *   rules: [rule1],
 *   toolsets: [toolset1],
 *   resources: [resource1]
 * };
 * ```
 */
export const SkillMetadataSchema = z.object({
  rules: z
    .array(RuleSchema)
    .optional()
    .describe("Global rules that apply across the entire skill."),
  resources: z
    .array(SkillResourceSchema)
    .optional()
    .describe("External resources required or used by the skill."),
  toolsets: z
    .array(
      ToolsetSchema.omit({ tools: true })
        .extend({
          tools: z.array(z.string())
            .describe('All tools discovered from this toolset')
        }))
    .optional()
    .describe("Dynamic toolsets discovered for this skill"),
});

/**
 * SkillSchema: Main schema defining a complete Fractal skill.
 * @description Represents the complete structure of a skill including metadata and content.
 * @example
 * ```typescript
 * const skill = {
 *   name: "Data Analyzer",
 *   description: "Analyzes and processes data insights",
 *   metadata: { rules: [], toolsets: [], resources: [] }
 * };
 * ```
 */
export const SkillSchema = z.object({
  id: z.string().describe("Unique identifier for the skill"),
  name: z
    .string()
    .describe("Display name of the skill. Example: Data Analyzer"),
  description: z
    .string()
    .describe(
      "Concise description of skill functionality. Example: Analyzes datasets and provides insights"
    ),
  active: z
    .boolean()
    .optional()
    .default(true)
    .describe(
      "Whether the skill is currently active and enabled. Example: true"
    ),
  content: z
    .string()
    .optional()
    .describe(
      "Content of the skill. Example: Analyzes datasets and provides insights"
    ),
  metadata: SkillMetadataSchema.optional().describe(
    "Structured metadata defining skill behavior, rules, and resources"
  ),
});

/**
 * CreateSkillSchema: Schema for creating a new skill (excludes generated fields).
 * @description Used in API POST requests and CLI create commands.
 */
export const CreateSkillSchema = SkillSchema.omit({
  id: true,
  content: true,
  metadata: true,
});

/**
 * UpdateSkillSchema: Schema for updating an existing skill.
 * @description Used in API PATCH/PUT requests and CLI update commands.
 */
export const UpdateSkillSchema = SkillSchema.omit({
  id: true,
  content: true,
  metadata: true,
}).partial();

/**
 * SkillQueryInputSchema: Input validation schema for querying skills
 */
export const SkillQueryInputSchema = Schema.object({
  query: z
    .string()
    .optional()
    .describe(
      "Fuzzy search term across skill fields (e.g. 'react best practices', 'automation workflow')"
    ),
  limit: z
    .number()
    .optional()
    .describe("Maximum number of results to return. Example: 10"),
  offset: z
    .number()
    .optional()
    .describe("Number of results to skip for pagination. Example: 0"),
});

/**
 * SkillDiscoveryQueryInputSchema: Input validation schema for discovering community skills
 */
export const SkillDiscoveryQueryInputSchema = Schema.object({
  query: z
    .string()
    .optional()
    .describe(
      "Vector-based discovery search term. Examples: 'react', 'nextjs auth', 'database schema generator'. Has broader search capability than list query."
    ),
  limit: z
    .string()
    .optional()
    .describe("Maximum number of results to return. Example: 10"),
  offset: z
    .string()
    .optional()
    .describe("Number of results to skip for pagination. Example: 0"),
});

/**
 * SkillRemoteSourceInputSchema: Input validation for remote community skill sources.
 * @description Supports either a package/repository source plus optional skill name,
 * or the legacy compact format `owner/repo@skill-name`.
 */
export const SkillRemoteSourceInputSchema = Schema.object({
  source: z
    .string()
    .describe(
      "Package source used by `npx skills add`. Examples: 'vercel-labs/agent-browser', 'https://github.com/vercel-labs/agent-browser', or legacy 'owner/repo@skill-name'"
    ),
  skill: z
    .string()
    .optional()
    .describe(
      "Specific skill to install or preview from the package source. Example: 'agent-browser'"
    ),
});

/**
 * Type inference for Skill.
 * @description Automatically inferred type from SkillSchema.
 */
export type Skill = z.infer<typeof SkillSchema>;

/**
 * Type inference for CreateSkillInput.
 * @description Automatically inferred type from CreateSkillSchema for creation operations.
 */
export type CreateSkillInput = z.infer<typeof CreateSkillSchema>;

/**
 * Type inference for UpdateSkillInput.
 * @description Automatically inferred type from UpdateSkillSchema for update operations.
 */
export type UpdateSkillInput = z.infer<typeof UpdateSkillSchema>;

/**
 * Type inference for SkillQueryInput.
 * @description Parameter structure for list queries.
 */
export type SkillQueryInput = z.infer<typeof SkillQueryInputSchema>;

/**
 * Type inference for SkillDiscoveryQueryInput.
 * @description Parameter structure for discovery queries.
 */
export type SkillDiscoveryQueryInput = z.infer<typeof SkillDiscoveryQueryInputSchema>;

/**
 * Type inference for SkillRemoteSourceInput.
 * @description Parameter structure for install/preview of remote community skills.
 */
export type SkillRemoteSourceInput = z.infer<typeof SkillRemoteSourceInputSchema>;

/**
 * Type inference for Skill Metadata.
 * @description Automatically inferred type for metadata structure.
 */
export type SkillMetadata = z.infer<typeof SkillMetadataSchema>;

/**
 * Type inference for Skill Resource.
 * @description Automatically inferred type for external resources.
 */
export type SkillResource = z.infer<typeof SkillResourceSchema>;

/**
 * Type inference for Skill Rule.
 * @description Automatically inferred type for behavioral rules.
 */
export type SkillRule = z.infer<typeof RuleSchema>;

/**
 * SkillDiscoveryResult: Represents a single community skill found in the open registry.
 * @description Returned by the discovery operation when searching the open Agent Skills registry.
 */
export type SkillServiceDiscoveryResult = ResponseWithCTA<{
  results: {
    /** Package source to install this skill (e.g. "owner/repo" or "https://github.com/owner/repo"). */
    source: string;
    /** Full compact identifier as returned by community discovery (e.g. "owner/repo@skill-name"). */
    fullSource: string;
    /** Repository owner / organization identifier. */
    owner: string;
    /** Repository name. */
    repo: string;
    /** Skill name within the repository. */
    skill: string;
    /** Human-readable install count. */
    installs: string;
    /** Direct URL to the skill page on skills.sh. */
    url: string;
  }[],
  stats: {
    count: number,
  }
}>

/**
 * SkillInstallResult: Returned after a successful remote skill installation.
 * @description Contains the persisted skill record and contextual instructions for the agent.
 */
export type SkillServiceInstallResult = ResponseWithCTA<{
  /** The full skill record as persisted in local collections after installation. */
  skill: Skill;
}>

export type SkillServiceGetResult = ResponseWithCTA<{
  skill: Skill
}>

export type SkillServiceCreateResult = ResponseWithCTA<{
  skill: Skill
}>

export type SkillServiceUpdateResult = ResponseWithCTA<{
  skill: Skill
}>

export type SkillServiceListResult = ResponseWithCTA<{
  skills: Skill[]
}>

export type SkillServiceDeleteResult = ResponseWithCTA<{
  skill: string
}>

export type SkillServicePreviewResult = ResponseWithCTA<{
  /** The package source used to fetch this preview. */
  source: string;
  /** The compact source representation when available. */
  fullSource?: string;
  /** The skill name derived from the source identifier. */
  name: string;
  /** The raw markdown content of the SKILL.md file from the community repository. */
  content: string;
}>

/**
 * @interface ISkillService
 * @description Defines the contract for the SkillService managing skills within the Fractal ecosystem.
 * All methods return `ResponseWithCTA`-wrapped data to provide rich CLI call-to-action hints.
 */
export interface ISkillService {
  /**
   * Lists all skills from the collection, excluding heavy content and resources.
   * @param params Optional query parameters for filtering and pagination.
   */
  list(params: SkillQueryInput): Promise<SkillServiceListResult>;

  /**
   * Retrieves a single skill by its unique identifier.
   * @param id The skill identifier.
   */
  getById(id: string): Promise<SkillServiceGetResult>;

  /**
   * Searches the open Agent Skills community registry.
   * @param query Search term for the discovery operation.
   */
  discovery(query: string): Promise<SkillServiceDiscoveryResult>;

  /**
   * Installs a skill from the community registry into the local `.fractal` directory.
   * @param source Remote package source and optional skill name.
   */
  install(source: SkillRemoteSourceInput): Promise<SkillServiceInstallResult>;

  /**
   * Creates a new skill record in the collection.
   * @param data Skill creation payload.
   */
  create(data: CreateSkillInput): Promise<SkillServiceCreateResult>;

  /**
   * Updates an existing skill record.
   * @param id The skill identifier.
   * @param data The partial update payload.
   */
  update(id: string, data: UpdateSkillInput): Promise<SkillServiceUpdateResult>;

  /**
   * Removes a skill from the collection.
   * @param id The skill identifier.
   */
  delete(id: string): Promise<SkillServiceDeleteResult>;

  /**
   * Fetches the SKILL.md preview for a community registry skill directly from GitHub.
   * @param source Remote package source and optional skill name.
   */
  preview(source: SkillRemoteSourceInput): Promise<SkillServicePreviewResult>;
}
