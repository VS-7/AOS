import * as React from "react";
import { MarkdownPlugin } from "@platejs/markdown";
import type { PlateEditor } from "platejs/react";

interface MarkdownBindingOptions {
  editor: PlateEditor;
  /** The markdown the editor should show. */
  markdown: string;
  /** Called with the editor's markdown after a person edits it. */
  onEdit: (markdown: string) => void;
}

/**
 * Binds a Plate editor to a controlled markdown string.
 *
 * Every screen hands MarkdownEditor a react-hook-form field, so each edit
 * travels up as `onEdit` and straight back down as a new `markdown` prop.
 * The one rule that makes that safe: the document is replaced only when the
 * markdown differs from what the editor already holds. Replacing it resets
 * the selection to the start, and doing that for the echo of the person's
 * own keystroke is what wrote "XYZ", typed at the end of "Line one", as
 * "ZYLine oneX" — on disk, after Save.
 */
export function useMarkdownBinding({ editor, markdown, onEdit }: MarkdownBindingOptions) {
  const isSyncingRef = React.useRef(false);
  // The last `markdown` prop acted on; null until the first sync.
  const lastMarkdownRef = React.useRef<string | null>(null);
  // What the editor's document serializes to right now.
  const heldRef = React.useRef<string | null>(null);

  const readHeld = React.useCallback(
    () =>
      // An empty document serializes as a zero-width space, which `trim()`
      // keeps, so a required field emptied by hand could never be reported
      // as empty again.
      editor.api.isEmpty() ? "" : editor.getApi(MarkdownPlugin).markdown.serialize(),
    [editor],
  );

  const sync = React.useCallback(
    (next: string) => {
      const nodes = next.trim() ? editor.getApi(MarkdownPlugin).markdown.deserialize(next) : [];
      // Plate fires onChange for a programmatic replace; that is not an edit.
      isSyncingRef.current = true;
      try {
        // `setValue`, not `replaceNodes` with the nodes as they come: a blank
        // value (or markdown that deserializes to nothing) would leave the
        // document with no blocks, and Plate renders no editable for that —
        // a new goal, routine or agent had nowhere to type — while Slate
        // throws on every keystroke into it. `setValue` puts in an empty
        // paragraph instead.
        editor.tf.setValue(nodes);
      } finally {
        isSyncingRef.current = false;
      }
      heldRef.current = readHeld();
    },
    [editor, readHeld],
  );

  React.useEffect(() => {
    if (markdown === lastMarkdownRef.current) return;
    lastMarkdownRef.current = markdown;
    if (markdown === heldRef.current) return;
    sync(markdown);
  }, [markdown, sync]);

  const handleChange = React.useCallback(() => {
    if (isSyncingRef.current) return;
    const held = readHeld();
    // Selection moves and round-trip echoes change nothing worth publishing.
    if (held === heldRef.current) return;
    heldRef.current = held;
    onEdit(held);
  }, [onEdit, readHeld]);

  return { handleChange };
}
