import type {
  CollectionViewActionResult,
  CollectionViewRenderResult,
  Spec,
} from "@/features/view/interfaces/collections.interfaces";
// `IgniterCollectionViewDefinition` is defined in `view.interfaces.ts` (an
// earlier task's dormant-domain stand-in built expecting this exact name),
// not in the collections stub above — kept as a separate import rather than
// re-declaring the shape twice.
import type { IgniterCollectionViewDefinition } from "@/features/view/interfaces/view.interfaces";

export type ViewActionParams = {
  type: string;
  properties?: Record<string, ViewActionParamProperty>;
  required?: string[];
};

export type ViewActionParamProperty = {
  type?: string;
  format?: string;
  enum?: unknown[];
  description?: string;
  minimum?: number;
  maximum?: number;
  default?: unknown;
  items?: ViewActionParamProperty;
};

export type ViewActionDefinition = {
  description?: string;
  params?: ViewActionParams;
  confirm?: ViewActionConfirmConfig;
  handler?: string;
};

export type ViewActionConfirmConfig = {
  title: string;
  message: string;
  variant?: "default" | "danger" | "warning" | "info";
};

/**
 * View definition shape used by the presentation layer.
 * Spec is the json-render UI tree; state is merged server-side at render.
 */
export type ViewDefinition = {
  name: string;
  title: string;
  description?: string;
  spec?: Spec;
  actions?: Record<string, ViewActionDefinition>;
  getData?: string | IgniterCollectionViewDefinition["getData"];
  metadata?: Record<string, unknown>;
};

/**
 * Alias of Igniter's render result — `{ view, spec, renderedAt }`.
 */
export type ViewRenderResult = CollectionViewRenderResult;

export type ViewActionResult = CollectionViewActionResult;

export class ViewDataHelper {
  /**
   * Returns a renderable json-render {@link Spec} from an Igniter render result.
   *
   * Recovers when Igniter's `safeStringify` collapses shared `elements` refs into
   * the literal `"[Circular]"` string (shared between `result.view.spec` and
   * `result.spec`). In that case we rebuild from `view.spec.elements` + `spec.state`.
   */
  public static getSpec(result: ViewRenderResult | null): Spec | null {
    if (!result?.spec?.root) {
      return null;
    }

    const rendered = result.spec;
    if (this._has_named_elements(rendered.elements)) {
      return rendered;
    }

    const viewSpec = (result.view as ViewDefinition | undefined)?.spec;
    if (!viewSpec?.root || !this._has_named_elements(viewSpec.elements)) {
      return null;
    }

    return {
      root: viewSpec.root,
      elements: viewSpec.elements,
      state: (rendered.state as Record<string, unknown> | undefined) ?? {},
    };
  }

  public static getState(
    result: ViewRenderResult | null,
  ): Record<string, unknown> {
    return (this.getSpec(result)?.state as Record<string, unknown> | undefined) ?? {};
  }

  public static isEmpty(result: ViewRenderResult | null): boolean {
    if (!result?.spec) return true;
    const state = this.getState(result);
    return Object.keys(state).length === 0;
  }

  /**
   * True when `elements` is a flat named map (`{ page: { type, props, ... } }`),
   * not a corrupted string/array from circular JSON serialization.
   */
  private static _has_named_elements(
    elements: unknown,
  ): elements is NonNullable<Spec["elements"]> {
    if (typeof elements !== "object" || elements === null || Array.isArray(elements)) {
      return false;
    }

    const values = Object.values(elements as Record<string, unknown>);
    if (values.length === 0) {
      return false;
    }

    return values.some(
      (value) =>
        typeof value === "object" &&
        value !== null &&
        "type" in value &&
        typeof (value as { type?: unknown }).type === "string",
    );
  }

  public static getActions(
    view: ViewDefinition | null,
  ): Record<string, ViewActionDefinition> {
    return view?.actions ?? {};
  }

  public static getAction(
    view: ViewDefinition | null,
    actionId: string,
  ): ViewActionDefinition | undefined {
    return view?.actions?.[actionId];
  }

  public static hasAction(view: ViewDefinition | null, actionId: string): boolean {
    return !!view?.actions?.[actionId];
  }

  public static getActionParams(
    action: ViewActionDefinition | undefined,
  ): ViewActionParams | undefined {
    return action?.params;
  }

  public static getActionConfirm(
    action: ViewActionDefinition | undefined,
  ): ViewActionConfirmConfig | undefined {
    return action?.confirm;
  }

  public static getAllActionIds(view: ViewDefinition | null): string[] {
    return Object.keys(view?.actions ?? {});
  }
}
