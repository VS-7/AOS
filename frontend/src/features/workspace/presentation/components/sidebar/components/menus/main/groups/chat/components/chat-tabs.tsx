import * as React from "react";
import {
  CheckListIcon,
  TextNumberSignIcon,
  RepeatIcon,
  UserGroupIcon,
} from "@hugeicons/core-free-icons";
import { HugeiconsIcon, type IconSvgElement } from "@hugeicons/react";
import {
  TabsSubtle,
  TabsSubtleItem,
} from "@/components/ui/tabs-subtle";
import type { IconComponent } from "@/lib/icon-context";
import type { IconComponentProps } from "@/lib/icon-map";

export type ChatSidebarTab = "channels" | "team" | "tasks" | "runs";

const TAB_STORAGE_KEY = "aos.sidebar.chatTab";

const TABS: Array<{
  id: ChatSidebarTab;
  label: string;
  icon: IconSvgElement;
}> = [
  { id: "channels", label: "Channels", icon: TextNumberSignIcon },
  { id: "team", label: "Team", icon: UserGroupIcon },
  { id: "tasks", label: "Tasks", icon: CheckListIcon },
  { id: "runs", label: "Runs", icon: RepeatIcon },
];

const TAB_IDS = TABS.map((tab) => tab.id);

/**
 * Adapts a HugeIcons glyph to the {@link IconComponent} contract used by TabsSubtle.
 */
function asHugeIcon(icon: IconSvgElement): IconComponent {
  return function HugeIconAdapter({
    size = 16,
    strokeWidth = 1.5,
    className,
  }: IconComponentProps) {
    return (
      <HugeiconsIcon
        icon={icon}
        size={size}
        strokeWidth={strokeWidth}
        className={className}
      />
    );
  };
}

const TAB_ICON_COMPONENTS: Record<ChatSidebarTab, IconComponent> = {
  channels: asHugeIcon(TextNumberSignIcon),
  team: asHugeIcon(UserGroupIcon),
  tasks: asHugeIcon(CheckListIcon),
  runs: asHugeIcon(RepeatIcon),
};

interface ChatTabsProps {
  value: ChatSidebarTab;
  onValueChange: (tab: ChatSidebarTab) => void;
  badges?: Partial<Record<ChatSidebarTab, number>>;
}

/**
 * Reads the last Chat sidebar tab from sessionStorage.
 */
export function readStoredChatTab(): ChatSidebarTab {
  if (typeof window === "undefined") {
    return "channels";
  }

  const stored = window.sessionStorage.getItem(TAB_STORAGE_KEY);
  if (
    stored === "channels" ||
    stored === "team" ||
    stored === "tasks" ||
    stored === "runs"
  ) {
    return stored;
  }

  // Migrate legacy "agents" / "live" tab storage.
  if (stored === "agents") {
    return "team";
  }

  return "channels";
}

/**
 * Persists the active Chat sidebar tab.
 */
export function storeChatTab(tab: ChatSidebarTab): void {
  if (typeof window === "undefined") {
    return;
  }

  window.sessionStorage.setItem(TAB_STORAGE_KEY, tab);
}

/**
 * Chat sidebar tab strip — TabsSubtle with activeLabel animations.
 */
export function ChatTabs({ value, onValueChange, badges }: ChatTabsProps) {
  const selectedIndex = Math.max(0, TAB_IDS.indexOf(value));

  return (
    <TabsSubtle
      activeLabel
      selectedIndex={selectedIndex}
      onSelect={(index) => {
        const next = TAB_IDS[index];
        if (next) {
          onValueChange(next);
        }
      }}
      idPrefix="sidebar-chat-tabs"
      className="w-full pb-4"
      // Sidebar surface matches muted/accent — use sidebar tokens so the active pill reads clearly.
      indicatorClassName="bg-sidebar-accent"
      hoverIndicatorClassName="bg-sidebar-accent/60"
    >
      {TABS.map((tab, index) => {
        const count = badges?.[tab.id] ?? 0;
        return (
          <TabsSubtleItem
            key={tab.id}
            index={index}
            icon={TAB_ICON_COMPONENTS[tab.id]}
            label={tab.label}
            count={count > 0 ? count : undefined}
            aria-label={tab.label}
          />
        );
      })}
    </TabsSubtle>
  );
}
