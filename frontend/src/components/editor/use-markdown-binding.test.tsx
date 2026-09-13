import { afterEach, describe, expect, it } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { MarkdownPlugin } from "@platejs/markdown";
import { Plate, PlateContent, usePlateEditor, type PlateEditor } from "platejs/react";
import { useState } from "react";

import { useMarkdownBinding } from "./use-markdown-binding";

afterEach(cleanup);

let setFromOutside: (value: string) => void = () => {};
let rerenderParent: () => void = () => {};
let currentEditor: PlateEditor | null = null;

// A controlled editor the way every screen uses MarkdownEditor: the value
// lives in the parent (a react-hook-form field), and each edit comes back
// down as a new `markdown` prop.
function Controlled({ initial }: { initial: string }) {
  const [value, setValue] = useState(initial);
  const [, setTick] = useState(0);
  setFromOutside = setValue;
  rerenderParent = () => setTick((tick) => tick + 1);
  const editor = usePlateEditor({ plugins: [MarkdownPlugin] }, []);
  currentEditor = editor;
  const { handleChange } = useMarkdownBinding({ editor, markdown: value, onEdit: setValue });
  return (
    <>
      <Plate editor={editor} onChange={handleChange}>
        <PlateContent />
      </Plate>
      <output data-testid="value">{value}</output>
    </>
  );
}

function mountedEditor(): PlateEditor {
  const root = document.querySelector("[data-slate-editor]");
  expect(root, "the editor rendered no editable").not.toBeNull();
  // The editor the component mounted, handed out directly rather than read
  // back from the DOM through `slate-dom`, which this package does not depend on.
  return currentEditor as PlateEditor;
}

async function type(text: string) {
  for (const character of text) {
    await act(async () => {
      mountedEditor().tf.insertText(character);
    });
  }
}

const value = () => screen.getByTestId("value").textContent;

describe("useMarkdownBinding", () => {
  // Every published keystroke came back down as a "new" value, was taken for
  // an outside change, and replaced the whole document — which resets the
  // selection to its start. The next key landed there: "XYZ" typed at the
  // end of "Line one" was saved as "ZYLine oneX".
  it("keeps the caret where the person is typing", async () => {
    render(<Controlled initial="Line one" />);
    await act(async () => {});
    await act(async () => {
      const editor = mountedEditor();
      editor.tf.select(editor.api.end([]));
    });

    await type("XYZ");

    expect(value()).toBe("Line oneXYZ\n");
    expect(mountedEditor().api.string([])).toBe("Line oneXYZ");
  });

  // Plate renders no editable at all for a document with no blocks, and
  // Slate throws on every keystroke into one. A blank value used to become
  // exactly that, so a new goal, routine, agent or instruction had nowhere
  // to type its content.
  it("gives a blank value a paragraph to type into", async () => {
    render(<Controlled initial="" />);
    await act(async () => {});
    await act(async () => rerenderParent());

    const editor = mountedEditor();
    expect(editor.children).toHaveLength(1);
    await act(async () => {
      editor.tf.select(editor.api.start([]));
    });
    await type("Rev");

    expect(value()).toBe("Rev\n");
  });

  // An empty paragraph serializes as a zero-width space, which `trim()`
  // keeps: "Prompt is required" could never fire again once someone had
  // typed and deleted.
  it("publishes an emptied editor as an empty string", async () => {
    render(<Controlled initial="abc" />);
    await act(async () => {});
    await act(async () => {
      const editor = mountedEditor();
      editor.tf.select([]);
      editor.tf.deleteFragment();
    });

    expect(value()).toBe("");
  });

  it("shows a value that changed outside the editor", async () => {
    render(<Controlled initial="First" />);
    await act(async () => {});

    await act(async () => setFromOutside("Second"));
    expect(mountedEditor().api.string([])).toBe("Second");

    await act(async () => setFromOutside(""));
    expect(mountedEditor().children).toHaveLength(1);
    expect(mountedEditor().api.string([])).toBe("");
  });
});
