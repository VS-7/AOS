import { z } from "zod";

/** Shared nullable string used across registry prop schemas. */
export const zNullableString = z.string().nullable();

/** Shared nullable className prop. */
export const zClassName = z
  .string()
  .nullable()
  .describe("Additional Tailwind CSS classes");

/** Shared validation checks for form fields. */
export const validationCheckSchema = z
  .array(
    z.object({
      type: z.string(),
      message: z.string(),
      args: z.record(z.string(), z.unknown()).optional(),
    }),
  )
  .nullable();

export const validateOnSchema = z
  .enum(["change", "blur", "submit"])
  .nullable();
