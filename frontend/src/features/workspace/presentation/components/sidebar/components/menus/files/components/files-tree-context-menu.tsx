import * as React from "react";
import { createPortal } from "react-dom";
import {
  ClipboardCopy,
  ClipboardPaste,
  Copy,
  ExternalLink,
  FilePlus,
  FolderPlus,
  Pencil,
  Scissors,
  Trash2,
} from "lucide-react";
import { toast } from "sonner";
import type {
  ContextMenuItem,
  ContextMenuOpenContext,
} from "@pierre/trees";
import { api } from "@/lib/aos-facade";
import { useAlert } from "@/components/ui/alert-provider";
import { DropdownMenuSeparator } from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import type { FileTree as PierreFileTree } from "@pierre/trees";
import type {
  FileExplorerContext,
  FileExplorerSnapshot,
} from "@/features/file/interfaces/file.interfaces";
import {
  explorerContextsEqual,
  lookupPathIndex,
  parentPathOf,
} from "@/features/file/presentation/helpers/files-explorer.helper";
import type { FilesClipboardState } from "@/features/file/presentation/stores/files.store";
import { pasteFilesClipboard } from "./files-tree-clipboard.helper";

export interface FilesCreateNodeRequest {
  parentPath: string;
  type: "file" | "directory";
}

interface FilesTreeContextMenuProps {
  item?: ContextMenuItem;
  context: ContextMenuOpenContext;
  model: PierreFileTree;
  snapshot: FileExplorerSnapshot | undefined;
  explorerContext: FileExplorerContext;
  clipboard: FilesClipboardState | null;
  onCreateNode: (request: FilesCreateNodeRequest) => void;
  onClipboardChange: (clipboard: FilesClipboardState | null) => void;
  onFilesChanged: () => void;
  /** Pointer coords for empty-space menus (viewport). */
  anchor?: { x: number; y: number };
}

function MenuButton({
  children,
  disabled,
  destructive,
  onClick,
}: {
  children: React.ReactNode;
  disabled?: boolean;
  destructive?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className={cn(
        "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm outline-hidden select-none",
        "hover:bg-accent hover:text-accent-foreground",
        "disabled:pointer-events-none disabled:opacity-50",
        destructive && "text-destructive hover:bg-destructive/10 hover:text-destructive",
      )}
    >
      {children}
    </button>
  );
}

function writeClipboard(text: string) {
  void navigator.clipboard.writeText(text).then(
    () => toast.success("Copied to clipboard."),
    () => toast.error("Unable to copy to clipboard."),
  );
}

function resolveMenuPosition(params: {
  anchor?: { x: number; y: number };
  anchorRect: ContextMenuOpenContext["anchorRect"];
  menuWidth: number;
  menuHeight: number;
}) {
  const rawX = params.anchor?.x ?? params.anchorRect.x ?? params.anchorRect.left;
  const rawY = params.anchor?.y ?? params.anchorRect.y ?? params.anchorRect.top;
  const padding = 8;
  const maxX = window.innerWidth - params.menuWidth - padding;
  const maxY = window.innerHeight - params.menuHeight - padding;

  return {
    left: Math.max(padding, Math.min(rawX, maxX)),
    top: Math.max(padding, Math.min(rawY, maxY)),
  };
}

