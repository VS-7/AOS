/**
 * Minimal local declaration, same spirit as
 * `core/interfaces/response.interfaces.ts`'s own doc comment.
 *
 * The real source (`_extracted/v401/web/src/@core/errors/fractal.error.ts`)
 * builds `AppError` via `FractalError.create().merge(...).build()` —
 * a registry class assembled from all 26 features' own `errors/*.errors.ts`
 * files, itself built on `@core/builders/error.builder.ts`'s
 * `@igniter-js/core`-coupled base. Task 9 deliberately did not copy any
 * `errors/` folder (30 files, all importing that same coupled builder, none
 * of them imported by the UI on their own) — recovering this file
 * faithfully would mean pulling that whole tree back in.
 *
 * Every consumer of this file only does `error instanceof AppError`
 * then reads `.message` — so a plain `Error` subclass is enough to
 * typecheck and behave correctly for that one pattern. `code`/`cta` are
 * kept as optional fields since some call sites may grow to read them;
 * nothing currently constructs a `AppError` instance from the
 * frontend (the facade forwards `error` as a plain `EnvelopeError` object,
 * not this class — see `lib/aos-facade.ts`), so this class is only ever
 * the *type* on the `instanceof` checks' unreachable branch today.
 */
export class AppError extends Error {
  code?: string;
  cta?: unknown;

  constructor(message: string, code?: string) {
    super(message);
    this.name = "AppError";
    this.code = code;
  }
}
