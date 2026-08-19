import { defineCatalog } from "@json-render/core";
import { schema } from "@json-render/react/schema";
import {
  FRACTAL_REGISTRY_COMPONENT_NAMES,
  fractalCatalogDefinitions,
} from "../components/registry/definitions/fractal-catalog.definitions";

/**
 * Fractal View catalog — Fractal @app components + extended shadcn definitions.
 *
 * Used by `fractal views components` (`catalog.prompt()`), scaffold validation,
 * and the React registry. State actions (`setState`, etc.) are built into the
 * React schema and do not need to be declared here.
 */
export const fractalViewCatalog = defineCatalog(schema, {
  components: fractalCatalogDefinitions,
  actions: {},
});

/**
 * Component type names available in the Fractal View catalog.
 */
export const FRACTAL_VIEW_COMPONENT_NAMES = FRACTAL_REGISTRY_COMPONENT_NAMES;
