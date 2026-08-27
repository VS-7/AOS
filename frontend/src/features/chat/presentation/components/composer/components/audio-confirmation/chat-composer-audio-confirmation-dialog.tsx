import { t } from "@/lib/i18n";
import * as React from "react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { HugeiconsIcon } from "@hugeicons/react"
import { FileAudioIcon } from "@hugeicons/core-free-icons"
import { ComposerHelper } from "@/features/chat/presentation/helpers/composer.helper"
import type { PendingAudioClip } from "../../composer.types"

interface ChatComposerAudioConfirmationDialogProps {
  clip: PendingAudioClip | null
  onConfirm: () => void
  onOpenChange: (open: boolean) => void
}

export function ChatComposerAudioConfirmationDialog({
  clip,
  onConfirm,
  onOpenChange,
}: ChatComposerAudioConfirmationDialogProps) {
  return (
    <Dialog onOpenChange={onOpenChange} open={Boolean(clip)}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("Attach voice note")}</DialogTitle>
          <DialogDescription>
            {t("Review the recording before attaching it to the next message.")}
          </DialogDescription>
        </DialogHeader>

        {clip ? (
          <div className="space-y-3 rounded-xl border bg-muted/30 p-4">
            <div className="flex items-center gap-3">
              <div className="flex size-10 items-center justify-center rounded-full bg-primary/10 text-primary">
                <HugeiconsIcon icon={FileAudioIcon} className="size-4" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-foreground">{clip.filename}</p>
                <p className="text-xs text-muted-foreground">
                  {ComposerHelper.formatDuration(clip.durationMs)} {t("voice note")}
                </p>
              </div>
            </div>

            <audio className="w-full" controls src={clip.url} />
          </div>
        ) : null}

        <DialogFooter>
          <Button onClick={() => onOpenChange(false)} type="button" variant="ghost">
            {t("Discard")}
          </Button>
          <Button onClick={onConfirm} type="button">
            {t("Attach audio")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
