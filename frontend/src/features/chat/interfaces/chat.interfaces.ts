import { z } from "zod";
import type { ResponseWithCTA } from "@/core/interfaces/response.interfaces";
import type { UIMessage } from "ai";

/**
 * Recovered from `_extracted/index/src/features/chat/chat.interfaces.ts`
 * (Task 9, replacing the earlier task's hand-written placeholder per
 * controller ruling R16). Kept close to the source: this file already used
 * `Chat`/`ChatMessage` — the exact names the freshly-copied
 * `v401/web` presentation code (`use-chat.ts`, `chat-thread.helper.ts`,
 * `chat-message-item.component.tsx`, …) imports — so no renaming was
 * needed here, unlike `agent.interfaces.ts`'s `Agent`→`Agent`.
 *
 * The message-metadata section below (`execution`/`runs`/dispatch) is
 * grounded in `_extracted/v401/server/src/features/chat/schemas/chat.
 * schema.ts` instead of `index/`'s version: `index/`'s agent-metadata
 * schema has no `execution` field, but the copied presentation code
 * (`use-chat.ts`'s `isCompletedAgentMessage`) reads `message.metadata.
 * execution?.status` — the same "index/ is an older, thinner snapshot"
 * gap the brief warned about, here in a file R16 didn't call out by name.
 * `v401/server`'s schema has the matching field, so it's the base for this
 * section; the top-level `Chat`/`ChatSchema` fields stayed on `index/`'s
 * simpler version, since nothing copied needed more.
 *
 * This is the AI-SDK-shaped message (`metadata.type === "agent"`,
 * `metadata.execution`) the earlier placeholder's own doc comment
 * explicitly said AOS's real Go entity does *not* match (Author/Runs/
 * Reactions as flat top-level fields instead). That mismatch is real and
 * unresolved here — reconciling it against the live Go backend is this
 * port's per-feature verification pass (a follow-up task per controller
 * ruling R17), not this bulk-copy one. `components/ui/chat-timeline.tsx`
 * (task's execution tab) is hand-adapted to this shape in this same
 * commit so it keeps compiling and rendering against the real backend's
 * simpler payload; see that file's own comment.
 */
export const ChatMessageExecutionStatusSchema = z
  .enum(["pending", "running", "completed", "error", "interrupted"])
  .describe(
    "Execution lifecycle status. pending waits in the queue, running streams the response, completed and error are terminal, interrupted was stopped cooperatively.",
  );

export const ChatMessageReactionSchema = z.object({
  actor: z
    .string()
    .describe("Temporary username string that identifies who reacted"),
  value: z.string().describe("Emoji or reaction value"),
  timestamp: z.string().datetime().describe("Timestamp when the reaction was added"),
});

export const ChatMessageRunTokensUsageSchema = z.object({
  inputTokens: z.number().describe("Tokens consumed by the prompt."),
  outputTokens: z.number().describe("Tokens produced by the completion."),
  totalTokens: z.number().describe("Total tokens consumed by the run."),
});

export const ChatMessageExecutionErrorSchema = z.object({
  code: z.string().describe("Stable machine-readable error code."),
  message: z.string().describe("Human-readable error message."),
  details: z.record(z.string(), z.unknown()).optional(),
});

/** One outbound agent dispatch recorded on the source message's metadata. */
export const ChatMessageDispatchSchema = z.object({
  agentId: z.string().describe("Agent targeted by this dispatch."),
  jobId: z.string().describe("Queue job identifier associated with the dispatch."),
  sourceMessageId: z.string().describe("Message that triggered the dispatch."),
  responseMessageId: z
    .string()
    .optional()
    .describe("Agent response message created for this dispatch."),
  status: ChatMessageExecutionStatusSchema,
  startedAt: z.string().datetime(),
  completedAt: z.string().datetime().optional(),
  usage: ChatMessageRunTokensUsageSchema.optional(),
  error: ChatMessageExecutionErrorSchema.optional(),
});

