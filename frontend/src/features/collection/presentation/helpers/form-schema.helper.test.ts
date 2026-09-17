import { describe, expect, it } from "vitest";

import { FormSchemaHelper } from "./form-schema.helper";
import { fieldsToJsonSchema, type CollectionField } from "./collection-fields.helper";

const contacts: CollectionField[] = [
  { name: "name", type: "string", required: true },
  { name: "email", type: "string" },
  { name: "age", type: "number" },
  { name: "active", type: "boolean" },
  { name: "stage", type: "enum", enum: ["lead", "customer"] },
  { name: "birthday", type: "date" },
  { name: "tags", type: "list" },
];

describe("buildUpsertFormValues", () => {
  // Every input the record form renders registers a value of its own — an
  // empty box is "", an unticked one `false`. A field the record does not
  // carry was left out of these defaults, so react-hook-form compared
  // `{name}` with `{name, email: "", age: "", …}`, found more keys and
  // called the form dirty: every record page opened reading "Unsaved
  // changes" over a record nobody had edited.
  it("gives every declared field the value its input registers", () => {
    const values = FormSchemaHelper.buildUpsertFormValues(
      fieldsToJsonSchema(contacts),
      { name: "Ada Lovelace", age: 36 },
    );

    expect(values.data).toEqual({
      name: "Ada Lovelace",
      email: "",
      age: 36,
      active: false,
      stage: "",
      birthday: "",
      tags: [],
    });
  });

  it("keeps what the record stores, and the body beside it", () => {
    const values = FormSchemaHelper.buildUpsertFormValues(
      fieldsToJsonSchema(contacts),
      { name: "Alan", active: false, tags: ["math"], stage: "customer" },
      "# Kickoff\n",
    );

    expect(values.data).toMatchObject({
      name: "Alan",
      active: false,
      tags: ["math"],
      stage: "customer",
    });
    expect(values.content).toBe("# Kickoff\n");
  });

  it("prefers a field's declared default to an empty value", () => {
    const values = FormSchemaHelper.buildUpsertFormValues(
      fieldsToJsonSchema([{ name: "stage", type: "enum", enum: ["lead"], default: "lead" }]),
      {},
    );

    expect(values.data).toEqual({ stage: "lead" });
  });
});

describe("enum options", () => {
  // The select renders each choice as JSON, and read the choice back with
  // `JSON.parse`. A field the record does not carry holds "" — no option
  // matches it, the hidden native select Radix keeps inside a form then
  // reports the empty choice, and parsing "" threw an uncaught
  // SyntaxError over the whole page.
  it("reads an empty choice back as an empty value instead of throwing", () => {
    expect(FormSchemaHelper.toEnumOption("")).toBe("");
    expect(FormSchemaHelper.toEnumOption(undefined)).toBe("");
    expect(FormSchemaHelper.fromEnumOption("")).toBe("");
    expect(FormSchemaHelper.fromEnumOption("not json")).toBe("");
  });

  it("carries a chosen value there and back", () => {
    expect(FormSchemaHelper.toEnumOption("lead")).toBe('"lead"');
    expect(FormSchemaHelper.fromEnumOption('"lead"')).toBe("lead");
    expect(FormSchemaHelper.toEnumOption(3)).toBe("3");
    expect(FormSchemaHelper.fromEnumOption("3")).toBe(3);
    expect(FormSchemaHelper.fromEnumOption("false")).toBe(false);
  });
});
