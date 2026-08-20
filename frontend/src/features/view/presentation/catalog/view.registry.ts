import { defineRegistry } from "@json-render/react";
import { viewCatalog } from "./view.catalog";
import { viewComponents } from "../components/registry";

/**
 * AOS View React registry — maps catalog components to AOS @app implementations.
 *
 * CAST: @json-render/react expects an actions property in DefineRegistryOptions,
 * but the view feature is dormant (no Go backend). Empty actions object satisfies
 * the type. Revisit when view domain is implemented.
 */
export const { registry: viewRegistry } = defineRegistry(
  viewCatalog,
  {
    components: viewComponents as never,
    actions: {} as any,
  },
);