/** Execution record persisted inside the agent-authored message's own metadata. */
export const ChatMessageExecutionSchema = z.object({
  agentId: z.string().describe("Agent that generated the message."),
  jobId: z.string().describe("Queue job identifier associated with the execution."),
  sourceMessageId: z.string().describe("Message that triggered the execution."),
  status: ChatMessageExecutionStatusSchema,
  startedAt: z.string().datetime(),
  completedAt: z.string().datetime().optional(),
  usage: ChatMessageRunTokensUsageSchema.optional(),
  error: ChatMessageExecutionErrorSchema.optional(),
});

const ChatMessageBaseMetadataSchema = z.object({
  reactions: z.array(ChatMessageReactionSchema).optional(),
  runs: z
    .array(ChatMessageDispatchSchema)
    .optional()
    .describe("Dispatch attempts created from this source message."),
  status: z
    .enum(["streaming", "done"])
    .optional()
    .describe("Current status of the message generation"),
  processing: z
    .object({
      jobId: z
        .string()
        .optional()
        .describe(
          "Queue job/run currently or previously associated with this message",
        ),
      agentId: z
        .string()
        .optional()
        .describe("Agent responsible for processing this message"),
      sourceMessageId: z
        .string()
        .optional()
        .describe("User message that produced this agent response"),
      responseMessageId: z
        .string()
        .optional()
        .describe("Agent response produced for this user message"),
      status: z
        .enum(["running", "completed"])
        .describe("Processing lifecycle state for retry deduplication"),
      startedAt: z.string().datetime()
        .optional()
        .describe("Timestamp when processing started"),
      completedAt: z.string().datetime()
        .optional()
        .describe("Timestamp when processing completed"),
    })
    .optional()
    .describe(
      "Queue processing metadata used to make message retries idempotent",
    ),
  createdAt: z.string().datetime().describe("Timestamp when the chat was created"),
  updatedAt: z.string().datetime()
    .describe("Timestamp when the chat was last updated"),
});

export const ChatMessageUserMetadataSchema =
  ChatMessageBaseMetadataSchema.extend({
    type: z.literal("user"),
    data: z.object({
      id: z.string(),
    }),
  });

export const ChatMessageAgentMetadataSchema =
  ChatMessageBaseMetadataSchema.extend({
    type: z.literal("agent"),
    data: z.object({
      id: z.string(),
    }),
    execution: ChatMessageExecutionSchema.optional().describe(
      "Execution record for the authored message.",
    ),
    usage: z
      .object({
        inputTokens: z.number(),
        outputTokens: z.number(),
        totalTokens: z.number(),
      })
      .optional()
      .describe("Token usage from the AI SDK step that produced this message"),
  });

export const ChatMessageSystemMetadataSchema =
  ChatMessageBaseMetadataSchema.extend({
    type: z.literal("system"),
  });

export const ChatMessageSchema = z.discriminatedUnion("type", [
  ChatMessageUserMetadataSchema,
  ChatMessageAgentMetadataSchema,
  ChatMessageSystemMetadataSchema,
]);

export const ChatChannelMetadataSchema = z.object({
  provider: z.string().describe("Messaging provider slug (e.g. 'telegram')"),
  chatId: z.string().describe("Provider-specific chat identifier"),
  userId: z.string().optional().describe("Provider-specific user identifier"),
});

export const ChatMetadataSchema = z
  .object({
    channel: ChatChannelMetadataSchema.optional().describe(
      "External channel mapping when this chat originated from a messaging provider",
    ),
  })
  .passthrough()
  .describe(
    "Chat-level metadata; additional keys are allowed for extensibility",
  );

export const ChatRunSchema = z.object({
  id: z.string().describe("Unique identifier for the chat run"),
  status: z
    .enum(["pending", "running", "completed", "error"])
    .describe("Current status of the chat run"),
});

/**
 * `kind`/`visibility`/`participants` below are grounded in `_extracted/
 * v401/server/src/features/chat/schemas/chat.schema.ts` rather than
 * `index/`: `index/`'s `ChatSchema` (this file's base) has none of these
 * three ACL fields, but `services/chat/chat-kind.helper.ts` — copied
 * verbatim as part of this task's bulk feature copy — reads `chat.kind`,
 * `chat.participants`, and (through `chat.metadata?.channel`, already
 * covered above) the channel mapping to classify a chat's sidebar surface.
 * Same "v401/server has the fuller schema" situation the message-metadata
 * section above documents.
 */
