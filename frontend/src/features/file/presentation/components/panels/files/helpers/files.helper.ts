import { getIconForExtension } from "@/lib/utils";
import type { WorkspaceFile } from "@/features/file/interfaces/file.interfaces";
import type { LucideIcon } from "lucide-react";

export interface FilesTreeNode {
  children: FilesTreeNode[];
  entry?: WorkspaceFile;
  id: string;
  name: string;
  path: string;
  type: "directory" | "file";
}

export function getFileIcon(entry: WorkspaceFile): LucideIcon {
  return getIconForExtension(entry.browserUrl);
}

export function filterFiles(entries: WorkspaceFile[], searchQuery: string) {
  const query = searchQuery.trim().toLowerCase();

  if (!query) return entries;

  return entries.filter((entry) => {
    return (
      entry.name.toLowerCase().includes(query) ||
      entry.path.toLowerCase().includes(query) ||
      (entry.parentPath ?? "").toLowerCase().includes(query)
    );
  });
}

export function buildFilesTree(entries: WorkspaceFile[]): FilesTreeNode[] {
  const root = new Map<string, MutableFilesTreeNode>();

  for (const entry of entries) {
    const segments = entry.path.split("/");
    let currentLevel = root;
    let currentPath = "";

    segments.forEach((segment, index) => {
      currentPath = currentPath ? `${currentPath}/${segment}` : segment;
      const isFile = index === segments.length - 1 && entry.type === "file";

      if (!currentLevel.has(segment)) {
        currentLevel.set(segment, {
          childrenMap: new Map<string, MutableFilesTreeNode>(),
          children: [],
          entry: isFile ? entry : undefined,
          id: currentPath,
          name: segment,
          path: currentPath,
          type: isFile ? "file" : "directory",
        });
      }

      const node = currentLevel.get(segment)!;

      if (index === segments.length - 1) {
        node.entry = entry;
        node.type = entry.type;
      } else {
        currentLevel = node.childrenMap;
      }
    });
  }

  return sortTreeNodes(Array.from(root.values()));
}

export function formatFileSize(size: number) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

export function getFileBadgeLabel(entry: WorkspaceFile) {
  if (entry.type === "directory") return "Directory";

  switch (entry.viewer) {
    case "text":
      return entry.isEditable ? "Editable" : "Text";
    case "markdown":
      return entry.isEditable ? "Editable" : "Markdown";
    case "json":
      return entry.isEditable ? "Editable" : "JSON";
    case "image":
      return "Image";
    case "pdf":
      return "PDF";
    case "excalidraw":
      return "Excalidraw";
    case "video":
      return "Video";
    case "audio":
      return "Audio";
    case "docx":
      return "DOCX";
    case "xlsx":
      return "Excel";
    case "csv":
      return "CSV";
    case "archive":
      return "Archive";
    default:
      return "External";
  }
}

export function getFileTypeLabel(entry: WorkspaceFile) {
  if (entry.type === "directory") return "FOLDER";
  if (!entry.extension) return "Unknown";
  return entry.extension.toUpperCase();
}

export function getRootFolderLabel(entries: WorkspaceFile[]) {
  return entries[0]?.path.split("/")[0] ?? "Files";
}

interface MutableFilesTreeNode extends FilesTreeNode {
  childrenMap: Map<string, MutableFilesTreeNode>;
}

function sortTreeNodes(nodes: MutableFilesTreeNode[]): FilesTreeNode[] {
  return nodes
    .map((node) => ({
      ...node,
      children: sortTreeNodes(Array.from(node.childrenMap.values())),
    }))
    .sort((left, right) => {
      if (left.type !== right.type) return left.type === "directory" ? -1 : 1;
      return left.name.localeCompare(right.name);
    });
}
