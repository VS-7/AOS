/** What the project page's form holds. */
export interface ProjectFormFields {
  name: string;
  status?: string;
  icon?: string;
  description?: string;
  content?: string;
  source?: string;
}

/**
 * The id the daemon will derive from a name — internal/core/slug.Generate,
 * rule for rule: decompose so an accent is its own mark, drop the marks,
 * lowercase, turn whitespace and hyphens into one hyphen, drop what is not a
 * letter, digit or underscore.
 *
 * The preview used to keep only ASCII letters and digits, the rule the
 * daemon's own project slug also had, so "Ação Rápida" previewed and saved as
 * a-o-r-pida while a goal of the same title became acao-rapida.
 */
export function projectIdPreview(name: string): string {
  return deriveProjectId(name) || "project-id";
}

function deriveProjectId(name: string): string {
  let out = "";
  let pendingHyphen = false;
  const folded = name.normalize("NFD").replace(/\p{Mn}/gu, "").normalize("NFC").trim().toLowerCase();
  for (const char of folded) {
    if (/\s|-/u.test(char)) {
      pendingHyphen = out.length > 0;
    } else if (/[\p{L}\p{N}_]/u.test(char)) {
      if (pendingHyphen) out += "-";
      pendingHyphen = false;
      out += char;
    }
  }
  return out;
}

/** Whether a name has anything to derive an id from. */
export function hasSluggableCharacter(name: string): boolean {
  return deriveProjectId(name) !== "";
}

/** Whether a source is absolute, which the daemon requires. */
export function isAbsoluteSource(source: string): boolean {
  return source.startsWith("/") || /^[A-Za-z]:[\\/]/.test(source);
}

/**
 * The body of projects_update. A field the person emptied is sent as "",
 * which Go reads as "clear"; undefined was dropped from the JSON and read as
 * "leave unchanged".
 */
export function projectUpdateBody(values: ProjectFormFields) {
  return {
    name: values.name.trim(),
    ...(values.status ? { status: values.status } : {}),
    icon: (values.icon ?? "").trim(),
    description: (values.description ?? "").trim(),
    content: (values.content ?? "").trim(),
    source: (values.source ?? "").trim(),
  };
}

/** The body of projects_create: only what was filled in. */
export function projectCreateBody(values: ProjectFormFields) {
  const optional = {
    icon: values.icon?.trim(),
    description: values.description?.trim(),
    content: values.content?.trim(),
    source: values.source?.trim(),
  };
  return {
    name: values.name.trim(),
    ...Object.fromEntries(Object.entries(optional).filter(([, value]) => value)),
    ...(values.status ? { status: values.status } : {}),
  } as Record<string, string>;
}
