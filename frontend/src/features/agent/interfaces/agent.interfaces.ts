import { z } from "zod";
import type { ResponseWithCTA } from "@/core/interfaces/response.interfaces";

/**
 * Recovered from `_extracted/index/src/features/agent/agent.interfaces.ts`
 * (Task 9, replacing the earlier task's hand-written placeholder per
 * controller ruling R16 — the AOS presentation code this task copies
 * in imports `Agent`, not `Agent`).
 *
 * The source file's `Agent`/`*Agent*` types are renamed to `Agent`/
 * `*Agent*` throughout — a mechanical, disclosed rename: the
 * `v401/web` presentation code (`chat-thread.helper.ts`, `agents.context.
 * tsx`, etc.) uniformly imports `Agent`, so this file's source
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
export const AgentChannelSchema = z.object({
  provider: z
    .string()
    .describe("Channel provider slug. Example: 'telegram'"),
  data: z
    .record(z.string(), z.any())
    .describe("Provider-specific configuration payload for the channel."),
});

/**
 * AgentSchema: Primary schema for a AOS Agent record.
 * @description Defines the complete structure of a AOS Agent, including its identity, AI configuration, and system instructions.
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
export const AgentSchema = z.object({
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
    .array(AgentChannelSchema)
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
 * CreateAgentSchema: Schema for creating a new agent.
 * @description Excludes generated or internal fields if any (currently includes ID as it is used for file naming).
 */
export const CreateAgentSchema = AgentSchema.omit({ id: true });

/**
 * UpdateAgentSchema: Schema for updating an existing agent.
 * @description Allows partial updates to an agent record, excluding the immutable ID.
 */
export const UpdateAgentSchema = AgentSchema.omit({ id: true })
  .extend({ agent: z.string() })
  .partial();

/**
 * GetAgentByIdSchema: Schema for retrieving an agent by ID.
 */
export const GetAgentByIdSchema = z.object({
  id: z
    .string()
    .optional()
    .describe(
      "The unique identifier (slug) of the agent to retrieve. Examples: 'neo', 'atlas', 'aurora'",
    ),
});

/**
 * DeleteAgentSchema: Schema for deleting an agent by ID.
 */
export const DeleteAgentSchema = z.object({
  agent: z
    .string()
    .describe(
      "The unique identifier (slug) of the agent to delete. Examples: 'neo', 'atlas', 'aurora'",
    ),
});

/**
 * Agent: TypeScript type inferred from AgentSchema.
 * @description Represents the complete structure of a AOS Agent record as defined by the schema. This type is used throughout the application to ensure type safety when working with agent data.
 */
export type Agent = z.infer<typeof AgentSchema>;

/** CreateAgentInput: Input type for agent creation. */
export type CreateAgentInput = z.infer<typeof CreateAgentSchema>;

/** UpdateAgentInput: Input type for agent updates. */
export type UpdateAgentInput = z.infer<typeof UpdateAgentSchema>;

/** GetAgentByIdInput: Input type for fetching an agent by its identifier. */
export type GetAgentByIdInput = z.infer<typeof GetAgentByIdSchema>;

/** DeleteAgentInput: Input type for agent deletion. */
export type DeleteAgentInput = z.infer<typeof DeleteAgentSchema>;

/**
 * @interface IAgentService
 * @description Defines the contract for the AgentService managing AOS Agents.
 * All methods return `ResponseWithCTA`-wrapped data to provide rich CLI call-to-action hints.
 */
export interface IAgentService {
  list(): Promise<ResponseWithCTA<{ agents: Agent[] }>>;
  getById(
    params: GetAgentByIdInput,
  ): Promise<ResponseWithCTA<{ agent: Agent | null }>>;
  create(params: CreateAgentInput): Promise<ResponseWithCTA<Agent>>;
  update(params: UpdateAgentInput): Promise<ResponseWithCTA<Agent>>;
  delete(params: DeleteAgentInput): Promise<ResponseWithCTA>;
  /**
   * Resolves the current agent's full identity record.
   * When called from within an agent execution context, returns that agent's record.
   * When called from a terminal (human user), returns the orchestrator agent's record.
   */
  me(): Promise<ResponseWithCTA<{ agent: Agent }>>;
}
