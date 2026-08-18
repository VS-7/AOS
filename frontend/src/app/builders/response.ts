import { redirect, notFound } from "@tanstack/react-router";

/**
 * Standardized response helper for Aos procedures and loaders.
 */
export class AosResponse {
  /**
   * Triggers a navigation redirect.
   * 
   * @param url - The destination path or URL.
   * @param status - Optional HTTP status code for the redirect.
   * @throws A redirect command that is caught by the TanStack Router.
   * 
   * @example
   * ```typescript
   * response.redirect("/login");
   * ```
   */
  redirect(url: string, status?: number): never {
    throw redirect({ to: url as any, statusCode: status });
  }

  /**
   * Triggers a 404 Not Found response.
   * @throws A notFound command that is caught by the TanStack Router.
   * 
   * @example
   * ```typescript
   * response.notFound();
   * ```
   */
  notFound(): never {
    throw notFound();
  }
}
