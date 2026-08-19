import { z } from "zod";
import type { ResponseWithCTA } from "@/core/interfaces/response.interfaces";
import { RuleSchema, RuleTypeSchema } from "@/core/interfaces/rule.interfaces";
import { Schema } from "@/core/helpers/schema.helper";

/**
 * @const TemplateSchema
 * @description Zod schema for a Fractal Template record.
 * Maps to the .fractal/templates/*.template.md structure.
 */
export const TemplateSchema = z.object({
  /**
   * @description Unique identifier for the template (slug/filename). eg. 'plan'
   */
  id: z.string().describe("Unique identifier for the template (slug/filename). eg. 'plan'. This is auto generated."),
  /**
   * @description Unique identifier for the template (slug/filename). eg. 'plan'
   */
  name: z.string().describe("Unique identifier for the template (slug/filename). eg. 'plan'"),
  /**
   * @description Optional skill ID if this template is a skill template.
   */
  skill: z.string().optional().describe("Optional skill ID if this template is a skill template. eg. 'agent'. If not set, the template is a global template on .fractal/templates/."),
  /**
   * @description Detailed description of the template's purpose.
   */
  description: z.string().describe("A comprehensive description of what this template produces with instructions with when and how to use it. eg. 'A Spec Plan template for proposal structured changes in a project.'"),
  /**
   * @description Optional default output path or pattern.
   */
  output: z.string().optional().describe("Default output relative path when rendering without passing a path explicitly. You can use Liquid syntax here. eg. '.fractal/artifacts/plans/{{name}}.plan.md'"),
  /**
   * @description Optional JSON Schema for the template variables.
   */
  schema: z.any().optional().describe("JSON Schema (as a stringified JSON) to enforce validation on the input data before rendering. eg. '{\"type\":\"object\",\"properties\":{\"name\":{\"type\":\"string\"}}}'"),
  /**
   * @description Optional array of rules specifying when and how agents should use this template.
   */
  rules: z.array(RuleSchema).optional().describe("Optional array of rules specifying when and how agents should use this template. eg. [{\"type\":\"always\",\"instruction\":\"Always use this template to propose changes on project\"}]"),
  /**
   * @description The raw Liquid/template content.
   */
  content: z.string().optional().describe("The raw Liquid/template content follows syntax based on your output file type (markdown, json, yaml, ts, etc). eg. '## {{name}}...'"),
});

/**
 * @typedef Template
 * @description TypeScript type inferred from TemplateSchema.
 */
export type Template = z.infer<typeof TemplateSchema>;

/**
 * @const CreateTemplateSchema
 * @description Schema for creating a new template.
 */
export const CreateTemplateSchema = TemplateSchema.omit({ id: true, rules: true });

/**
 * @typedef CreateTemplateInput
 * @description Input for creating a template.
 */
export type CreateTemplateInput = z.infer<typeof CreateTemplateSchema>;

/**
 * @const UpdateTemplateSchema
 * @description Schema for updating an existing template.
 */
export const UpdateTemplateSchema = TemplateSchema.omit({ id: true, rules: true, name: true }).partial();

/**
 * @typedef UpdateTemplateInput
 * @description Input for updating a template.
 */
export type UpdateTemplateInput = z.infer<typeof UpdateTemplateSchema>;

/**
 * @const RenderTemplateSchema
 * @description Schema for rendering a template.
 */
export const RenderTemplateSchema = z.object({
  /**
   * @description The ID of the template to render.
   */
  template: z.string().describe("The exact ID of registered template. eg. plan."),
  /**
   * @description Optional output path for the rendered content.
   */
  output: z.string().optional().describe("Absolute or relative filesystem path to write the compiled content. If omitted, the rendered file will be created on default output path and if not defined on template, we only return the template on console. eg. './src/features/auth/auth.ts'"),
  /**
   * @description Data to populate the template variables.
   */
  data: z.any().optional().describe("Key-value mapping containing variables to populate the template. Pass structured JSON to this field. eg. '{\"name\":\"Auth\",\"domain\":\"src/features/auth\"}'"),
});

/**
 * @typedef RenderTemplateInput
 * @description Input for rendering a template.
 */
export type RenderTemplateInput = z.infer<typeof RenderTemplateSchema>;

/**
 * @const TemplateQueryInputSchema
 * @description Schema for querying and filtering the templates list.
 */
export const TemplateQueryInputSchema = Schema.object({
  /**
   * @description Filter templates by a specific skill ID.
   */
  skill: z.string().optional().describe("Filter the templates catalog by a specific parent skill ID. eg. 'browser'"),
  /**
   * @description Text search query to match against template content, name, or description.
   */
  query: z.string().optional().describe("Perform a text search against the template content, name, or description. eg. 'React Component'"),
  /**
   * @description Filter templates by the presence of a specific rule type (e.g., 'always', 'workflow').
   */
  byRule: RuleTypeSchema.optional().describe("Filter templates that enforce a specific rule type. eg. 'always', 'workflow', or 'ask'")
});

/**
 * @typedef TemplateQueryInput
 * @description Input for querying templates.
 */
export type TemplateQueryInput = z.infer<typeof TemplateQueryInputSchema>;

/**
 * @interface ITemplateService
 * @description Defines the contract for the TemplateService managing templates and rendering.
 */
export interface ITemplateService {
  /**
   * Retrieves a list of templates, excluding schema and content fields.
   */
  list(query?: TemplateQueryInput): Promise<ResponseWithCTA<{ templates: Omit<Template, "schema" | "content">[] }>>;

  /**
   * Retrieves a single template by its ID.
   * @param id The template identifier.
   */
  getById(id: string): Promise<ResponseWithCTA<Template>>;

  /**
   * Renders a template with the given input data.
   * @param input The render parameters.
   */
  render(input: RenderTemplateInput): Promise<ResponseWithCTA<{
    content: string;
    output: string | null;
    filename: string | null;
    extension: string | null;
  }>>;

  /**
   * Creates a new template.
   * @param data The template data.
   */
  create(data: CreateTemplateInput): Promise<ResponseWithCTA<Template>>;

  /**
   * Updates an existing template.
   * @param id The template ID.
   * @param data The partial data to update.
   */
  update(id: string, data: UpdateTemplateInput): Promise<ResponseWithCTA<Template>>;

  /**
   * Removes a template.
   * @param id The template ID.
   */
  delete(id: string): Promise<ResponseWithCTA>;
}
