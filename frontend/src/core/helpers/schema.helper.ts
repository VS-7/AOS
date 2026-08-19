import { z } from "zod";

/**
 * Shorthand for any Zod schema type accepted by {@link Schema} internals.
 *
 * Works with both the public `ZodType` alias and Zod v4 internal `$ZodType`
 * representations so type guards and unwrapping logic stay version-stable.
 */
type AnyZodSchema = z.ZodTypeAny;

/**
 * Zod v4 object shape: a record of field name to field schema.
 *
 * Mirrors `core.$ZodLooseShape` and is used when walking shapes for per-field
 * coercion during {@link Schema._cleanInputValues}.
 */
type ZodShape = Record<string, AnyZodSchema>;

/**
 * Field names that {@link Schema.stringify} converted from array/object to string.
 *
 * Tracked so the post-parse step knows which values must be restored via
 * `JSON.parse` after Zod validates the flat string payload.
 */
type StringifiedFields = Set<string>;

/**
 * Maps each key in a Zod raw shape to `z.string()` when the original field was
 * an array or object; all other keys keep their original schema type.
 *
 * Used only as the return-type helper for {@link Schema.stringify}.
 */
type StringifiedShape<T extends z.ZodRawShape> = {
  [K in keyof T]: T[K] extends z.ZodObject<any> | z.ZodArray<any>
    ? z.ZodString
    : T[K];
};

/**
 * Zod schema factory with Fractal-specific input normalization for HTTP, CLI, and MCP.
 *
 * Fractal receives untyped external input from multiple surfaces. Query strings and CLI
 * flags always arrive as strings; MCP and HTTP bodies may send native JSON types. The
 * {@link Schema} helper wraps Zod `parse` / `safeParse` so callers get consistent,
 * sanitized payloads without duplicating coercion logic in every controller or command.
 *
 * ## Capabilities (via {@link Schema.object})
 *
 * - Strips keys whose value is `undefined`, or the literal strings `"undefined"` / `"null"`
 * - Coerces numeric strings to `number` when the field schema expects `ZodNumber`
 * - Parses JSON strings into arrays/objects when the field schema expects those types
 * - Converts literal `\\n` sequences in strings to real newlines (CLI escaping)
 * - Recursively cleans nested object fields using the nested shape when available
 * - Re-wraps proxied results from `.extend()`, `.pick()`, `.omit()`, `.merge()`, etc.
 *
 * ## When to use {@link Schema.object} vs {@link Schema.stringify}
 *
 * | Surface | Prefer |
 * | :--- | :--- |
 * | HTTP controllers, feature schemas, {@link FractalCommand} (automatic via `_wrap_cli_schema`) | {@link Schema.object} |
 * | MCP tools that must expose complex fields as `string` in JSON Schema | {@link Schema.stringify} (opt-in) |
 *
 * {@link Schema.object} accepts **both** CLI JSON strings and native MCP arrays/objects.
 * {@link Schema.stringify} changes the exposed schema so array/object fields become
 * `z.string()` — use only when you intentionally want MCP clients to pass JSON text.
 *
 * @see {@link FractalCommand} — applies {@link Schema.object} to every command at build time
 *
 * @example HTTP / controller body
 * ```typescript
 * export const FractalTaskListQuerySchema = Schema.object({
 *   limit: z.number().optional(),
 *   status: z.string().optional(),
 * });
 * // ?limit=10&status=undefined → { limit: 10 } (status stripped)
 * ```
 *
 * @example CLI command (usually no manual wrap — builder does it)
 * ```typescript
 * FractalCommand.create("create")
 *   .withOptions(z.object({ tags: z.array(z.string()).optional() }))
 *   .build();
 * // --tags '["a","b"]' and MCP { tags: ["a","b"] } both work after build()
 * ```
 */
