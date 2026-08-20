/**
 * Mirrors internal/domain/activity/entity.go — the original was type-only
 * and did not survive the bundle, but this domain has a backend, so the
 * reconstruction is verifiable rather than guessed.
 *
 * Verified field-by-field against `internal/domain/activity/entity.go:21`
 * (Activity struct) and `internal/domain/activity/schema.go:27` (ListOutput,
 * for ActivityList).
 */
export type ActivityActorType = "agent" | "user" | "system";

export interface Activity {
  id: string;
  namespace: string;
  event: string;
  title: string;
  body?: string;
  icon?: string;
  /** Shaped by the namespace: a task event carries the task. */
  data?: Record<string, unknown>;
  actor: string;
  actorType: ActivityActorType;
  createdAt: string;
}

export interface ActivityList {
  activities: Activity[];
  total: number;
  unread: number;
  actor: string;
}

/**
 * One registered activity event type, as `GET /activity/events` (not yet a
 * live command — `activity.listEvents` is `null` in `lib/command-map.ts`)
 * would return it: which namespace/event pair it is, an optional
 * human-readable description, and the JSON Schema of its payload, used to
 * derive filterable field names for routine triggers.
 *
 * Not in any extraction (activity's own `entity.go`/`schema.go` mirror
 * above has no notion of a *catalog* of event types, only individual
 * `Activity` records) — reconstructed from
 * `presentation/helpers/activity-event.helper.ts`'s `ActivityEventHelper`,
 * this type's sole consumer.
 */
export interface ActivityEventDefinition {
  namespace: string;
  event: string;
  title?: string;
  description?: string;
  schema: {
    properties?: Record<string, unknown>;
    [key: string]: unknown;
  };
}