export const ChatKindSchema = z
  .enum(["dm", "channel", "task", "run", "external"])
  .describe(
    "Chat surface kind. dm = private direct message, channel = workspace or invite-only channel, task = task-linked thread, run = routine run, external = provider-bound messenger.",
  );

export const ChatVisibilitySchema = z
  .enum(["private", "workspace"])
  .describe(
    "private = only participants may access; workspace = any workspace member may access.",
  );

export const ChatParticipantRoleSchema = z.enum(["member", "admin"]);
export const ChatParticipantTypeSchema = z.enum(["user", "agent"]);

export const ChatParticipantSchema = z.object({
  type: ChatParticipantTypeSchema,
  id: z.string().min(1).describe("Participant identifier — user UUID or agent slug."),
  role: ChatParticipantRoleSchema.default("member"),
  joinedAt: z.string().datetime(),
});

/**
 * ChatSchema
 * Primary schema for a AOS Chat session.
 */
export const ChatSchema = z.object({
  id: z.string().describe("Unique identifier for the chat session"),
  kind: ChatKindSchema.default("channel"),
  visibility: ChatVisibilitySchema.default("workspace"),
  participants: z.array(ChatParticipantSchema).default([]),
  task: z.string().optional().describe("Task ID for this chat"),
  routine: z.string().optional().describe("Parent routine slug associated with this chat"),
  agent: z.string().optional().describe("Owning agent slug"),
  run: z
    .string()
    .optional()
    .describe("Routine run ID associated with this chat"),
  metadata: ChatMetadataSchema.optional().describe(
    "Optional chat-level metadata used for external channel mappings",
  ),
  title: z.string().describe("Chat title"),
  messages: z
    .array(z.custom<ChatMessage>())
    .describe("List of UIMessage objects representing the conversation"),
  createdAt: z.string().datetime().describe("Timestamp when the chat was created"),
  updatedAt: z.string().datetime()
    .describe("Timestamp when the chat was last updated"),
});

export type ChatKind = z.infer<typeof ChatKindSchema>;
export type ChatVisibility = z.infer<typeof ChatVisibilitySchema>;
export type ChatParticipant = z.infer<typeof ChatParticipantSchema>;

/**
 * CreateChatSchema
 * Schema used when creating a new chat session.
 */
export const CreateChatSchema = z.object({
  id: z
    .string()
    .optional()
    .describe("Optional unique identifier for the chat session"),
  run: z
    .string()
    .optional()
    .describe("Optional routine run ID associated with the new chat"),
  metadata: ChatMetadataSchema.optional().describe(
    "Optional chat-level metadata used for external channel mappings",
  ),
  title: z.string().describe("Chat title"),
});

/**
 * UpdateChatSchema
 * Schema used when updating an existing chat session.
 */
export const UpdateChatSchema = z.object({
  id: z.string().describe("Unique identifier for the chat session to update"),
  title: z.string().optional().describe("Updated title for the chat session"),
  metadata: ChatMetadataSchema.optional().describe(
    "Updated chat-level metadata used for external channel mappings",
  ),
});

/**
 * DeleteChatSchema
 * Schema used when deleting an existing chat session.
 */
export const DeleteChatSchema = z.object({
  id: z.string().describe("Unique identifier for the chat session to delete"),
});

/**
 * SendMessageSchema
 * Schema for appending a message to an existing chat and routing the next execution.
 */
export const SendMessageSchema = z.object({
  id: z.string().describe("Existing chat session identifier"),
  task: z
    .string()
    .optional()
    .describe("Optional task context forwarded to the downstream agent run"),
  message: z
    .custom<ChatMessage>()
    .describe(
      'Message appended to the chat before routing. In shared chats, `<mention id="agent-id">` tags trigger tagged agents; direct-message chats infer the target agent from the chat ID',
    ),
});

export const ToggleReactionSchema = z.object({
  id: z.string().describe("Existing chat session identifier"),
  messageId: z
    .string()
    .describe("Message identifier that will receive the reaction toggle"),
  value: z.string().describe("Reaction emoji or value to toggle"),
  actor: z
    .string()
    .optional()
    .describe("Temporary username string identifying the reacting actor"),
  timestamp: z.string().datetime()
    .optional()
    .describe("Optional reaction timestamp override"),
});

