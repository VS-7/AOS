import { api, errorMessage } from "@/lib/aos-facade";
import { t } from "@/lib/i18n";

/** `routines_fire`'s refusal when the routine did run, and the run failed. */
export const ROUTINE_RUN_FAILED_CODE = "AOS_ROUTINE_RUN_FAILED";

interface FireFailure {
  title: string;
  description?: string;
}

/**
 * What to tell a person whose "Run now" did not work.
 *
 * A refusal to start (the routine is disabled, there is no runtime) says why
 * in its own message. A run that started and failed does not:
 * `AOS_ROUTINE_RUN_FAILED` reads only "the routine ran and failed", because
 * the cause it wraps — the provider that did not answer, the timeout — is not
 * part of the error on the wire. The run it recorded keeps that sentence in
 * `error`, and runs are listed newest first, so the reason is one read away.
 * Without it the only trace of why was a separate "<name> failed" activity
 * toast with no reason at all.
 */
export async function describeFireFailure(routineId: string, error: unknown): Promise<FireFailure> {
  const message = errorMessage(error);
  const code = (error as { code?: unknown } | null | undefined)?.code;
  if (code !== ROUTINE_RUN_FAILED_CODE) {
    return { title: t("Failed to start routine"), description: message };
  }

  const runs = await api.routine!.runs!.query<{ runs?: Array<{ error?: string }> }>({
    params: { routine: routineId },
    query: { limit: 1 },
  });
  return {
    title: t("The routine ran and failed."),
    description: runs.data?.runs?.[0]?.error || message,
  };
}
