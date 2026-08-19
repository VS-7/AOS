/**
 * Ambient type stubs for third-party packages the copied Fractal
 * presentation code imports.
 *
 * Review round 2: nine of the original twelve packages here
 * (`@pierre/trees`, `@pierre/trees/react`, `@pierre/diffs/react`,
 * `@visual-json/react`, `@visual-json/core`, `uploadthing/types`,
 * `@uploadthing/react`, `react-force-graph-3d`, `frimousse`,
 * `react-resizable-panels`) are now actually installed (the reviewer had
 * install permission Task 9 did not) — their `declare module` blocks are
 * gone. An ambient module declaration resolves ahead of `node_modules`,
 * so leaving them in place after install would have kept every consumer
 * typed `any` while silently hiding real type errors — that was flagged
 * and fixed (see the fix report for what deleting each block surfaced).
 *
 * `@json-render/*` (`react`, `react/schema`, `shadcn`, `shadcn/catalog`,
 * `core`, `devtools-react`) stay stubbed deliberately, not because
 * installation failed: `@json-render/shadcn` requires zod 4, and this
 * project is on zod 3 (`package.json`) — installing it would force a zod
 * major-version bump across every one of the ~61 files that build schemas
 * with `zod`, for a feature (`features/view`'s declarative renderer) that
 * has no Go backend yet and cannot run regardless (`view.*` is dormant in
 * `lib/command-map.ts`). `tsc --noEmit` is the gate; a bundler will still
 * fail to resolve these six until that tradeoff is revisited.
 *
 * Every export here is loosely typed (`any`) — compile-only, not a
 * reconstruction of the real API surface. Consumers are all inside the
 * dormant `features/view/` json-render registry (13 files).
 */

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
