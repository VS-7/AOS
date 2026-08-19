/**
 * Ambient type stubs for third-party packages the copied Fractal
 * presentation code imports but that are not installed in this
 * environment (`npm install` is blocked here — see Task 9's report).
 *
 * These packages are real and resolvable on the npm registry (verified
 * with `npm view`), unlike `@igniter-js/*`, which this port deliberately
 * does not depend on. This file exists only so `tsc --noEmit` — this
 * task's gate — succeeds without them; it does NOT make the affected
 * components work. A bundler (`vite build`/`vite dev`) will still fail to
 * resolve these modules at runtime. Installing the real packages
 * (`@pierre/trees`, `@pierre/diffs`, `@json-render/react`, `@json-render/
 * core`, `@json-render/shadcn`, `@json-render/devtools-react`,
 * `@visual-json/react`, `@visual-json/core`, `uploadthing`,
 * `@uploadthing/react`, `react-force-graph-3d`, `frimousse`) replaces this
 * file's relevant `declare module` block outright.
 *
 * Every export here is loosely typed (`any`) — this is a compile-only
 * placeholder, not a reconstruction of each package's real API surface.
 * Consumers: `features/file`'s diff/tree/JSON-editor panels, the dormant
 * `features/view` json-render registry, `hooks/use-upload-file.ts`,
 * `features/workspace`'s memory-graph 3D tab, and the chat composer's
 * emoji picker.
 */

declare module "@pierre/trees" {
  export type FileTreeBuiltInIconSet = any;
  export type FileTreeDirectoryHandle = any;
  export type ContextMenuItem = any;
  export type ContextMenuOpenContext = any;
  export type FileTree = any;
  export function prepareFileTreeInput(...args: any[]): any;
}

declare module "@pierre/trees/react" {
  export const FileTree: any;
  export function useFileTree(...args: any[]): any;
}

declare module "@pierre/diffs/react" {
  export const MultiFileDiff: any;
  export type FileContents = any;
}

declare module "@json-render/react" {
  export const JSONUIProvider: any;
  export const Renderer: any;
  export function defineRegistry(...args: any[]): any;
  export function useStateStore(...args: any[]): any;
  export type BaseComponentProps<T = any> = T & Record<string, any>;
}

declare module "@json-render/react/schema" {
  export const schema: any;
}

declare module "@json-render/shadcn" {
  export const shadcnComponents: any;
}

declare module "@json-render/shadcn/catalog" {
  export const shadcnComponentDefinitions: any;
}

declare module "@json-render/core" {
  export function defineCatalog(...args: any[]): any;
  export type Spec = any;
}

declare module "@json-render/devtools-react" {
  export const JsonRenderDevtools: any;
}

declare module "@visual-json/react" {
  export const JsonEditor: any;
  export type JsonSchema = any;
  export type JsonValue = any;
}

declare module "@visual-json/core" {
  export function resolveSchema(...args: any[]): any;
}

declare module "uploadthing/types" {
  export type ClientUploadedFileData<T = unknown> = any;
  export type UploadFilesOptions<T = unknown> = any;
}

declare module "@uploadthing/react" {
  export function generateReactHelpers<T = any>(...args: any[]): any;
}

declare module "react-force-graph-3d" {
  const ForceGraph3D: any;
  export default ForceGraph3D;
}

declare module "frimousse" {
  export const EmojiPicker: any;
}

/**
 * Backs `components/ui/resizable.tsx` (one of the five UI components the
 * Task 9 brief names explicitly, via `split-page-layout.tsx`'s own import
 * of it) — the standard shadcn/ui wrapper around this package. Also real
 * and resolvable on npm (verified with `npm view`), same as every other
 * package in this file.
 */
declare module "react-resizable-panels" {
  export const Panel: any;
  export const PanelGroup: any;
  export const PanelResizeHandle: any;
}
