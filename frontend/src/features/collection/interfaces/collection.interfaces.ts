import { Schema } from "@/core/helpers/schema.helper";
import type { ResponseWithCTA } from "@/core/interfaces/response.interfaces";
import { z } from "zod";

/**
 * Era `IIgniterCollectionModel`. Domínio dormente — contrato mínimo.
 * Genérico porque um call-site abaixo usa `IIgniterCollectionModel<Record<string, any>>`.
 */
export interface IIgniterCollectionModel<TFields = Record<string, unknown>> {
  name: string;
  fields?: TFields;
  [key: string]: unknown;
}

/**
 * Represents a hook callback for collection lifecycle events.
 * The function receives the record value and must return the value (possibly transformed).
 *
 * @example
 * ```typescript
 * // Simple passthrough
 * async ({ value }) => value
 *
 * // With transformation
 * async ({ value, fractal }) => {
 *   const workspace = fractal.workspaces.resolve();
 *   await workspace.customCollections.create({ ... });
 *   return value;
 * }
 * ```
 */
export type FractalCollectionHookCallback = (
  ctx: {
    value: Record<string, unknown>;
    previousValue?: Record<string, unknown>;
    fractal: unknown;
    workspace: unknown;
  }
) => Promise<Record<string, unknown> | boolean>;

/**
 * Optional hooks configuration for a custom collection.
 * Each hook is called at a specific point in the collection's lifecycle.
 *
 * @example
 * ```typescript
 * const hooks = {
 *   onCreated: async ({ value }) => {
 *     console.log('Record created:', value);
 *     return value;
 *   },
 *   onUpdated: async ({ value, previousValue }) => {
 *     console.log('Record updated from:', previousValue, 'to:', value);
 *     return value;
 *   }
 * };
 * ```
 */
export type FractalCollectionHooks = {
  /** Called after a record is created. Return the value to proceed or throw to reject. */
  onCreated?: FractalCollectionHookCallback;
  /** Called after a record is updated. Receives both new and previous value. */
  onUpdated?: FractalCollectionHookCallback;
  /** Called after a record is deleted. Return true to proceed or throw to reject. */
  onDeleted?: FractalCollectionHookCallback;
  /** Called when listing records. Can filter or transform the result set. */
  onList?: (ctx: { value: Record<string, unknown>[]; query?: Record<string, unknown>; fractal: unknown; workspace: unknown }) => Promise<Record<string, unknown>[]>;
  /** Called when reading a single record. Can transform or enrich the record. */
  onRead?: FractalCollectionHookCallback;
};

export const FractalCustomCollectionScopeSchema = z.enum([
  "workspace",
  "skill",
]);

export const FractalCustomCollectionFormatSchema = z.enum(["json", "md"]);

export const FractalCustomCollectionSchema = z.object({
  name: z
    .string()
    .describe("Collection name."),
  scope: FractalCustomCollectionScopeSchema.describe(
    "Whether the collection belongs to the workspace or a skill.",
  ),
  skill: z
    .string()
    .optional()
    .describe("Skill identifier when the collection is owned by a skill."),
  format: FractalCustomCollectionFormatSchema.describe(
    "Primary storage format for records.",
  ),
  path: z
    .string()
    .describe("Relative path to the collection root inside the workspace."),
  schema: z.any().describe("Relative path to the collection schema file."),
  patterns: z.array(z.string()).describe('')
});