export class Schema {
  /**
   * Creates a proxied `z.object()` schema with Fractal input normalization.
   *
   * Intercepts `parse` and `safeParse` to run {@link Schema._cleanInputValues} before
   * Zod validation and {@link Schema._stripUndefined} afterward. Also intercepts
   * `extend`, `pick`, `omit`, `partial`, `required`, and `merge` so child schemas
   * keep the same normalization behavior — required when schemas are composed before
   * being passed to {@link FractalCommand.withOptions}.
   *
   * @typeParam T - Zod raw shape (field name → field schema).
   * @param shape - Object shape identical to `z.object({ ... })`.
   * @returns Proxied `ZodObject` with identical `z.infer` / `z.output` typing.
   *
   * @remarks
   * Prefer this over raw `z.object()` for any schema that crosses a Fractal boundary
   * (HTTP query/body, CLI flags, MCP tool arguments).
   *
   * @example
   * ```typescript
   * const s = Schema.object({
   *   page: z.number().optional(),
   *   tags: z.array(z.string()).optional(),
   * });
   * s.parse({ page: "2", tags: '["x"]' }); // → { page: 2, tags: ["x"] }
   * s.parse({ page: 2, tags: ["x"] });     // native MCP/JSON — also valid
   * ```
   */
  static object<T extends z.ZodRawShape>(shape: T) {
    return Schema._wrap(z.object(shape));
  }

  /**
   * Wraps an existing {@link z.ZodObject} instance with the same coercion proxy
   * used by {@link Schema.object}. Unlike reconstructing from `.shape`, wrapping
   * the instance preserves its zod def — including strict mode
   * (`unknownKeys: "strict"`), which `z.object(shape)` would silently reset to
   * the default strip behavior.
   */
  private static _wrap<T extends z.ZodObject<any>>(original: T): T {
    const shape = original.shape;

    return new Proxy(original, {
      get(target, prop, receiver) {
        if (prop === "parse") {
          return (data: unknown, params?: any) => {
            // [Data Cleaning]: Pre-process input to clean invalid string representations and coerce numbers
            const cleanedInput = Schema._cleanInputValues(
              data,
              shape as unknown as ZodShape,
            );
            const parsed = target.parse(cleanedInput, params);
            return Schema._stripUndefined(parsed);
          };
        }
        if (prop === "safeParse") {
          return (data: unknown, params?: any) => {
            // [Data Cleaning]: Pre-process input to clean invalid string representations and coerce numbers
            const cleanedInput = Schema._cleanInputValues(
              data,
              shape as unknown as ZodShape,
            );
            const parsed = target.safeParse(cleanedInput, params) as any;
            if (parsed.success) {
              return { ...parsed, data: Schema._stripUndefined(parsed.data) };
            }
            return parsed;
          };
        }

        // [Proxy Preservation]: Intercept ZodObject methods that return NEW ZodObject instances,
        // re-wrapping them so the coercion proxy is never lost. This is critical because
        // `FractalCommand.withOptions()` calls `.extend()` on schema objects, and without
        // re-wrapping the result loses the `_cleanInputValues` coercion entirely.
        if (
          prop === "extend" ||
          prop === "pick" ||
          prop === "omit" ||
          prop === "partial" ||
          prop === "required"
        ) {
          return (...args: any[]) => {
            const result = (target as any)[prop](...args);
            if (result instanceof z.ZodObject) {
              return Schema.object(result.shape as z.ZodRawShape);
            }
            return result;
          };
        }

        // `.strict()` must wrap the strict INSTANCE, not reconstruct from shape:
        // the reconstructed object would lose `unknownKeys: "strict"` and silently
        // strip unknown keys again.
        if (prop === "strict") {
          return () => {
            const result = (target as any).strict();
            if (result instanceof z.ZodObject) {
              return Schema._wrap(result);
            }
            return result;
          };
        }

        if (prop === "merge") {
          return (...args: any[]) => {
            const result = (target as any).merge(...args);
            if (result instanceof z.ZodObject) {
              return Schema.object(result.shape as z.ZodRawShape);
            }
            return result;
          };
        }

        return Reflect.get(target, prop, receiver);
      },
    });
  }

