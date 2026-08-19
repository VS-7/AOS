/**
 * Mirrors internal/domain/activity/entity.go — the original was type-only
 * and did not survive the bundle, but this domain has a backend, so the
 * reconstruction is verifiable rather than guessed.
 *
 * Verified field-by-field against `internal/domain/activity/entity.go:21`
 * (Activity struct) and `internal/domain/activity/schema.go:27` (ListOutput,
 * for FractalActivityList).
 */
export type FractalActivityActorType = "agent" | "user" | "system";

export interface FractalActivity {
  id: string;
  namespace: string;
  event: string;
  title: string;
  body?: string;
  icon?: string;
  /** Shaped by the namespace: a task event carries the task. */
  data?: Record<string, unknown>;
  actor: string;
  actorType: FractalActivityActorType;
  createdAt: string;
}

export interface FractalActivityList {
  activities: FractalActivity[];
  total: number;
  unread: number;
  actor: string;
}
