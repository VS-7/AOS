/**
 * Minimal local declaration. The real `@core/interfaces/response.interfaces`
 * (with its full CTA/response contract) is reconstructed in a later task;
 * this shape is just enough for `ResponseWithCTA<T>` to typecheck where the
 * ported `task` feature references it.
 */
export interface ResponseWithCTA<TData = unknown> {
  data?: TData;
  error?: { code?: string; message?: string };
  cta?: Array<{ label: string; command?: string; tool?: string }>;
}
