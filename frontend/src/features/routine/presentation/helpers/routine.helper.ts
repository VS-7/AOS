import { ROUTINE_STATUS_CONFIG, ROUTINE_STATUS_ORDER } from "@/features/routine/presentation/consts/routine";
import { RoutineTriggersHelper } from "@/features/routine/presentation/helpers/routine-triggers.helper";
import type { ActivityEventDefinition } from "@/features/activity/interfaces/activity.interfaces";
import { ActivityEventHelper } from "@/features/activity/presentation/helpers/activity-event.helper";
import type {
  Routine,
  RoutineStatus,
} from "@/features/routine/interfaces/routine.interfaces";
import { getLocale, t } from "@/lib/i18n";

/**
 * Pure routine presentation utilities: status display and grouping, agent
 * labels, trigger summaries. Side-effect free — no context or persistence.
 */
export class RoutineHelper {
  /**
   * Returns UI status config for a routine status value.
   *
   * @param status - Persisted routine status.
   * @returns Label / color config from {@link ROUTINE_STATUS_CONFIG}.
   */
  public static getStatus(status: RoutineStatus) {
    return ROUTINE_STATUS_CONFIG[status] ?? ROUTINE_STATUS_CONFIG.disabled;
  }

  /**
   * Groups routines by status in canonical display order.
   *
   * Omits statuses with zero routines.
   *
   * @param routines - Routines to group.
   * @returns Map of status → routines (only non-empty buckets).
   */
  public static groupByStatus(routines: Routine[]) {
    return ROUTINE_STATUS_ORDER.reduce(
      (acc, status) => {
        const items = routines.filter((routine) => routine.status === status);
        if (items.length > 0) acc[status] = items;
        return acc;
      },
      {} as Record<RoutineStatus, Routine[]>,
    );
  }

  /**
   * A routine's owner as a person reads it.
   *
   * A routine belongs to a real agent: its directory is under that agent's,
   * and Go has no "orchestrator" or "all agents" to resolve at fire time. The
   * two were offered as owners and every create with one was refused with
   * AOS_ROUTINE_NO_SUCH_AGENT.
   *
   * @param agentId - Agent slug from a routine record.
   * @param agents - Workspace agents for slug lookup.
   * @returns Human-readable label for UI surfaces.
   */
  public static getAgentLabel(
    agentId: string,
    agents: Array<{ id: string; name: string }>,
  ): string {
    return agents.find((agent) => agent.id === agentId)?.name ?? agentId;
  }

  /**
   * The agent a new routine starts with: the workspace's orchestrator, which
   * is who a routine with no better owner belongs to.
   */
  public static getDefaultAgentId(
    agents: Array<{ id: string; orchestrator?: boolean }>,
  ): string {
    return agents.find((agent) => agent.orchestrator)?.id ?? agents[0]?.id ?? "";
  }

  /**
   * How long ago something happened, in the interface's language ("14m ago"
   * was English in every locale).
   */
  public static relativeTime(date: string | Date, now: number = Date.now()): string {
    const seconds = Math.round((new Date(date).getTime() - now) / 1000);
    if (!Number.isFinite(seconds)) return "";
    const format = new Intl.RelativeTimeFormat(getLocale(), { numeric: "auto", style: "short" });
    const steps: Array<[Intl.RelativeTimeFormatUnit, number]> = [
      ["year", 31_536_000],
      ["month", 2_592_000],
      ["week", 604_800],
      ["day", 86_400],
      ["hour", 3_600],
      ["minute", 60],
    ];
    for (const [unit, size] of steps) {
      if (Math.abs(seconds) >= size) return format.format(Math.round(seconds / size), unit);
    }
    // Under a minute reads as "now" rather than as a count of seconds.
    return format.format(0, "second");
  }

  /** The first characters of an id: enough to tell rows apart. */
  public static shortId(id: string): string {
    return id.length > 8 ? id.slice(0, 8) : id;
  }

  /**
   * Builds a compact single-line label for routine triggers (list rows, tooltips).
   *
   * @param triggers - Trigger definitions from a routine record.
   * @param events - The activity catalogue, for an event's title.
   * @param maxVisible - Maximum trigger labels before collapsing to `+N`.
   * @returns Human-readable summary joined with middle dots.
   */
  public static getTriggersInlineLabel(
    triggers: Routine["triggers"],
    events: ActivityEventDefinition[] = [],
    maxVisible = 2,
  ): string {
    if (triggers.length === 0) {
      return t("No triggers");
    }

    const labels = triggers.map((trigger) => {
      if (trigger.type === "scheduled") {
        const inferred = RoutineTriggersHelper.inferScheduledConfig(
          trigger.config.cron,
        );
        const summary = RoutineTriggersHelper.getScheduledSummary({
          ...inferred,
          cron: trigger.config.cron,
        });

        if (inferred.preset === "daily" || inferred.preset === "weekly") {
          return `${summary} · ${inferred.time}`;
        }

        if (inferred.preset === "custom") {
          return trigger.config.cron;
        }

        return summary;
      }

      if (trigger.type === "activity") {
        const { namespace, event = "" } = trigger.config;
        if (!namespace) return t("Activity");
        return RoutineTriggersHelper.eventTitle(
          ActivityEventHelper.findDefinition(events, namespace, event),
          namespace,
          event,
        );
      }

      return t("Webhook");
    });

    if (labels.length <= maxVisible) {
      return labels.join(" · ");
    }

    return `${labels.slice(0, maxVisible).join(" · ")} +${labels.length - maxVisible}`;
  }
}
