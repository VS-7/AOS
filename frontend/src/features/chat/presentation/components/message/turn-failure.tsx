import * as React from "react";
import { AlertTriangle } from "lucide-react";

/**
 * One recorded attempt to answer a message
 * (`internal/domain/chat/entity.go`'s `Run`).
 *
 * The daemon has always written these — a failed turn appends a `Run` with
 * `status: "error"` and a `{code, message}` onto the *user* message that
 * asked for it, which is why they arrive here rather than as an assistant
 * message. Nothing in the ported interface read them, so every failure was
 * invisible: the message sat in the conversation with no answer and no
 * reason, and the only record was a line in the daemon's log the person
 * cannot see.
 *
 * Deliberately not an assistant message: a failure is not something the
 * agent said, and inventing a message for it would put the error text into
 * the transcript the model reads on the next turn.
 */
export interface ChatMessageRun {
  agentId?: string;
  status?: string;
  error?: { code?: string; message?: string } | null;
}

/**
 * The failure to show for a message, or `null` when there is none.
 *
 * Only the *last* attempt counts. A message that failed and was then
 * answered on a retry has both runs recorded, and showing the old error
 * next to a successful answer would be a lie about the current state.
 */
export function latestFailure(runs: ChatMessageRun[] | undefined): ChatMessageRun | null {
  const last = runs?.[runs.length - 1];
  return last?.status === "error" ? last : null;
}

/** Reads the runs off a message, which the AI-SDK `UIMessage` type has no field for. */
export function runsOf(message: unknown): ChatMessageRun[] | undefined {
  return (message as { runs?: ChatMessageRun[] } | null | undefined)?.runs;
}

export function ChatTurnFailure({ run }: { run: ChatMessageRun }) {
  const message = run.error?.message?.trim();
  const code = run.error?.code?.trim();

  return (
    <div className="px-6 py-1.5" role="alert">
      <div className="flex items-start gap-2 rounded-lg border border-destructive/20 bg-destructive/5 px-3 py-2">
        <AlertTriangle className="mt-0.5 size-3.5 shrink-0 text-destructive" />
        <div className="min-w-0 flex-1">
          <p className="text-xs font-medium text-destructive">
            {run.agentId ? `${run.agentId} could not answer` : "The agent could not answer"}
          </p>
          {message ? (
            <p className="mt-0.5 text-xs break-words text-muted-foreground">{message}</p>
          ) : null}
          {code ? (
            <p className="mt-1 font-mono text-[10px] text-muted-foreground/70">{code}</p>
          ) : null}
        </div>
      </div>
    </div>
  );
}

export default ChatTurnFailure;
