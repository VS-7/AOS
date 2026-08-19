import { z } from "zod";

/**
 * InstructionSchema: Main schema defining a complete Fractal Instruction.
 * Simplified to align with modern IDE context patterns (Copilot/Claude).
 * @description Represents the complete structure of an instruction including identity, content, and scope.
 */
export const InstructionSchema = z.object({
  id: z
    .string()
    .describe(
      "Unique instruction identifier (slug) used in file paths and references. Example: feature-protocol"
    ),
  name: z
    .string()
    .describe("Display name of the instruction. Example: Feature Protocol"),
  type: z
    .string()
    .describe(
      "Categorization of the instruction for organizational purposes. Example: standards, patterns, workflows"
    ),
  description: z
    .string()
    .optional()
    .describe(
      "Concise description of the instruction purpose. Example: Enforces coding standards"
    ),
  content: z
    .string()
    .optional()
    .describe(
      "Markdown content body of the instruction. Example: # Usage\n..."
    ),
  paths: z
    .array(z.string())
    .optional()
    .describe(
      "List of glob patterns matching files this instruction applies to. Example: ['src/features/**/*.ts']"
    ),
});

/**
 * CreateInstructionSchema: Schema for creating a new instruction.
 * @description Omit generated or optional fields for creation payloads.
 */
export const CreateInstructionSchema = InstructionSchema.omit({
  id: true,
});

/**
 * UpdateInstructionSchema: Schema for updating an existing instruction.
 * @description All fields are optional for partial updates.
 */
export const UpdateInstructionSchema = InstructionSchema.omit({
  id: true,
}).partial();

/**
 * Type inference for Instruction.
 * @description Automatically inferred type from InstructionSchema.
 */
export type Instruction = z.infer<typeof InstructionSchema>;

/**
 * Type inference for CreateInstructionInput.
 * @description Automatically inferred type from CreateInstructionSchema.
 */
export type CreateInstructionInput = z.infer<typeof CreateInstructionSchema>;

/**
 * Type inference for UpdateInstructionInput.
 * @description Automatically inferred type from UpdateInstructionSchema.
 */
export type UpdateInstructionInput = z.infer<typeof UpdateInstructionSchema>;

/**
 * Service Layer Parameters & Results
 */

import type { ResponseWithCTA } from "@/core/interfaces/response.interfaces";

export interface FractalInstructionListParams { }
export interface FractalInstructionListResult {
  instructions: Instruction[];
}

export interface FractalInstructionGetParams {
  id: string;
}
export interface FractalInstructionGetResult {
  instruction: Instruction;
}

export interface FractalInstructionCreateParams extends CreateInstructionInput { }
export type FractalInstructionCreateResult = ResponseWithCTA<{
  instruction: Instruction;
}>;

export interface FractalInstructionUpdateParams {
  id: string;
  data: UpdateInstructionInput;
}
export type FractalInstructionUpdateResult = ResponseWithCTA<{
  instruction: Instruction;
}>;

export interface FractalInstructionDeleteParams {
  id: string;
}
export type FractalInstructionDeleteResult = ResponseWithCTA<{
  id: string;
}>;

/**
 * Alias for the freshly-copied `v401/web` presentation code, which
 * imports the `Fractal`-prefixed name — same reasoning as `skill.
 * interfaces.ts`'s `FractalSkill` alias.
 */
export type FractalInstruction = Instruction;
