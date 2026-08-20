import { z } from "zod";
import type { ResponseWithCTA } from "@/core/interfaces/response.interfaces";
import { Schema } from "@/core/helpers/schema.helper";

/**
 * MemoryCategorySchema: The semantic classification of a memory.
 * @description Defines the type of information stored in the memory.
 * @enum ["preference", "architecture", "workflow", "context", "lesson", "constraint", "tooling", "security", "reference"]
 */
export const MemoryCategorySchema = z.enum([
  "preference",
  "architecture",
  "workflow",
  "context",
  "lesson",
  "constraint",
  "tooling",
  "security",
  "reference",
]);

/**
 * MemoryStatusSchema: The lifecycle status of a memory.
 * @description Defines if a memory is active, superseded, archived or expired.
 */
export const MemoryStatusSchema = z.enum([
  "active", // Valid and current knowledge
  "deprecated", // Replaced by a newer memory
  "archived", // Historically relevant but outside the active context loop
  "ttl_expired", // Automatically expired based on time
]);

/**
 * MemorySchema: Main schema defining a AOS memory.
 * @description Represents a piece of persistent information created by an agent.
 * @example
 * ```typescript
 * const memory = {
 *   id: "550e8400-e29b-41d4-a716-446655440000",
 *   title: "System Design Choice",
 *   description: "Decision to use UUIDs for memory identification for better scalability.",
 *   category: "architecture",
 *   tags: ["design", "decisions"],
 *   content: "# Architecture\nWe chose UUIDs because of scalability.",
 *   agent: "atlas",
 *   status: "active",
 *   confidence: 1
 * };
 * ```
 */
export const MemorySchema = z.object({
  id: z.string().describe("Unique UUID identifier for the memory."),
  title: z
    .string()
    .describe(
      "A brief summary of the memory content. Example: IDE configuration preference",
    ),
  description: z
    .string()
    .describe(
      "Short, vector-optimized summary of the memory content for better retrieval.",
    ),
  category: MemoryCategorySchema.describe(
    "The semantic category of the memory. Example: 'preference'",
  ),
  tags: z
    .array(z.string())
    .describe("Searchable tags for the memory. Example: ['node', 'setup']"),
  content: z
    .string()
    .optional()
    .describe(
      "The full markdown content of the memory. Example: # My Memory\nContent here...",
    ),
  agent: z
    .string()
    .describe("ID of the agent that created this memory. Example: atlas"),

  // [Knowledge Graph & Versioning]
  confidence: z
    .number()
    .min(0)
    .max(1)
    .default(1)
    .describe("Confidence level of the information (0-1)."),
  links: z
    .array(z.string())
    .default([])
    .describe("Related memory UUIDs creating a knowledge graph."),

  // [Supersede Flow]
  supersedes: z
    .array(
      z.object({
        id: z.string(),
        reason: z.string().describe("Why this memory replaces the old one."),
      }),
    )
    .default([])
    .describe("List of memory IDs that this new memory replaces."),

  // [Depreciation Metadata]
  status: MemoryStatusSchema.default("active").describe(
    "The current lifecycle status of the memory.",
  ),
  deprecatedBy: z
    .string()
    .optional()
    .describe("UUID of the memory that replaced this one."),
  deprecatedAt: z
    .string()
    .optional()
    .describe("ISO timestamp of when this memory was deprecated."),
  deprecatedReason: z
    .string()
    .optional()
    .describe("Reason why this memory was marked as deprecated or forgotten."),

  // [Lifecycle & Scopes]
  scopes: z
    .array(z.string())
    .optional()
    .describe(
      "Glob patterns defining where this memory applies. Examples: ['src/features/**/*.ts', 'package.json', '*.md']",
    ),
  expiresAt: z
    .string()
    .optional()
    .describe("Optional ISO timestamp for automatic TTL expiration."),
  createdAt: z
    .string()
    .describe("ISO timestamp of creation. Example: 2024-03-20T10:00:00Z"),
  updatedAt: z
    .string()
    .describe("ISO timestamp of last update. Example: 2024-03-20T10:00:00Z"),
  metadata: z
    .any()
    .optional()
    .describe("Additional structured metadata. Example: { priority: 'high' }"),
});

/**
 * CreateMemorySchema: Schema for creating a new memory.
 * @description Used in API POST requests and CLI create commands.
 */
export const CreateMemorySchema = MemorySchema.omit({
  id: true,
  status: true,
  deprecatedBy: true,
  deprecatedAt: true,
  deprecatedReason: true,
  createdAt: true,
  updatedAt: true,
});

/**
 * ForgetMemorySchema: Schema for deprecating/forgetting a memory.
 * @description Used when an agent wants to invalidate a specific piece of knowledge.
 */
export const ForgetMemorySchema = z.object({
  id: z.string().describe("The UUID of the memory to forget."),
  reason: z
    .string()
    .min(5)
    .describe("Semantic reason why this memory is being forgotten."),
});