  /**
   * Creates a CLI-oriented schema that exposes array/object fields as strings.
   *
   * Builds a parallel shape where every field whose innermost type is `ZodArray`,
   * `ZodObject`, or `ZodRecord` (including `.optional()`, `.nullable()`, `.default()`
   * wrappers) is replaced with `z.string()`. After Zod validates the flat string
   * payload, {@link Schema._restoreStringified} parses those fields back to their
   * original JavaScript structures.
   *
   * Inherits all primitive normalization from {@link Schema.object} (numeric coercion,
   * `"undefined"` / `"null"` stripping, `\\n` unescaping).
   *
   * ## When to use
   *
   * - Opt-in when a command or tool must advertise complex fields as `string` in MCP
   *   JSON Schema (agents pass `'{"enabled":true}'` instead of native objects).
   * - **Do not** use for the default {@link FractalCommand} pipeline — the builder
   *   applies {@link Schema.object} automatically so MCP native JSON keeps working.
   *
   * @typeParam T - Zod raw shape taken from an existing schema (e.g. `MySchema.shape`).
   * @param shape - The `.shape` of any `ZodObject` schema.
   * @returns Proxied `ZodObject` with string-typed complex fields and restored output types.
   *
   * @throws When a stringified field contains malformed JSON (descriptive
   * `[Schema.stringify]` error message).
   *
   * @example Reuse a feature schema shape without redeclaring fields as strings
   * ```typescript
   * options: Schema.stringify(FractalTemplateCreateParamsSchema.shape)
   * ```
   */
  static stringify<T extends z.ZodRawShape>(
    shape: T,
  ): z.ZodObject<StringifiedShape<T>> {
    // [Stringify Mapping]: Build a parallel shape where array/object fields become z.string().optional()
    const stringifiedFields: StringifiedFields = new Set();
    const stringShape: ZodShape = {};

    for (const [key, fieldSchema] of Object.entries(shape)) {
      const typed = fieldSchema as AnyZodSchema;
      if (Schema._expectsArrayOrObject(typed)) {
        stringifiedFields.add(key);
        // [Preserve Optional]: Keep the field optional if the original was optional/nullable/default
        const isOptional = Schema._isOptionalLike(typed);
        stringShape[key] = isOptional
          ? z
              .string()
              .optional()
              .describe(typed.description || "")
          : z.string().describe(typed.description || "");
      } else {
        stringShape[key] = typed;
      }
    }

    const original = z.object(stringShape);

    return new Proxy(original, {
      get(target, prop, receiver) {
        if (prop === "parse") {
          return (data: unknown, params?: any) => {
            // [Data Cleaning]: Apply standard input cleaning (null/undefined strings, numeric coercion)
            const cleanedInput = Schema._cleanInputValues(data, stringShape);
            const parsed = target.parse(cleanedInput, params);
            // [JSON Restore]: Parse stringified array/object fields back to their original structure
            return Schema._restoreStringified(
              Schema._stripUndefined(parsed),
              stringifiedFields,
            );
          };
        }
        if (prop === "safeParse") {
          return (data: unknown, params?: any) => {
            // [Data Cleaning]: Apply standard input cleaning
            const cleanedInput = Schema._cleanInputValues(data, stringShape);
            const parsed = target.safeParse(cleanedInput, params) as any;
            if (parsed.success) {
              // [JSON Restore]: Parse stringified array/object fields back to their original structure
              const restored = Schema._restoreStringified(
                Schema._stripUndefined(parsed.data),
                stringifiedFields,
              );
              return { ...parsed, data: restored };
            }
            return parsed;
          };
        }
        return Reflect.get(target, prop, receiver);
      },
    }) as any;
  }

  /**
   * Creates a proxied `z.array()` schema that drops `undefined` elements after parsing.
   *
   * Useful when optional items inside an array should not survive validation
   * (e.g. sparse arrays from loosely typed client input).
   *
   * @typeParam T - Element schema for the array.
   * @param schema - Zod schema describing each array item.
   * @returns Proxied `ZodArray` with filtered `parse` / `safeParse`.
   *
   * @example
   * ```typescript
   * const s = Schema.array(z.string().optional());
   * s.parse(["a", undefined, "b"]); // → ["a", "b"]
   * ```
   */
  static array<T extends z.ZodTypeAny>(schema: T) {
    const original = z.array(schema);

    return new Proxy(original, {
      get(target, prop, receiver) {
        if (prop === "parse") {
          return (data: unknown, params?: any) => {
            const parsed = target.parse(data, params);
            return Array.isArray(parsed)
              ? parsed.filter((i: any) => i !== undefined)
              : parsed;
          };
        }
        if (prop === "safeParse") {
          return (data: unknown, params?: any) => {
            const parsed = target.safeParse(data, params) as any;
            if (parsed.success) {
              const cleaned = Array.isArray(parsed.data)
                ? parsed.data.filter((i: any) => i !== undefined)
                : parsed.data;
              return { ...parsed, data: cleaned };
            }
            return parsed;
          };
        }
        return Reflect.get(target, prop, receiver);
      },
    });
  }

