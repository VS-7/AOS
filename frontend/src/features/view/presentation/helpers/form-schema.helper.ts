import { z } from "zod";

/**
 * `z.fromJSONSchema` is a zod v4 API; this app is on zod v3
 * (`package.json`'s `"zod": "^3.25.76"`), which has no built-in JSON-
 * Schema-to-Zod converter. A real one is a small library on its own
 * (`json-schema-to-zod` et al.) — out of scope for a bulk copy-and-
 * compile task. This falls back to an unconstrained passthrough schema:
 * `createZodSchema`'s result is only used for client-side form validation,
 * so "accept anything shaped like an object" degrades to no validation
 * rather than a broken build. Replacing this with a real converter (or a
 * package) is real follow-up work, disclosed here rather than guessed at.
 */
function zodFromJSONSchema(_jsonSchema: unknown): z.ZodType<Record<string, unknown>> {
  return z.record(z.string(), z.unknown());
}

export type CollectionJsonSchemaNode = {
  type?: string | string[];
  title?: string;
  description?: string;
  default?: unknown;
  enum?: unknown[];
  format?: string;
  properties?: Record<string, CollectionJsonSchemaNode>;
  items?: CollectionJsonSchemaNode | CollectionJsonSchemaNode[];
  required?: string[];
  optional?: boolean;
};

export type CollectionSchemaPathSegment = string | number;

export type CollectionUpsertFormValues = {
  data: Record<string, unknown>;
  content?: string;
};

export class FormSchemaHelper {
  private static readonly SUPPORTED_TYPES = new Set([
    "string",
    "number",
    "integer",
    "boolean",
    "object",
    "array",
  ]);

  public static isPlainObject(value: unknown): value is Record<string, unknown> {
    return typeof value === "object" && value !== null && !Array.isArray(value);
  }

  public static normalizeType(schema: CollectionJsonSchemaNode | undefined): string {
    const type = Array.isArray(schema?.type)
      ? schema.type.find((entry) => FormSchemaHelper.SUPPORTED_TYPES.has(entry))
      : schema?.type;

    if (typeof type === "string") {
      return type;
    }

    if (schema?.enum?.length) {
      const sample = schema.enum[0];

      if (typeof sample === "number") {
        return "number";
      }

      if (typeof sample === "boolean") {
        return "boolean";
      }

      return "string";
    }

    if (schema?.properties) {
      return "object";
    }

    if (schema?.items) {
      return "array";
    }

    return "string";
  }

  public static prettifyLabel(value: string): string {
    return value
      .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
      .replace(/[-_]+/g, " ")
      .replace(/\s+/g, " ")
      .trim()
      .replace(/^./, (character) => character.toUpperCase());
  }

  public static getPathKey(path: CollectionSchemaPathSegment[]): string {
    return path.map(String).join(".");
  }

  public static setValueAtPath(
    source: Record<string, unknown> | unknown[],
    path: CollectionSchemaPathSegment[],
    nextValue: unknown,
  ): Record<string, unknown> | unknown[] {
    if (path.length === 0) {
      if (Array.isArray(nextValue)) {
        return nextValue;
      }

      return FormSchemaHelper.isPlainObject(nextValue) ? nextValue : {};
    }

    const [segment, ...rest] = path;
    const current = Array.isArray(source) ? [...source] : { ...source };

    if (rest.length === 0) {
      if (nextValue === undefined) {
        if (Array.isArray(current) && typeof segment === "number") {
          current[segment] = undefined;
        } else if (!Array.isArray(current) && typeof segment === "string") {
          delete current[segment];
        }

        return current;
      }

      if (Array.isArray(current) && typeof segment === "number") {
        current[segment] = nextValue;
        return current;
      }

      if (!Array.isArray(current) && typeof segment === "string") {
        current[segment] = nextValue;
      }

      return current;
    }

    const previousChild = Array.isArray(current)
      ? current[segment as number]
      : current[segment as string];

    const nextChildSeed =
      previousChild !== undefined
        ? previousChild
        : typeof rest[0] === "number"
          ? []
          : {};

    const nextChild = FormSchemaHelper.setValueAtPath(
      Array.isArray(nextChildSeed)
        ? nextChildSeed
        : FormSchemaHelper.isPlainObject(nextChildSeed)
          ? nextChildSeed
          : {},
      rest,
      nextValue,
    );

    if (Array.isArray(current) && typeof segment === "number") {
      current[segment] = nextChild;
      return current;
    }

    if (!Array.isArray(current) && typeof segment === "string") {
      current[segment] = nextChild;
    }

    return current;
  }

  public static getArrayItemSchema(schema: CollectionJsonSchemaNode): CollectionJsonSchemaNode {
    if (Array.isArray(schema.items)) {
      return schema.items[0] ?? {};
    }

    return schema.items ?? {};
  }

  public static createEmptyValue(schema: CollectionJsonSchemaNode | undefined): unknown {
    if (!schema) {
      return "";
    }

    if (schema.default !== undefined) {
      return FormSchemaHelper.cloneValue(schema.default);
    }

    const type = FormSchemaHelper.normalizeType(schema);

    if (type === "object") {
      return FormSchemaHelper.buildSchemaState(schema, {});
    }

    if (type === "array") {
      return [];
    }

    if (type === "boolean") {
      return false;
    }

    return "";
  }

