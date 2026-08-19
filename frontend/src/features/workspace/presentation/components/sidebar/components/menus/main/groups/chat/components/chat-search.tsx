import * as React from "react";
import { AnimatePresence, motion } from "motion/react";
import {
  Cancel01Icon,
  Search01Icon,
} from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Avatar, AvatarAgentFallback } from "@/components/ui/avatar";
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar";
import { cn } from "@/lib/utils";
import { aos } from "@/app/aos";
import type { FractalAgent } from "@/features/agent/interfaces/agent.interfaces";
import type { Chat } from "@/features/chat/interfaces/chat.interfaces";
import { openAgentDmTab, openChatTab } from "@/features/chat/presentation/helpers/open-chat-tab.helper";
import {
  FractalChatSearchHelper,
  type ChatSearchHit,
} from "../helpers/chat-search.helper";
import { ChatActivityStamp } from "./chat-activity-stamp";
import { ChatRowKindIcon } from "./chat-row-kind-icon";

interface ChatSearchProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  chats: Chat[];
  agents: FractalAgent[];
  agentIds: ReadonlySet<string>;
  currentChatId?: string;
}

/**
 * Expandable chat finder — cross-kind search with recent fallback + motion.
 */
export function ChatSearch({
  open,
  onOpenChange,
  chats,
  agents,
  agentIds,
  currentChatId,
}: ChatSearchProps) {
  const inputRef = React.useRef<HTMLInputElement>(null);
  const [query, setQuery] = React.useState("");
  const [activeIndex, setActiveIndex] = React.useState(0);

  const hits = React.useMemo(
    () =>
      FractalChatSearchHelper.search({
        query,
        chats,
        agents,
        agentIds,
      }),
    [agentIds, agents, chats, query],
  );

  React.useEffect(() => {
    if (!open) {
      setQuery("");
      setActiveIndex(0);
      return;
    }

    const id = window.requestAnimationFrame(() => inputRef.current?.focus());
    return () => window.cancelAnimationFrame(id);
  }, [open]);

  React.useEffect(() => {
    setActiveIndex(0);
  }, [query]);

  const selectHit = React.useCallback(
    async (hit: ChatSearchHit) => {
      if (
        hit.kind === "dm" &&
        agents.some((agent) => agent.id === hit.chatId)
      ) {
        await openAgentDmTab({ agentId: hit.chatId, title: hit.title });
      } else {
        openChatTab({ chatId: hit.chatId, title: hit.title });
      }
      onOpenChange(false);
    },
    [agents, onOpenChange],
  );

  const onKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Escape") {
      event.preventDefault();
      if (query) {
        setQuery("");
        return;
      }
      onOpenChange(false);
      return;
    }

    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActiveIndex((index) => Math.min(index + 1, Math.max(hits.length - 1, 0)));
      return;
    }

    if (event.key === "ArrowUp") {
      event.preventDefault();
      setActiveIndex((index) => Math.max(index - 1, 0));
      return;
    }

    if (event.key === "Enter") {
      event.preventDefault();
      const hit = hits[activeIndex];
      if (hit) {
        selectHit(hit);
      }
    }
  };

  return (
    <div className="space-y-1">
      <AnimatePresence initial={false} mode="popLayout">
        {open ? (
          <motion.div
            key="search-field"
            initial={{ opacity: 0, height: 0, y: -4 }}
            animate={{ opacity: 1, height: "auto", y: 0 }}
            exit={{ opacity: 0, height: 0, y: -4 }}
            transition={{ duration: 0.18, ease: [0.22, 1, 0.36, 1] }}
            className="overflow-hidden px-1"
          >
            <div className="relative flex items-center">
              <HugeiconsIcon
                icon={Search01Icon}
                className="pointer-events-none absolute left-2 size-3.5 text-muted-foreground/70"
              />
              <Input
                ref={inputRef}
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                onKeyDown={onKeyDown}
                placeholder="Search…"
                aria-label="Search chats"
                className={cn(
                  "h-8 border-0 bg-transparent pl-7 pr-8 text-xs shadow-none",
                  "focus-visible:border focus-visible:border-border/50 focus-visible:bg-sidebar-accent/40 focus-visible:ring-1 focus-visible:ring-ring/30",
                )}
              />
              <Button
                type="button"
                size="icon-xs"
                variant="ghost"
                className="absolute right-1 text-muted-foreground"
                onClick={() => onOpenChange(false)}
                aria-label="Close search"
              >
                <HugeiconsIcon icon={Cancel01Icon} className="size-3.5" />
              </Button>
            </div>
          </motion.div>
        ) : null}
      </AnimatePresence>

      <AnimatePresence initial={false}>
        {open ? (
          <motion.div
            key="search-results"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.12 }}
          >
            {hits.length === 0 ? (
              <p className="px-2 py-4 text-center text-xs text-muted-foreground/60">
                No chats match “{query}”
              </p>
            ) : (
              <SidebarMenu>
                {hits.map((hit, index) => (
                  <motion.div
                    key={hit.chatId}
                    initial={{ opacity: 0, x: -4 }}
                    animate={{ opacity: 1, x: 0 }}
                    transition={{
                      duration: 0.14,
                      delay: Math.min(index * 0.02, 0.12),
                    }}
                  >
                    <SidebarMenuItem>
                      <SidebarMenuButton
                        isActive={
                          currentChatId === hit.chatId || index === activeIndex
                        }
                        data-active={
                          currentChatId === hit.chatId || index === activeIndex
                        }
                        className="group/chat-row"
                        onClick={() => selectHit(hit)}
                        onMouseEnter={() => setActiveIndex(index)}
                      >
                        <ChatSearchHitIcon chatId={hit.chatId} kind={hit.kind} />
                        <span className="min-w-0 flex-1 truncate">
                          {hit.title}
                        </span>
                        {hit.subtitle && hit.kind !== "dm" ? (
                          <span className="max-w-[3.5rem] truncate text-[10px] text-muted-foreground/50">
                            {hit.subtitle}
                          </span>
                        ) : null}
                        <ChatActivityStamp at={hit.updatedAt} />
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  </motion.div>
                ))}
              </SidebarMenu>
            )}
          </motion.div>
        ) : null}
      </AnimatePresence>
    </div>
  );
}

