import * as React from "react";
import type { Spec } from "@/features/view/interfaces/collections.interfaces";
import { Page, PageBody } from "@/components/ui/page";
import { aos } from "@/app/aos";
import { isDormant } from "@/lib/command-map";
import { DormantGate } from "@/components/DormantDomain";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import {
  CollectionViewProvider,
  ViewRenderer,
  type ViewDefinition,
  type ViewRenderResult,
} from "../../components/views";
import type { ViewActionHandler } from "../../components/views/view-renderer";
import { ViewDataHelper } from "../../helpers";

export const ViewPage = aos
  .page("/views/$id")
  .withMetadata({
    title: "View",
    description: "Render an independent view",
  })
  .use(WorkspacePageMiddleware())
  .withLoader(async ({ client, request, response }) => {
    const { id: viewId } = request.params;

    // The domain is live — `views_*` is a real Go group — and this stays as
    // the same short-circuit every other page has, so that if it is ever
    // declared dormant the empty envelope does not reach the `!view` check
    // below and preempt `DormantGate` with a 404.
    if (isDormant("view")) {
      return { view: null, viewId, renderResult: null };
    }

    const viewResult = await client.view.getById.query({
      params: { view: viewId },
    });

    const view = viewResult.data?.view ?? null;

    if (!view) {
      return response.notFound();
    }

    const renderResult = await client.view.render.query({
      params: { view: viewId },
      query: {},
    });

    // `.result` was a wrapper Go never sent, so this was `null` on every
    // view. The facade now answers `{view, spec, records, renderedAt}` — see
    // `view.render` in `command-map.ts`.
    const renderResponse = (renderResult.data ?? null) as ViewRenderResult | null;

    return {
      view,
      viewId,
      renderResult: renderResponse,
    };
  })
  .withComponent(({ route }) => {
    // Bails before any hook below runs — `handlers` (the `useMemo`
    // further down) calls `ViewDataHelper.getAllActionIds(viewDef)` on
    // loader data this dormant stub does not shape correctly, and
    // `isDormant("view")` is a build-time constant (never toggles for a
    // mounted instance), so skipping every hook here on every render is
    // consistent, not a rules-of-hooks violation in practice.
    if (isDormant("view")) {
      return <DormantGate feature="view">{null}</DormantGate>;
    }

    const { view, viewId, renderResult } = route.useLoaderData();
    const [spec, setSpec] = React.useState<Spec | null>(() =>
      ViewDataHelper.getSpec(renderResult),
    );

    React.useEffect(() => {
      setSpec(ViewDataHelper.getSpec(renderResult));
    }, [renderResult]);

    // The actions live on the tree's nodes in Go; the facade lifts them onto
    // the view, which is where `getAllActionIds` looks for them.
    const viewDef = {
      ...(view as ViewDefinition),
      actions:
        (renderResult?.view as ViewDefinition | undefined)?.actions ??
        (view as ViewDefinition).actions,
    } as ViewDefinition;

    const handlers = React.useMemo<Record<string, ViewActionHandler>>(() => {
      const actionIds = ViewDataHelper.getAllActionIds(viewDef);
      return Object.fromEntries(
        actionIds.map((actionId) => [
          actionId,
          async (params: Record<string, unknown>) => {
            const response = await aos.client.view.executeAction.mutate({
              params: { view: viewId, actionId },
              body: { params },
            });

            const result = response.data?.result as
              | { success?: boolean; updates?: Record<string, unknown> }
              | undefined;

            if (result?.updates) {
              setSpec((current) => {
                if (!current) return current;
                const nextState = {
                  ...((current.state as Record<string, unknown> | undefined) ??
                    {}),
                };
                for (const [pointer, value] of Object.entries(result.updates!)) {
                  applyJsonPointer(nextState, pointer, value);
                }
                return { ...current, state: nextState };
              });
            }

            return result;
          },
        ]),
      );
    }, [viewDef, viewId]);

    return (
      <Page>
        <PageBody className="!p-0">
          <CollectionViewProvider
            view={viewDef}
            viewId={viewId}
            viewName={viewId}
            renderResult={renderResult}
            isLoading={false}
            error={null}
            onExecuteAction={async (actionId, params) => {
              const response = await aos.client.view.executeAction.mutate({
                params: { view: viewId, actionId },
                body: { params },
              });
              return (response.data?.result ?? {
                success: false,
                error: "No result",
              }) as {
                success: boolean;
                data?: unknown;
                error?: string;
                updates?: Record<string, unknown>;
              };
            }}
          >
            <div className="flex h-full min-h-0 w-full flex-1 flex-col">
              <ViewRenderer spec={spec} handlers={handlers} />
            </div>
          </CollectionViewProvider>
        </PageBody>
      </Page>
    );
  })
  .build();

/**
 * Applies a JSON Pointer path into a mutable state object (RFC 6901 subset).
 */
function applyJsonPointer(
  target: Record<string, unknown>,
  pointer: string,
  value: unknown,
): void {
  const path = pointer.startsWith("/") ? pointer.slice(1) : pointer;
  const parts = path.split("/").map((part) => part.replace(/~1/g, "/").replace(/~0/g, "~"));
  if (parts.length === 0 || parts[0] === "") {
    return;
  }

  let cursor: Record<string, unknown> = target;
  for (let i = 0; i < parts.length - 1; i++) {
    const key = parts[i]!;
    const next = cursor[key];
    if (typeof next !== "object" || next === null || Array.isArray(next)) {
      cursor[key] = {};
    }
    cursor = cursor[key] as Record<string, unknown>;
  }

  cursor[parts[parts.length - 1]!] = value;
}
