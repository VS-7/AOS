import { defineCatalog } from "@json-render/core";
import { schema } from "@json-render/react/schema";
import {
  AOS_REGISTRY_COMPONENT_NAMES,
  catalogDefinitions,
} from "../components/registry/definitions/catalog.definitions";

/**
 * AOS View catalog — AOS @app components + extended shadcn definitions.
 *
 * Used by `aos views components` (`catalog.prompt()`), scaffold validation,
 * and the React registry. State actions (`setState`, etc.) are built into the
 * React schema and do not need to be declared here.
 *
 * CAST: @json-render/core is built against zod 4's ZodType internals; this project
 * is on zod 3, so our ZodObject lacks properties it expects (def, type, toJSONSchema, etc.).
 * The view feature is dormant with no Go backend yet. Revisit when view domain exists.
 */
export const viewCatalog = defineCatalog(schema, {
  components: catalogDefinitions as any,
  actions: {},
});

/**
 * Component type names available in the AOS View catalog.
 */
export const AOS_VIEW_COMPONENT_NAMES = AOS_REGISTRY_COMPONENT_NAMES;
