import type { Run, RunStatus } from "@/features/routine/interfaces/routine.interfaces";
import { getLocale, t } from "@/lib/i18n";

/**
 * How the run history reads a run.
 *
 * The history was written for a run shape Go has never had — pending,
 * completed, error, `finishedAt`, `chat` — so once runs were loaded at all,
 * every one showed a red "Failed", every duration was "—", the counters stayed
 * at 0, and opening one opened a conversation named after the run's own id.
 */
export type RunOutcome = "running" | "succeeded" | "failed" | "skipped";

export class RoutineRunHelper {
  /** Four outcomes a person tells apart; a timeout is a way of failing. */
  public static outcome(status: RunStatus | string): RunOutcome {
    switch (status) {
      case "running":
        return "running";
      case "succeeded":
        return "succeeded";
      case "skipped":
        return "skipped";
      default:
        return "failed";
    }
  }

  public static outcomeLabel(outcome: RunOutcome): string {
    switch (outcome) {
      case "running":
        return t("Running");
      case "succeeded":
        return t("Succeeded");
      case "skipped":
        return t("Skipped");
      default:
        return t("Failed");
    }
  }

  /** What caused a run, as a person reads it. */
  public static triggerLabel(trigger: Run["trigger"]): string {
    switch (trigger) {
      case "manual":
        return t("By hand");
      case "scheduled":
        return t("Schedule");
      case "webhook":
        return t("Webhook");
      case "activity":
        return t("Activity");
      default:
        return t("Run");
    }
  }

  /** How long a run took, or "—" while it has not ended. */
  public static duration(run: Pick<Run, "startedAt" | "endedAt">): string {
    if (!run.endedAt) return "—";

    const ms = new Date(run.endedAt).getTime() - new Date(run.startedAt).getTime();
    if (!Number.isFinite(ms) || ms < 0) return "—";
    if (ms < 1_000) return "< 1s";
    if (ms < 60_000) return `${Math.round(ms / 1_000)}s`;

    const minutes = Math.round(ms / 60_000);
    if (minutes < 60) return `${minutes}m`;

    const hours = Math.floor(minutes / 60);
    const rem = minutes % 60;
    return rem === 0 ? `${hours}h` : `${hours}h ${rem}m`;
  }

  /** Runs with this outcome that ended (or started) within the window. */
  public static countInWindow(
    runs: Run[],
    outcome: RunOutcome,
    windowMs: number,
    now: number = Date.now(),
  ): number {
    const cutoff = now - windowMs;
    return runs.filter(
      (run) =>
        this.outcome(run.status) === outcome &&
        new Date(run.endedAt ?? run.startedAt).getTime() >= cutoff,
    ).length;
  }

  /** When a run started, in the interface's language. */
  public static formatStartedAt(iso: string): string {
    return new Date(iso).toLocaleString(getLocale(), {
      month: "short",
      day: "numeric",
      hour: "numeric",
      minute: "2-digit",
    });
  }
}
