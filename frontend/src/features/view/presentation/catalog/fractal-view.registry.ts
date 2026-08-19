import { defineRegistry } from "@json-render/react";
import { fractalViewCatalog } from "./fractal-view.catalog";
import { fractalViewComponents } from "../components/registry";

/**
 * Fractal View React registry — maps catalog components to Fractal @app implementations.
 *
 * CAST: @json-render/react expects an actions property in DefineRegistryOptions,
 * but the view feature is dormant (no Go backend). Empty actions object satisfies
 * the type. Revisit when view domain is implemented.
 */
export const { registry: fractalViewRegistry } = defineRegistry(
  fractalViewCatalog,
  {
    components: fractalViewComponents as never,
    actions: {} as any,
  },
);
