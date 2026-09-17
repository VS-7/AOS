import * as React from "react";
import { AlertTriangle } from "lucide-react";

import { aos } from "@/app/aos";
import { Button } from "@/components/ui/button";
import type { SettingsSectionId } from "@/features/workspace/presentation/components/settings/constants";
import { t } from "@/lib/i18n";

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
  error?: { code?: string; message?: string; cta?: RunCallToAction[] | null } | null;
}

/** One step out of a failure, as Go's `apperr.CallToAction` serializes it. */
export interface RunCallToAction {
  label?: string;
  command?: string;
  tool?: string;
  input?: unknown;
}

/**
 * The screen in this window where a call to action's tool is done by hand, or
 * `null` when there is none.
 *
 * The tool names are what the daemon attaches to a failure (`config_update`
 * and `models_list` for a model slot, `agents_update` for an agent's own
 * model). A person cannot run a tool, but they can open the settings that make
 * the same change — which is the one thing the card could not tell them.
 */
export function settingsSectionFor(cta: RunCallToAction): SettingsSectionId | null {
  switch (cta.tool) {
    case "config_update":
    case "models_list":
      return "user.agents";
    case "agents_update":
      return "workspace.agents";
    default:
      return null;
  }
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

/**
 * A failure this window can explain better than the daemon's advice does.
 *
 * The daemon's call to action is written for whoever holds the tools — "point
 * the default slot at a provider and a model in .aos/config.json" — in
 * English. For the failures a person meets first, before any provider is set
 * up or after a key stops working, the fix is a screen in this window, and
 * the card says so in the interface's language and opens it.
 */
export function explainFailure(
  run: ChatMessageRun,
): { text: string; section: SettingsSectionId } | null {
  const code = run.error?.code?.trim();
  const message = run.error?.message ?? "";
  switch (code) {
    case "AOS_AGENT_PROVIDER_NOT_ENABLED":
    case "AOS_AGENT_NO_PROVIDER":
      return {
        text: t("No AI provider is connected for this agent. Connect one in Settings › AI Providers and choose a model."),
        section: "user.agents",
      };
    case "AOS_OAUTH_FILE_MISSING":
      return {
        text: t("This provider's sign-in is missing. Sign in again in Settings › AI Providers."),
        section: "user.agents",
      };
    case "AOS_AGENT_PROVIDER_FAILED":
      // Only a refused credential: a timeout or a rate limit is not fixed on
      // that screen, and sending the person there would be a wrong answer.
      if (/\b(401|403)\b|unauthori[sz]ed|forbidden|api[ -]?key/i.test(message)) {
        return {
          text: t("The AI provider refused the credentials. Check the key, or sign in again, in Settings › AI Providers."),
          section: "user.agents",
        };
      }
      return null;
    default:
      return null;
  }
}

export function ChatTurnFailure({
  run,
  agentName,
}: {
  run: ChatMessageRun;
  /** The agent's display name. The run records only its id — a slug. */
  agentName?: string;
}) {
  const message = run.error?.message?.trim();
  const code = run.error?.code?.trim();
  const explained = explainFailure(run);
  // The daemon's own advice, unless the explanation above replaces it.
  const actions = explained
    ? []
    : (run.error?.cta ?? []).filter((cta) => cta?.label?.trim());
  // The first action that has a screen, because the order is the daemon's:
  // the most specific advice comes first.
  const section =
    explained?.section ??
    actions.map(settingsSectionFor).find((found) => found !== null) ??
    null;
  const agent = agentName?.trim() || run.agentId;

  return (
    <div className="px-6 py-1.5" role="alert">
      <div className="flex items-start gap-2 rounded-lg border border-destructive/20 bg-destructive/5 px-3 py-2">
        <AlertTriangle className="mt-0.5 size-3.5 shrink-0 text-destructive" />
        <div className="min-w-0 flex-1">
          <p className="text-xs font-medium text-destructive">
            {agent
              ? t("{{agent}} could not answer", { agent })
              : t("The agent could not answer")}
          </p>
          {explained ? (
            <p className="mt-0.5 text-xs break-words text-foreground/80">{explained.text}</p>
          ) : null}
          {message ? (
            <p className="mt-0.5 text-xs break-words text-muted-foreground">{message}</p>
          ) : null}
          {actions.length > 0 ? (
            <ul className="mt-1 space-y-0.5 text-xs break-words text-foreground/80">
              {actions.map((cta, index) => (
                <li key={index}>→ {cta.label}</li>
              ))}
            </ul>
          ) : null}
          {section ? (
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="mt-1.5 h-6 px-2 text-xs"
              onClick={() => aos.stores.viewport.actions.openSettings(section)}
            >
              {section === "workspace.agents" ? t("Open agent settings") : t("Open AI Providers")}
            </Button>
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
