import { z } from "zod";
import { RuleSchema } from "@/core/interfaces/rule.interfaces";
import { ResponseWithCTA } from "@/core/interfaces/response.interfaces";

/**
 * Defines the possible connection configurations for a toolset.
 * Allows integration with MCP servers, REST APIs, CLIs, or custom implementations.
 */
export const ToolsetConnectionSchema = z.discriminatedUnion("type", [
  z.object({
    type: z.literal("mcp-server::stdio"),
    command: z.string(),
    args: z.array(z.string()),
    env: z.record(z.string(), z.string()).optional(),
  }),
  z.object({
    type: z.literal("mcp-server::http"),
    url: z.string(),
    headers: z.record(z.string(), z.string()).optional(),
  }),
  z.object({
    type: z.literal("rest-api"),
    url: z.string(),
    headers: z.record(z.string(), z.string()).optional(),
  }),
  z.object({
    type: z.literal("cli"),
    command: z.string(),
    env: z.record(z.string(), z.string()).optional(),
  }),
  z.object({
    type: z.literal("custom"),
    env: z.record(z.string(), z.string()).optional(),
  }),
]);

/**
 * Defines a single tool's structure, including its identifier, description, and rules.
 * The schema determines the required input format, and the handler processes the execution.
 */
export const ToolSchema = z.object({
  name: z.string().describe("Custom tool identifier. Example: 'get-schema'"),
  skill: z.string().describe("Skill ID this custom tool belongs to. Example: 'database-admin'"),
  toolset: z.string().describe("Toolset ID this custom tool belongs to. Example: 'db-tools::postgres'"),
  description: z.string().optional().describe("Custom tool description. Example: 'Fetches the schema of a database table'"),
  rules: z.array(RuleSchema).optional().describe("Custom tool specific rules. Example: [{ instruction: 'Never fetch passwords' }]"),
  schema: z.any().describe("Custom tool Zod schema or JSON schema. Example: z.object({ table: z.string() })"), // JSON schema or ZodType
  handler: z.any().optional().describe("Custom tool handler function. Example: async ({ input }) => { ... }"),
});

/**
 * Defines a logical grouping of custom tools explicitly associated with a specific skill.
 * Represents an external integration, containing authentication rules and connection info.
 */
export const ToolsetSchema = z.object({
  id: z.string().describe("Custom skill toolset unique identifier. Example: 'db-tools::postgres'"),
  skill: z.string().describe("Skill ID this custom toolset belongs to. Example: 'database-admin'"),
  name: z.string().describe("Custom skill toolset display name. Example: 'Postgres Utilities'"),
  description: z.string().describe("Purpose and functionality of the custom skill toolset. Example: 'Tools to interact with a Postgres DB'"),
  rules: z.array(RuleSchema).describe("Array of enforcement rules. Example: [{ instruction: 'Use read-only queries' }]"),
  connection: ToolsetConnectionSchema.describe("Connection configuration. Example: { type: 'custom', env: { URL: 'DB_URL' } }"),
  tools: z.array(ToolSchema).describe("Custom tools associated with this toolset. Example: [{ name: 'query' }]"),
});

/**
 * Validates the input for querying and filtering toolsets in the system.
 */
export const ToolsetQueryInputSchema = z.object({
  query: z.string().optional().describe("Fuzzy search term across custom toolset fields. Example: 'postgres'"),
  skill: z.string().optional().describe("Filter custom toolsets by skill ID. Example: 'database-admin'"),
});

/**
 * Validates the input for retrieving a specific custom tool within a skill toolset.
 */
export const ToolsetGetInputSchema = z.object({
  toolset: z.string().describe("Custom skill toolset ID. Example: 'db-tools::postgres'"),
  tool: z.string().optional().describe("Custom tool name. Example: 'query'"),
});

/**
 * Validates the payload required to execute a specific custom tool within a skill toolset.
 */
export const ToolsetCallInputSchema = z.object({
  toolset: z.string().describe("Custom skill toolset ID. Example: 'db-tools::postgres'"),
  tool: z.string().describe("Custom tool name. Example: 'query'"),
  data: z.any().describe("Input data validated against custom tool schema. Example: { table: 'users' }"),
});

/**
 * Standardized execution wrapper to ensure agents receive structural feedback even on errors,
 * allowing LLMs to auto-correct inputs or understand API failures without hard-crashing the process.
 */
export const ToolExecutionResultSchema = z.object({
  status: z.enum(["success", "error"]).describe("Execution status flag."),
  data: z.any().nullable().describe("Execution result payload if successful."),
  error: z.object({
    message: z.string(),
    code: z.union([z.number(), z.string()]).optional(),
    issue: z.any().optional()
  }).nullable().describe("Detailed structural error if execution failed.")
});

export type ToolsetConnection = z.infer<typeof ToolsetConnectionSchema>;
export type Tool = z.infer<typeof ToolSchema>;
export type Toolset = z.infer<typeof ToolsetSchema>;
export type ToolsetQueryInput = z.infer<typeof ToolsetQueryInputSchema>;
export type ToolsetGetInput = z.infer<typeof ToolsetGetInputSchema>;
export type ToolsetCallInput = z.infer<typeof ToolsetCallInputSchema>;
export type ToolExecutionResult = z.infer<typeof ToolExecutionResultSchema>;

export interface IToolsetAdapter {
  type: string;
  discover(toolset: Partial<Toolset>): Promise<Tool[]>;
  call(tool: Tool, data: any, aosContext: any): Promise<ToolExecutionResult>;
}

/**
 * Response type for toolset listing.
 */
export type ToolsetServiceListResponse = ResponseWithCTA<{
  toolsets: Toolset[]
  stats: {
    total: number;
    tools: number;
  }
}>;

/**
 * Response type for toolset retrieval.
 */
export type ToolsetServiceGetResponse = ResponseWithCTA<{
  toolset?: Toolset,
  tool?: Tool
}>;

/**
 * Response type for tool execution.
 */
export type ToolsetServiceCallResponse = ResponseWithCTA<ToolExecutionResult>;

/**
 * Service interface for discovering, listing, and executing toolsets globally.
 */
export interface IToolsetService {
  list(params: ToolsetQueryInput): Promise<ToolsetServiceListResponse>;
  get(params: ToolsetGetInput): Promise<ToolsetServiceGetResponse>;
  getTools(): Promise<Tool[]>;
  call(params: ToolsetCallInput): Promise<ToolsetServiceCallResponse>;
}

/**
 * One environment-variable-shaped config requirement for a toolset
 * connection (e.g. an API key a marketplace plugin needs). Not in this
 * file's source — reconstructed from
 * `features/marketplace/presentation/components/inventory/plugin-
 * inventory-item-sheet.component.tsx`'s config tab, this type's sole
 * consumer (`req.lookupKey`, `req.isSet`).
 */
export interface ToolsetConfigRequirement {
  lookupKey: string;
  isSet: boolean;
  [key: string]: unknown;
}
