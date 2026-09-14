import * as React from "react";
import { HugeiconsIcon } from "@hugeicons/react";
import {
  Delete01Icon,
  EraserIcon,
  MoreHorizontalIcon,
  PencilIcon,
} from "@hugeicons/core-free-icons";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { t } from "@/lib/i18n";
import {
  useChatActions,
  type ChatActionsTarget,
} from "../hooks/use-chat-actions";

interface ChatActionsMenuProps {
  chat: ChatActionsTarget;
  /** Takes the server's transcript whole after a clear — see useChatActions. */
  onCleared?: () => void;
}

/**
 * The conversation's own menu, in its header: rename, clear, delete.
 *
 * It exists for every kind of conversation because the sidebar only ever had
 * one for channels. A DM, a task thread or a run transcript could be opened
 * and never renamed or removed — a conversation somebody opened with
 * themselves by mistake stayed in the workspace for good.
 */
export function ChatActionsMenu({ chat, onCleared }: ChatActionsMenuProps) {
  const [renaming, setRenaming] = React.useState(false);
  const [draft, setDraft] = React.useState(chat.title ?? "");
  const actions = useChatActions(chat, {
    onCleared,
    onRenamed: () => setRenaming(false),
  });

  const openRename = () => {
    setDraft(chat.title ?? "");
    setRenaming(true);
  };

  const submitRename = (event: React.FormEvent) => {
    event.preventDefault();
    if (!actions.rename(draft)) setRenaming(false);
  };

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            type="button"
            size="icon-xs"
            variant="ghost"
            className="text-muted-foreground hover:text-foreground"
            aria-label={t("Conversation actions")}
          >
            <HugeiconsIcon icon={MoreHorizontalIcon} className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-48">
          <DropdownMenuItem onSelect={openRename}>
            <HugeiconsIcon icon={PencilIcon} />
            {t("Rename")}
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => void actions.clear()}>
            <HugeiconsIcon icon={EraserIcon} />
            {t("Clear messages")}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            variant="destructive"
            onSelect={() => void actions.remove()}
          >
            <HugeiconsIcon icon={Delete01Icon} />
            {t("Delete")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog open={renaming} onOpenChange={setRenaming}>
        <DialogContent className="sm:max-w-md">
          <form onSubmit={submitRename} className="flex flex-col gap-4">
            <DialogHeader>
              <DialogTitle>{t("Rename conversation")}</DialogTitle>
              <DialogDescription>
                {t("The new name shows in the sidebar, in search and on its tab.")}
              </DialogDescription>
            </DialogHeader>
            <Input
              autoFocus
              aria-label={t("Name")}
              value={draft}
              maxLength={120}
              onChange={(event) => setDraft(event.target.value)}
              disabled={actions.isRenaming}
            />
            <DialogFooter>
              <Button type="button" variant="ghost" onClick={() => setRenaming(false)}>
                {t("Cancel")}
              </Button>
              <Button type="submit" disabled={actions.isRenaming || !draft.trim()}>
                {t("Rename")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
