import React, { startTransition, useMemo, useRef } from "react";
import Editor, { loader } from "@monaco-editor/react";
import { LoaderCircle } from "lucide-react";
import { aos } from "@/app/aos";

import type { WorkspaceFile } from "@/features/file/interfaces/file.interfaces";
import { buildMonacoTheme, resolveMonacoLanguage } from "../../helpers/files-editor.helper";

/**
 * The one divergence from the original in this file, and a deliberate one.
 *
 * The original ran `loader.config({ paths: { vs: "/monaco/vs" } })` here,
 * pointing at a path its Electron main process served. AOS ships Monaco
 * inside the bundle instead — see the "Monaco no bundle" decision in
 * `docs/05 - Transporte/Artifacts e Estáticos.md`, which takes it as removing
 * both a transport and a path-traversal surface. Kept verbatim, that call
 * asked this app for a URL it does not serve (there is no `public/`), so the
 * loader never resolved and the editor never appeared — while
 * `lib/monaco-setup.ts`, the module that implements the decision, sat
 * unimported.
 *
 * Importing that module *is* the configuration: it calls `loader.config({
 * monaco })` with the bundled copy and installs `MonacoEnvironment`'s
 * per-language worker factory.
 *
 * Dynamic rather than static, and cached in a module-level promise: Monaco is
 * ~10 MB, and a static import pulls all of it into the entry chunk — enough,
 * with source maps on, to take `vite build` out of memory. Lazily it is its
 * own chunk, fetched the first time somebody opens a file. The cost of that
 * is ordering, which is what `monacoReady` below exists to enforce:
 * `loader.config` throws once `loader.init` has run, and `<Editor>` calls
 * `loader.init` itself as it mounts. So nothing renders `<Editor>` until this
 * promise has resolved.
 */
const monacoReady: Promise<unknown> =
  typeof window === "undefined" ? Promise.resolve() : import("@/lib/monaco-setup");

interface FilesTextViewerProps {
  content: string;
  file: WorkspaceFile;
  isLoading?: boolean;
  readOnly?: boolean;
  onChange: (value: string) => void;
}

export function FilesTextViewer({
  content,
  file,
  isLoading,
  readOnly = false,
  onChange,
}: FilesTextViewerProps) {
  const [isEditorReady, setIsEditorReady] = React.useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const editorRef = useRef<Parameters<NonNullable<React.ComponentProps<typeof Editor>["onMount"]>>[0] | null>(null);
  const themeState = aos.stores.theme.useState();
  const language = useMemo(() => resolveMonacoLanguage(file), [file.extension, file.name]);
  const monacoTheme = useMemo(() => buildMonacoTheme(themeState), [themeState]);
  /** Slightly below UI/code settings — Monaco can feel oversized next to 13px sans UI. */
  const editorFontSize = useMemo(() => {
    const raw = monacoTheme.options.fontSize;
    const n = typeof raw === "number" && Number.isFinite(raw) ? raw : 12;
    const scaled = Math.round(n * 0.92);
    return Math.min(12, Math.max(10, scaled));
  }, [monacoTheme.options.fontSize]);

  const editorLineHeight = useMemo(() => Math.round(editorFontSize * 1.5), [editorFontSize]);

  React.useEffect(() => {
    let cancelled = false;
    void monacoReady.then(() => {
      if (!cancelled) setIsEditorReady(true);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  React.useEffect(() => {
    // Chained off `monacoReady` for the same reason `<Editor>` is: `loader
    // .init()` is what freezes the configuration, so it must not run first.
    void monacoReady
      .then(() => loader.init())
      .then((instance) => {
        instance.editor.defineTheme(monacoTheme.name, monacoTheme.theme);
        instance.editor.setTheme(monacoTheme.name);
      });
  }, [monacoTheme]);

  React.useEffect(() => {
    editorRef.current?.updateOptions({
      fontSize: editorFontSize,
      lineHeight: editorLineHeight,
    });
  }, [editorFontSize, editorLineHeight]);

  React.useEffect(() => {
    const container = containerRef.current;

    if (!container || typeof ResizeObserver === "undefined") return;

    let frameId = 0;
    const observer = new ResizeObserver(() => {
      if (frameId) cancelAnimationFrame(frameId);

      frameId = requestAnimationFrame(() => {
        editorRef.current?.layout();
      });
    });

    observer.observe(container);

    return () => {
      if (frameId) cancelAnimationFrame(frameId);
      observer.disconnect();
    };
  }, []);

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center gap-2 text-sm text-muted-foreground">
        <LoaderCircle className="size-4 animate-spin" />
        Loading file content...
      </div>
    );
  }

  const lines = content ? content.split("\n").length : 1;

  return (
    <div ref={containerRef} className="grid h-full grid-rows-[1fr_auto]">
      {isEditorReady ? (
      <Editor
        beforeMount={(instance) => {
          instance.editor.defineTheme(monacoTheme.name, monacoTheme.theme);
        }}
        height="100%"
        language={language}
        loading={
          <div className="flex h-full items-center justify-center gap-2 text-sm text-muted-foreground">
            <LoaderCircle className="size-4 animate-spin" />
            Loading editor...
          </div>
        }
        onChange={(value) => {
          if (readOnly) return;
          startTransition(() => {
            onChange(value ?? "");
          });
        }}
        onMount={(editor) => {
          editorRef.current = editor;
          requestAnimationFrame(() => {
            editor.layout();
          });
        }}
        options={{
          automaticLayout: false,
          bracketPairColorization: { enabled: true },
          cursorBlinking: "smooth",
          cursorSmoothCaretAnimation: "on",
          folding: false,
          fontFamily: monacoTheme.options.fontFamily,
          fontLigatures: true,
          fontSize: editorFontSize,
          glyphMargin: false,
          lineDecorationsWidth: 6,
          lineHeight: editorLineHeight,
          lineNumbersMinChars: 3,
          minimap: { enabled: lines > 40 },
          padding: { bottom: 8, top: 4 },
          quickSuggestions: !readOnly,
          readOnly,
          renderLineHighlight: "all",
          roundedSelection: true,
          scrollBeyondLastLine: false,
          scrollbar: {
            horizontalScrollbarSize: 10,
            verticalScrollbarSize: 10,
          },
          smoothScrolling: true,
          stickyScroll: { enabled: true },
          tabSize: 2,
          wordWrap: "on",
        }}
        path={file.path}
        saveViewState
        theme={monacoTheme.name}
        value={content}
      />
      ) : (
        // The same affordance `<Editor>`'s own `loading` prop renders, shown
        // for the moment before it while the Monaco chunk arrives.
        <div className="flex h-full items-center justify-center gap-2 text-sm text-muted-foreground">
          <LoaderCircle className="size-4 animate-spin" />
          Loading editor...
        </div>
      )}

      <div className="flex items-center justify-between border-t bg-muted/20 px-4 py-2 text-[11px] text-muted-foreground">
        <span>{file.path}</span>
        <span>{lines} lines</span>
      </div>
    </div>
  );
}
