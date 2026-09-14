import * as React from "react";
import { motion, AnimatePresence } from "motion/react";
import { HugeiconsIcon } from "@hugeicons/react";
import {
  Tick01Icon,
  TextNumberSignIcon,
  Loading03Icon,
  MoreHorizontalIcon,
  PencilIcon,
  Delete01Icon,
  Cancel01Icon,
} from "@hugeicons/core-free-icons";
import {
  SidebarMenuAction,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from "@/components/ui/sidebar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { aos } from "@/app/aos";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { Chat } from "@/features/chat/interfaces/chat.interfaces";
import { openChatTab } from "@/features/chat/presentation/helpers/open-chat-tab.helper";
import { useChatActions } from "@/features/chat/presentation/hooks/use-chat-actions";
import { ChatActivityStamp } from "../../chat/components/chat-activity-stamp";
import { t } from "@/lib/i18n";

interface ChannelItemProps {
  chat: Chat;
  isActive: boolean;
  unreadCount?: number;
  onChanged?: () => void;
}

export function ChannelItem({
  chat,
  isActive,
  unreadCount = 0,
  onChanged,
}: ChannelItemProps) {
  const [isEditing, setIsEditing] = React.useState(false);
  const [draftTitle, setDraftTitle] = React.useState(chat.title);

  React.useEffect(() => {
    if (!isEditing) {
      setDraftTitle(chat.title);
    }
  }, [chat.title, isEditing]);

  // Rename and delete are shared with every other kind of conversation (the
  // chat header menu); see useChatActions for what a delete cleans up.
  const { rename, remove, isRenaming, isDeleting } = useChatActions(chat, {
    onRenamed: () => {
      setIsEditing(false);
      onChanged?.();
    },
    onDeleted: onChanged,
  });

  function handleOpen() {
    if (isEditing) {
      return;
    }

    openChatTab({
      chatId: chat.id,
      title: chat.title || chat.id,
    });
  }

  function startEditing() {
    setDraftTitle(chat.title);
    setIsEditing(true);
  }

  function cancelEditing() {
    setDraftTitle(chat.title);
    setIsEditing(false);
  }

  function handleRename() {
    if (!rename(draftTitle)) {
      cancelEditing();
    }
  }

  return (
    <SidebarMenuSubItem className="group/menu-item">
      <SidebarMenuSubButton
        isActive={isActive || isEditing}
        data-active={isActive || isEditing}
        className={cn("group/chat-row rounded-md", isEditing && "pr-2")}
        onClick={handleOpen}
      >
        <motion.div
          whileHover={{ rotate: isEditing ? 0 : -8 }}
          transition={{ duration: 0.16 }}
        >
          <HugeiconsIcon
            icon={TextNumberSignIcon}
            className="size-3.5 text-muted-foreground"
          />
        </motion.div>

        <span className="flex min-w-0 flex-1 items-center justify-between gap-2">
          <AnimatePresence mode="wait" initial={false}>
            {isEditing ? (
              <motion.span
                key="editing"
                initial={{ opacity: 0, x: -6 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: 6 }}
                transition={{ duration: 0.16, ease: "easeOut" }}
                className="flex min-w-0 flex-1 items-center gap-1"
              >
                <input
                  autoFocus
                  value={draftTitle}
                  onChange={(event) => setDraftTitle(event.target.value)}
                  onClick={(event) => event.stopPropagation()}
                  onKeyDown={(event) => {
                    event.stopPropagation();

                    if (event.key === "Enter") {
                      event.preventDefault();
                      handleRename();
                    }

                    if (event.key === "Escape") {
                      event.preventDefault();
                      cancelEditing();
                    }
                  }}
                  className="min-w-0 flex-1 bg-transparent text-sm outline-hidden"
                />

                <motion.div
                  initial={{ opacity: 0, scale: 0.92 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={{ opacity: 0, scale: 0.92 }}
                  transition={{ duration: 0.16, ease: "easeOut" }}
                  className="flex items-center gap-1"
                >
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <motion.div
                          whileHover={{ y: -1 }}
                          whileTap={{ scale: 0.96 }}
                        >
                          <Button
                            type="button"
                            size="icon-xs"
                            variant="ghost"
                            className="text-emerald-500 hover:bg-emerald-500/10 hover:text-emerald-400"
                            onClick={(event) => {
                              event.stopPropagation();
                              handleRename();
                            }}
                            disabled={isRenaming}
                          >
                            {isRenaming ? (
                              <HugeiconsIcon
                                icon={Loading03Icon}
                                className="size-3 animate-spin"
                              />
                            ) : (
                              <HugeiconsIcon
                                icon={Tick01Icon}
                                className="size-3"
                              />
                            )}
                          </Button>
                        </motion.div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{t("Confirm rename")}</TooltipContent>
                    </Tooltip>

                    <Tooltip>
                      <TooltipTrigger asChild>
                        <motion.div
                          whileHover={{ y: -1 }}
                          whileTap={{ scale: 0.96 }}
                        >
                          <Button
                            type="button"
                            size="icon-xs"
                            variant="ghost"
                            className="text-muted-foreground hover:bg-muted hover:text-foreground"
                            onClick={(event) => {
                              event.stopPropagation();
                              cancelEditing();
                            }}
                          >
                            <HugeiconsIcon
                              icon={Cancel01Icon}
                              className="size-3"
                            />
                          </Button>
                        </motion.div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{t("Cancel rename")}</TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                </motion.div>
              </motion.span>
            ) : (
              <motion.span
                key="view"
                initial={{ opacity: 0, x: -6 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: 6 }}
                transition={{ duration: 0.16, ease: "easeOut" }}
                className="flex min-w-0 flex-1 items-center gap-2"
              >
                <span className="min-w-0 flex-1 truncate">{chat.title}</span>
                {unreadCount > 0 ? (
                  <Badge className="h-5 shrink-0 rounded-full px-1.5 text-[10px]">
                    {unreadCount}
                  </Badge>
                ) : null}
                {/* Hidden while the row's menu is open too, not only on hover:
                    moving the pointer into the menu dropped the hover, and the
                    ⋯ trigger then sat on top of the stamp ("1m…"). */}
                <ChatActivityStamp
                  at={chat.updatedAt}
                  className="group-hover/menu-item:opacity-0 group-has-[[data-state=open]]/menu-item:opacity-0"
                />
              </motion.span>
            )}
          </AnimatePresence>
        </span>
      </SidebarMenuSubButton>

      {!isEditing ? (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuAction
              showOnHover
              aria-label={t("Open actions for {{title}}", { title: chat.title })}
            >
              {isDeleting ? (
                <HugeiconsIcon
                  icon={Loading03Icon}
                  className="animate-spin"
                />
              ) : (
                <HugeiconsIcon icon={MoreHorizontalIcon} />
              )}
            </SidebarMenuAction>
          </DropdownMenuTrigger>

          <DropdownMenuContent align="start" side="right" className="w-44">
            <DropdownMenuLabel>{t("Channel Actions")}</DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onClick={(event) => {
                event.stopPropagation();
                startEditing();
              }}
            >
              <HugeiconsIcon icon={PencilIcon} />
              {t("Rename")}
            </DropdownMenuItem>
            <DropdownMenuItem
              variant="destructive"
              onClick={(event) => {
                event.stopPropagation();
                void remove();
              }}
            >
              <HugeiconsIcon icon={Delete01Icon} />
              {t("Delete")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ) : null}
    </SidebarMenuSubItem>
  );
}
