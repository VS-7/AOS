import type { CollectionJsonSchemaNode } from "./form-schema.helper";

/** Mirrors internal/domain/collection.Field. */
export interface CollectionField {
  name: string;
  type: "string" | "number" | "boolean" | "date" | "enum" | "ref" | "list";
  description?: string;
  required?: boolean;
  enum?: string[];
  ref?: string;
  default?: unknown;
  unique?: boolean;
}

/** Mirrors internal/domain/collection.Collection. */
export interface CollectionDefinition {
  id: string;
  name: string;
  description?: string;
  scope: "workspace" | "skill";
  skill?: string;
  format: "json" | "md";
  fields: CollectionField[];
  createdAt?: string;
  updatedAt?: string;
}

/** Mirrors internal/domain/collection.Record. */
export interface CollectionRecord {
  id: string;
  collection: string;
  data: Record<string, unknown>;
  content?: string;
  createdAt?: string;
  updatedAt?: string;
}

/** The fields a collection declares, or none for a response that has not arrived. */
export function fieldsOf(collection: { fields?: unknown } | null | undefined): CollectionField[] {
  return Array.isArray(collection?.fields) ? (collection.fields as CollectionField[]) : [];
}

/**
 * A collection's declared fields as the JSON Schema the record form renders.
 *
 * The form read `collection.schema`, which the daemon never sends — a
 * collection declares `fields` — so it rendered "no declared properties" for
 * every collection and nothing could be filled in. The mapping is the
 * daemon's own type rules (`internal/domain/collection/validate.go`): a date
 * is an RFC 3339 string, an enum one of its values, a ref the id of a record,
 * a list any array (edited here as a list of text).
 */
export function fieldsToJsonSchema(fields: CollectionField[]): CollectionJsonSchemaNode & {
  type: "object";
  properties: Record<string, CollectionJsonSchemaNode>;
  required: string[];
} {
  const properties: Record<string, CollectionJsonSchemaNode> = {};
  for (const field of fields) {
    const base: CollectionJsonSchemaNode = {
      title: field.name,
      ...(field.description ? { description: field.description } : {}),
      ...(field.default !== undefined ? { default: field.default } : {}),
    };
    switch (field.type) {
      case "number":
        properties[field.name] = { ...base, type: "number" };
        break;
      case "boolean":
        properties[field.name] = { ...base, type: "boolean" };
        break;
      case "date":
        properties[field.name] = { ...base, type: "string", format: "date" };
        break;
      case "enum":
        properties[field.name] = { ...base, type: "string", enum: field.enum ?? [] };
        break;
      case "list":
        properties[field.name] = { ...base, type: "array", items: { type: "string" } };
        break;
      default:
        properties[field.name] = { ...base, type: "string" };
    }
  }
  return {
    type: "object",
    properties,
    required: fields.filter((field) => field.required).map((field) => field.name),
  };
}

export function requiredFieldCount(fields: CollectionField[]): number {
  return fields.filter((field) => field.required).length;
}

/**
 * What the form holds, as the record data the daemon accepts.
 *
 * Only declared fields go, because the daemon refuses any other — saving used
 * to send the whole record envelope (`collection`, `createdAt`…) as data, and
 * every save was refused. A field left empty is left out rather than sent as
 * "": an empty string is not an enum value or a date, and for a required
 * field leaving it out is what makes the daemon name the field that is
 * missing.
 *
 * `stored` is the record's own data — `{}` when one is being created. An
 * unticked checkbox reads `false` whether the reader turned it off or never
 * touched it, so it is only sent for a record that already carries the
 * field: saving a record nobody edited must not write data into it. Left
 * out, every value given is cleaned as it stands.
 */
export function cleanRecordData(
  fields: CollectionField[],
  values: Record<string, unknown> | null | undefined,
  stored?: Record<string, unknown> | null,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const field of fields) {
    const raw = values?.[field.name];
    if (raw === undefined || raw === null) continue;

    switch (field.type) {
      case "number": {
        if (raw === "") continue;
        const number = typeof raw === "number" ? raw : Number(raw);
        if (Number.isFinite(number)) out[field.name] = number;
        break;
      }
      case "boolean": {
        const checked = raw === true || raw === "true";
        if (!checked && stored !== undefined && !(stored !== null && field.name in stored)) {
          continue;
        }
        out[field.name] = checked;
        break;
      }
      case "list": {
        const items = (Array.isArray(raw) ? raw : [raw]).filter(
          (item) => item !== undefined && item !== null && item !== "",
        );
        if (items.length > 0) out[field.name] = items;
        break;
      }
      case "date": {
        const text = String(raw).trim();
        if (!text) continue;
        // A calendar day from a date input is stored as that day at midnight
        // UTC — the RFC 3339 form the daemon requires.
        out[field.name] = /^\d{4}-\d{2}-\d{2}$/.test(text) ? `${text}T00:00:00Z` : text;
        break;
      }
      default: {
        const text = typeof raw === "string" ? raw : String(raw);
        if (text === "") continue;
        out[field.name] = text;
      }
    }
  }
  return out;
}

/**
 * What a record is called: its first declared text field with something in it.
 * The page title used to be the record's UUID turned into words.
 */
export function recordLabel(
  fields: CollectionField[],
  data: Record<string, unknown> | null | undefined,
): string | undefined {
  for (const field of fields) {
    if (field.type !== "string") continue;
    const value = data?.[field.name];
    if (typeof value === "string" && value.trim()) return value.trim();
  }
  return undefined;
}
