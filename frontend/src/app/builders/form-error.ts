/**
 * What `useForm` hands `onResponse` when a submission fails.
 *
 * A form's `onSubmit` rethrows whatever its call answered with, and not all
 * of those are Errors: the auth store's actions answer `{ error: { message
 * } }`, so the profile and password forms threw a plain object, and
 * `new Error(String(object))` turned the daemon's explanation into
 * "[object Object]". The message, and a code when there is one, survive.
 */
export function toSubmitError(err: unknown): Error {
  if (err instanceof Error) return err;
  if (err && typeof err === "object" && typeof (err as { message?: unknown }).message === "string") {
    const { message, code } = err as { message: string; code?: unknown };
    return Object.assign(new Error(message), typeof code === "string" ? { code } : {});
  }
  return new Error(String(err));
}
