/**
 * Review round 2 fix: this file used to build its own `AosStore.router({
 * prefix: "fractal", stores: {...} })` registry straight from the pristine
 * per-feature store singletons (`WorkspaceStore`, `AuthStore`, `ProjectStore`,
 * `GoalStore`, plus `ArtifactStore`/`RealtimeStore`/`ViewStore`/`RoutineStore`
 * nothing reads through this object — confirmed by grepping every
 * `aos.stores.X`/`stores.X` access in the ported code before removing them).
 *
 * That was a second, never-initialized source of truth for `workspace`
 * (and `auth`/`projects`/`goals`) alongside `app/stores.ts`'s own — see
 * that file's `stores` export for the full incident writeup. This file now
 * just re-exports the one registry `app/aos.tsx` actually binds via
 * `.withStores(...)`, so every consumer that imports `{ stores } from
 * "@/app/lib/stores"` (`layout/index.tsx`, the sidebar menu group files,
 * `request-close-file-tab.helper.ts`, …) reads the same, actually-
 * initialized store instances `aos.stores.X` does.
 */
export { stores } from "@/app/stores";
