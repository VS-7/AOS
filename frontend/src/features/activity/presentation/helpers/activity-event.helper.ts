import type { FractalActivityEventDefinition } from "@/features/activity/interfaces/activity.interfaces";

/**
 * Pure helpers for workspace notification event definitions consumed by the UI.
 *
 * Transforms {@link FractalActivityEventDefinition} records returned by
 * `GET /activity/events` into stable keys, labels, and filter field lists
 * without duplicating backend notification registries in the frontend.
 */
export class FractalActivityEventHelper {
  /**
   * Default filter field when a JSON Schema exposes no top-level properties.
   */
  public static readonly DEFAULT_FILTER_FIELD = "causer";

  /**
   * Builds the canonical `namespace.event` identifier for an event definition.
   *
   * @param namespace - Feature namespace (for example `task`, `routine`).
   * @param event - Event name within the namespace.
   * @returns Stable composite key used for deduplication in trigger pickers.
   */
  public static getEventKey(namespace: string, event: string): string {
    return `${namespace}.${event}`;
  }

  /**
   * Finds a single event definition by namespace and event name.
   *
   * @param definitions - Event definitions loaded from the activity API.
   * @param namespace - Target namespace.
   * @param event - Target event name.
   * @returns Matching definition or `undefined` when not registered.
   */
  public static findDefinition(
    definitions: FractalActivityEventDefinition[],
    namespace: string,
    event: string,
  ): FractalActivityEventDefinition | undefined {
    return definitions.find(
      (definition) =>
        definition.namespace === namespace && definition.event === event,
    );
  }

  /**
   * Returns event definitions that are not already selected in routine triggers.
   *
   * @param definitions - Full list of routine-eligible event definitions.
   * @param usedKeys - Composite keys (`namespace.event`) already attached to triggers.
   * @returns Definitions available for new activity triggers.
   */
  public static getAvailableDefinitions(
    definitions: FractalActivityEventDefinition[],
    usedKeys: Set<string>,
  ): FractalActivityEventDefinition[] {
    return definitions.filter(
      (definition) =>
        !usedKeys.has(
          FractalActivityEventHelper.getEventKey(
            definition.namespace,
            definition.event,
          ),
        ),
    );
  }

  /**
   * Resolves a short human label for menus and trigger rows.
   *
   * Prefers the API `description`, then falls back to the raw event name.
   *
   * @param definition - Event definition from the activity API.
   * @returns Display label for UI surfaces.
   */
  public static getDisplayLabel(
    definition: FractalActivityEventDefinition,
  ): string {
    const description = definition.description?.trim();
    if (description) {
      return description;
    }

    return definition.event;
  }

  /**
   * Extracts filterable payload field paths from a JSON Schema document.
   *
   * Uses top-level `properties` keys from the serialized schema. When the
   * schema has no object properties, returns {@link FractalActivityEventHelper.DEFAULT_FILTER_FIELD}.
   *
   * @param definition - Event definition carrying a JSON Schema payload.
   * @returns Sorted list of dot-path roots available in the filter editor.
   */
  public static getFilterableFields(
    definition: FractalActivityEventDefinition | undefined,
  ): string[] {
    if (!definition) {
      return [FractalActivityEventHelper.DEFAULT_FILTER_FIELD];
    }

    const properties = definition.schema.properties;

    if (properties && typeof properties === "object") {
      const keys = Object.keys(properties as Record<string, unknown>);

      if (keys.length > 0) {
        return keys.sort();
      }
    }

    return [FractalActivityEventHelper.DEFAULT_FILTER_FIELD];
  }
}
