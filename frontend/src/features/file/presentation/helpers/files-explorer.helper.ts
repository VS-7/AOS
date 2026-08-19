import type { CSSProperties } from "react";
import type {
  FractalFile,
  FractalFileExplorerContext,
  FractalFileExplorerSnapshot,
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
  context: FractalFileExplorerContext,
): string {
  return JSON.stringify(context);
}

export function parseExplorerContext(
  value: unknown,
): FractalFileExplorerContext {
  if (!value) return { type: "main" };

  if (typeof value === "object" && value !== null && "type" in value) {
    const candidate = value as FractalFileExplorerContext;
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
  context: FractalFileExplorerContext,
  snapshot?: Pick<FractalFileExplorerSnapshot, "tasks"> | null,
): string {
  if (context.type === "main") return "main";
  if (context.type === "branch") return context.branch;
  const task = snapshot?.tasks?.find((entry) => entry.id === context.taskId);
  return task?.title || context.taskId;
}

export function explorerContextsEqual(
  left: FractalFileExplorerContext,
  right: FractalFileExplorerContext,
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
 * Resolves where New File / New Folder should land:
 * focused/selected directory → that folder; file → its parent; else workspace root.
 */
export function resolveCreateParentPath(params: {
  focusedPath?: string | null;
  selectedPaths?: readonly string[];
  pathIndex?: Record<string, { type: "file" | "directory" }>;
  isDirectoryPath?: (path: string) => boolean;
}): string {
  const candidate =
    params.focusedPath ||
    params.selectedPaths?.at(-1) ||
    null;

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
): FractalFile {
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
 * Tab / fallback icons aligned with Fractal file helpers.
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
  file: FractalFile,
  explorerContext: FractalFileExplorerContext,
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