/**
 * MemoryOrderBySchema: Sort options for memory listings.
 * @description Allows callers to sort by createdAt, updatedAt, or confidence.
 */
export const MemoryOrderBySchema = z
  .object({
    createdAt: z.enum(["asc", "desc"]),
    updatedAt: z.enum(["asc", "desc"]),
    confidence: z.enum(["asc", "desc"]),
  })
  .partial()
  .optional()
  .describe("Sort memories by timestamp or confidence.");

/**
 * MemoryQuerySchema: Schema for listing memories with filters.
 * @description Used to filter and paginate memory collections.
 */
export const MemoryQuerySchema = Schema.object({
  status: MemoryStatusSchema.optional()
    .default("active")
    .describe("Filter by lifecycle status."),
  category: MemoryCategorySchema.optional().describe(
    "Filter by semantic category. Example: lesson",
  ),
  scopes: z
    .array(z.string())
    .optional()
    .describe(
      "Filter memories by applicable glob scopes. Examples: ['src/**/*.ts', 'README.md']",
    ),
  scopesMode: z
    .enum(["strict", "lax"])
    .optional()
    .describe(
      "Scope filter behavior. 'strict' excludes memories without scopes.",
    ),
  orderBy: MemoryOrderBySchema.describe(
    "Sort memories by createdAt, updatedAt, or confidence.",
  ),
  query: z
    .string()
    .optional()
    .describe("Search query for title, description, tags and content."),
  limit: z.coerce
    .number()
    .int()
    .positive()
    .optional()
    .describe("Maximum number of items to return. Example: 10"),
  offset: z.coerce
    .number()
    .int()
    .nonnegative()
    .optional()
    .describe("Number of items to skip. Example: 0"),
});

/**
 * MemoryGraphQuerySchema: Query options for graph generation.
 * Uses Schema.object() for input coercion (array/object JSON parsing from query strings)
 * and z.coerce.boolean() for query-param-safe boolean handling.
 */
export const MemoryGraphQuerySchema = Schema.object({
  agent: z
    .string()
    .optional()
    .describe("Optional agent id to scope graph data."),
  category: MemoryCategorySchema.optional().describe(
    "Optional category filter for the graph payload.",
  ),
  scopes: z
    .array(z.string())
    .optional()
    .describe("Optional glob scopes used to narrow the graph payload."),
  scopesMode: z
    .enum(["strict", "lax"])
    .optional()
    .default("lax")
    .describe(
      "Scope filter behavior. 'strict' excludes memories without scopes.",
    ),
  minConfidence: z.coerce
    .number()
    .min(0)
    .max(1)
    .optional()
    .describe("Minimum confidence threshold for included memories."),
  summary: z.coerce
    .boolean()
    .optional()
    .default(true)
    .describe("Include the computed cognitive summary in the response."),
  top: z.coerce
    .number()
    .int()
    .positive()
    .optional()
    .describe("How many top connected memories to include in the summary."),
  isolated: z.coerce
    .boolean()
    .optional()
    .default(false)
    .describe("Return only isolated memories in the graph payload."),
  byCategory: z.coerce
    .boolean()
    .optional()
    .default(false)
    .describe("Request category distribution emphasis in the summary."),
  byAgent: z.coerce
    .boolean()
    .optional()
    .default(false)
    .describe("Request agent distribution emphasis in the summary."),
  format: z
    .enum(["json", "table"])
    .optional()
    .default("json")
    .describe("Preferred output format for the CLI adapter."),
});

/**
 * MemoryGraphNodeSchema: Node model ready for force-graph rendering.
 */
export const MemoryGraphNodeSchema = z.object({
  id: z.string(),
  label: z.string(),
  category: MemoryCategorySchema,
  status: MemoryStatusSchema,
  group: z.string(),
  val: z.number().optional(),
});

/**
 * MemoryGraphLinkSchema: Edge model ready for force-graph rendering.
 */
export const MemoryGraphLinkSchema = z.object({
  source: z.string(),
  target: z.string(),
  type: z.enum(["reference", "supersedes"]),
  weight: z.number().optional(),
});

/**
 * MemoryGraphSchema: Complete graph payload ready for frontend rendering.
 */
/**
 * MemoryGraphLinkTypeSchema: Link types available in the graph.
 */
export const MemoryGraphLinkTypeSchema = z.enum(["reference", "supersedes"]);

/**
 * MemoryGraphCategorySummarySchema: Category breakdown for summary analysis.
 */
export const MemoryGraphCategorySummarySchema = z.object({
  count: z.number().describe("Total nodes in this category"),
  linked: z.number().describe("Nodes with at least one edge"),
  isolated: z.number().describe("Nodes with zero edges"),
});

/**
 * MemoryGraphTopConnectedSchema: Top connected node descriptor.
 */
export const MemoryGraphTopConnectedSchema = z.object({
  id: z.string(),
  title: z.string(),
  connections: z.number().describe("Total degree (in + out)"),
  category: MemoryCategorySchema,
});

