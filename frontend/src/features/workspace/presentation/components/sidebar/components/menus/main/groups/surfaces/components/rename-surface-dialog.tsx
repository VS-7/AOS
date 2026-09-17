import * as React from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { t } from "@/lib/i18n";

interface RenameSurfaceDialogProps {
  open: boolean;
  currentName: string;
  onOpenChange: (open: boolean) => void;
  /** Resolves when the rename landed; a rejection keeps the dialog open. */
  onRename: (name: string) => Promise<void>;
}

/** The one field an artifact's name is changed through. */
export function RenameSurfaceDialog({ open, currentName, onOpenChange, onRename }: RenameSurfaceDialogProps) {
  const [name, setName] = React.useState(currentName);
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    if (open) setName(currentName);
  }, [open, currentName]);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = name.trim();
    if (!trimmed || trimmed === currentName) {
      onOpenChange(false);
      return;
    }
    setSaving(true);
    try {
      await onRename(trimmed);
      onOpenChange(false);
    } catch {
      // The caller has already said why; the dialog stays for another try.
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>{t("Rename")}</DialogTitle>
        </DialogHeader>
        <form className="flex flex-col gap-4" onSubmit={(event) => void submit(event)}>
          <div className="flex flex-col gap-2">
            <Label htmlFor="surface-rename">{t("Name")}</Label>
            <Input
              id="surface-rename"
              autoFocus
              value={name}
              maxLength={120}
              disabled={saving}
              onChange={(event) => setName(event.target.value)}
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" size="sm" onClick={() => onOpenChange(false)}>
              {t("Cancel")}
            </Button>
            <Button type="submit" size="sm" disabled={saving || !name.trim()}>
              {t("Rename")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
