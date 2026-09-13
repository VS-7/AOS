import { describe, expect, it } from "vitest";
import { createSlateEditor } from "platejs";
import { BaseBasicBlocksPlugin, BaseBasicMarksPlugin } from "@platejs/basic-nodes";
import { MarkdownPlugin } from "@platejs/markdown";

import { MarkdownKit } from "./markdown-kit";

function roundTrip(markdown: string) {
  const editor = createSlateEditor({ plugins: [BaseBasicBlocksPlugin, BaseBasicMarksPlugin, ...MarkdownKit] });
  const api = editor.getApi(MarkdownPlugin).markdown;
  editor.tf.setValue(api.deserialize(markdown));
  return { editor, text: editor.api.string([]), markdown: api.serialize() };
}

// Agent instructions, routine prompts and instruction bodies are prompts, and
// the runtime renders `{{ }}` in them as Liquid when a section asks for it.
// Under MDX a `{…}` is a JavaScript expression, which Plate has no node for:
// it was dropped on load and gone from the file after the next save.
describe("MarkdownKit braces", () => {
  it.each([
    ["a Liquid output tag", "Hi {{ name }}"],
    ["a single-brace placeholder", "Use {placeholder} here"],
    ["a JSON example", 'Return JSON like {"a": 1} please'],
    ["a Liquid block on its own line", "{% if version %}Version {{ version }}{% endif %}"],
  ])("keeps %s through a load and a save", (_label, source) => {
    const loaded = roundTrip(source);
    expect(loaded.text).toBe(source);
    expect(loaded.markdown).toBe(`${source}\n`);
    expect(roundTrip(loaded.markdown).markdown).toBe(`${source}\n`);
  });

  it("keeps a paragraph that is only an expression", () => {
    const { text, markdown } = roundTrip("Intro\n\n{{ summary }}\n\nOutro");
    expect(text).toContain("{{ summary }}");
    expect(markdown).toBe("Intro\n\n{{ summary }}\n\nOutro\n");
  });

  it("still drops an HTML comment rather than showing it as a JSX one", () => {
    expect(roundTrip("A <!-- note --> B").markdown).toBe("A  B\n");
    expect(roundTrip("<!-- note -->\n\nText").markdown).toBe("Text\n");
  });

  // What MDX is here for: marks that markdown has no syntax for.
  it("still writes underline as MDX", () => {
    const editor = createSlateEditor({ plugins: [BaseBasicBlocksPlugin, BaseBasicMarksPlugin, ...MarkdownKit] });
    editor.tf.setValue([{ type: "p", children: [{ text: "under", underline: true }] }]);
    const markdown = editor.getApi(MarkdownPlugin).markdown.serialize();
    expect(markdown).toBe("<u>under</u>\n");
    expect(roundTrip(markdown).markdown).toBe("<u>under</u>\n");
  });
});
