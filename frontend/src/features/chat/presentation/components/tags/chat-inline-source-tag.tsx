import * as React from "react";
import { useNavigate } from "@tanstack/react-router";
import { FileSymlink, Folder, Sparkles } from "lucide-react";
import { aos } from "@/app/aos";
import { cn } from "@/lib/utils";
import type { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";
import type { ChatInlineSourceType } from "../../helpers/chat-inline-markup.helper";

interface ChatInlineSourceTagProps {
  className?: string;
  name: string;
  path: string;
  sourceType: ChatInlineSourceType;
}

export function ChatInlineSourceTag({
  className,
  name,
  path,
  sourceType,
}: ChatInlineSourceTagProps) {
  const navigate = useNavigate();
  const workspace = aos.stores.workspace.useState((state) => state.current);
  const viewportTabs = aos.stores.viewport.useState((state) => state.tabs);
  const isSkill = sourceType === "skill";
  const isFolder = sourceType === "folder";
  const Icon = isSkill ? Sparkles : isFolder ? Folder : FileSymlink;

  const handleClick = React.useCallback(() => {
    if (isSkill) {
      aos.stores.viewport.actions.closeSettings();
      void navigate({ to: "/marketplace", params: { name } });
      return;
    }

    if (isFolder) {
      aos.stores.viewport.actions.setSidebarMenu("files");
      return;
    }

    if (!path) {
      return;
    }

    const relativePath =
      workspace?.path && path.startsWith(`${workspace.path}/`)
        ? path.slice(workspace.path.length + 1)
        : path;

    const existingTab = viewportTabs.items.find(
      (tab) => tab.type === "file" && getTabFilePath(tab) === relativePath,
    );

    if (existingTab) {
      aos.stores.viewport.actions.setActiveTab(existingTab.id);
      return;
    }

    const placeholderTab = viewportTabs.items.find(
      (tab) => tab.type === "file" && !getTabFilePath(tab),
    );

    const metadata = {
      fileAbsolutePath: path,
      fileBrowserUrl: toBrowserUrl(path),
      fileExtension: getExtension(name),
      fileIsEditable: getViewer(name) === "text",
      fileName: name,
      filePath: relativePath,
      fileViewer: getViewer(name),
    };

    if (placeholderTab) {
      aos.stores.viewport.actions.updateTab(placeholderTab.id, {
        title: name,
        metadata: {
          ...placeholderTab.metadata,
          ...metadata,
        },
      });
      aos.stores.viewport.actions.setActiveTab(placeholderTab.id);
      return;
    }

    const createdTabId = aos.stores.viewport.actions.createTab({
      type: "file",
      title: name,
      closable: true,
      metadata,
    });

    if (createdTabId) {
      aos.stores.viewport.actions.setActiveTab(createdTabId);
    }
  }, [isFolder, isSkill, name, path, viewportTabs.items, workspace?.path]);

  return (
    <button
      className={cn(
        "inline-flex h-7 max-w-full items-center gap-1.5 rounded-full border px-2.5 text-[12px] font-medium",
        isSkill
          ? "border-violet-200/80 bg-violet-50 text-violet-700 hover:bg-violet-100 dark:border-violet-500/30 dark:bg-violet-500/10 dark:text-violet-200"
          : isFolder
            ? "border-amber-200/80 bg-amber-50 text-amber-700 hover:bg-amber-100 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200"
            : "border-sky-200/80 bg-sky-50 text-sky-700 hover:bg-sky-100 dark:border-sky-500/30 dark:bg-sky-500/10 dark:text-sky-200",
        className,
      )}
      onClick={handleClick}
      type="button"
    >
      <Icon className="size-3.5 shrink-0" />
      <span className="truncate">{name.replace("user-content-", "")}</span>
    </button>
  );
}

function getTabFilePath(tab: ViewportTabState) {
  return typeof tab.metadata?.filePath === "string"
    ? tab.metadata.filePath
    : undefined;
}

function toBrowserUrl(path: string) {
  return `file://${encodeURI(path)}`;
}

function getExtension(name: string) {
  const parts = name.split(".");
  return parts.length > 1 ? (parts.at(-1)?.toLowerCase() ?? "") : "";
}

function getViewer(name: string) {
  const extension = getExtension(name);

  if (["png", "jpg", "jpeg", "gif", "svg", "webp", "ico"].includes(extension))
    return "image";
  if (["mp4", "mov", "webm", "avi", "mkv"].includes(extension)) return "video";
  if (["mp3", "wav", "ogg", "m4a", "aac"].includes(extension)) return "audio";
  if (["pdf"].includes(extension)) return "pdf";
  if (["csv", "xls", "xlsx"].includes(extension)) return "spreadsheet";
  if (["zip", "tar", "gz", "rar", "7z"].includes(extension)) return "archive";
  return "text";
}
