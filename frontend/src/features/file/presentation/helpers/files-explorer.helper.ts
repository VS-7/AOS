import type { CSSProperties } from "react";
import type {
  WorkspaceFile,
  FileExplorerContext,
  FileExplorerSnapshot,
} from "@/features/file/interfaces/file.interfaces";
import type { ThemeIconsConfig } from "@/features/theme/presentation/stores/theme.store";
import { getIconForExtension } from "@/lib/utils";
import type { LucideIcon } from "lucide-react";
import { File, FileCode, FileImage, FileText, Folder } from "lucide-react";
import {
  getFileExtension,
  isEditableFileViewer,
  resolveFileViewer,
} from "@/features/file/presentation/helpers/file-viewer.helper";

export function serializeExplorerContext(
  context: FileExplorerContext,
): string {
  return JSON.stringify(context);
}

export function parseExplorerContext(
  value: unknown,
): FileExplorerContext {
  if (!value) return { type: "main" };

  if (typeof value === "object" && value !== null && "type" in value) {
    const candidate = value as FileExplorerContext;
    if (candidate.type === "main") return { type: "main" };
    if (candidate.type === "task" && typeof candidate.taskId === "string" && candidate.taskId) {
      return { type: "task", taskId: candidate.taskId };
    }
    if (candidate.type === "branch" && typeof candidate.branch === "string" && candidate.branch) {
      return { type: "branch", branch: candidate.branch };
    }
    return { type: "main" };
  }

  if (typeof value === "string") {
    const trimmed = value.trim();
    if (!trimmed || trimmed === "main") return { type: "main" };

    try {
      return parseExplorerContext(JSON.parse(trimmed));
    } catch {
      return { type: "main" };
    }
  }

  return { type: "main" };
}

export function formatExplorerContextLabel(
  context: FileExplorerContext,
  snapshot?: Pick<FileExplorerSnapshot, "tasks"> | null,
): string {
  if (context.type === "main") return "main";
  if (context.type === "branch") return context.branch;
  const task = snapshot?.tasks?.find((entry) => entry.id === context.taskId);
  return task?.title || context.taskId;
}

export function explorerContextsEqual(
  left: FileExplorerContext,
  right: FileExplorerContext,
): boolean {
  if (left.type !== right.type) return false;
  if (left.type === "task" && right.type === "task") {
    return left.taskId === right.taskId;
  }
  if (left.type === "branch" && right.type === "branch") {
    return left.branch === right.branch;
  }
  return true;
}

export function getAncestorPaths(filePath: string): string[] {
  const segments = filePath.replace(/\/+$/, "").split("/").filter(Boolean);
  if (segments.length <= 1) return [];
  // Pierre expand paths must use trailing `/` for directories.
  return segments
    .slice(0, -1)
    .map((_, index) => `${segments.slice(0, index + 1).join("/")}/`);
}

export function parentPathOf(filePath: string): string {
  const segments = filePath.replace(/\/+$/, "").split("/").filter(Boolean);
  if (segments.length <= 1) return "";
  return `${segments.slice(0, -1).join("/")}/`;
}

export function lookupPathIndex<T>(
  pathIndex: Record<string, T> | undefined,
  filePath: string,
): T | undefined {
  if (!pathIndex) return undefined;
  const stripped = filePath.replace(/\/+$/, "");
  return (
    pathIndex[filePath] ??
    pathIndex[stripped] ??
    (stripped ? pathIndex[`${stripped}/`] : undefined)
  );
}

/**
 * Resolves where New File / New Folder should land: a selected directory →
 * that folder; a selected file → its parent; nothing selected → the root.
 *
 * Selection only, never focus. `@pierre/trees` focuses its first row when the
 * tree mounts, so reading focus as intent put every header "New file" inside
 * whichever folder happened to sort first, with nothing selected at all.
 * `focusedPath` is still accepted so a caller passing it compiles; it is not
 * read.
 */
export function resolveCreateParentPath(params: {
  focusedPath?: string | null;
  selectedPaths?: readonly string[];
  pathIndex?: Record<string, { type: "file" | "directory" }>;
  isDirectoryPath?: (path: string) => boolean;
}): string {
  const candidate = params.selectedPaths?.at(-1) || null;

  if (!candidate) return "";

  const indexed = lookupPathIndex(params.pathIndex, candidate);
  const isDirectory =
    candidate.endsWith("/") ||
    indexed?.type === "directory" ||
    params.isDirectoryPath?.(candidate) === true;

  if (isDirectory) {
    return candidate.replace(/\/+$/, "");
  }

  return parentPathOf(candidate).replace(/\/+$/, "");
}

/**
 * Where a pasted copy of `destination` should go without replacing anything:
 * the path itself when it is free, otherwise "name copy.ext", then
 * "name copy 2.ext" and on — the names a file manager gives a duplicate.
 *
 * `taken` holds workspace-relative paths; a trailing slash on either side is
 * ignored, since the tree marks directories with one and the daemon does not.
 */
