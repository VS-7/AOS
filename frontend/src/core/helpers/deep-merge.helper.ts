import { z } from "zod";

export class DeepMergeHelper {
  /**
   * Deep merges multiple objects and validates the final result against a Zod schema.
   * The arguments are processed from left to right, meaning the last object provided
   * has the highest priority and will overwrite properties from earlier objects.
   * Arrays from the source entirely replace arrays from the target (no concatenation).
   *
   * @param schema The Zod schema to validate the final merged object against.
   * @param args A variadic list of objects to merge.
   * @returns The merged and validated object.
   */
  public static merge<TSchema extends z.ZodTypeAny>(
    schema: TSchema,
    ...args: Record<string, unknown>[]
  ): z.infer<TSchema> {
    if (args.length === 0) {
      return schema.parse({});
    }

    const merged = args.reduce(
      (acc, current) => {
        if (!current) return acc;
        return this.deepMergeObjects(acc, current);
      },
      {} as Record<string, unknown>,
    );

    return schema.parse(merged);
  }

  private static isObject(item: unknown): item is Record<string, unknown> {
    return (
      item !== null &&
      typeof item === "object" &&
      !Array.isArray(item) &&
      !(item instanceof Date) &&
      !(item instanceof RegExp)
    );
  }

  private static deepMergeObjects(
    target: Record<string, unknown>,
    source: Record<string, unknown>,
  ): Record<string, unknown> {
    const output = { ...target };

    if (!this.isObject(target) || !this.isObject(source)) {
      return source; // Cannot merge, source overwrites
    }

    Object.keys(source).forEach((key) => {
      const sourceValue = source[key];
      const targetValue = target[key];

      if (Array.isArray(sourceValue)) {
        // Source overwrites target entirely for arrays
        // (avoids duplicating entries when the caller already built the full array)
        output[key] = sourceValue;
      } else if (this.isObject(sourceValue) && this.isObject(targetValue)) {
        output[key] = this.deepMergeObjects(
          targetValue as Record<string, unknown>,
          sourceValue as Record<string, unknown>,
        );
      } else {
        // Source overwrites target
        if (sourceValue !== undefined) {
          output[key] = sourceValue;
        }
      }
    });

    return output;
  }
}