/**
 * MemoryGraphSummarySchema: Computed analysis layer for cognitive graph.
 */
export const MemoryGraphSummarySchema = z.object({
  totalNodes: z.number(),
  totalLinks: z.number(),
  linkTypeDistribution: z.object({
    reference: z.number(),
    supersedes: z.number(),
  }),
  categories: z.record(z.string(), MemoryGraphCategorySummarySchema),
  topConnected: z
    .array(MemoryGraphTopConnectedSchema)
    .describe("Top N hubs, sorted by degree desc"),
  isolatedCount: z.number(),
  supersedeChains: z
    .number()
    .describe("Number of supersede edge chains detected"),
  memoryHealth: z
    .enum(["healthy", "siloed", "fragmented"])
    .describe("Heuristic health assessment"),
});

/**
 * MemoryGraphSchema: Complete graph payload ready for frontend rendering.
 */
export const MemoryGraphSchema = z.object({
  nodes: z.array(MemoryGraphNodeSchema),
  links: z.array(MemoryGraphLinkSchema),
  summary: MemoryGraphSummarySchema.optional().describe(
    "Computed cognitive analysis (present when summary=true or by default)",
  ),
});

/**
 * Memory: Type inference for a memory record.
 */
export type Memory = z.infer<typeof MemorySchema>;

/**
 * CreateMemoryInput: Type inference for memory creation input.
 */
export type CreateMemoryInput = z.infer<typeof CreateMemorySchema>;

/**
 * ForgetMemoryInput: Type inference for forgetting a memory.
 */

export type ForgetMemoryInput = z.infer<typeof ForgetMemorySchema>;

/**
 * MemoryQueryInput: Type inference for memory query parameters.
 */
export type MemoryQueryInput = z.infer<typeof MemoryQuerySchema>;

/**
 * MemoryOrderByInput: Type inference for memory list ordering.
 */
export type MemoryOrderByInput = z.infer<typeof MemoryOrderBySchema>;

/**
 * MemoryListResult: Paginated response wrapper for memory listings.
 * @description Returned by the list endpoint, containing the filtered results
 * and the total count for client-side pagination.
 */
export type MemoryListResult = {
  data: Memory[];
  totalCount: number;
};

export type MemoryGraphQueryInput = z.infer<typeof MemoryGraphQuerySchema>;

/**
 * MemoryGraphNode: Type inference for graph nodes.
 */
export type MemoryGraphNode = z.infer<typeof MemoryGraphNodeSchema>;

/**
 * MemoryGraphLink: Type inference for graph links.
 */
export type MemoryGraphLink = z.infer<typeof MemoryGraphLinkSchema>;

/**
 * MemoryGraphLinkType: Type inference for graph link types.
 */
export type MemoryGraphLinkType = z.infer<typeof MemoryGraphLinkTypeSchema>;

/**
 * MemoryGraphCategorySummary: Type inference for category graph statistics.
 */
export type MemoryGraphCategorySummary = z.infer<
  typeof MemoryGraphCategorySummarySchema
>;

/**
 * MemoryGraphTopConnected: Type inference for top connected nodes.
 */
export type MemoryGraphTopConnected = z.infer<
  typeof MemoryGraphTopConnectedSchema
>;

/**
 * MemoryGraphSummary: Type inference for computed graph summary.
 */
export type MemoryGraphSummary = z.infer<typeof MemoryGraphSummarySchema>;

/**
 * MemoryGraph: Type inference for graph payload.
 */
export type MemoryGraph = z.infer<typeof MemoryGraphSchema>;

/**
 * @interface IMemoryService
 * @description Defines the contract for the MemoryService managing agent memories in the AOS ecosystem.
 * All methods return `ResponseWithCTA`-wrapped data to provide rich CLI call-to-action hints.
 */
export interface IMemoryService {
  /**
   * Lists memories from the collection with optional filtering by category, agent, scopes, ordering, and full-text search.
   * @param params Query and pagination parameters.
   */
  list(params: MemoryQueryInput): Promise<ResponseWithCTA<MemoryListResult>>;

  /**
   * Builds a graph payload (nodes + links) from current memory records.
   */
  graph(params?: MemoryGraphQueryInput): Promise<ResponseWithCTA<MemoryGraph>>;

  /**
   * Retrieves a single memory record by its unique UUID.
   * @param id The memory UUID.
   */
  getById(id: string): Promise<ResponseWithCTA<Memory>>;

  /**
   * Creates a new memory record with automatic UUID generation and supersede logic.
   * @param data The memory creation payload.
   */
  create(data: CreateMemoryInput): Promise<ResponseWithCTA<Memory>>;

  /**
   * Deprecates a memory record logically instead of physically deleting it.
   * @param params The forget parameters including reason.
   */
  forgot(params: ForgetMemoryInput): Promise<ResponseWithCTA<Memory>>;
}

