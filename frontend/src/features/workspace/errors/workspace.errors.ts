/**
 * Minimal local declaration — same reasoning as `core/errors/fractal.
 * error.ts`. The real source (`_extracted/index/src/features/workspace/
 * workspace.errors.ts`) builds `WorkspaceError` via `FractalError.create().
 * addError(...).build()`, the same `@igniter-js/core`-coupled builder Task
 * 9 deliberately did not copy back in (see this port's `errors/`
 * exclusion note).
 *
 * `presentation/stores/workspace.store.ts` only ever constructs this with
 * `new FractalWorkspaceError({ code: "FRACTAL_WORKSPACE_NOT_FOUND" })` and
 * returns it as a store action's `error` field — nothing reads `.message`
 * off it today, so a plain `Error` subclass is enough.
 */
export class FractalWorkspaceError extends Error {
  code: string;

  constructor(params: { code: string; message?: string }) {
    super(params.message ?? params.code);
    this.name = "FractalWorkspaceError";
    this.code = params.code;
  }
}
