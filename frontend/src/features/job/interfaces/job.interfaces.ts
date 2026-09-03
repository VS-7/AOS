/**
 * The queue, as the daemon describes it.
 *
 * These mirror `internal/domain/job`'s `Job`, `ListOutput` and `StatsOutput`.
 * They live in a feature's `interfaces/` folder like every other ported
 * domain's types rather than inside the screen that reads them: a type
 * declared in a component is a type the next reader of the same data will
 * declare again, slightly differently, and the two will drift without
 * anything failing.
 *
 * They are not generated. `lib/schema.ts` types every command's *input*
 * precisely and its output as `unknown` — the registry publishes no output
 * schema — so a screen that reads a field has to say what it expects
 * somewhere. Here is that somewhere.
 */

/** Where one job stands. Go's `job.Status`. */
export type JobStatus = "pending" | "claimed" | "succeeded" | "failed" | "dead" | "retrying";

/** One unit of deferred work. */
export interface Job {
  id: string;
  queue: string;
  kind: string;
  status: JobStatus | string;
  attempts: number;
  maxTries: number;
  workspace?: string;
  runAt?: string;
  createdAt?: string;
  updatedAt?: string;
  error?: string;
}

/** The shape of the queue right now. */
export interface QueueStats {
  total: number;
  byStatus?: Record<string, number>;
  byQueue?: Record<string, number>;
  /** Jobs that failed their last attempt. */
  dead?: string[];
  /**
   * Jobs still marked claimed whose lease has lapsed.
   *
   * Not "busy": the worker holding them is gone, and recovering is what
   * hands them back to the queue.
   */
  stale?: string[];
  at?: string;
}
