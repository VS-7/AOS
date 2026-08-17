/** Mirrors internal/domain/agent/entity.go — the shape agents_list/agents_get return. */

export interface AgentChannel {
  provider: string;
  chatId: string;
}

export interface AgentExec {
  allow?: string[];
  denyArgs?: string[];
  allowShell?: boolean;
  policy?: string;
}

export interface AgentSandbox {
  permissions?: string[];
  exec?: AgentExec;
}

export interface Agent {
  id: string;
  skill?: string;
  name: string;
  image?: string;
  description?: string;
  role?: string;
  leader?: string;
  provider?: string;
  model?: string;
  voice?: string;
  channels?: AgentChannel[];
  orchestrator: boolean;
  reasoning?: string;
  sandbox?: AgentSandbox;
  createdAt?: string;
  updatedAt?: string;
  content?: string;
}
