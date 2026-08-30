/**
 * A tool call waiting on a person.
 *
 * Mirrors `internal/domain/event.ApprovalRequest` field for field, as
 * `approvals_list` returns it. The interface reads every one of them: the tool
 * and the payload are what is being decided, the reason is what the hook said,
 * the risk drives how loudly it is presented, and `expiresAt` is not
 * decoration — a request nobody answers becomes a *denial* when it passes, so
 * the person has to be able to see the clock running.
 */
export interface ApprovalRequest {
  id: string;
  sessionId?: string;
  workspace?: string;
  /** Slug of the agent that wants to make the call. */
  agent?: string;
  /** Tool the agent is asking to run. */
  tool: string;
  /** Payload the tool would be called with, as the model proposed it. */
  input?: unknown;
  /** Why the hook asked instead of deciding on its own. */
  reason?: string;
  risk: ApprovalRisk;
  /** RFC 3339. When the request stops waiting and becomes a denial. */
  expiresAt: string;
  createdAt: string;
}

export type ApprovalRisk = "low" | "medium" | "high";

/**
 * How long an approval lasts.
 *
 * The master prompt is explicit that approving once is not approving forever,
 * which is why `session` is what the plain Approve button sends and `always`
 * takes a second, deliberate action (ADR-0007).
 */
export type ApprovalRemember = "none" | "session" | "always";

/** What `approvals_list` answers with. */
export interface PendingApprovals {
  pending: ApprovalRequest[];
  total: number;
}

/** What `approvals_decide` answers with. */
export interface ApprovalDecision {
  id: string;
  /**
   * False when nothing was waiting under this identifier — which is what a
   * request that already timed out looks like from here. It must not read as
   * success: the call it belonged to was denied when the deadline passed.
   */
  settled: boolean;
}
