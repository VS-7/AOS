/**
 * Mirrors internal/domain/chat/entity.go — not the original's AI-SDK-shaped
 * `FractalChatMessage` (`metadata.type === "agent"`, `metadata.execution`,
 * `metadata.runs`). AOS's Go entity carries Author/Runs/Reactions as direct
 * top-level fields, so the frontend types follow the Go struct, not the
 * original's TypeScript.
 */

export type ChatKind = "dm" | "channel" | "task" | "run" | "external";
export type ChatVisibility = "private" | "workspace";
export type ActorType = "user" | "agent";
export type MessageRole = "system" | "user" | "assistant" | "tool";
export type PartType = "text" | "reasoning" | "tool-call" | "tool-result" | "file";
export type RunStatus = "pending" | "running" | "completed" | "error" | "interrupted";

export interface Participant {
  type: ActorType;
  id: string;
  role?: string;
  joinedAt: string;
}

export interface ChannelMeta {
  provider: string;
  chatId: string;
  userId?: string;
}

export interface Author {
  type: ActorType;
  id: string;
}

export interface Part {
  type: PartType;
  text?: string;
  toolName?: string;
  toolCallId?: string;
  input?: unknown;
  output?: unknown;
  mediaType?: string;
  uri?: string;
}

export interface Reaction {
  actor: string;
  value: string;
  timestamp: string;
}

export interface TokenUsage {
  inputTokens?: number;
  outputTokens?: number;
  totalTokens?: number;
}

export interface Run {
  agentId: string;
  jobId?: string;
  status: RunStatus;
  usage?: TokenUsage;
  costUsd?: number;
  error?: string;
  startedAt?: string;
  completedAt?: string;
}

export interface Message {
  id: string;
  role: MessageRole;
  author?: Author;
  parts: Part[];
  reactions?: Reaction[];
  runs?: Run[];
  createdAt: string;
}

export interface Chat {
  id: string;
  key?: string;
  title: string;
  kind: ChatKind;
  visibility: ChatVisibility;
  participants: Participant[];
  task?: string;
  routine?: string;
  agent?: string;
  channel?: ChannelMeta;
  messages: Message[];
  createdAt: string;
  updatedAt: string;
}
