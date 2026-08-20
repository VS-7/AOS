import * as React from "react";
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuMotionItem,
} from "@/components/ui/sidebar";
import {
  Avatar,
  AvatarAgentFallback,
  AvatarFallback,
  AvatarImage,
} from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverAnchor,
  PopoverContent,
  PopoverHeader,
  PopoverTitle,
  PopoverDescription,
} from "@/components/ui/popover";
import { HugeiconsIcon } from "@hugeicons/react";
import { ArrowRightIcon } from "@hugeicons/core-free-icons";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import { aos } from "@/app/aos";
import type { Agent } from "@/features/agent/interfaces/agent.interfaces";
import type { WorkspaceMember } from "@/features/workspace/interfaces/workspace.interfaces";
import type { WorkspaceDirectoryAgentProcessing } from "@/features/workspace/interfaces/directory.interfaces";
import { ChatKindHelper } from "@/features/chat/services/chat/chat-kind.helper";
import {
  findAgentDmChatId,
  findUserDmChatId,
  openAgentDmTab,
  openChatTab,
  openUserDmTab,
} from "@/features/chat/presentation/helpers/open-chat-tab.helper";
import { ChatActivityStamp } from "./chat-activity-stamp";

interface ChatTeamListProps {
  agents: Agent[];
  currentChatId?: string;
}

interface AgentRowProps {
  agent: Agent;
  isActive: boolean;
  onClick: () => void;
  processing: WorkspaceDirectoryAgentProcessing[];
  updatedAt?: string | Date;
}

