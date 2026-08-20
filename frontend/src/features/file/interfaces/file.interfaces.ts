import { z } from "zod";
import type { ResponseWithCTA } from "@/core/interfaces/response.interfaces";
import { Schema } from "@/core/helpers/schema.helper";

function queryBoolean(defaultValue = false) {
  return z.preprocess((value) => {
    if (value === undefined || value === null || value === "") return undefined;
    if (typeof value === "boolean") return value;

    if (typeof value === "string") {
      const normalized = value.trim().toLowerCase();
      if (["true", "1", "yes", "on"].includes(normalized)) return true;
      if (["false", "0", "no", "off"].includes(normalized)) return false;
    }

    return value;
  }, z.boolean().default(defaultValue));
}

/**
 * Defines the supported node kinds in the workspace file explorer.
 * @example
 * ```typescript
 * const kind = FileKindSchema.parse("file");
 * ```
 */
export const FileKindSchema = z.enum(["file", "directory"]);

/**
 * Defines the supported in-app viewers for a workspace file.
 * @example
 * ```typescript
 * const viewer = FileViewerSchema.parse("text");
 * ```
 */
export const FileViewerSchema = z.enum([
  "text",
  "image",
  "pdf",
  "excalidraw",
  "video",
  "audio",
  "spreadsheet",
  "archive",
  "other",
  // Task 9 additions — this file's source enum lacked these, but
  // `presentation/components/panels/files/helpers/files.helper.ts` (viewer
  // resolution by extension) and `content/index.tsx`/`content/files-
  // external-viewer.component.tsx` (viewer-specific rendering) switch on
  // all five as their own distinct viewer kinds, not folded into
  // `"spreadsheet"`/`"other"`.
  "markdown",
  "json",
  "docx",
  "xlsx",
  "csv",
]);

/**
 * Describes a file-system node exposed by the Files feature.
 * @example
 * ```typescript
 * const file = FileSchema.parse({
 *   absolutePath: "/workspace/src/index.ts",
 *   browserUrl: "file:///workspace/src/index.ts",
 *   extension: "ts",
 *   isEditable: true,
 *   name: "index.ts",
 *   path: "src/index.ts",
 *   parentPath: "src",
 *   size: 120,
 *   type: "file",
 *   viewer: "text",
 *   createdAt: new Date().toISOString(),
 *   updatedAt: new Date().toISOString(),
 * });
 * ```
 */
export const FileSchema = z.object({
  absolutePath: z.string().describe("Absolute path on disk."),
  browserUrl: z.string().describe("File URL used by external viewers."),
  childCount: z.number().int().nonnegative().optional().describe("Optional direct visible children count for directories."),
  createdAt: z.string().describe("ISO timestamp derived from filesystem metadata."),
  extension: z.string().describe("Normalized lowercase extension without leading dot."),
  hasChildren: z.boolean().optional().describe("Whether the directory currently exposes visible children."),
  isEditable: z.boolean().describe("Whether the file is supported by the in-app text editor."),
  mimeType: z.string().optional().describe("Optional MIME type inferred from the extension."),
  name: z.string().describe("Base file or directory name."),
  path: z.string().describe("Workspace-relative path using POSIX separators."),
  parentPath: z.string().optional().describe("Workspace-relative parent directory path."),
  size: z.number().int().nonnegative().describe("Filesystem size in bytes."),
  type: FileKindSchema.describe("Whether the node is a file or directory."),
  updatedAt: z.string().describe("ISO timestamp derived from filesystem metadata."),
  viewer: FileViewerSchema.describe("Suggested viewer strategy for the UI."),
});

/**
 * Represents a file-system node available inside the current workspace.
 * Inferred from {@link FileSchema}.
 */
export type WorkspaceFile = z.infer<typeof FileSchema>;

/**
 * Defines the list query accepted by the Files controller.
 * @example
 * ```typescript
 * const query = FileListQuerySchema.parse({ recursive: true, includeIgnored: false });
 * ```
 */
export const FileListQuerySchema = Schema.object({
  path: z.string().optional().describe("Optional workspace-relative directory to list from."),
  recursive: queryBoolean(false).describe("Whether nested descendants should be traversed."),
  includeIgnored: queryBoolean(false).describe("Whether heavy and internal folders should be included."),
});

/**
 * Input parameters for listing workspace files.
 * Inferred from {@link FileListQuerySchema}.
 */
export type FileListQueryInput = z.infer<typeof FileListQuerySchema>;

