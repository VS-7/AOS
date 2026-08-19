import { defineRegistry } from "@json-render/react";
import { fractalViewCatalog } from "./fractal-view.catalog";
import { fractalViewComponents } from "../components/registry";

/**
 * Fractal View React registry — maps catalog components to Fractal @app implementations.
 */
export const { registry: fractalViewRegistry } = defineRegistry(
  fractalViewCatalog,
  {
    components: fractalViewComponents as never,
  },
);
