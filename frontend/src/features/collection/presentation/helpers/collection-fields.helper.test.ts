import { describe, expect, it } from "vitest";
import {
  cleanRecordData,
  fieldsToJsonSchema,
  recordLabel,
  requiredFieldCount,
  type CollectionField,
} from "./collection-fields.helper";

const contacts: CollectionField[] = [
  { name: "name", type: "string", required: true },
  { name: "email", type: "string", description: "Where to write" },
  { name: "age", type: "number" },
  { name: "active", type: "boolean" },
  { name: "stage", type: "enum", enum: ["lead", "customer"] },
  { name: "birthday", type: "date" },
  { name: "tags", type: "list" },
  { name: "company", type: "ref", ref: "companies" },
];

describe("fieldsToJsonSchema", () => {
  // The record form read `collection.schema`, a JSON Schema the daemon never
  // sends — it declares `fields` — so every form rendered "no declared
  // properties" and a record could not be created at all.
  it("describes every declared field in the shape the form renders", () => {
    const schema = fieldsToJsonSchema(contacts);

    expect(schema.required).toEqual(["name"]);
    expect(schema.properties).toMatchObject({
      name: { type: "string", title: "name" },
      email: { type: "string", description: "Where to write" },
      age: { type: "number" },
      active: { type: "boolean" },
      stage: { type: "string", enum: ["lead", "customer"] },
      birthday: { type: "string", format: "date" },
      tags: { type: "array", items: { type: "string" } },
      company: { type: "string" },
    });
    expect(Object.keys(schema.properties)).toEqual(contacts.map((field) => field.name));
    expect(requiredFieldCount(contacts)).toBe(1);
  });
});

describe("cleanRecordData", () => {
  // The daemon refuses a field it does not declare, an enum value outside its
  // list and a date that is not RFC 3339. A form's untouched inputs are empty
  // strings, and saving the record envelope sent `collection`, `createdAt`…
  // as fields — every save was refused.
  it("sends only declared fields, and leaves out the ones left empty", () => {
    const cleaned = cleanRecordData(contacts, {
      name: "Ada",
      email: "",
      age: undefined,
      active: false,
      stage: "",
      birthday: "",
      tags: ["math", "", "poet"],
      collection: "contacts",
      createdAt: "0001-01-01T00:00:00Z",
    });

    expect(cleaned).toEqual({ name: "Ada", active: false, tags: ["math", "poet"] });
  });

  it("keeps a number of zero and turns a typed number back into one", () => {
    expect(cleanRecordData(contacts, { name: "A", age: 0 })).toEqual({ name: "A", age: 0 });
    expect(cleanRecordData(contacts, { name: "A", age: "41" })).toEqual({ name: "A", age: 41 });
  });

  it("drops an empty list rather than sending one the daemon has to store", () => {
    expect(cleanRecordData(contacts, { name: "A", tags: [] })).toEqual({ name: "A" });
  });

  // The form seeds every declared field, so an untouched checkbox reads
  // `false` like one the reader deliberately left empty. Saving a record
  // nobody edited must not write a field into it that it never carried.
  it("leaves out a box nobody ticked on a record that never had the field", () => {
    expect(
      cleanRecordData(contacts, { name: "A", active: false }, { name: "A" }),
    ).toEqual({ name: "A" });
  });

  it("keeps a box turned off on a record that carries the field", () => {
    expect(
      cleanRecordData(contacts, { name: "A", active: false }, { name: "A", active: true }),
    ).toEqual({ name: "A", active: false });
  });

  it("keeps a box the reader ticked", () => {
    expect(cleanRecordData(contacts, { name: "A", active: true }, {})).toEqual({
      name: "A",
      active: true,
    });
  });
});

describe("recordLabel", () => {
  // The record page's title was the record's UUID, "prettified" into words.
  it("names a record by its first text field that holds something", () => {
    expect(recordLabel(contacts, { name: "Ada Lovelace", email: "ada@x.io" })).toBe("Ada Lovelace");
    expect(recordLabel(contacts, { name: "", email: "ada@x.io" })).toBe("ada@x.io");
    expect(recordLabel(contacts, { age: 3 })).toBeUndefined();
  });
});
