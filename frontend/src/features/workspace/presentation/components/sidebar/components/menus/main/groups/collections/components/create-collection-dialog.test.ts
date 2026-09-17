import { describe, expect, it, vi } from "vitest";

vi.mock("@/app/aos", () => ({ aos: {} }));

const { draftToFields, slugifyCollectionId } = await import("./create-collection-dialog");

describe("slugifyCollectionId", () => {
  // The id is the collection's directory; the daemon refuses anything but
  // lowercase letters, digits, hyphen and underscore.
  it("turns a human name into an id the daemon accepts", () => {
    expect(slugifyCollectionId("Meeting notes")).toBe("meeting-notes");
    expect(slugifyCollectionId("Reuniões  2026!")).toBe("reunioes-2026");
  });
});

describe("draftToFields", () => {
  it("sends named rows only, with enum values and ref targets where the type has them", () => {
    expect(
      draftToFields([
        { name: "name", type: "string", required: true, options: "" },
        { name: "", type: "number", required: false, options: "" },
        { name: "stage", type: "enum", required: false, options: "lead, won ,," },
        { name: "company", type: "ref", required: false, options: " companies " },
      ]),
    ).toEqual([
      { name: "name", type: "string", required: true },
      { name: "stage", type: "enum", enum: ["lead", "won"] },
      { name: "company", type: "ref", ref: "companies" },
    ]);
  });
});