function ChatSearchHitIcon({
  chatId,
  kind,
}: {
  chatId: string;
  kind: ChatSearchHit["kind"];
}) {
  const isProcessing = aos.stores.agent.useState((s) =>
    Object.values(s.occupancy).some((ids) => ids.includes(chatId)),
  );

  if (kind === "dm") {
    return (
      <Avatar className="size-5 overflow-visible rounded-full">
        <AvatarAgentFallback name={chatId} />
        <span
          className={cn(
            "absolute -right-0.5 -top-0.5 z-10 size-2.5 rounded-full border border-sidebar shadow-[0_0_0_1px_hsl(var(--sidebar-background))]",
            isProcessing ? "animate-pulse bg-amber-500" : "bg-emerald-500",
          )}
        />
      </Avatar>
    );
  }

  return <ChatRowKindIcon kind={kind} />;
}

interface ChatSearchToggleProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  className?: string;
}

/**
 * Compact search affordance for the Chat group header.
 */
export function ChatSearchToggle({
  open,
  onOpenChange,
  className,
}: ChatSearchToggleProps) {
  return (
    <Button
      type="button"
      size="icon-xs"
      variant="ghost"
      className={cn(
        "text-sidebar-foreground/60 hover:text-sidebar-foreground",
        open && "bg-sidebar-accent text-sidebar-foreground",
        className,
      )}
      aria-label={open ? "Close chat search" : "Search chats"}
      aria-pressed={open}
      onClick={() => onOpenChange(!open)}
    >
      <HugeiconsIcon
        icon={open ? Cancel01Icon : Search01Icon}
        className="size-3.5"
      />
    </Button>
  );
}