/**
 * Defines the search query accepted by the Files controller.
 * @example
 * ```typescript
 * const query = FileSearchQuerySchema.parse({ query: "task", limit: 50 });
 * ```
 */
export const FileSearchQuerySchema = Schema.object({
  path: z.string().optional().describe("Optional workspace-relative directory used as the search root."),
  query: z.string().min(1).describe("Fuzzy query applied to file and folder names."),
  includeIgnored: queryBoolean(false).describe("Whether heavy and internal folders should be included."),
  limit: z.coerce.number().int().positive().max(500).default(200).describe("Maximum number of matched nodes before ancestors are added."),
});

/**
 * Input parameters for searching workspace files.
 * Inferred from {@link FileSearchQuerySchema}.
 */
export type FileSearchQueryInput = z.infer<typeof FileSearchQuerySchema>;

/**
 * Defines the read query accepted by the Files controller.
 * @example
 * ```typescript
 * const query = FileReadSchema.parse({ path: "src/index.ts" });
 * ```
 */
export const FileReadSchema = z.object({
  path: z.string().min(1).describe("Workspace-relative file path to read."),
});

/**
 * Input parameters for reading a file.
 * Inferred from {@link FileReadSchema}.
 */
export type FileReadInput = z.infer<typeof FileReadSchema>;

/**
 * Defines the write payload accepted by the Files controller.
 * @example
 * ```typescript
 * const payload = FileWriteSchema.parse({ path: "README.md", content: "# Updated" });
 * ```
 */
export const FileWriteSchema = z.object({
  content: z.string().describe("UTF-8 text content to persist on disk."),
  path: z.string().min(1).describe("Workspace-relative file path to write."),
});

/**
 * Input parameters for writing a file.
 * Inferred from {@link FileWriteSchema}.
 */
export type FileWriteInput = z.infer<typeof FileWriteSchema>;

/**
 * Defines the create payload accepted by the Files controller.
 * @example
 * ```typescript
 * const payload = FileCreateSchema.parse({
 *   path: "src/features/file/index.ts",
 *   type: "file",
 *   content: "export {}",
 * });
 * ```
 */
export const FileCreateSchema = z.object({
  content: z.string().optional().describe("Optional initial UTF-8 content for new text files."),
  overwrite: z.boolean().default(false).describe("Whether an existing file may be replaced."),
  path: z.string().min(1).describe("Workspace-relative target path for the new node."),
  type: FileKindSchema.describe("Whether to create a file or a directory."),
});

/**
 * Input parameters for creating a new file-system node.
 * Inferred from {@link FileCreateSchema}.
 */
export type FileCreateInput = z.infer<typeof FileCreateSchema>;

/**
 * Contract for the workspace-scoped Files service.
 */
export interface IFileService {
  /**
   * Lists files and directories from the current workspace.
   *
   * @param params - Listing options validated by {@link FileListQuerySchema}.
   * @returns A response containing flat file-system nodes for tree building in the UI.
   *
   * @example
   * ```typescript
   * const result = await fileService.list({ path: "src", recursive: true });
   * ```
  */
  list(params?: FileListQueryInput): Promise<ResponseWithCTA<{ files: WorkspaceFile[] }>>;

  /**
   * Searches files and folders from the current workspace.
   *
   * @param params - Search options validated by {@link FileSearchQuerySchema}.
   * @returns Matching file-system nodes plus ancestor folders for tree rendering in the UI.
   *
   * @example
   * ```typescript
   * const result = await fileService.search({ query: "task", limit: 50 });
   * ```
   */
  search(params: FileSearchQueryInput): Promise<ResponseWithCTA<{ files: WorkspaceFile[] }>>;

  /**
   * Reads UTF-8 content from a workspace file.
   *
   * @param params - Read options validated by {@link FileReadSchema}.
   * @returns The requested file metadata plus its current content.
   *
   * @example
   * ```typescript
   * const result = await fileService.read({ path: "README.md" });
   * ```
   */
  read(params: FileReadInput): Promise<ResponseWithCTA<{ file: WorkspaceFile; content: string }>>;

  /**
   * Resolves a workspace file for HTTP content serving.
   *
   * @param params - Read options validated by {@link FileReadSchema}.
   * @returns File metadata plus the absolute path used by the transport layer.
   */
  content(params: FileReadInput): Promise<ResponseWithCTA<{ file: WorkspaceFile; absolutePath: string }>>;

