import { ROUTINE_STATUS_CONFIG, ROUTINE_STATUS_ORDER, ROUTINE_RESERVED_AGENT_CONFIG } from "@/features/routine/presentation/consts/routine";
import { RoutineTriggersHelper } from "@/features/routine/presentation/helpers/routine-triggers.helper";
import type {
  FractalRoutine,
  FractalRoutineReservedAgent,
} from "@/features/routine/interfaces/routine.interfaces";

/**
 * Pure routine presentation and domain-adjacent utilities.
 *
 * Owns status display grouping and reserved-agent guards that must not live in
 * `interfaces/` (types-only). Side-effect free — no context or persistence.
 */
export class FractalRoutineHelper {
  /**
   * Reserved routine agent targets stored on routine records.
   *
   * - `orchestrator` — resolves to the workspace orchestrator at fire time.
   * - `all` — manual fan-out to every agent (manual fire only).
   */
  public static readonly RESERVED_AGENTS = [
    "orchestrator",
    "all",
  ] as const satisfies readonly FractalRoutineReservedAgent[];

  /**
   * Returns UI status config for a routine status value.
   *
   * @param status - Persisted routine status.
   * @returns Label / color config from {@link ROUTINE_STATUS_CONFIG}.
   */
  public static getStatus(status: FractalRoutine["status"]) {
    return ROUTINE_STATUS_CONFIG[status];
  }

  /**
   * Groups routines by status in canonical display order.
   *
   * Omits statuses with zero routines.
   *
   * @param routines - Routines to group.
   * @returns Map of status → routines (only non-empty buckets).
   */
  public static groupByStatus(routines: FractalRoutine[]) {
    return ROUTINE_STATUS_ORDER.reduce(
      (acc, status) => {
        const items = routines.filter((routine) => routine.status === status);
        if (items.length > 0) acc[status] = items;
        return acc;
      },
      {} as Record<FractalRoutine["status"], FractalRoutine[]>,
    );
  }

  /**
   * Returns whether a routine agent field uses a reserved workspace target.
   *
   * @param agent - Agent slug from a routine record.
   * @returns `true` when `agent` is {@link FractalRoutineReservedAgent}.
   */
  public static isReservedAgent(
    agent: string,
  ): agent is FractalRoutineReservedAgent {
    // [Condition]: Narrow to the reserved-agent union.
    return (FractalRoutineHelper.RESERVED_AGENTS as readonly string[]).includes(
      agent,
    );
  }

  /**
   * Resolves a display label for a routine agent field (slug or reserved target).
   *
   * @param agentId - Agent slug or reserved target from a routine record.
   * @param agents - Workspace agents for slug lookup.
   * @returns Human-readable label for UI surfaces.
   */
  public static getAgentLabel(
    agentId: string,
    agents: Array<{ id: string; name: string }>,
  ): string {
    if (FractalRoutineHelper.isReservedAgent(agentId)) {
      return ROUTINE_RESERVED_AGENT_CONFIG[agentId].label;
    }

    return agents.find((agent) => agent.id === agentId)?.name ?? agentId;
  }

  /**
   * Builds a compact single-line label for routine triggers (list rows, tooltips).
   *
   * @param triggers - Trigger definitions from a routine record.
   * @param maxVisible - Maximum trigger labels before collapsing to `+N`.
   * @returns Human-readable summary joined with middle dots.
   */
  public static getTriggersInlineLabel(
    triggers: FractalRoutine["triggers"],
    maxVisible = 2,
  ): string {
    if (triggers.length === 0) {
      return "No triggers";
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
        const { namespace, event } = trigger.config;
        if (namespace && event) return `${namespace}.${event}`;
        if (namespace) return namespace;
        return "Activity";
      }

      return "Webhook";
    });

    if (labels.length <= maxVisible) {
      return labels.join(" · ");
    }

    return `${labels.slice(0, maxVisible).join(" · ")} +${labels.length - maxVisible}`;
  }
}
