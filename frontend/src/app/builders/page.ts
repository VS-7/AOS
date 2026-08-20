import {
  createRoute,
  AnyRoute,
  useRouter,
  Route,
} from "@tanstack/react-router";
import { z } from "zod";
import React from "react";
import { AosResponse } from "./response";
import { DefaultContext, AosAppConfig, RouteContextAPI } from "./types";

/**
 * Builder for creating type-safe pages and routes within the Aos ecosystem.
 * 
 * @template TPath - The path string of the page.
 * @template TParentRoute - The parent route instance.
 * @template TClient - The API client type.
 * @template TContext - The application context type.
 * @template TQuerySchema - Zod schema for search parameters.
 * @template TLoaderData - The type of data returned by the page loader.
 */
export class AosPage<
  TPath extends string,
  TParentRoute extends AnyRoute,
  TClient = any,
  TContext = DefaultContext,
  TStores = any,
  TQuerySchema extends z.ZodTypeAny = z.ZodTypeAny,
  TLoaderData = any
> {
  private _metadata?: { title: string; description: string };
  private _path: TPath;
  private _layoutRoute: TParentRoute;
  private _use: any[] = [];
  private _query?: TQuerySchema;
  private _loader?: any;
  private _component?: any;

  constructor(
    path: TPath,
    public parentRoute: TParentRoute,
    public config: AosAppConfig<TClient, TContext, TStores>
  ) {
    this._path = path;
    this._layoutRoute = parentRoute;
  }

  /**
   * Sets the page metadata (SEO/Title).
   */
  withMetadata(metadata: { title: string; description: string }) {
    this._metadata = metadata;
    return this;
  }

  /**
   * Retrieves the page metadata.
   */
  getMetadata() {
    return this._metadata;
  }

  /**
   * Overrides the parent route with a layout route.
   * @param layoutRoute - The layout route instance.
   */
  withLayout<TParent extends AnyRoute>(layoutRoute: TParent) {
    const builder = new AosPage<TPath, TParent, TClient, TContext, TStores, TQuerySchema, TLoaderData>(
      this._path,
      layoutRoute,
      this.config
    );
    builder._metadata = this._metadata;
    builder._path = this._path;
    builder._use = this._use;
    builder._query = this._query;
    builder._loader = this._loader;
    return builder;
  }

  /**
   * Configures search parameter validation using Zod.
   * @param schema - The Zod schema for validation.
   */
  withQuery<TSchema extends z.ZodTypeAny>(schema: TSchema) {
    const builder = new AosPage<TPath, TParentRoute, TClient, TContext, TStores, TSchema, TLoaderData>(
      this._path,
      this.parentRoute,
      this.config
    );
    builder._metadata = this._metadata;
    builder._path = this._path;
    builder._layoutRoute = this._layoutRoute;
    builder._use = this._use;
    builder._query = schema;
    builder._loader = this._loader;
    return builder;
  }

  /**
   * Injects a procedure into the route lifecycle.
   * Procedures execute during `beforeLoad`.
   */
  use<TNewContext>(procedure: any) {
    // [Business Rule]: Merge current context with the context produced by the procedure.
    const builder = new AosPage<TPath, TParentRoute, TClient, TContext & TNewContext, TStores, TQuerySchema, TLoaderData>(
      this._path,
      this.parentRoute,
      this.config as any
    );
    builder._metadata = this._metadata;
    builder._layoutRoute = this._layoutRoute;
    builder._use = [...this._use, procedure];
    builder._query = this._query;
    builder._loader = this._loader;
    return builder;
  }

  /**
   * Defines a data loader for the page.
   * @param loader - Function to fetch data for the route.
   */
  withLoader<TData>(
    loader: (args: {
      context: TContext;
      client: TClient;
      request: { url: string; query: z.infer<TQuerySchema>; params: Record<string, string> };
      response: AosResponse;
      stores: TStores;
    }) => Promise<TData> | TData
  ) {
    const builder = new AosPage<TPath, TParentRoute, TClient, TContext, TStores, TQuerySchema, TData>(
      this._path,
      this.parentRoute,
      this.config
    );
    builder._metadata = this._metadata;
    builder._path = this._path;
    builder._layoutRoute = this._layoutRoute;
    builder._use = this._use;
    builder._query = this._query;
    builder._loader = loader;
    return builder;
  }

  /**
   * Sets the React component for the page.
   * @param component - The page component.
   */
  withComponent(
    component: (args: {
      route: RouteContextAPI<TLoaderData, any, z.infer<TQuerySchema>>;
      client: TClient;
    }) => React.ReactNode
  ) {
    this._component = component;
    return this;
  }

  /**
   * Finalizes the page configuration and returns a TanStack Route.
   * @returns The configured {@link Route}.
   */
  build() {
    // [Business Rule]: Internal component wrapper to provide route-specific context and helpers.
    const InternalComponent = () => {
      const router = useRouter();

      return this._component({
        route: route as any,
        client: this.config.client,
      });
    };

    const route = createRoute({
      path: this._path,
      getParentRoute: () => this._layoutRoute,
      //validateSearch: this._query,
      beforeLoad: async (ctx: any) => {
        // [Data Extraction]: Extract context from the root provider.
        let currentContext = { ...ctx.context.context };

        const req = {
          url: ctx.location.href,
          query: ctx.location.search,
          params: ctx.params,
        };

        const res = new AosResponse();

        // [Global Middleware]: Execute the application-wide beforePageLoad handler if defined.
        if (this.config.beforePageLoad) {
          await this.config.beforePageLoad({
            client: this.config.client as TClient,
            context: currentContext,
            stores: this.config.stores as TStores,
            request: req,
            response: res,
            page: this,
          });
        }

        // [Loop]: Execute all registered procedures sequentially.
        for (const proc of this._use) {
          if (proc.loader) {
            const result = await proc.loader({
              context: currentContext,
              client: this.config.client,
              options: proc.options,
              request: req,
              response: res,
              stores: this.config.stores,
            });
            if (result) {
              currentContext = { ...currentContext, ...result };
            }
          }
        }
        return { context: currentContext };
      },
      loader: async (ctx: any) => {
        const req = {
          url: ctx.location.href,
          query: ctx.location.search,
          params: ctx.params,
        };

        const res = new AosResponse();

        // [Global Middleware]: Execute the application-wide onPageLoad handler if defined.
        if (this.config.onPageLoad) {
          await this.config.onPageLoad({
            client: this.config.client!,
            context: ctx.context.context,
            stores: this.config.stores!,
            request: req,
            response: res,
            page: this,
          });
        }

        // [Condition]: Skip if no custom loader is defined.
        if (!this._loader) return {};

        // [Return]: Execute custom loader logic.
        return this._loader({
          context: ctx.context.context,
          client: this.config.client,
          request: req,
          response: res,
          stores: this.config.stores,
        });
      },
      component: InternalComponent,
    });

    return route;
  }
}