  /**
   * Pre-processes raw input before Zod validation.
   *
   * Walks plain objects recursively and, when `shape` is provided, applies
   * per-field rules based on the declared Zod type for each key:
   *
   * - Drops entries whose value is the literal string `"undefined"` or `"null"`
   * - Replaces `\\n` with real newlines in string values (CLI escaping)
   * - Coerces numeric strings when the field expects {@link z.ZodNumber}
   * - Parses JSON strings when the field expects array/object/record types
   * - Recurses into nested objects using {@link Schema._extractObjectShape}
   *
   * If JSON parsing fails for an array/object field, the original string is kept
   * so Zod can surface a validation error. If numeric coercion yields `NaN`, the
   * original string is preserved for the same reason.
   *
   * @param data - Raw value from HTTP query, CLI flags, or MCP arguments.
   * @param shape - Optional top-level object shape for per-field inspection.
   * @returns Cleaned value passed to `ZodObject.parse`.
   */
  private static _cleanInputValues(data: unknown, shape?: ZodShape): unknown {
    if (data === null || data === undefined) {
      return data;
    }

    if (typeof data !== "object") {
      return data;
    }

    if (Array.isArray(data)) {
      return data.map((item) => Schema._cleanInputValues(item));
    }

    const cleaned: Record<string, unknown> = {};

    for (const [key, value] of Object.entries(
      data as Record<string, unknown>,
    )) {
      // [Data Cleaning]: Remove keys with invalid string representations
      if (value === "undefined" || value === "null") {
        continue;
      }

      // [Formatting Correction]: Convert explicit "\\n" sequences in strings to actual newlines
      // This fixes an issue where CLI commands send literal \\n within string arguments
      let processedValue = value;
      if (typeof processedValue === "string") {
        processedValue = processedValue.replace(/\\n/g, "\n");
      }

      const fieldSchema = shape?.[key] as AnyZodSchema | undefined;

      // [Numeric Coercion]: Convert string to number when the schema field expects a number
      if (
        fieldSchema &&
        typeof processedValue === "string" &&
        Schema._expectsNumber(fieldSchema)
      ) {
        const asNumber = Number(processedValue);
        cleaned[key] = Number.isNaN(asNumber) ? processedValue : asNumber;
        continue;
      }

      // [Array/Object Coercion]: Convert JSON string to array/object when the field expects one
      if (
        fieldSchema &&
        typeof processedValue === "string" &&
        Schema._expectsArrayOrObject(fieldSchema)
      ) {
        try {
          cleaned[key] = JSON.parse(processedValue);
        } catch {
          // If JSON parsing fails, keep the original string so Zod can surface the validation error
          cleaned[key] = processedValue;
        }
        continue;
      }

      // [Data Cleaning]: Recursively clean nested objects, propagating the nested shape if available
      const nestedShape = Schema._extractObjectShape(fieldSchema);
      cleaned[key] = Schema._cleanInputValues(
        processedValue,
        nestedShape ?? undefined,
      );
    }

    return cleaned;
  }

  /**
   * Returns whether the resolved inner type of `schema` is {@link z.ZodNumber}.
   *
   * Unwraps `ZodOptional`, `ZodNullable`, `ZodDefault`, and related wrappers via
   * {@link Schema._unwrapSchema} before inspecting the base type.
   *
   * @param schema - Field schema from an object shape.
   * @returns `true` when the field ultimately expects a number.
   */
  private static _expectsNumber(schema: AnyZodSchema): boolean {
    const unwrapped = Schema._unwrapSchema(schema);
    return unwrapped instanceof z.ZodNumber;
  }

  /**
   * Recursively unwraps Zod modifier wrappers to reach the base type.
   *
   * Handles `ZodOptional`, `ZodNullable`, `ZodDefault`, and `ZodCatch`.
   *
   * AOS ports this against `zod` v3 (see `package.json`), not the v4 this
   * helper's original comment described: v3 only exposes the public
   * `.unwrap()` accessor on `ZodOptional`/`ZodNullable` — `ZodDefault` and
   * `ZodCatch` instead expose `.removeDefault()`/`.removeCatch()`, and v3 has
   * no `ZodNonOptional` class at all (that branch is dropped here; v3 has no
   * equivalent wrapper to match). Calling `.unwrap()` on a v3 `ZodDefault`/
   * `ZodCatch` instance throws at runtime, not just a type error, so this is
   * a correctness fix, not only a typecheck one.
   *
   * @param schema - Zod type that may be wrapped in modifiers.
   * @returns Innermost non-wrapper schema instance.
   */
  private static _unwrapSchema(schema: AnyZodSchema): AnyZodSchema {
    if (schema instanceof z.ZodOptional || schema instanceof z.ZodNullable) {
      return Schema._unwrapSchema(schema.unwrap());
    }
    if (schema instanceof z.ZodDefault) {
      return Schema._unwrapSchema(schema.removeDefault());
    }
    if (schema instanceof z.ZodCatch) {
      return Schema._unwrapSchema(schema.removeCatch());
    }
    return schema;
  }

