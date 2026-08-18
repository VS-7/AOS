import { AnyRoute, createRootRoute } from "@tanstack/react-router";
import React from "react";
import { DefaultContext, AosAppConfig } from "./types";

/**
 * Builder for creating layout routes that wrap child routes.
 * 
 * @template TParentRoute - The parent route this layout belongs to.
 * @template TClient - The API client type.
 * @template TContext - The application context type.
 */
export class AosLayout<
  TParentRoute extends AnyRoute,
  TClient = any,
  TContext = DefaultContext,
  TStores = any
> {
  private _component?: any;

  constructor(
    public parentRoute: TParentRoute,
    public config: AosAppConfig<TClient, TContext, TStores>
  ) { }

  /**
   * Static factory for creating an AosLayout builder.
   */
  static create<TParentRoute extends AnyRoute, TClient = any, TContext = DefaultContext, TStores = any>(
    parentRoute: TParentRoute,
    config: AosAppConfig<TClient, TContext, TStores>
  ) {
    return new AosLayout<TParentRoute, TClient, TContext, TStores>(
      parentRoute,
      config
    );
  }

  /**
   * Defines the React component for this layout.
   * @param component - The layout component.
   * @returns The builder instance.
   */
  withComponent(component: React.ComponentType<any>) {
    this._component = component;
    return this;
  }

  /**
   * Finalizes the layout and returns a root route factory.
   */
  build() {
    return () => createRootRoute({
      component: this._component,
    });
  }
}