  /**
   * Persists UTF-8 content into an existing workspace file.
   *
   * @param params - Write options validated by {@link FileWriteSchema}.
   * @returns The refreshed file metadata plus the saved content.
   *
   * @example
   * ```typescript
   * const result = await fileService.write({ path: "README.md", content: "# Hello" });
   * ```
   */
  write(params: FileWriteInput): Promise<ResponseWithCTA<{ file: WorkspaceFile; content: string }>>;

  /**
   * Creates a new file or directory inside the workspace.
   *
   * @param params - Creation options validated by {@link FileCreateSchema}.
   * @returns The created file-system node.
   *
   * @example
   * ```typescript
   * const result = await fileService.create({ path: "src/features/file", type: "directory" });
   * ```
   */
  create(params: FileCreateInput): Promise<ResponseWithCTA<{ file: WorkspaceFile }>>;
}

/**
 * Inferred type alias for {@link FileViewerSchema} — the source
 * file had the schema but no bare exported type for it (same situation
 * `task.interfaces.ts`'s doc comment describes for `TaskStatus`).
 * `presentation/helpers/file-viewer.helper.ts` reads this as a return type.
 */
export type FileViewer = z.infer<typeof FileViewerSchema>;

/**
 * The file explorer's UI-only "scope" — which changes/tree the panel shows:
 * the live workspace, a task's worktree, or a specific branch. No Go
 * command returns this shape (`file.explorer`/`file.changes`/`file.search`
 * are all dormant in `lib/command-map.ts` — the UI-only concept has no
 * backend truth to check against yet), so this is reconstructed purely
 * from how `presentation/helpers/files-explorer.helper.ts` and
 * `presentation/helpers/open-changes-tab.helper.ts` construct, parse, and
 * compare it — not recovered from any extraction (no source file defines
 * it either).
 */
export type FileExplorerContext =
  | { type: "main" }
  | { type: "task"; taskId: string }
  | { type: "branch"; branch: string };

/**
 * One entry in the changes/diff panel's file list. Same "no Go command
 * returns this yet" situation as {@link FileExplorerContext} —
 * reconstructed from `presentation/helpers/changes.helper.ts`'s
 * `formatChangeStatusLabel`/`changeStatusClassName` switches (the
 * `status` union) and `changes-content.tsx`'s `file.path` read.
 */
export interface FileChangeEntry {
  path: string;
  status: "added" | "modified" | "deleted" | "renamed" | "untracked";
  /** Previous path for a renamed entry. */
  oldPath?: string;
  isBinary?: boolean;
  additions?: number;
  deletions?: number;
}

/**
 * The file explorer/changes panel's combined server snapshot — flat path
 * list plus an index for O(1) directory/file lookups, the task list used
 * to label a `{ type: "task" }` context, and the changed-files list the
 * Changes panel renders. Same reconstructed-from-usage situation as
 * {@link FileExplorerContext}; see `files-explorer-group.tsx`
 * (`.paths`, `.pathIndex`), `files-explorer.helper.ts` (`.tasks`), and
 * `changes-content.tsx` (`.files`).
 */
export interface FileExplorerSnapshot {
  paths: string[];
  pathIndex: Record<string, { type: "file" | "directory"; [key: string]: unknown }>;
  tasks?: Array<{ id: string; title: string }>;
  files?: FileChangeEntry[];
  /** True for a read-only context (e.g. a non-checked-out branch). */
  readOnly?: boolean;
  /**
   * Per-path git status handed straight to `@pierre/trees`'
   * `model.setGitStatus(...)` (`files-explorer-group.tsx`). Review round 2:
   * tightened from `unknown[]` to `@pierre/trees`' own real `GitStatusEntry`
   * shape (`{ path: string; status: GitStatus }`,
   * `node_modules/@pierre/trees/dist/publicTypes.d.ts`) now that the
   * package is actually installed and its real contract is visible — the
   * `unknown[]` stub predates that and was masking a real type mismatch at
   * the `setGitStatus` call site.
   */
  gitStatus?: FileGitStatusEntry[];
  /** Branch names offered by the explorer's context switcher. */
  branches?: string[];
}

/**
 * Mirrors `@pierre/trees`' own `GitStatusEntry`/`GitStatus` — kept as a
 * local, domain-layer copy rather than importing from the UI tree library
 * inside an `interfaces/` file (this file has no other dependency on
 * `@pierre/trees`, and interfaces files elsewhere in this port don't
 * depend on vendor UI packages either).
 */
export type FileGitStatus =
  | "added"
  | "deleted"
  | "ignored"
  | "modified"
  | "renamed"
  | "untracked";

export interface FileGitStatusEntry {
  path: string;
  status: FileGitStatus;
}
