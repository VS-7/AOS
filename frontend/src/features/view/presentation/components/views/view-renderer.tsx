import * as React from "react";
import {
  JSONUIProvider,
  Renderer,
} from "@json-render/react";
import type { Spec } from "@json-render/core";
import { t } from "@/lib/i18n";
import {
  viewCatalog,
  viewRegistry,
} from "../../catalog";

const JsonRenderDevtools = React.lazy(async () => {
  const mod = await import("@json-render/devtools-react");
  return { default: mod.JsonRenderDevtools };
});

export type ViewActionHandler = (
  params: Record<string, unknown>,
) => Promise<unknown> | unknown;

export type ViewRendererProps = {
  /** Complete json-render spec (structure + merged state from Igniter render). */
  spec: Spec | null;
  /** Action handlers bridged to the AOS view action API. */
  handlers?: Record<string, ViewActionHandler>;
  /** Optional loading flag while the spec is refreshing. */
  loading?: boolean;
};

/**
 * Renders a AOS view via `@json-render/react` with the shadcn catalog.
 *
 * Uses `JSONUIProvider` (State + Visibility + Action + Validation) per the
 * json-render React docs. State is seeded from `spec.state` (merged server-side).
 * Devtools mount only when `import.meta.env.DEV` is true (`Cmd+Shift+J`).
 */
export function ViewRenderer({
  spec,
  handlers,
  loading = false,
}: ViewRendererProps) {
  const initialState = React.useMemo(
    () => (spec?.state as Record<string, unknown> | undefined) ?? {},
    [spec],
  );

  const hasElements =
    !!spec?.root &&
    typeof spec.elements === "object" &&
    spec.elements !== null &&
    !Array.isArray(spec.elements) &&
    Object.keys(spec.elements).length > 0 &&
    typeof (spec.elements as Record<string, unknown>)[spec.root] === "object";

  if (!hasElements) {
    return (
      <div className="flex flex-col items-center justify-center gap-2 py-16 text-center">
        <p className="text-sm font-medium text-foreground">
          {t("This view has no renderable json-render spec.")}
        </p>
        <p className="text-muted-foreground text-sm max-w-md">
          {t("Expected a flat")} <code className="text-xs">elements</code> {t("map with a valid")} <code className="text-xs">root</code>. If the API returned
          <code className="text-xs"> &quot;[Circular]&quot;</code>{t(", restart the gateway after the view service serialization fix.")}
        </p>
      </div>
    );
  }

  return (
    <JSONUIProvider
      registry={viewRegistry}
      initialState={initialState}
      handlers={handlers}
    >
      <div className="flex min-h-0 flex-1 flex-col">
        <Renderer
          spec={spec}
          registry={viewRegistry}
          loading={loading}
        />
      </div>
      {import.meta.env.DEV ? (
        <React.Suspense fallback={null}>
          {/* `JsonRenderDevtools` resolves through `@json-render/devtools-react`'s
              `any` stub (vendor-stubs.d.ts), so `React.lazy` can't infer its
              props — spread cast rather than widen the lazy wrapper's type. */}
          <JsonRenderDevtools
            {...({
              spec,
              catalog: viewCatalog,
              hotkey: "mod+shift+j",
            } as any)}
          />
        </React.Suspense>
      ) : null}
    </JSONUIProvider>
  );
}
