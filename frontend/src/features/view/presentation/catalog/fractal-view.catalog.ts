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
 *
 * CAST: @json-render/core is built against zod 4's ZodType internals; this project
 * is on zod 3, so our ZodObject lacks properties it expects (def, type, toJSONSchema, etc.).
 * The view feature is dormant with no Go backend yet. Revisit when view domain exists.
 */
export const fractalViewCatalog = defineCatalog(schema, {
  components: fractalCatalogDefinitions as any,
  actions: {},
});

/**
 * Component type names available in the Fractal View catalog.
 */
export const FRACTAL_VIEW_COMPONENT_NAMES = FRACTAL_REGISTRY_COMPONENT_NAMES;
