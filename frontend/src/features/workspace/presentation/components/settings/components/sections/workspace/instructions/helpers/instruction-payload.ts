/** The instruction form's values, as the fields hold them. */
export type InstructionFormValues = {
  name: string;
  type: string;
  description?: string;
  content?: string;
  pathsText?: string;
};

function parsePaths(pathsText?: string): string[] {
  return (pathsText ?? "")
    .split("\n")
    .map((path) => path.trim())
    .filter(Boolean);
}

/**
 * What `instructions_create` is sent. Empty optional fields are left out, and
 * the body goes as typed: an instruction is injected into a prompt, and
 * trimming it — or re-serialising it as markdown — changed what the model
 * reads.
 */
export function instructionCreatePayload(values: InstructionFormValues) {
  const description = values.description?.trim();
  const paths = parsePaths(values.pathsText);
  return {
    name: values.name.trim(),
    type: values.type.trim(),
    ...(description ? { description } : {}),
    ...(values.content ? { content: values.content } : {}),
    ...(paths.length ? { paths } : {}),
  };
}

/**
 * What `instructions_update` is sent: every editable field, a cleared one as
 * "" or []. The daemon reads an absent field as "leave it as it is", so
 * clearing the description and paths to make an instruction global reported
 * "Instruction updated." and brought both back.
 */
export function instructionUpdatePayload(values: InstructionFormValues) {
  return {
    name: values.name.trim(),
    type: values.type.trim(),
    description: values.description?.trim() ?? "",
    content: values.content ?? "",
    paths: parsePaths(values.pathsText),
  };
}