  /**
   * Extracts the nested object shape from a field schema when present.
   *
   * Used by {@link Schema._cleanInputValues} to recurse into nested objects and
   * apply numeric / JSON coercion on inner keys.
   *
   * @param schema - Field schema from a parent shape, or `undefined` if unknown.
   * @returns Inner `ZodObject.shape` map, or `null` when the field is not an object.
   */
  private static _extractObjectShape(
    schema: AnyZodSchema | undefined,
  ): ZodShape | null {
    if (!schema) return null;
    const unwrapped = Schema._unwrapSchema(schema);
    if (unwrapped instanceof z.ZodObject) {
      return unwrapped.shape as ZodShape;
    }
    return null;
  }

  /**
   * Removes absent or sentinel keys from a parsed top-level object.
   *
   * Deletes keys whose value is `undefined`, `"undefined"`, or `"null"`.
   * Non-plain objects and arrays are returned unchanged.
   *
   * @param obj - Value returned from Zod parsing (typically a plain object).
   * @returns Sanitized object without sentinel keys.
   */
  private static _stripUndefined(obj: any): any {
    if (!obj || typeof obj !== "object" || Array.isArray(obj)) return obj;
    const cleaned = { ...obj };
    for (const key in cleaned) {
      const value = cleaned[key];
      if (value === undefined || value === "undefined" || value === "null") {
        delete cleaned[key];
      }
    }
    return cleaned;
  }

  /**
   * Returns whether the resolved inner type is array, object, or record.
   *
   * Used by {@link Schema.stringify} to decide which fields become `z.string()`
   * in the CLI-facing shape, and by {@link Schema._cleanInputValues} to decide
   * when to `JSON.parse` string payloads before validation.
   *
   * @param schema - Field schema from an object shape.
   * @returns `true` for `ZodArray`, `ZodObject`, or `ZodRecord` after unwrapping.
   */
  private static _expectsArrayOrObject(schema: AnyZodSchema): boolean {
    const unwrapped = Schema._unwrapSchema(schema);
    return (
      unwrapped instanceof z.ZodArray ||
      unwrapped instanceof z.ZodObject ||
      unwrapped instanceof z.ZodRecord
    );
  }

  /**
   * Returns whether the outermost wrapper makes the field optional-like.
   *
   * Treats `ZodOptional`, `ZodNullable`, `ZodDefault`, and `ZodCatch` as optional
   * so {@link Schema.stringify} preserves the required/optional contract on the
   * string replacement field.
   *
   * @param schema - Field schema from an object shape.
   * @returns `true` when the field may be omitted without failing validation.
   */
  private static _isOptionalLike(schema: AnyZodSchema): boolean {
    return (
      schema instanceof z.ZodOptional ||
      schema instanceof z.ZodNullable ||
      schema instanceof z.ZodDefault ||
      schema instanceof z.ZodCatch
    );
  }

  /**
   * Restores JSON-stringified fields after {@link Schema.stringify} validation.
   *
   * For each key in `stringifiedFields`, parses the string value with `JSON.parse`.
   * Skips `undefined`, `null`, and non-string values so optional fields and native
   * MCP payloads that bypassed stringification remain untouched.
   *
   * @param obj - Parsed flat object from Zod.
   * @param stringifiedFields - Keys that were exposed as `z.string()` in the shape.
   * @returns Object with complex fields restored to arrays/objects.
   * @throws When a designated field contains invalid JSON (includes field name in message).
   */
  private static _restoreStringified(
    obj: any,
    stringifiedFields: StringifiedFields,
  ): any {
    if (!obj || typeof obj !== "object" || Array.isArray(obj)) return obj;
    const result = { ...obj };
    for (const key of stringifiedFields) {
      const value = result[key];
      // [Guard]: Skip absent/non-string values — optional fields may simply be undefined
      if (value === undefined || value === null) continue;
      if (typeof value !== "string") continue;
      try {
        result[key] = JSON.parse(value);
      } catch {
        throw new Error(
          `[Schema.stringify] Field "${key}" must be a valid JSON string. Received: ${value}`,
        );
      }
    }
    return result;
  }
}
