import { t } from "@/lib/i18n";

/** The template form's values, as the fields hold them. */
export type TemplateFormValues = {
  name: string;
  skill?: string;
  description: string;
  output?: string;
  variablesText?: string;
  content?: string;
};

/**
 * The declared variables, as the daemon stores them.
 *
 * This textarea used to be labelled "JSON schema" and sent under a `schema`
 * key — a field `templates_create`/`templates_update` do not have, so the
 * decoder dropped it and nothing a person wrote here was ever saved. Go's
 * template carries `variables`: a list of `{name, type, description,
 * required, default}`, which is also what the Liquid body reads by name.
 *
 * Anything that is not a list is refused here rather than sent to be ignored,
 * and the message says which of the two problems it is — the caller used to
 * catch both and show "Template schema must be valid JSON." for either.
 */
function parseTemplateVariables(variablesText?: string): unknown[] | undefined {
  const trimmed = variablesText?.trim();
  if (!trimmed) return undefined;

  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    throw new Error(t("Variables must be valid JSON."));
  }
  if (!Array.isArray(parsed)) {
    throw new Error(t("Variables are a list, such as [{ \"name\": \"title\", \"type\": \"string\" }]."));
  }
  return parsed;
}

/**
 * What `templates_create` is sent. Empty optional fields are left out — there
 * is nothing to clear on a new template — and the body goes as typed: it is
 * Liquid a model reads, and trimming it changed it.
 */
export function templateCreatePayload(values: TemplateFormValues) {
  const variables = parseTemplateVariables(values.variablesText);
  const skill = values.skill?.trim();
  const output = values.output?.trim();
  return {
    name: values.name.trim(),
    ...(skill ? { skill } : {}),
    description: values.description.trim(),
    ...(output ? { output } : {}),
    ...(variables ? { variables } : {}),
    content: values.content ?? "",
  };
}

/**
 * What `templates_update` is sent: every editable field, a cleared one as ""
 * or []. The daemon reads an absent field as "leave it as it is", so dropping
 * the cleared ones reported a save and brought the old values back. `skill`
 * is not here: the update takes none, and the form shows it read-only once the
 * template exists.
 */
export function templateUpdatePayload(values: TemplateFormValues) {
  return {
    description: values.description.trim(),
    output: values.output?.trim() ?? "",
    variables: parseTemplateVariables(values.variablesText) ?? [],
    content: values.content ?? "",
  };
}
