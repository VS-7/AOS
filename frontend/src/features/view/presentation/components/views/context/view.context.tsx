import * as React from "react";
import {
  ViewDataHelper,
  type ViewDefinition,
  type ViewRenderResult,
  type ViewActionResult,
} from "../../../helpers";

export type CollectionViewContextValue = {
  view: ViewDefinition | null;
  viewId: string;
  viewName: string;
  renderResult: ViewRenderResult | null;
  isLoading: boolean;
  error: Error | null;
  actions: ViewActionsContext;
};

export type ViewActionsContext = {
  execute: (
    actionId: string,
    params?: Record<string, unknown>,
  ) => Promise<ViewActionResult>;
  isExecuting: boolean;
  executingAction: string | null;
  error: Error | null;
  clearError: () => void;
};

const CollectionViewContext =
  React.createContext<CollectionViewContextValue | null>(null);

export function useCollectionViewContext(): CollectionViewContextValue {
  const context = React.useContext(CollectionViewContext);

  if (!context) {
    throw new Error(
      "useCollectionViewContext must be used within CollectionViewProvider",
    );
  }

  return context;
}

export type CollectionViewProviderProps = {
  children: React.ReactNode;
  view: ViewDefinition | null;
  viewId: string;
  viewName: string;
  renderResult: ViewRenderResult | null;
  isLoading: boolean;
  error: Error | null;
  onExecuteAction?: (
    actionId: string,
    params?: Record<string, unknown>,
  ) => Promise<ViewActionResult>;
};

export function CollectionViewProvider({
  children,
  view,
  viewId,
  viewName,
  renderResult,
  isLoading,
  error,
  onExecuteAction,
}: CollectionViewProviderProps) {
  const [isExecuting, setIsExecuting] = React.useState(false);
  const [executingAction, setExecutingAction] = React.useState<string | null>(
    null,
  );
  const [actionError, setActionError] = React.useState<Error | null>(null);

  const actions = React.useMemo<ViewActionsContext>(
    () => ({
      execute: async (actionId: string, params?: Record<string, unknown>) => {
        if (!onExecuteAction) {
          return { success: false, error: "Action handler not configured" };
        }

        const action = ViewDataHelper.getAction(view, actionId);
        if (!action) {
          return { success: false, error: `Action "${actionId}" not found` };
        }

        setIsExecuting(true);
        setExecutingAction(actionId);
        setActionError(null);

        try {
          return await onExecuteAction(actionId, params);
        } catch (err) {
          const nextError = err instanceof Error ? err : new Error(String(err));
          setActionError(nextError);
          return { success: false, error: nextError.message };
        } finally {
          setIsExecuting(false);
          setExecutingAction(null);
        }
      },
      isExecuting,
      executingAction,
      error: actionError,
      clearError: () => setActionError(null),
    }),
    [view, onExecuteAction, isExecuting, executingAction, actionError],
  );

  const value = React.useMemo<CollectionViewContextValue>(
    () => ({
      view,
      viewId,
      viewName,
      renderResult,
      isLoading,
      error,
      actions,
    }),
    [view, viewId, viewName, renderResult, isLoading, error, actions],
  );

  return (
    <CollectionViewContext.Provider value={value}>
      {children}
    </CollectionViewContext.Provider>
  );
}

export function useViewActions(): ViewActionsContext {
  const { actions } = useCollectionViewContext();
  return actions;
}

export function useViewAction(actionId: string) {
  const { view, actions } = useCollectionViewContext();
  const action = ViewDataHelper.getAction(view, actionId);
  const isExecuting = actions.executingAction === actionId;

  return {
    action,
    execute: (params?: Record<string, unknown>) =>
      actions.execute(actionId, params),
    isExecuting,
    hasAction: !!action,
  };
}

export type { ViewDefinition, ViewRenderResult };