/**
 * AgentProcessMessage
 * Schema for the background queue's per-cycle processing payload.
 */
export const AgentProcessMessage = z.object({
  workspace: z
    .string()
    .describe("Workspace identifier that owns this processing run"),
  chat: z.string().optional().describe("Chat session identifier"),
  task: z.string().optional().describe("Task to process"),
  agent: z.string().optional().describe("Agent to process"),
  messageId: z
    .string()
    .optional()
    .describe("Message identifier that triggered the processing run"),
});

export type ChatMessageUserMetadata = z.infer<
  typeof ChatMessageUserMetadataSchema
>;

export type ChatMessageAgentMetadata = z.infer<
  typeof ChatMessageAgentMetadataSchema
>;

export type ChatReaction = z.infer<
  typeof ChatMessageReactionSchema
>;

export type ChatMessageTypeMetadata = z.infer<
  typeof ChatMessageSchema
>;

export type ChatMessageMetadata = z.infer<
  typeof ChatMessageSchema
>;

export type ChatMessage = UIMessage<ChatMessageMetadata>;

/** Type definition for a Chat record. */
export type Chat = z.infer<typeof ChatSchema>;

/** Type definition for an active Chat run. */
export type ChatRun = z.infer<typeof ChatRunSchema>;

/** Type definition for the input used to create a Chat. */
export type CreateChatInput = z.infer<typeof CreateChatSchema> & {
  task?: string;
};

/** Type definition for the input used to update a Chat. */
export type UpdateChatInput = z.infer<typeof UpdateChatSchema> & {
  messages?: ChatMessage[];
};

/** Type definition for the input used to delete a Chat. */
export type DeleteChatInput = z.infer<typeof DeleteChatSchema>;

/** Type definition for the message sending payload. */
export type SendMessageInput = z.infer<typeof SendMessageSchema>;

/** Type definition for toggling a reaction on a single chat message. */
export type ToggleReactionInput = z.infer<typeof ToggleReactionSchema>;

/**
 * @description Input payload used by the background queue to process one agent execution cycle.
 */
export type ChatProcessInput = z.infer<typeof AgentProcessMessage> & {
  runId: string;
};

/**
 * @description Result envelope returned after processing a chat execution cycle.
 */
export interface ChatProcessResult extends ResponseWithCTA {
  status: "completed" | "skipped";
  chatId: string;
  message: string;
}

/** Result of the chat listing operation. */
export interface ChatServiceListResult extends ResponseWithCTA {
  chats: Chat[];
}

/** Result of a single chat retrieval. */
export interface ChatServiceGetResult extends ResponseWithCTA {
  chat: Chat | null;
}

/** Result of a chat creation. */
export interface ChatServiceCreateResult extends ResponseWithCTA {
  chat: Chat;
}

/** Result of a chat update. */
export interface ChatServiceUpdateResult extends ResponseWithCTA {
  chat: Chat;
}

/** Result of a chat deletion. */
export interface ChatServiceDeleteResult extends ResponseWithCTA {
  id: string;
  success: boolean;
}

/**
 * IChatService
 * Defines the contract for chat management operations.
 */
export interface IChatService {
  list(params: {}): Promise<ChatServiceListResult>;
  getById(params: { id: string }): Promise<ChatServiceGetResult>;
  create(params: CreateChatInput): Promise<ChatServiceCreateResult>;
  update(params: UpdateChatInput): Promise<ChatServiceUpdateResult>;
  delete(params: DeleteChatInput): Promise<ChatServiceDeleteResult>;
  send(params: SendMessageInput): Promise<any>;
  sendFromChannel(params: {
    agentId: string;
    provider: string;
    channelId: string;
    userId?: string;
    message: ChatMessage;
    task?: string;
  }): Promise<any>;
  toggleReaction(params: ToggleReactionInput): Promise<ChatServiceUpdateResult>;
  process(params: ChatProcessInput): Promise<ChatProcessResult>;
}

