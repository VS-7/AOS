import * as React from "react";
import {
  ArrowUpRightIcon,
  CheckIcon,
  CopyIcon,
  RotateCwIcon,
  Trash2Icon,
  WebhookIcon,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import { t } from "@/lib/i18n";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from "@/components/ui/input-group";

/** What the webhook row knows about the routine it belongs to. */
export interface WebhookTriggerState {
  /** The fire URL, once the routine is saved with its webhook. */
  fireUrl: string | null;
  /** Mints a new token, invalidating the old one. Absent until saved. */
  onRotate?: () => void;
  rotating?: boolean;
}

interface WebhookTriggerRowProps {
  webhook?: WebhookTriggerState;
  onRemove: () => void;
}

export function WebhookTriggerRow({
  webhook,
  onRemove,
}: WebhookTriggerRowProps) {
  const [copied, setCopied] = React.useState(false);
  const fireUrl = webhook?.fireUrl ?? null;

  async function handleCopy() {
    if (!fireUrl) return;
    try {
      await navigator.clipboard.writeText(fireUrl);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard access can be refused; the URL stays selectable.
    }
  }

  return (
    <div className="group flex gap-2 px-3 py-2.5">
      <div className="flex h-7 w-4 shrink-0 items-center justify-center">
        <WebhookIcon className="size-4 text-muted-foreground" />
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex min-h-7 flex-wrap items-center gap-2">
          <span className="text-sm">{t("Webhook triggered")}</span>

          {fireUrl ? (
            <InputGroup className="h-8 max-w-full flex-1 bg-background/70">
              <InputGroupAddon align="inline-start">
                <ArrowUpRightIcon className="size-3.5" />
              </InputGroupAddon>
              <InputGroupInput
                readOnly
                value={fireUrl}
                className="text-xs"
                aria-label={t("Webhook fire URL")}
              />
              <InputGroupAddon align="inline-end">
                <InputGroupButton
                  type="button"
                  size="icon-xs"
                  onClick={() => void handleCopy()}
                  aria-label={t("Copy webhook URL")}
                >
                  {copied ? (
                    <CheckIcon className="size-3.5" />
                  ) : (
                    <CopyIcon className="size-3.5" />
                  )}
                </InputGroupButton>
              </InputGroupAddon>
            </InputGroup>
          ) : (
            <p className="text-xs text-muted-foreground">
              {t("Save this routine to get its fire URL and token.")}
            </p>
          )}

          {fireUrl && webhook?.onRotate ? (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-7 gap-1.5 px-2 text-xs text-muted-foreground"
              disabled={webhook.rotating}
              onClick={webhook.onRotate}
            >
              <RotateCwIcon className="size-3.5" />
              {t("New token")}
            </Button>
          ) : null}
        </div>
        {fireUrl ? (
          <p className="mt-1 text-xs text-muted-foreground">
            {t("POST to this address with the header \"Authorization: Bearer <token>\". The token was shown once, when the webhook was added.")}
          </p>
        ) : null}
      </div>

      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="size-7 shrink-0 self-center opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100"
        onClick={onRemove}
      >
        <Trash2Icon className="size-3.5" />
        <span className="sr-only">{t("Remove webhook trigger")}</span>
      </Button>
    </div>
  );
}