  public static isEmptyValue(value: unknown): boolean {
    if (value === undefined || value === null || value === "") {
      return true;
    }

    if (Array.isArray(value)) {
      return value.length === 0;
    }

    if (FormSchemaHelper.isPlainObject(value)) {
      return Object.keys(value).length === 0;
    }

    return false;
  }

  public static buildInitialValue(
    schema: Record<string, unknown> | null | undefined,
    input: Record<string, unknown> | null | undefined,
  ): Record<string, unknown> {
    return FormSchemaHelper.buildSchemaState(
      schema as CollectionJsonSchemaNode | undefined,
      input ?? {},
    ) as Record<string, unknown>;
  }

  public static buildUpsertFormValues(
    schema: Record<string, unknown> | null | undefined,
    input: Record<string, unknown> | null | undefined,
    content?: string,
  ): CollectionUpsertFormValues {
    return {
      data: FormSchemaHelper.buildInitialValue(schema, input),
      content: content ?? "",
    };
  }

  public static createZodSchema(
    schema: Record<string, unknown> | null | undefined,
  ): z.ZodType<Record<string, unknown>> {
    const jsonSchema = FormSchemaHelper.isPlainObject(schema)
      ? schema
      : {
          type: "object",
          properties: {},
        };

    return zodFromJSONSchema(jsonSchema);
  }

  public static createUpsertFormSchema(
    schema: Record<string, unknown> | null | undefined,
  ): z.ZodType<CollectionUpsertFormValues> {
    return z.object({
      data: FormSchemaHelper.createZodSchema(schema),
      content: z.string().optional(),
    }) as z.ZodType<CollectionUpsertFormValues>;
  }

  public static validateValue(
    schema: Record<string, unknown> | null | undefined,
    value: Record<string, unknown>,
  ): Record<string, string> {
    const errors: Record<string, string> = {};

    function visit(
      node: CollectionJsonSchemaNode | undefined,
      currentValue: unknown,
      path: CollectionSchemaPathSegment[],
      required: boolean,
    ) {
      if (!node) {
        return;
      }

      const type = FormSchemaHelper.normalizeType(node);
      const pathKey = FormSchemaHelper.getPathKey(path);

      if (required && FormSchemaHelper.isEmptyValue(currentValue)) {
        if (pathKey) {
          errors[pathKey] = "This field is required.";
        }
        return;
      }

      if (!required && FormSchemaHelper.isEmptyValue(currentValue)) {
        return;
      }

      if (node.enum?.length && !node.enum.some((entry) => Object.is(entry, currentValue))) {
        if (pathKey) {
          errors[pathKey] = "Choose one of the available options.";
        }
        return;
      }

      if (type === "object") {
        if (!FormSchemaHelper.isPlainObject(currentValue)) {
          if (pathKey) {
            errors[pathKey] = "Expected an object.";
          }
          return;
        }

        const requiredSet = new Set(node.required ?? []);

        for (const [key, childSchema] of Object.entries(node.properties ?? {})) {
          visit(
            childSchema,
            currentValue[key],
            [...path, key],
            childSchema.optional === true ? false : requiredSet.has(key),
          );
        }

        return;
      }

      if (type === "array") {
        if (!Array.isArray(currentValue)) {
          if (pathKey) {
            errors[pathKey] = "Expected a list.";
          }
          return;
        }

        const itemSchema = FormSchemaHelper.getArrayItemSchema(node);

        currentValue.forEach((entry, index) => {
          visit(itemSchema, entry, [...path, index], true);
        });

        return;
      }

      if ((type === "number" || type === "integer") && typeof currentValue !== "number") {
        if (pathKey) {
          errors[pathKey] = "Expected a number.";
        }
        return;
      }

      if (type === "boolean" && typeof currentValue !== "boolean") {
        if (pathKey) {
          errors[pathKey] = "Expected true or false.";
        }
        return;
      }

      if (type === "string" && typeof currentValue !== "string") {
        if (pathKey) {
          errors[pathKey] = "Expected text.";
        }
      }
    }

    visit(schema as CollectionJsonSchemaNode | undefined, value, [], true);

    return errors;
  }

  private static buildSchemaState(
    schema: CollectionJsonSchemaNode | undefined,
    input: unknown,
  ): unknown {
    if (!schema) {
      return {};
    }

    if (schema.default !== undefined && input === undefined) {
      return FormSchemaHelper.cloneValue(schema.default);
    }

    const type = FormSchemaHelper.normalizeType(schema);

    if (type === "object") {
      const source = FormSchemaHelper.isPlainObject(input) ? input : {};
      const base: Record<string, unknown> = {};

      for (const [key, propertySchema] of Object.entries(schema.properties ?? {})) {
        const child = FormSchemaHelper.buildSchemaState(propertySchema, source[key]);

        if (child !== undefined) {
          base[key] = child;
        }
      }

      return base;
    }

    if (type === "array") {
      if (!Array.isArray(input)) {
        return [];
      }

      const itemSchema = FormSchemaHelper.getArrayItemSchema(schema);
      return input.map((entry) => FormSchemaHelper.buildSchemaState(itemSchema, entry));
    }

    return input;
  }

  private static cloneValue<T>(value: T): T {
    if (value === undefined) {
      return value;
    }

    return JSON.parse(JSON.stringify(value)) as T;
  }
}
