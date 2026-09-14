import { z } from "zod";
import type { Agent } from "@/features/agent/interfaces/agent.interfaces";

/**
 * The agent editor's form, and the two different payloads it becomes.
 *
 * Kept apart from the context so the rules that decide what reaches the
 * daemon can be read — and tested — without a React tree around them.
 */

/**
 * The levels the runtime honours (`agentloop.levelOf`). Anything else it
 * quietly reads as the default, so offering more here would be offering a
 * choice that does nothing.
 */
export const AGENT_REASONING_LEVELS = ["none", "low", "medium", "high"] as const;

export const agentFormSchema = z.object({
  name: z.string().trim().min(1, "Name is required"),
  image: z.string().optional().or(z.literal("")),
  description: z.string().optional(),
  role: z.string().optional(),
  provider: z.string().optional(),
  model: z.string().optional(),
  reasoning: z.string().optional(),
  leader: z.string().optional(),
  content: z.string().optional(),
  orchestrator: z.boolean().default(false),
});

export type AgentFormValues = z.infer<typeof agentFormSchema>;

/**
 * The fields a person edits. The skill is not one of them: it is where the
 * AGENT.md file lives (`entity.go`, `collection:"path"`), neither input
 * accepts it, and a text box for it saved nothing while saying it had.
 */
const EDITABLE = [
  "name",
  "image",
  "description",
  "role",
  "provider",
  "model",
  "reasoning",
  "leader",
  "content",
  "orchestrator",
] as const satisfies readonly (keyof AgentFormValues)[];

export const EMPTY_AGENT_FORM: AgentFormValues = buildAgentFormValues(null);

export function buildAgentFormValues(agent?: Agent | null): AgentFormValues {
  return {
    name: agent?.name ?? "",
    image: agent?.image ?? "",
    description: agent?.description ?? "",
    role: agent?.role ?? "",
    provider: agent?.provider ?? "",
    model: agent?.model ?? "",
    reasoning: agent?.reasoning ?? "",
    leader: agent?.leader ?? "",
    content: agent?.content ?? "",
    orchestrator: agent?.orchestrator ?? false,
  };
}

/**
 * The instructions are a prompt, so they go out byte for byte: no trim, no
 * normalisation. A trailing space or a closing blank line is part of what the
 * model reads, and AGENT.md is versioned — a save that rewrote whitespace
 * nobody touched is a diff nobody asked for.
 */
function wireValue(field: (typeof EDITABLE)[number], value: unknown) {
  if (typeof value !== "string") return value;
  return field === "content" ? value : value.trim();
}

/**
 * Create: a blank field is simply not sent, so the daemon's defaults apply
 * (the name becomes the id's source, the sandbox the working default).
 */
export function buildCreatePayload(values: AgentFormValues): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  for (const field of EDITABLE) {
    const value = wireValue(field, values[field]);
    if (typeof value === "string") {
      if (field === "content" ? value.trim() === "" : value === "") continue;
    }
    body[field] = value;
  }
  return body;
}

/**
 * Update: only what the person changed, and a cleared field as "".
 *
 * `agents_update` is a patch (`schema.go`: every field a pointer, nil means
 * "leave it"). Turning a blank into `undefined` — which JSON drops — therefore
 * meant "leave it", and clearing the role or the model saved nothing while the
 * screen said "Agent updated.". The empty string is the patch's own way of
 * saying "clear". Sending only the dirty fields is what keeps a save from
 * rewriting the parts of AGENT.md somebody else changed in the meantime.
 */
export function buildUpdatePayload(
  values: AgentFormValues,
  dirty: Partial<Record<keyof AgentFormValues, unknown>>,
): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  for (const field of EDITABLE) {
    if (!dirty[field]) continue;
    const value = wireValue(field, values[field]);
    body[field] = value ?? "";
  }
  return body;
}

/**
 * The id the daemon will derive from a name — `internal/core/slug.Generate`,
 * rule for rule, then lowercased as `agent.normalizeID` does.
 *
 * Unicode-aware on purpose. `core/helpers/slug.helper.ts` uses `\w`, which in
 * JavaScript is ASCII only, so it would call "日本" unusable while the daemon
 * happily creates it — and a check that disagrees with the server is worse
 * than none.
 */
export function agentSlug(text: string): string {
  if (!text) return "";
  const folded = text.normalize("NFD").replace(/\p{Mn}/gu, "").normalize("NFC");
  let out = "";
  let pendingHyphen = false;
  for (const ch of folded.trim().toLowerCase()) {
    if (/\s/u.test(ch) || ch === "-") {
      pendingHyphen = out.length > 0;
    } else if (/[\p{L}\p{Nd}_]/u.test(ch)) {
      if (pendingHyphen) {
        out += "-";
        pendingHyphen = false;
      }
      out += ch;
    }
    // Anything else is dropped without resetting pendingHyphen, so
    // "a - b" and "a-b" agree, as they do in Go.
  }
  return out;
}