export function FilesTreeContextMenu({
  item,
  context,
  model,
  snapshot,
  explorerContext,
  clipboard,
  onCreateNode,
  onClipboardChange,
  onFilesChanged,
  anchor,
}: FilesTreeContextMenuProps) {
  const { confirm } = useAlert();
  const menuRef = React.useRef<HTMLDivElement>(null);
  const [menuSize, setMenuSize] = React.useState({ width: 208, height: 240 });
  const readOnly = snapshot?.readOnly ?? false;
  const canReveal = Boolean(window.aos?.system?.showItemInFolder);
  const isEmptySpaceMenu = item == null;
  const parentPath = item
    ? item.kind === "directory"
      ? item.path.replace(/\/+$/, "")
      : parentPathOf(item.path).replace(/\/+$/, "")
    : "";

  const absolutePath = item
    ? (lookupPathIndex(snapshot?.pathIndex, item.path)?.absolutePath as string | undefined)
    : undefined;

  const canPaste =
    !readOnly &&
    clipboard != null &&
    explorerContextsEqual(clipboard.context, explorerContext) &&
    clipboard.paths.length > 0;

  const position = resolveMenuPosition({
    anchor,
    anchorRect: context.anchorRect,
    menuWidth: menuSize.width,
    menuHeight: menuSize.height,
  });

  React.useLayoutEffect(() => {
    const node = menuRef.current;
    if (!node) return;
    const rect = node.getBoundingClientRect();
    setMenuSize({
      width: Math.ceil(rect.width),
      height: Math.ceil(rect.height),
    });
  }, [item, canPaste, canReveal, readOnly]);

  async function handleDelete() {
    if (!item) return;

    const accepted = await confirm({
      title: `Delete "${item.name}"?`,
      description: "This action cannot be undone.",
      confirmText: "Delete",
      variant: "destructive",
    });

    if (!accepted) return;

    context.close();

    const response = await api.file.delete.mutate({
      body: {
        path: item.path,
        context: explorerContext,
      },
    });

    if (response.error) {
      toast.error(
        (response.error as { message?: string })?.message ||
          `Unable to delete "${item.name}".`,
      );
      return;
    }

    toast.success("Deleted.");
    onFilesChanged();
  }

  async function handlePaste() {
    if (!canPaste || !clipboard) return;

    context.close();

    try {
      await pasteFilesClipboard({
        clipboard,
        targetParentPath: parentPath,
        explorerContext,
        pathIndex: snapshot?.pathIndex,
      });

      if (clipboard.mode === "cut") {
        onClipboardChange(null);
      }

      onFilesChanged();
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : "Unable to paste items.",
      );
    }
  }

  const menu = (
    <div
      ref={menuRef}
      data-file-tree-context-menu-root="true"
      className="fixed z-[9999] min-w-52 rounded-lg border bg-popover p-1 text-popover-foreground shadow-md ring-1 ring-foreground/10"
      style={{ top: position.top, left: position.left }}
    >
      {isEmptySpaceMenu ? (
        <>
          <MenuButton
            disabled={readOnly}
            onClick={() => {
              context.close();
              onCreateNode({ parentPath: "", type: "file" });
            }}
          >
            <FilePlus className="size-4" />
            New File
          </MenuButton>
          <MenuButton
            disabled={readOnly}
            onClick={() => {
              context.close();
              onCreateNode({ parentPath: "", type: "directory" });
            }}
          >
            <FolderPlus className="size-4" />
            New Folder
          </MenuButton>
          <DropdownMenuSeparator />
          <MenuButton disabled={!canPaste} onClick={() => void handlePaste()}>
            <ClipboardPaste className="size-4" />
            Paste
          </MenuButton>
        </>
      ) : (
        <>
          {canReveal && absolutePath ? (
            <MenuButton
              onClick={() => {
                context.close();
                void window.aos?.system?.showItemInFolder?.(absolutePath);
              }}
            >
              <ExternalLink className="size-4" />
              Reveal in Finder
            </MenuButton>
          ) : null}

          {canReveal && absolutePath ? <DropdownMenuSeparator /> : null}

          <MenuButton
            onClick={() => {
              context.close();
              writeClipboard(item.path);
            }}
          >
            <Copy className="size-4" />
            Copy Path
          </MenuButton>
          <MenuButton
            onClick={() => {
              context.close();
              writeClipboard(item.name);
            }}
          >
            <ClipboardCopy className="size-4" />
            Copy Relative Path
          </MenuButton>

          <DropdownMenuSeparator />

          <MenuButton
            disabled={readOnly}
            onClick={() => {
              context.close();
              onClipboardChange({
                mode: "cut",
                paths: [item.path],
                context: explorerContext,
              });
              toast.success("Cut to clipboard.");
            }}
          >
            <Scissors className="size-4" />
            Cut
          </MenuButton>
          <MenuButton
            disabled={readOnly}
            onClick={() => {
              context.close();
              onClipboardChange({
                mode: "copy",
                paths: [item.path],
                context: explorerContext,
              });
              toast.success("Copied to clipboard.");
            }}
          >
            <ClipboardCopy className="size-4" />
            Copy
          </MenuButton>
          <MenuButton disabled={!canPaste} onClick={() => void handlePaste()}>
            <ClipboardPaste className="size-4" />
            Paste
          </MenuButton>

          <DropdownMenuSeparator />

          <MenuButton
            disabled={readOnly}
            onClick={() => {
              context.close({ restoreFocus: false });
              model.startRenaming(item.path);
            }}
          >
            <Pencil className="size-4" />
            Rename
          </MenuButton>
          <MenuButton
            destructive
            disabled={readOnly}
            onClick={() => void handleDelete()}
          >
            <Trash2 className="size-4" />
            Delete
          </MenuButton>
        </>
      )}
    </div>
  );

  if (typeof document === "undefined") {
    return menu;
  }

  return createPortal(menu, document.body);
}