export const FractalCustomCollectionCreateSchema = Schema.object({
  name: z
    .string()
    .regex(/^[a-z0-9][a-z0-9-_]*$/)
    .describe(
      "Unique collection identifier. Use lowercase letters, numbers, hyphens, or underscores.",
    ),
  skill: z.string().optional().describe("Determine if collection is part of a skill."),
  schema: z.any().describe("Valid JSON Schema defining the record structure."),
  format: FractalCustomCollectionFormatSchema.default("json").describe("Primary storage format for records data."),
  hooks: z
    .object({
      onCreated: z
        .string()
        .optional()
        .describe("Callback executed after a record is created. Receives `{ value }` and must return the value."),
      onUpdated: z
        .string()
        .optional()
        .describe("Callback executed after a record is updated. Receives `{ value, previousValue }` and must return the value."),
      onDeleted: z
        .string()
        .optional()
        .describe("Callback executed after a record is deleted. Receives `{ value }` and must return `true` or throw."),
      onList: z
        .string()
        .optional()
        .describe("Callback executed when listing records. Receives `{ value, query }` and must return filtered array."),
      onRead: z
        .string()
        .optional()
        .describe("Callback executed when reading a record. Receives `{ value }` and must return the value."),
    })
    .optional()
    .describe("Optional lifecycle hooks for the collection."),
});

export const FractalCustomCollectionQuerySchema = Schema.object({
  scope: FractalCustomCollectionScopeSchema.optional().describe(
    "Filter collections by scope.",
  ),
  skill: z
    .string()
    .optional()
    .describe("Filter collections belonging to a specific skill."),
  query: z
    .string()
    .optional()
    .describe("Text search across collection metadata."),
});

export const FractalCollectionRecordListQuerySchema = Schema.object({
  where: z
    .any()
    .optional()
    .describe("Generic where clause forwarded to Igniter collections."),
  orderBy: z
    .any()
    .optional()
    .describe("Generic orderBy clause forwarded to Igniter collections."),
  take: z.number().optional().describe("Maximum number of records to return."),
  skip: z.number().optional().describe("Number of records to skip."),
});

export const FractalCollectionRecordCreateSchema = Schema.object({
  id: z.string().optional().describe("Optional custom record identifier."),
  data: z
    .any()
    .optional()
    .describe("Structured frontmatter/data payload for the record."),
  content: z
    .string()
    .optional()
    .describe("Optional markdown body content for .md collections."),
});

export const FractalCollectionRecordUpdateSchema = Schema.object({
  data: z
    .any()
    .optional()
    .describe("Partial data payload to merge into the record."),
  content: z
    .string()
    .optional()
    .describe("Optional markdown body content for .md collections."),
});

export type FractalCustomCollectionSchemaFile = {
  collectionName: string;
  patterns: string[];
  template?: string;
  schema?: Record<string, unknown>;
  hooks?: FractalCollectionHooks;
};

export type FractalCollectionPath = {
  relative: string;
  absolute: string;
};

export type IgniterCollectionDefinition = {
  name: string;
  patterns: string[];
  schema?: Record<string, unknown>;
};

export type FractalCustomCollection = z.infer<
  typeof FractalCustomCollectionSchema
>;
export type FractalCustomCollectionCreateInput = z.infer<
  typeof FractalCustomCollectionCreateSchema
>;
export type FractalCustomCollectionQueryInput = z.infer<
  typeof FractalCustomCollectionQuerySchema
>;
export type FractalCollectionRecordListQueryInput = z.infer<
  typeof FractalCollectionRecordListQuerySchema
>;
export type FractalCollectionRecordCreateInput = z.infer<
  typeof FractalCollectionRecordCreateSchema
>;
export type FractalCollectionRecordUpdateInput = z.infer<
  typeof FractalCollectionRecordUpdateSchema
>;

export interface IFractalCollectionService {
  list(
    query?: FractalCustomCollectionQueryInput,
  ): Promise<ResponseWithCTA<{ collections: FractalCustomCollection[] }>>;
  get(
    id: string,
  ): Promise<
    ResponseWithCTA<{
      collection: IIgniterCollectionModel<Record<string, any>>;
      definition: FractalCustomCollection | null;
    }>
  >;
  create(
    data: FractalCustomCollectionCreateInput,
  ): Promise<ResponseWithCTA<{ collection: FractalCustomCollection | null }>>;
  delete(
    id: string,
  ): Promise<ResponseWithCTA<{ collection: FractalCustomCollection }>>;
}
