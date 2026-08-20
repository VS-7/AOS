import z from "zod";

/**
 * RuleTypeSchema: The enforcement type of the rule.
 * @description Defines how a rule should be enforced in the skill behavior.
 * @enum ["always", "never", "allow", "ask", "workflow", "note"]
 */
export const RuleTypeSchema = z.enum([
  "always",
  "never",
  "allow",
  "ask",
  "workflow",
  "note",
]);

/**
 * RuleSchema: Defines a behavioral rule for the skill.
 * @description Rules enforce specific behaviors and constraints on skill execution.
 * @example
 * ```typescript
 * const rule = {
 *   type: "always",
 *   instruction: "Always greet the user before processing requests",
 *   when: "When the user initiates a new conversation"
 * };
 * ```
 */
export const RuleSchema = z.object({
  type: RuleTypeSchema.describe(
    "Enforcement type that determines how the rule applies. Example: 'always'"
  ),
  instruction: z
    .string()
    .describe(
      "Specific instruction or constraint to enforce. Example: Always validate input before processing"
    ),
  when: z
    .string()
    .optional()
    .describe(
      "Optional condition or context where this rule applies. Example: When processing user commands"
    ),
});