import { AnyRoute, createRouter, type ErrorRouteComponent } from "@tanstack/react-router";
import { AosAppBuilt } from "./types";

interface AosRouterBuildOptions {
  /**
   * Default error component rendered when a route loader or route component
   * throws an uncaught error. Receives TanStack Router's `ErrorComponentProps`.
   *
   * Wired to TanStack Router's `defaultErrorComponent` and provides feature-level
   * crash isolation so one broken route cannot affect the rest of the shell.
   */
  defaultErrorComponent?: ErrorRouteComponent;
}

/**
 * High-level router builder that orchestrates the route tree and TanStack Router instantiation.
 */
export class AosRouter<
  TRootRoute extends AnyRoute,
  TRoutes extends AnyRoute[] = []
> {
  private _routes: TRoutes;

  constructor(private _app: AosAppBuilt<any, any, any, TRootRoute>, routes?: TRoutes) {
    this._routes = routes || ([] as unknown as TRoutes);
  }

  /**
   * Static factory for creating an AosRouter.
   * @param app - The built application instance.
   */
  static create<TClient, TContext, TStores, TRootRoute extends AnyRoute>(
    app: AosAppBuilt<TClient, TContext, TStores, TRootRoute>
  ) {
    return new AosRouter<TRootRoute, []>(app, []);
  }

  /**
   * Adds a built route to the route tree.
   * @param route - The route instance created via {@link AosPage}.
   */
  addRoute<TNewRoute extends AnyRoute>(route: TNewRoute): AosRouter<TRootRoute, [...TRoutes, TNewRoute]> {
    this._routes.push(route as any);
    return this as unknown as AosRouter<TRootRoute, [...TRoutes, TNewRoute]>;
  }

  /**
   * Finalizes the route tree and returns a TanStack Router instance.
   * @param options - Optional build-time configuration, including a default error component.
   * @returns The configured {@link Router}.
   */
  build(options: AosRouterBuildOptions = {}) {
    // [Business Rule]: Combine all children into the root route.
    const routeTree = this._app.rootRoute.addChildren(this._routes);

    // [Return]: Create the final TanStack Router instance.
    const router = createRouter({
      routeTree,
      defaultPreload: this._app.config.defaultPreload || "intent",
      context: {
        client: this._app.config.client,
        context: {} as any, // Context will be dynamically populated via contextFn in beforeLoad if needed
      },
      ...(options.defaultErrorComponent
        ? { defaultErrorComponent: options.defaultErrorComponent }
        : {}),
    });

    return router;
  }
}
