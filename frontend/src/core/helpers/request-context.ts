/**
 * Minimal local declaration, same spirit as `core/interfaces/response.
 * interfaces.ts`.
 *
 * The real source (`_extracted/v401/server/src/@core/helpers/request-
 * context.ts`) is a Node.js server-side `AsyncLocalStorage` wrapper
 * (`node:async_hooks`) that resolves the ambient caller identity for a
 * request/job — backend-only machinery a Wails renderer process cannot run
 * and does not need. `features/config/interfaces/config.interfaces.ts`
 * (this file's only consumer) imports just the `RequestActor` type for one
 * field's annotation, not the `RequestContext` class, so only the type is
 * reproduced here.
 */
export type RequestActor =
  | { type: "agent"; data: { id: string } }
  | { type: "user"; data: { id: string } };
