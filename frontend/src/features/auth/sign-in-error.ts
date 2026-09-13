import { DomainError, isDaemonUnreachable } from "@/lib/client";
import { t } from "@/lib/i18n";

/**
 * What the sign-in form says when signing in fails.
 *
 * The daemon's own sentence is English and lowercase — "those credentials do
 * not match an account" — and the form showed it verbatim, in a Portuguese
 * interface too. The two failures a person can actually do something about get
 * a sentence of their own; anything else is still the daemon's message, which
 * beats a shrug.
 */
export function signInErrorMessage(err: unknown): string {
  if (err instanceof DomainError && err.code === "AOS_AUTH_INVALID_CREDENTIALS") {
    return t("Those credentials do not match an account.");
  }
  if (isDaemonUnreachable(err)) {
    return t("The daemon is not answering. Try again in a moment.");
  }
  return err instanceof DomainError ? err.message : t("Something went wrong.");
}
