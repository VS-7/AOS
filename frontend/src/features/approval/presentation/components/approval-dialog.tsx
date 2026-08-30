import { useCallback, useEffect, useState } from "react";
import type { JSX } from "react";
import { toast } from "sonner";

import { client } from "@/lib/client";
import { t } from "@/lib/i18n";
import { useRealtime } from "@/hooks/use-realtime";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type {
  ApprovalRemember,
  ApprovalRequest,
  ApprovalRisk,
} from "@/features/approval/interfaces/approval.interfaces";

/**
 * The desktop end of the approval channel (ADR-0007).
 *
 * A hook can answer "ask" instead of allow or deny, and when it does the tool
 * call stops and waits for a person. The daemon had all of it — the broker,
 * `approvals_list`, `approvals_decide`, and an `approval.request` event on the
 * socket — and the interface had none: no screen called either command, so an
 * agent asking for permission waited out its deadline and was denied by
 * timeout. Every time, silently, with the chat showing a tool stuck mid-call
 * and no button anywhere that could answer it.
 *
 * It is mounted on the workspace layout rather than inside the chat because
 * the asking is not the chat's: a routine or a background task can be the one
 * waiting, and the person has to be able to answer wherever they happen to be.
 *
 * Requests are answered one at a time, oldest first, which is also the order
 * the daemon returns them in. A queue behind a modal is deliberate: these are
 * decisions, and a list of them side by side invites the one thing the master
 * prompt rules out — approving in bulk without reading.
 */
export function ApprovalDialog(): JSX.Element | null {
  const [pending, setPending] = useState<ApprovalRequest[]>([]);
  const [deciding, setDeciding] = useState(false);
  // `always` is a second, deliberate action: the first click arms it, the
  // second sends it. The master prompt is explicit that approving once does
  // not authorise an action in every future context, so the button that does
  // authorise it forever cannot be the same size as the one that does not.
  const [armedAlways, setArmedAlways] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const answer = (await client.invoke("approvals_list", {
        _reasoning: "the window is showing the tool calls waiting on a person",
      })) as { pending?: ApprovalRequest[] } | undefined;
      setPending(answer?.pending ?? []);
    } catch {
      // A daemon that is not answering has no waiting approvals to show. The
      // socket event brings this back the moment one arrives.
      setPending([]);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // Both halves of the event — asked and settled — mean the same thing here:
  // ask again what is waiting.
  useRealtime("approval:requested", () => {
    void refresh();
  });

  const current = pending[0];

  // A request nobody answers becomes a denial when its deadline passes, so the
  // dialog stops offering buttons for one that has already run out rather than
  // sending a decision the broker will refuse.
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!current) return;
    const tick = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(tick);
  }, [current]);

  useEffect(() => {
    setArmedAlways(false);
  }, [current?.id]);

  if (!current) return null;

  const expiresAt = Date.parse(current.expiresAt);
  const secondsLeft = Number.isNaN(expiresAt)
    ? null
    : Math.max(0, Math.round((expiresAt - now) / 1000));
  const expired = secondsLeft === 0;

  const decide = async (approved: boolean, remember: ApprovalRemember) => {
    setDeciding(true);
    try {
      const answer = (await client.invoke("approvals_decide", {
        id: current.id,
        approved,
        remember,
        _reasoning: "a person answered a tool call that was waiting on them",
      })) as { settled?: boolean } | undefined;

      if (answer?.settled === false) {
        // Nothing was waiting under that id — it expired while the dialog was
        // open. Saying "approved" here would be a lie: the call was denied
        // when the deadline passed.
        toast.error(t("This request had already expired — the call was refused."));
      }
      await refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("The decision could not be sent."));
    } finally {
      setDeciding(false);
      setArmedAlways(false);
    }
  };

  return (
    <Dialog open onOpenChange={() => undefined}>
      <DialogContent
        className="sm:max-w-lg"
        // Not dismissible: closing this without deciding looks like it went
        // away, and what actually happens is the agent waits out its deadline
        // and is refused. Deny is the way out, and it says so.
        onEscapeKeyDown={(event) => event.preventDefault()}
        onPointerDownOutside={(event) => event.preventDefault()}
        showCloseButton={false}
      >
        <DialogHeader>
          <div className="flex items-center gap-2">
            <DialogTitle>{t("An agent is asking permission")}</DialogTitle>
            <RiskBadge risk={current.risk} />
          </div>
          <DialogDescription>
            {current.agent
              ? t("{agent} wants to run {tool}.")
                  .replace("{agent}", current.agent)
                  .replace("{tool}", current.tool)
              : t("An agent wants to run {tool}.").replace("{tool}", current.tool)}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {current.reason ? (
            <div className="space-y-1">
              <p className="text-xs font-medium text-muted-foreground">{t("Why it is asking")}</p>
              <p className="text-sm">{current.reason}</p>
            </div>
          ) : null}

          <div className="space-y-1">
            <p className="text-xs font-medium text-muted-foreground">{t("What it would run")}</p>
            <pre className="max-h-52 overflow-auto rounded-lg border border-border/60 bg-muted/40 p-3 text-xs">
              <code>{formatInput(current.input)}</code>
            </pre>
          </div>

          <p className="text-xs text-muted-foreground">
            {expired
              ? t("This request has expired. Unanswered requests are refused, never allowed.")
              : t("Refused automatically in {seconds}s if nobody answers.").replace(
                  "{seconds}",
                  String(secondsLeft ?? 0),
                )}
          </p>

          {pending.length > 1 ? (
            <p className="text-xs text-muted-foreground">
              {t("{count} more waiting after this one.").replace(
                "{count}",
                String(pending.length - 1),
              )}
            </p>
          ) : null}
        </div>

        <DialogFooter className="gap-2 sm:justify-between">
          <Button
            variant="destructive"
            disabled={deciding || expired}
            onClick={() => decide(false, "none")}
          >
            {t("Deny")}
          </Button>
          <div className="flex gap-2">
            <Button
              variant="secondary"
              disabled={deciding || expired}
              onClick={() => {
                if (!armedAlways) {
                  setArmedAlways(true);
                  return;
                }
                void decide(true, "always");
              }}
            >
              {armedAlways ? t("Click again to always allow") : t("Always allow")}
            </Button>
            <Button disabled={deciding || expired} onClick={() => decide(true, "session")}>
              {t("Approve once")}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function RiskBadge({ risk }: { risk: ApprovalRisk }): JSX.Element {
  const label: Record<ApprovalRisk, string> = {
    low: t("Low risk"),
    medium: t("Medium risk"),
    high: t("High risk"),
  };
  return (
    <Badge variant={risk === "high" ? "destructive" : risk === "medium" ? "default" : "secondary"}>
      {label[risk] ?? risk}
    </Badge>
  );
}

/**
 * The payload as the model proposed it, pretty-printed.
 *
 * This is the thing actually being decided, so it is shown in full rather than
 * summarised — a person cannot consent to a call whose arguments they were not
 * shown.
 */
function formatInput(input: unknown): string {
  if (input === undefined || input === null || input === "") return "{}";
  if (typeof input === "string") {
    try {
      return JSON.stringify(JSON.parse(input), null, 2);
    } catch {
      return input;
    }
  }
  try {
    return JSON.stringify(input, null, 2);
  } catch {
    return String(input);
  }
}
