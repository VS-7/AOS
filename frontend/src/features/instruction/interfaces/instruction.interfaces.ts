import { z } from "zod";

/**
 * InstructionSchema: Main schema defining a complete AOS Instruction.
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

export interface InstructionListParams { }
export interface InstructionListResult {
  instructions: Instruction[];
}

export interface InstructionGetParams {
  id: string;
}
export interface InstructionGetResult {
  instruction: Instruction;
}

export interface InstructionCreateParams extends CreateInstructionInput { }
export type InstructionCreateResult = ResponseWithCTA<{
  instruction: Instruction;
}>;

export interface InstructionUpdateParams {
  id: string;
  data: UpdateInstructionInput;
}
export type InstructionUpdateResult = ResponseWithCTA<{
  instruction: Instruction;
}>;

export interface InstructionDeleteParams {
  id: string;
}
export type InstructionDeleteResult = ResponseWithCTA<{
  id: string;
}>;

