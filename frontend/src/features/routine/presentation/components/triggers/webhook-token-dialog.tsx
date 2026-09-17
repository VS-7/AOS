import * as React from "react";
import { CheckIcon, CopyIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { RoutineWebhookHelper } from "@/features/routine/presentation/helpers/routine-webhook.helper";
import { t } from "@/lib/i18n";

interface WebhookTokenDialogProps {
  /** The token to show. The dialog is open while there is one. */
  token: string | null;
  fireUrl: string;
  onClose: () => void;
}

/**
 * The one moment a webhook token can be seen.
 *
 * Go stores only its hash and returns the token once — from create, from an
 * update that added the webhook, or from a rotation. The editor used to
 * discard it, which left a webhook nobody could ever call.
 */
export function WebhookTokenDialog({ token, fireUrl, onClose }: WebhookTokenDialogProps) {
  return (
    <Dialog open={token !== null} onOpenChange={(open) => !open && onClose()}>
      {/* grid-cols-1 with minmax(0,1fr): the dialog is a grid, and a long
          token or command would otherwise widen its column past the dialog. */}
      <DialogContent className="grid-cols-[minmax(0,1fr)] sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("Copy this webhook token now")}</DialogTitle>
          <DialogDescription>
            {t("It is shown once. Only its hash is stored, so the only way to get another is to rotate it, which stops this one working.")}
          </DialogDescription>
        </DialogHeader>

        {token ? (
          <div className="flex min-w-0 flex-col gap-3 text-sm">
            <CopyField label={t("Token")} value={token} />
            <CopyField label={t("Fire URL")} value={fireUrl} />
            <CopyField
              label={t("Example")}
              value={RoutineWebhookHelper.curlExample(fireUrl, token)}
              multiline
            />
            <p className="text-xs text-muted-foreground">
              {t("This is the address this window reaches the daemon at. A sender on another machine needs an address it can reach, such as the workspace's tunnel.")}
            </p>
          </div>
        ) : null}

        <DialogFooter>
          <Button type="button" onClick={onClose}>
            {t("I have copied it")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function CopyField({ label, value, multiline }: { label: string; value: string; multiline?: boolean }) {
  const [copied, setCopied] = React.useState(false);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard access can be refused; the value stays selectable.
    }
  }

  return (
    <div className="flex min-w-0 flex-col gap-1">
      <span className="text-xs text-muted-foreground">{label}</span>
      <div className="flex min-w-0 items-start gap-2">
        <pre
          className={
            "min-w-0 flex-1 select-all overflow-x-auto rounded-md border bg-muted/40 px-2 py-1.5 font-mono text-xs " +
            (multiline ? "whitespace-pre" : "whitespace-nowrap")
          }
        >
          {value}
        </pre>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-7 shrink-0"
          onClick={() => void handleCopy()}
          aria-label={t("Copy {{what}}", { what: label })}
        >
          {copied ? <CheckIcon className="size-3.5" /> : <CopyIcon className="size-3.5" />}
        </Button>
      </div>
    </div>
  );
}
