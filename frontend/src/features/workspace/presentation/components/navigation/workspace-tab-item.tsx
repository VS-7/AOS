"use client";

import * as React from "react";
import { AnimatePresence, motion } from "motion/react";
import {
  AppWindow,
  GitCompareArrows,
  Globe,
  LoaderCircle,
  MessageSquare,
  X,
} from "lucide-react";
import { useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import type { DraggableAttributes } from "@dnd-kit/core";
import { cn } from "@/lib/utils";
import { aos } from "@/app/aos";
import { getTabLabel } from "@/app/lib/tabs";
import { springs } from "@/lib/springs";
import { useAlert } from "@/components/ui/alert-provider";
import { getFileTabIcon } from "@/features/file/presentation/helpers/files-explorer.helper";
import { requestCloseFileTab } from "@/features/file/presentation/helpers/request-close-file-tab.helper";
import type { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";
import { t } from "@/lib/i18n";

const tabIconTransition = springs.fast;

type TabDragListeners = ReturnType<typeof useSortable>["listeners"];

function renderTabIcon(
  tab: ViewportTabState,
  icons?: { set: string; colored: boolean },
) {
  if (tab.type === "in-app") {
    return <AppWindow className="size-3.5 text-primary" />;
  }

  if (tab.type === "file") {
    const path =
      typeof tab.metadata?.filePath === "string"
        ? tab.metadata.filePath
        : typeof tab.metadata?.fileExtension === "string"
          ? tab.metadata.fileExtension
          : tab.title;
    const Icon = getFileTabIcon(path, icons as any);
    return <Icon className="size-3.5 text-primary" />;
  }

  if (tab.type === "changes") {
    return <GitCompareArrows className="size-3.5 text-primary" />;
  }

  if (tab.type === "chat") {
    return <MessageSquare className="size-3.5 text-primary" />;
  }

  if (tab.status === "loading") {
    return (
      <LoaderCircle className="size-3.5 animate-spin text-muted-foreground" />
    );
  }

  if (tab.favicon) {
    return <img src={tab.favicon} className="size-3.5 rounded-sm" alt="" />;
  }

  return <Globe className="size-3.5 text-muted-foreground" />;
}

export type WorkspaceTabItemVariant = "static" | "sortable" | "overlay";

interface WorkspaceTabItemProps {
  tab: ViewportTabState;
  isActive: boolean;
  onSelect: (tabId: string) => void;
  variant?: WorkspaceTabItemVariant;
  isDragging?: boolean;
  nodeRef?: (element: HTMLElement | null) => void;
  style?: React.CSSProperties;
  dragAttributes?: DraggableAttributes;
  dragListeners?: TabDragListeners;
}

export function WorkspaceTabItem({
  tab,
  isActive,
  onSelect,
  variant = "static",
  isDragging = false,
  nodeRef,
  style,
  dragAttributes,
  dragListeners,
}: WorkspaceTabItemProps) {
  const { promptUnsaved } = useAlert();
  const themeIcons = aos.stores.theme.useState(
    (state) => state.icons ?? { set: "complete", colored: true },
  );
  const [isHovered, setIsHovered] = React.useState(false);
  const closable = tab.closable !== false;
  const showClose = closable && isHovered && variant !== "overlay";
  const isDirty = tab.type === "file" && Boolean(tab.metadata?.fileDirty);

  return (
    <div
      ref={nodeRef}
      style={style}
      {...dragAttributes}
      {...(variant === "overlay" ? undefined : dragListeners)}
      onClick={() => onSelect(tab.id)}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      className={cn(
        "group flex min-w-0 max-w-60 select-none no-drag items-center gap-1.5 rounded-md border px-1.5 text-sm h-8",
        variant === "overlay"
          ? "cursor-grabbing border-input bg-background text-foreground shadow-md scale-[1.02]"
          : isDragging
            ? "cursor-grabbing transition-colors"
            : "cursor-default transition-colors",
        isDragging && variant !== "overlay" && "opacity-40",
        isActive
          ? "bg-secondary border-transparent text-foreground"
          : "bg-transparent border-transparent text-muted-foreground hover:border-border/60 hover:text-foreground",
      )}
    >
      <div className="relative flex size-5 shrink-0 items-center justify-center">
        <AnimatePresence initial={false}>
          {showClose ? (
            <motion.button
              key="close"
              type="button"
              aria-label={`Close ${getTabLabel(tab)}`}
              initial={{ opacity: 0, scale: 0.88 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.88 }}
              transition={tabIconTransition}
              onPointerDown={(event) => event.stopPropagation()}
              onClick={(event) => {
                event.stopPropagation();
                void requestCloseFileTab(tab.id, promptUnsaved);
              }}
              className="absolute inset-0 flex items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
            >
              <X className="size-3.5" />
            </motion.button>
          ) : (
            <motion.span
              key="icon"
              initial={{ opacity: 0, scale: 0.88 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.88 }}
              transition={tabIconTransition}
              className="absolute inset-0 flex items-center justify-center"
            >
              {renderTabIcon(tab, themeIcons)}
            </motion.span>
          )}
        </AnimatePresence>
      </div>

      <span className="min-w-0 flex-1 truncate">{getTabLabel(tab)}</span>
      {isDirty ? (
        <span
          aria-label={t("Unsaved changes")}
          className="size-1.5 shrink-0 rounded-md bg-primary"
        />
      ) : null}
    </div>
  );
}

interface WorkspaceSortableTabItemProps {
  tab: ViewportTabState;
  isActive: boolean;
  onSelect: (tabId: string) => void;
}

export function WorkspaceSortableTabItem({
  tab,
  isActive,
  onSelect,
}: WorkspaceSortableTabItemProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({
    id: tab.id,
    disabled: tab.id === "aos",
  });

  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <WorkspaceTabItem
      tab={tab}
      isActive={isActive}
      onSelect={onSelect}
      variant="sortable"
      isDragging={isDragging}
      nodeRef={setNodeRef}
      style={style}
      dragAttributes={attributes}
      dragListeners={listeners}
    />
  );
}