export function copyDestinationPath(destination: string, taken: ReadonlySet<string>): string {
  const normalized = destination.replace(/\/+$/, "");
  const isTaken = (path: string) => taken.has(path) || taken.has(`${path}/`);
  if (!isTaken(normalized)) return normalized;

  const parent = parentPathOf(normalized).replace(/\/+$/, "");
  const name = basenameOf(normalized);
  // The extension starts at the last dot that is not the first character, so
  // ".env.sample" keeps ".sample" and ".gitignore" has no extension at all.
  const dot = name.lastIndexOf(".");
  const stem = dot > 0 ? name.slice(0, dot) : name;
  const extension = dot > 0 ? name.slice(dot) : "";

  for (let attempt = 1; ; attempt++) {
    const suffix = attempt === 1 ? " copy" : ` copy ${attempt}`;
    const candidate = joinWorkspacePath(parent, `${stem}${suffix}${extension}`);
    if (!isTaken(candidate)) return candidate;
  }
}

export function formatCreateDestinationPath(parentPath: string): string {
  const normalized = parentPath.replace(/\/+$/, "");
  return normalized || "/";
}

export function joinWorkspacePath(...parts: string[]): string {
  return parts
    .flatMap((part) => part.split("/"))
    .filter(Boolean)
    .join("/");
}

export function basenameOf(filePath: string): string {
  const segments = filePath.split("/").filter(Boolean);
  return segments.at(-1) ?? filePath;
}

export function synthesizeFileFromPath(
  filePath: string,
  allPaths: readonly string[],
  explorerRootAbsolute?: string,
): WorkspaceFile {
  const hasTrailingSlash = /\/$/.test(filePath);
  const normalized = filePath.replace(/\/+$/, "");
  const name = basenameOf(normalized);
  const isDirectory =
    hasTrailingSlash ||
    allPaths.some(
      (entry) =>
        entry === `${normalized}/` || entry.startsWith(`${normalized}/`),
    );
  const extension = isDirectory ? "" : getFileExtension(name);
  const explorerPath = isDirectory ? `${normalized}/` : normalized;
  const absolutePath = explorerRootAbsolute
    ? `${explorerRootAbsolute.replace(/\/$/, "")}/${normalized}`
    : normalized;

  const viewer = isDirectory ? "other" : resolveFileViewer(name);

  return {
    absolutePath,
    browserUrl: `file://${absolutePath}`,
    createdAt: new Date(0).toISOString(),
    extension,
    hasChildren: isDirectory || undefined,
    isEditable: !isDirectory && isEditableFileViewer(viewer),
    name,
    path: explorerPath,
    parentPath: parentPathOf(normalized) || undefined,
    size: 0,
    type: isDirectory ? "directory" : "file",
    updatedAt: new Date(0).toISOString(),
    viewer,
  };
}

/**
 * Tab / fallback icons aligned with AOS file helpers.
 * Pierre tree icons stay inside the shadow DOM via theme.icons;
 * tabs reuse Lucide mapped by extension for a consistent language.
 */
export function getFileTabIcon(
  pathOrExtension: string,
  _icons?: ThemeIconsConfig,
): LucideIcon {
  const value = pathOrExtension.toLowerCase();
  const extension = value.includes(".")
    ? (value.split(".").pop() ?? "")
    : value;

  if (!extension) return File;

  if (["ts", "tsx", "js", "jsx", "mjs", "cjs"].includes(extension)) {
    return FileCode;
  }

  if (["png", "jpg", "jpeg", "gif", "svg", "webp", "avif"].includes(extension)) {
    return FileImage;
  }

  if (["md", "mdx", "txt", "pdf"].includes(extension)) {
    return FileText;
  }

  try {
    return getIconForExtension(`file:///workspace/file.${extension}`);
  } catch {
    return File;
  }
}

export function getDirectoryTabIcon(): LucideIcon {
  return Folder;
}

export function buildFileTabMetadata(
  file: WorkspaceFile,
  explorerContext: FileExplorerContext,
  extras?: { fileDirty?: boolean; fileReadOnly?: boolean },
) {
  return {
    fileAbsolutePath: file.absolutePath,
    fileBrowserUrl: file.browserUrl,
    fileExtension: file.extension,
    fileIsEditable: file.isEditable,
    fileName: file.name,
    filePath: file.path,
    fileViewer: file.viewer,
    fileExplorerContext: serializeExplorerContext(explorerContext),
    fileDirty: extras?.fileDirty ?? false,
    fileReadOnly: extras?.fileReadOnly ?? false,
  };
}

export const FILES_TREE_THEME_STYLE = {
  "--trees-bg-override": "transparent",
  "--trees-theme-sidebar-bg": "transparent",
  "--trees-padding-inline-override": "0px",
  "--trees-item-margin-x-override": "0px",
  "--trees-theme-list-active-selection-bg":
    "color-mix(in oklab, var(--accent) 24%, transparent)",
  "--trees-theme-list-hover-bg":
    "color-mix(in oklab, var(--accent) 12%, transparent)",
  "--trees-theme-focus-ring": "var(--ring)",
} as CSSProperties;
