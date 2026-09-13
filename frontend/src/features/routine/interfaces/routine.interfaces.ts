/**
 * A routine as the daemon answers it (`internal/domain/routine`).
 *
 * These types were reconstructed from the screens that read them, and the
 * screens had drifted from Go: a third status Go refuses ("paused"), filters
 * named `path` where Go names them `field` and keeps them beside `config`
 * rather than inside it, a webhook `token` Go never stores, and a run shape
 * (pending/completed/error, `finishedAt`, `chat`) Go has never had. Every one
 * of those produced a refusal or an empty screen. They follow Go now.
 */

/** `routine.Status`: whether a routine may fire at all. */
export type RoutineStatus = "enabled" | "disabled";

/** `routine.RunStatus`: how one firing ended. */
export type RunStatus = "running" | "succeeded" | "failed" | "timed_out" | "skipped";

/** What a firing records as its cause. "manual" is Fire's default. */
export type RunTrigger = "manual" | "scheduled" | "webhook" | "activity";

/** `routine.Filter`: one condition on an activity payload. */
export interface RoutineActivityFilter {
  field: string;
  operator: "eq" | "neq" | "contains";
  /** Go compares any JSON value; the editor edits its text. */
  value: string;
}

/**
 * `routine.Trigger` as stored and answered: the settings nested under
 * `config`, and an activity trigger's filters beside it.
 */
export type RoutineTrigger =
  | { type: "scheduled"; config: { cron: string } }
  | { type: "webhook"; config: { tokenHash?: string } }
  | {
      type: "activity";
      config: { namespace: string; event?: string };
      filters?: Array<Omit<RoutineActivityFilter, "value"> & { value: unknown }>;
    };

/** `routine.Scope`: what a routine may do while it runs. */
export interface RoutineScope {
  allowCreateTasks: boolean;
  allowExternalCalls: boolean;
  allowedTools?: string[];
}

/** `routine.View`: the routine, with what the file cannot hold. */
export interface Routine {
  id: string;
  name: string;
  agent: string;
  /** The prompt. `routines_list` does not load it; `routines_get` does. */
  content?: string;
  triggers: RoutineTrigger[];
  status: RoutineStatus;
  scope?: RoutineScope;
  createdAt: string;
  updatedAt: string;
  lastFiredAt?: string;
  /** The scheduler's tick, e.g. "15m0s". A finer cron fires once per tick. */
  effectiveInterval?: string;
  /** When the schedule fires next, computed by the daemon. */
  nextRun?: string;
  /** Anything about this routine that will surprise somebody later. */
  warnings?: string[];
}

/** `routine.Run`: the audit record of one firing. */
export interface Run {
  id: string;
  routine: string;
  agent: string;
  trigger: RunTrigger | (string & {});
  payload?: Record<string, unknown>;
  /** The conversation the run executed in. Empty when it never started one. */
  chatId?: string;
  status: RunStatus;
  startedAt: string;
  endedAt?: string;
  /** Why it failed, or why it was skipped. */
  error?: string;
  usage?: { input: number; output: number; total: number; costUsd: number };
}

/** `routine.TriggerInput`: the flat shape create and update take. */
export interface RoutineTriggerInput {
  type: RoutineTrigger["type"];
  cron?: string;
  namespace?: string;
  event?: string;
  filters?: RoutineActivityFilter[];
}