function AgentRow({
  agent,
  isActive,
  onClick,
  processing,
  updatedAt,
}: AgentRowProps) {
  const isProcessing = processing.length > 0;

  const [open, setOpen] = React.useState(false);
  const closeTimeoutRef = React.useRef<ReturnType<typeof setTimeout> | null>(
    null,
  );

  const openPopover = () => {
    if (closeTimeoutRef.current) {
      clearTimeout(closeTimeoutRef.current);
      closeTimeoutRef.current = null;
    }
    setOpen(true);
  };

  const closePopover = () => {
    closeTimeoutRef.current = setTimeout(() => {
      setOpen(false);
      closeTimeoutRef.current = null;
    }, 120);
  };

  const handleViewInSettings = (e: React.MouseEvent) => {
    e.stopPropagation();
    aos.stores.viewport.actions.openSettings("workspace.agents");
    setOpen(false);
  };

  return (
    <SidebarMenuItem onMouseEnter={openPopover} onMouseLeave={closePopover}>
      {/* Hover preview via PopoverAnchor — PopoverTrigger would steal the click
          meant for openAgentDmTab. */}
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverAnchor asChild>
          <SidebarMenuButton
            isActive={isActive}
            data-active={isActive}
            className="group/chat-row"
            onClick={onClick}
          >
            <Avatar className="size-5 rounded-full overflow-visible">
              <AvatarAgentFallback name={agent.id} />
              <span
                className={cn(
                  "absolute -right-0.5 -top-0.5 z-10 size-2.5 rounded-full border border-sidebar shadow-[0_0_0_1px_var(--sidebar)]",
                  isProcessing
                    ? "bg-amber-500 animate-pulse"
                    : "bg-emerald-500",
                )}
              />
            </Avatar>
            <span className="min-w-0 flex-1 truncate">{agent.name}</span>
            <ChatActivityStamp at={updatedAt} />
          </SidebarMenuButton>
        </PopoverAnchor>
        <PopoverContent
          side="right"
          align="start"
          sideOffset={8}
          className="w-72 p-3"
          onMouseEnter={openPopover}
          onMouseLeave={closePopover}
          onOpenAutoFocus={(event) => event.preventDefault()}
        >
          <PopoverHeader className="flex-row items-start gap-3">
            <Avatar className="size-9 rounded-full overflow-visible shrink-0">
              <AvatarAgentFallback name={agent.id} size={36} />
              <span
                className={cn(
                  "absolute -right-0.5 -top-0.5 z-10 size-2.5 rounded-full border border-popover shadow-[0_0_0_1px_var(--popover)]",
                  isProcessing
                    ? "bg-amber-500 animate-pulse"
                    : "bg-emerald-500",
                )}
              />
            </Avatar>
            <div className="min-w-0 flex-1">
              <PopoverTitle className="truncate text-sm">
                {agent.name}
              </PopoverTitle>
              {agent.role ? (
                <div className="text-[10px] text-muted-foreground truncate">
                  {agent.role}
                </div>
              ) : null}
            </div>
          </PopoverHeader>

          {agent.description ? (
            <PopoverDescription className="line-clamp-2 text-xs mt-1.5">
              {agent.description}
            </PopoverDescription>
          ) : null}

          {processing.length > 0 ? (
            <div className="mt-3 space-y-1.5">
              <p className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground/70">
                Running now
              </p>
              <ul className="space-y-1">
                {processing.map((entry) => (
                  <li key={`${agent.id}:${entry.chatId}`}>
                    <button
                      type="button"
                      className="flex w-full items-center gap-2 rounded-md px-1.5 py-1 text-left text-xs hover:bg-muted/60"
                      onClick={(event) => {
                        event.stopPropagation();
                        openChatTab({
                          chatId: entry.chatId,
                          title: entry.title,
                        });
                        setOpen(false);
                      }}
                    >
                      <span className="min-w-0 flex-1 truncate font-medium">
                        {entry.title}
                      </span>
                      <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] capitalize text-muted-foreground">
                        {entry.kind}
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          ) : (
            <p className="mt-3 text-[11px] text-muted-foreground/80">
              Idle — no active runs.
            </p>
          )}

          <Button
            variant="outline"
            size="sm"
            className="mt-3 h-7 px-2.5 text-xs"
            onClick={handleViewInSettings}
          >
            View Details
            <HugeiconsIcon icon={ArrowRightIcon} className="size-3 ml-1" />
          </Button>
        </PopoverContent>
      </Popover>
    </SidebarMenuItem>
  );
}

interface UserRowProps {
  userId: string;
  name: string;
  image?: string;
  role?: WorkspaceMember["role"];
  isActive: boolean;
  onClick: () => void;
  updatedAt?: string | Date;
}

function UserRow({
  userId,
  name,
  image,
  role,
  isActive,
  onClick,
  updatedAt,
}: UserRowProps) {
  const initials = name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("");

  return (
    <SidebarMenuItem>
      <SidebarMenuButton
        isActive={isActive}
        data-active={isActive}
        className="group/chat-row"
        onClick={onClick}
      >
        <Avatar className="size-5 rounded-full overflow-hidden">
          {image ? (
            <AvatarImage src={image} alt="" />
          ) : (
            <AvatarFallback className="rounded-full text-[9px] font-medium">
              {initials || userId.slice(0, 2).toUpperCase()}
            </AvatarFallback>
          )}
        </Avatar>
        <span className="min-w-0 flex-1 truncate">{name}</span>
        {role ? (
          <span className="shrink-0 text-[10px] text-muted-foreground/70 capitalize">
            {role}
          </span>
        ) : null}
        <ChatActivityStamp at={updatedAt} />
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
}

/**
 * Team tab — peer agents + people as DM starters.
 *
 * Peers come from `stores.workspace.directory` (viewer-relative: self excluded).
 * Live processing merges occupancy + chat list on top of the directory seed.
 */
export function ChatTeamList({ agents, currentChatId }: ChatTeamListProps) {
  const chats = aos.stores.chat.useState((s) => s.items);
  const occupancy = aos.stores.agent.useState((s) => s.occupancy);
  const members =
    aos.stores.workspace.useState((s) => s.current?.members) ?? [];
  const directory = aos.stores.workspace.useState(
    (s) => s.directory ?? { users: [], agents: [] },
  );
  const selfUserId = aos.stores.auth.useState((s) => s.user?.id);

  React.useEffect(() => {
    if (directory.users.length === 0 && directory.agents.length === 0) {
      void aos.stores.workspace.actions.refreshDirectory();
      void aos.stores.agent.actions.refresh();
    }
  }, [directory.agents.length, directory.users.length]);

  const peerUsers = directory.users;
  const teamAgents = React.useMemo(() => {
    if (agents.length > 0) {
      return agents;
    }
    return directory.agents.map(
      (agent) =>
        ({
          id: agent.id,
          name: agent.name,
          image: agent.image,
          role: agent.role,
          description: agent.description,
          orchestrator: agent.orchestrator,
        }) as Agent,
    );
  }, [agents, directory.agents]);
  const roleByUserId = React.useMemo(
    () => new Map(members.map((member) => [member.userId, member.role])),
    [members],
  );
  const directoryAgentsById = React.useMemo(
    () => new Map(directory.agents.map((agent) => [agent.id, agent])),
    [directory.agents],
  );
  const agentIds = React.useMemo(
    () => new Set(teamAgents.map((agent) => agent.id)),
    [teamAgents],
  );

  const chatUpdatedAt = React.useMemo(() => {
    const map = new Map<string, string | Date>();
    for (const chat of chats) {
      if (chat.updatedAt) {
        map.set(chat.id, chat.updatedAt);
      }
    }
    return map;
  }, [chats]);

  const resolvePeerName = React.useCallback(
    (user: (typeof peerUsers)[number]) => {
      if (user.name?.trim()) {
        return user.name.trim();
      }
      if (user.username?.trim()) {
        return user.username.trim();
      }
      if (user.email?.trim()) {
        return user.email.trim();
      }

      if (selfUserId) {
        const dmId = findUserDmChatId(chats, user.id, selfUserId);
        const dm = dmId ? chats.find((chat) => chat.id === dmId) : undefined;
        if (dm?.title?.trim()) {
          return dm.title.trim();
        }
      }

      return "Teammate";
    },
    [chats, selfUserId],
  );

  const resolveProcessing = React.useCallback(
    (agentId: string): WorkspaceDirectoryAgentProcessing[] => {
      const live = ChatKindHelper.list_processing_for_agent(
        agentId,
        occupancy,
        chats,
        agentIds,
      );
      if (live.length > 0) {
        return live;
      }
      return directoryAgentsById.get(agentId)?.processing ?? [];
    },
    [agentIds, chats, directoryAgentsById, occupancy],
  );

  if (teamAgents.length === 0 && peerUsers.length === 0) {
    return (
      <p className="px-2 py-3 text-xs text-muted-foreground/70">
        No teammates yet
      </p>
    );
  }

  let motionIndex = 0;

  return (
    <div className="space-y-2">
      {teamAgents.length > 0 ? (
        <div className="space-y-0.5">
          <p className="px-2 text-[10px] font-medium uppercase tracking-wide text-muted-foreground/55">
            Agents
          </p>
          <SidebarMenu>
            {teamAgents.map((agent) => {
              const index = motionIndex++;
              const dmChatId = findAgentDmChatId(chats, agent.id);
              return (
                <SidebarMenuMotionItem key={`agent:${agent.id}`} index={index}>
                  <AgentRow
                    agent={agent}
                    isActive={Boolean(
                      currentChatId &&
                        (currentChatId === dmChatId ||
                          currentChatId === agent.id),
                    )}
                    processing={resolveProcessing(agent.id)}
                    updatedAt={
                      (dmChatId && chatUpdatedAt.get(dmChatId)) ||
                      chatUpdatedAt.get(agent.id)
                    }
                    onClick={() => {
                      void openAgentDmTab({
                        agentId: agent.id,
                        title: agent.name || agent.id,
                      }).catch((error) => {
                        toast.error(
                          error instanceof Error
                            ? error.message
                            : "Unable to open agent DM.",
                        );
                      });
                    }}
                  />
                </SidebarMenuMotionItem>
              );
            })}
          </SidebarMenu>
        </div>
      ) : null}

      {peerUsers.length > 0 ? (
        <div className="space-y-0.5">
          <p className="px-2 text-[10px] font-medium uppercase tracking-wide text-muted-foreground/55">
            People
          </p>
          <SidebarMenu>
            {peerUsers.map((user) => {
              const index = motionIndex++;
              const name = resolvePeerName(user);
              const dmChatId = selfUserId
                ? findUserDmChatId(chats, user.id, selfUserId)
                : undefined;
              return (
                <SidebarMenuMotionItem
                  key={`user:${user.id}`}
                  index={index}
                >
                  <UserRow
                    userId={user.id}
                    name={name}
                    image={user.image}
                    role={roleByUserId.get(user.id)}
                    isActive={Boolean(
                      currentChatId && currentChatId === dmChatId,
                    )}
                    updatedAt={
                      dmChatId ? chatUpdatedAt.get(dmChatId) : undefined
                    }
                    onClick={() => {
                      void openUserDmTab({
                        userId: user.id,
                        title: name,
                      }).catch((error) => {
                        toast.error(
                          error instanceof Error
                            ? error.message
                            : "Unable to open user DM.",
                        );
                      });
                    }}
                  />
                </SidebarMenuMotionItem>
              );
            })}
          </SidebarMenu>
        </div>
      ) : null}
    </div>
  );
}
