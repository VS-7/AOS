import { z } from "zod";
import type { ResponseWithCTA } from "@/core/interfaces/response.interfaces";

/**
 * Recovered from `_extracted/index/src/features/agent/agent.interfaces.ts`
 * (Task 9, replacing the earlier task's hand-written placeholder per
 * controller ruling R16 — the Fractal presentation code this task copies
 * in imports `FractalAgent`, not `Agent`).
 *
 * The source file's `Agent`/`*Agent*` types are renamed to `FractalAgent`/
 * `*FractalAgent*` throughout — a mechanical, disclosed rename: the
 * `v401/web` presentation code (`chat-thread.helper.ts`, `agents.context.
 * tsx`, etc.) uniformly imports `FractalAgent`, so this file's source
 * (the backend-side snapshot, where the type is just `Agent`) didn't match
 * the frontend's own now-erased copy by name.
 *
 * Everything backend-runtime-only from the source file is dropped: sandbox
 * file/command execution (`IAgentSandboxService` and its input/output
 * types), `AgentToolContext`, `SpawnOptions`/`NodeJS.ProcessEnv`,
 * `BackgroundJobs`, `IgniterLogger`, `AgentBrowserService`, `ToolLoopAgent`.
 * No presentation file imports any of them (confirmed by grep across
 * `v401/web/src` for named imports from this file) — they belong to the
 * agent execution runtime, not the frontend's agent CRUD/list UI.
 */
export const FractalAgentChannelSchema = z.object({
  provider: z
    .string()
    .describe("Channel provider slug. Example: 'telegram'"),
  data: z
    .record(z.string(), z.any())
    .describe("Provider-specific configuration payload for the channel."),
});

/**
 * AgentSchema: Primary schema for a Fractal Agent record.
 * @description Defines the complete structure of a Fractal Agent, including its identity, AI configuration, and system instructions.
 * @example
 * ```typescript
 * const agent = {
 *   id: "neo",
 *   name: "Neo",
 *   description: "Agent Feature Specialist",
 *   provider: "openai",
 *   model: "gpt-4o",
 *   content: "# System Instructions\nYou are Neo..."
 * };
 * ```
 */
export const FractalAgentSchema = z.object({
  id: z
    .string()
    .describe(
      "Unique agent identifier (slug) used for file naming and references. Example: 'neo'",
    ),

  name: z.string().describe("Human-readable name of the agent. Example: 'Neo'"),

  image: z
    .string()
    .optional()
    .describe("Optional avatar URL or data URI."),

  description: z
    .string()
    .optional()
    .describe(
      "Instructions for the orchestrator to know when this agent should be called. Describe the agent's expertise and the specific tasks it can handle.",
    ),

  skill: z
    .string()
    .optional()
    .describe("Optional skill association ID. Example: 'memory-management'"),

  role: z
    .string()
    .optional()
    .describe(
      "The specific role/function of the agent. Example: 'Quality Assurance Specialist'",
    ),

  leader: z
    .string()
    .optional()
    .describe(
      "The ID of the leader agent this agent reports to, allowing for hierarchical structures.",
    ),

  content: z
    .string()
    .optional()
    .describe(
      "Markdown content containing the detailed system instructions. Example: '# Instructions\\nHandle agent CRUD...'",
    ),

  provider: z.string().optional().describe("LLM provider for the agent."),

  model: z
    .string()
    .optional()
    .describe(
      "LLM model for the agent. Supports '{model} ({provider})' format to avoid VS Code validation issues. Example: 'Gemini 3 Flash Preview (gemini)'",
    ),

  voice: z.string().optional().describe("Voice name for speech output."),

  channels: z
    .array(FractalAgentChannelSchema)
    .optional()
    .describe("Configured communication channels for the agent."),

  orchestrator: z
    .boolean()
    .default(false)
    .describe(
      "Marks the agent as the workspace orchestrator fallback for non-direct chats without explicit mentions.",
    ),
});

/**
 * CreateFractalAgentSchema: Schema for creating a new agent.
 * @description Excludes generated or internal fields if any (currently includes ID as it is used for file naming).
 */
export const CreateFractalAgentSchema = FractalAgentSchema.omit({ id: true });

/**
 * UpdateFractalAgentSchema: Schema for updating an existing agent.
 * @description Allows partial updates to an agent record, excluding the immutable ID.
 */
export const UpdateFractalAgentSchema = FractalAgentSchema.omit({ id: true })
  .extend({ agent: z.string() })
  .partial();

/**
 * GetFractalAgentByIdSchema: Schema for retrieving an agent by ID.
 */
export const GetFractalAgentByIdSchema = z.object({
  id: z
    .string()
    .optional()
    .describe(
      "The unique identifier (slug) of the agent to retrieve. Examples: 'neo', 'atlas', 'aurora'",
    ),
});

/**
 * DeleteFractalAgentSchema: Schema for deleting an agent by ID.
 */
export const DeleteFractalAgentSchema = z.object({
  agent: z
    .string()
    .describe(
      "The unique identifier (slug) of the agent to delete. Examples: 'neo', 'atlas', 'aurora'",
    ),
});

/**
 * FractalAgent: TypeScript type inferred from FractalAgentSchema.
 * @description Represents the complete structure of a Fractal Agent record as defined by the schema. This type is used throughout the application to ensure type safety when working with agent data.
 */
export type FractalAgent = z.infer<typeof FractalAgentSchema>;

/** CreateFractalAgentInput: Input type for agent creation. */
export type CreateFractalAgentInput = z.infer<typeof CreateFractalAgentSchema>;

/** UpdateFractalAgentInput: Input type for agent updates. */
export type UpdateFractalAgentInput = z.infer<typeof UpdateFractalAgentSchema>;

/** GetFractalAgentByIdInput: Input type for fetching an agent by its identifier. */
export type GetFractalAgentByIdInput = z.infer<typeof GetFractalAgentByIdSchema>;

/** DeleteFractalAgentInput: Input type for agent deletion. */
export type DeleteFractalAgentInput = z.infer<typeof DeleteFractalAgentSchema>;

/**
 * @interface IFractalAgentService
 * @description Defines the contract for the AgentService managing Fractal Agents.
 * All methods return `ResponseWithCTA`-wrapped data to provide rich CLI call-to-action hints.
 */
export interface IFractalAgentService {
  list(): Promise<ResponseWithCTA<{ agents: FractalAgent[] }>>;
  getById(
    params: GetFractalAgentByIdInput,
  ): Promise<ResponseWithCTA<{ agent: FractalAgent | null }>>;
  create(params: CreateFractalAgentInput): Promise<ResponseWithCTA<FractalAgent>>;
  update(params: UpdateFractalAgentInput): Promise<ResponseWithCTA<FractalAgent>>;
  delete(params: DeleteFractalAgentInput): Promise<ResponseWithCTA>;
  /**
   * Resolves the current agent's full identity record.
   * When called from within an agent execution context, returns that agent's record.
   * When called from a terminal (human user), returns the orchestrator agent's record.
   */
  me(): Promise<ResponseWithCTA<{ agent: FractalAgent }>>;
}
