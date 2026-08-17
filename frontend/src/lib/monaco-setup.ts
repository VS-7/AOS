import { loader } from "@monaco-editor/react";
import * as monaco from "monaco-editor";
// monaco-editor's package.json exports map is `"./*": "./esm/vs/*.js"` — the
// specifier is everything AFTER esm/vs/, not including it, unlike older
// versions' docs (and their `monaco-editor/esm/vs/...` examples), which no
// longer match what 0.56 actually publishes.
import editorWorker from "monaco-editor/editor/editor.worker?worker";
import jsonWorker from "monaco-editor/language/json/json.worker?worker";
import cssWorker from "monaco-editor/language/css/css.worker?worker";
import htmlWorker from "monaco-editor/language/html/html.worker?worker";
import tsWorker from "monaco-editor/language/typescript/ts.worker?worker";

/**
 * Points `@monaco-editor/react` at the copy of Monaco already in this
 * bundle instead of the CDN it defaults to, and tells Monaco which worker
 * to spin up per language — Vite's `?worker` import is what turns each of
 * these into its own chunk, loaded lazily the first time that language is
 * actually opened.
 *
 * Importing this module is the side effect; it must run before the first
 * `<Editor>` mounts, which is why it's imported once at the top of the
 * files feature rather than inside the component itself.
 */
self.MonacoEnvironment = {
  getWorker(_workerId: string, label: string) {
    switch (label) {
      case "json":
        return new jsonWorker();
      case "css":
      case "scss":
      case "less":
        return new cssWorker();
      case "html":
      case "handlebars":
      case "razor":
        return new htmlWorker();
      case "typescript":
      case "javascript":
        return new tsWorker();
      default:
        return new editorWorker();
    }
  },
};

loader.config({ monaco });

export { monaco };
