/**
 * Mirrors internal/domain/routine/entity.go (type Run). Verified
 * field-by-field against `internal/domain/routine/entity.go:181` and
 * `internal/domain/routine/schema.go:158` (RunsOutput, for
 * FractalRoutineRunList) — both `usage` on Run and `total` on the list
 * wrapper are present in the Go structs but missing from the task brief's
 * draft; added here since neither is `omitempty`, so both are always on
 * the wire.
 */
export type FractalRunStatus = "running" | "succeeded" | "failed" | "timed_out" | "skipped";

export interface FractalRunUsage {
  input: number;
  output: number;
  total: number;
  costUsd: number;
}

export interface FractalRoutineRun {
  agent: string;
  routine: string;
  id: string;
  trigger: string;
  payload?: Record<string, unknown>;
  chatId?: string;
  status: FractalRunStatus;
  startedAt: string;
  endedAt?: string;
  error?: string;
  usage: FractalRunUsage;
}

export interface FractalRoutineRunList {
  routine: string;
  runs: FractalRoutineRun[];
  total: number;
}
