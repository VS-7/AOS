import * as React from "react";
import {
  ArrowUpRightIcon,
  CheckIcon,
  CopyIcon,
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

interface WebhookTriggerRowProps {
  fireUrl?: string | null;
  onRemove: () => void;
}

export function WebhookTriggerRow({
  fireUrl,
  onRemove,
}: WebhookTriggerRowProps) {
  const [copied, setCopied] = React.useState(false);

  function handleCopy() {
    if (!fireUrl) return;
    navigator.clipboard.writeText(fireUrl);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 2000);
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
                  onClick={handleCopy}
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
              {t("Save this routine to generate the public fire URL.")}
            </p>
          )}
        </div>
      </div>

      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="size-7 shrink-0 self-center opacity-0 transition-opacity group-hover:opacity-100"
        onClick={onRemove}
      >
        <Trash2Icon className="size-3.5" />
        <span className="sr-only">{t("Remove webhook trigger")}</span>
      </Button>
    </div>
  );
}
