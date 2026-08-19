import { Schema } from "@/core/helpers/schema.helper";
import type { ResponseWithCTA } from "@/core/interfaces/response.interfaces";
import { z } from "zod";

/** Era `IgniterCollectionViewDefinition`. Domínio dormente — contrato mínimo. */
export interface IgniterCollectionViewDefinition {
  name: string;
  tree?: unknown;
  [key: string]: unknown;
}

/**
 * Local stand-in for `./presentation/components/views/registry/metadata`,
 * which has not been ported yet (only interfaces/@core are in scope for
 * this recovery). Shape mirrors the source registry exactly so that when
 * the real presentation component lands, this declaration can simply be
 * deleted and replaced by an import with no shape changes.
 */
export type ViewComponentPropMetadata = {
  name: string;
  type: string;
  required?: boolean;
  description?: string;
};

export type ViewComponentCategory =
  | "layout"
  | "data-display"
  | "form"
  | "media"
  | "feedback"
  | "action";

export type ViewComponentMetadata = {
  name: string;
  component: string;
  category: ViewComponentCategory;
  description: string;
  props: ViewComponentPropMetadata[];
};

/**
 * Scope of the view - workspace level or skill-level
 */
export const FractalViewScopeSchema = z.enum(["workspace", "skill"]);
export type FractalViewScope = z.infer<typeof FractalViewScopeSchema>;

export const FractalViewTemplateSchema = z.enum(["basic", "table", "dashboard"]);
export type FractalViewTemplate = z.infer<typeof FractalViewTemplateSchema>;

/**
 * Schema for creating a view
 */
export const FractalViewCreateSchema = Schema.object({
  name: z
    .string()
    .regex(/^[a-z0-9][a-z0-9-_]*$/)
    .describe(
      "Unique view identifier. Use lowercase letters, numbers, hyphens, or underscores.",
    ),
  title: z.string().optional().describe("Human-readable title for UI display. Defaults to a titleized version of the view name."),
  description: z
    .string()
    .optional()
    .describe("Optional description explaining the view purpose."),
  collection: z
    .string()
    .optional()
    .describe("Optional collection identifier this view should read from by default."),
  template: FractalViewTemplateSchema.default("basic").describe(
    "Scaffold template to generate for the view.",
  ),
  scope: FractalViewScopeSchema.default("workspace").describe(
    "Whether the view belongs to the workspace or a skill.",
  ),
  skill: z
    .string()
    .optional()
    .describe("Skill identifier when the view is owned by a skill."),
  getData: z
    .string()
    .optional()
    .describe("Path to a getData hook file (.ts) or inline function."),
  tree: z
    .array(z.any())
    .optional()
    .describe("UI component tree (json-render compatible)."),
  actions: z
    .any()
    .optional()
    .describe("View actions with handlers and params."),
});

export type FractalViewCreateInput = z.input<typeof FractalViewCreateSchema>;

/**
 * Schema for querying views
 */
export const FractalViewQuerySchema = Schema.object({
  scope: FractalViewScopeSchema.optional().describe("Filter views by scope."),
  skill: z
    .string()
    .optional()
    .describe("Filter views belonging to a specific skill."),
  query: z.string().optional().describe("Text search across view metadata."),
});

export type FractalViewQueryInput = z.infer<typeof FractalViewQuerySchema>;

/**
 * Schema for rendering a view with optional query overrides
 */
export const FractalViewRenderSchema = Schema.object({
  where: z
    .any()
    .optional()
    .describe("Optional where override for view rendering."),
  orderBy: z
    .record(z.string(), z.enum(["asc", "desc"]))
    .optional()
    .describe("Optional orderBy override for view rendering."),
  take: z
    .number()
    .optional()
    .describe("Maximum number of items to return when rendering the view."),
  skip: z
    .number()
    .optional()
    .describe("Number of items to skip when rendering the view."),
});

export type FractalViewRenderInput = z.infer<typeof FractalViewRenderSchema>;

/**
 * Schema for executing a view action
 */
export const FractalViewActionSchema = Schema.object({
  params: z
    .any()
    .optional()
    .describe("Parameters forwarded to the action handler."),
});

export type FractalViewActionInput = z.infer<typeof FractalViewActionSchema>;

/**
 * View summary (lightweight representation for listing)
 */
export type FractalViewSummary = {
  id: string;
  name: string;
  title: string;
  description?: string;
  collection?: string;
  scope: FractalViewScope;
  source?: "built-in" | "discovered" | undefined;
  skill?: string;
  metadata?: Record<string, any> | undefined
};

/**
 * View location paths
 */
export type FractalViewLocation = {
  root: string;
  definition: string;
  hooks: string;
};

/**
 * Full view definition with paths
 */
export type FractalView = FractalViewSummary & {
  getData?: string;
  tree: IgniterCollectionViewDefinition["tree"];
  actions?: IgniterCollectionViewDefinition["actions"];
  path: string;
};

/**
 * Interface for the view service
 */
export interface IFractalViewService {
  list(
    query?: FractalViewQueryInput,
  ): Promise<ResponseWithCTA<{ views: FractalViewSummary[] }>>;
  get(
    id: string,
  ): Promise<
    ResponseWithCTA<{
      view: IgniterCollectionViewDefinition | null;
      definition: FractalView | null;
    }>
  >;
  create(
    data: FractalViewCreateInput,
  ): Promise<ResponseWithCTA<{ view: FractalView | null }>>;
  listComponents(): Promise<ResponseWithCTA<{
    components: ViewComponentMetadata[];
    groups: Record<ViewComponentCategory, ViewComponentMetadata[]>;
  }>>;
  delete(id: string): Promise<ResponseWithCTA<{ view: FractalView }>>;
  render(
    id: string,
    query?: FractalViewRenderInput,
  ): Promise<ResponseWithCTA<{ result: unknown }>>;
  listActions(id: string): Promise<ResponseWithCTA<{ actions: string[] }>>;
  executeAction(
    id: string,
    actionId: string,
    input?: FractalViewActionInput,
  ): Promise<ResponseWithCTA<{ result: unknown }>>;
}
