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
 * const kind = FractalFileKindSchema.parse("file");
 * ```
 */
export const FractalFileKindSchema = z.enum(["file", "directory"]);

/**
 * Defines the supported in-app viewers for a workspace file.
 * @example
 * ```typescript
 * const viewer = FractalFileViewerSchema.parse("text");
 * ```
 */
export const FractalFileViewerSchema = z.enum([
  "text",
  "image",
  "pdf",
  "excalidraw",
  "video",
  "audio",
  "spreadsheet",
  "archive",
  "other",
]);

/**
 * Describes a file-system node exposed by the Files feature.
 * @example
 * ```typescript
 * const file = FractalFileSchema.parse({
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
export const FractalFileSchema = z.object({
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
  type: FractalFileKindSchema.describe("Whether the node is a file or directory."),
  updatedAt: z.string().describe("ISO timestamp derived from filesystem metadata."),
  viewer: FractalFileViewerSchema.describe("Suggested viewer strategy for the UI."),
});

/**
 * Represents a file-system node available inside the current workspace.
 * Inferred from {@link FractalFileSchema}.
 */
export type FractalFile = z.infer<typeof FractalFileSchema>;

/**
 * Defines the list query accepted by the Files controller.
 * @example
 * ```typescript
 * const query = FractalFileListQuerySchema.parse({ recursive: true, includeIgnored: false });
 * ```
 */
export const FractalFileListQuerySchema = Schema.object({
  path: z.string().optional().describe("Optional workspace-relative directory to list from."),
  recursive: queryBoolean(false).describe("Whether nested descendants should be traversed."),
  includeIgnored: queryBoolean(false).describe("Whether heavy and internal folders should be included."),
});

/**
 * Input parameters for listing workspace files.
 * Inferred from {@link FractalFileListQuerySchema}.
 */
export type FractalFileListQueryInput = z.infer<typeof FractalFileListQuerySchema>;

/**
 * Defines the search query accepted by the Files controller.
 * @example
 * ```typescript
 * const query = FractalFileSearchQuerySchema.parse({ query: "task", limit: 50 });
 * ```
 */
export const FractalFileSearchQuerySchema = Schema.object({
  path: z.string().optional().describe("Optional workspace-relative directory used as the search root."),
  query: z.string().min(1).describe("Fuzzy query applied to file and folder names."),
  includeIgnored: queryBoolean(false).describe("Whether heavy and internal folders should be included."),
  limit: z.coerce.number().int().positive().max(500).default(200).describe("Maximum number of matched nodes before ancestors are added."),
});

/**
 * Input parameters for searching workspace files.
 * Inferred from {@link FractalFileSearchQuerySchema}.
 */
export type FractalFileSearchQueryInput = z.infer<typeof FractalFileSearchQuerySchema>;

/**
 * Defines the read query accepted by the Files controller.
 * @example
 * ```typescript
 * const query = FractalFileReadSchema.parse({ path: "src/index.ts" });
 * ```
 */
export const FractalFileReadSchema = z.object({
  path: z.string().min(1).describe("Workspace-relative file path to read."),
});

/**
 * Input parameters for reading a file.
 * Inferred from {@link FractalFileReadSchema}.
 */
export type FractalFileReadInput = z.infer<typeof FractalFileReadSchema>;

/**
 * Defines the write payload accepted by the Files controller.
 * @example
 * ```typescript
 * const payload = FractalFileWriteSchema.parse({ path: "README.md", content: "# Updated" });
 * ```
 */
export const FractalFileWriteSchema = z.object({
  content: z.string().describe("UTF-8 text content to persist on disk."),
  path: z.string().min(1).describe("Workspace-relative file path to write."),
});

/**
 * Input parameters for writing a file.
 * Inferred from {@link FractalFileWriteSchema}.
 */
export type FractalFileWriteInput = z.infer<typeof FractalFileWriteSchema>;

/**
 * Defines the create payload accepted by the Files controller.
 * @example
 * ```typescript
 * const payload = FractalFileCreateSchema.parse({
 *   path: "src/features/file/index.ts",
 *   type: "file",
 *   content: "export {}",
 * });
 * ```
 */
export const FractalFileCreateSchema = z.object({
  content: z.string().optional().describe("Optional initial UTF-8 content for new text files."),
  overwrite: z.boolean().default(false).describe("Whether an existing file may be replaced."),
  path: z.string().min(1).describe("Workspace-relative target path for the new node."),
  type: FractalFileKindSchema.describe("Whether to create a file or a directory."),
});

/**
 * Input parameters for creating a new file-system node.
 * Inferred from {@link FractalFileCreateSchema}.
 */
export type FractalFileCreateInput = z.infer<typeof FractalFileCreateSchema>;

/**
 * Contract for the workspace-scoped Files service.
 */
export interface IFileService {
  /**
   * Lists files and directories from the current workspace.
   *
   * @param params - Listing options validated by {@link FractalFileListQuerySchema}.
   * @returns A response containing flat file-system nodes for tree building in the UI.
   *
   * @example
   * ```typescript
   * const result = await fileService.list({ path: "src", recursive: true });
   * ```
  */
  list(params?: FractalFileListQueryInput): Promise<ResponseWithCTA<{ files: FractalFile[] }>>;

  /**
   * Searches files and folders from the current workspace.
   *
   * @param params - Search options validated by {@link FractalFileSearchQuerySchema}.
   * @returns Matching file-system nodes plus ancestor folders for tree rendering in the UI.
   *
   * @example
   * ```typescript
   * const result = await fileService.search({ query: "task", limit: 50 });
   * ```
   */
  search(params: FractalFileSearchQueryInput): Promise<ResponseWithCTA<{ files: FractalFile[] }>>;

  /**
   * Reads UTF-8 content from a workspace file.
   *
   * @param params - Read options validated by {@link FractalFileReadSchema}.
   * @returns The requested file metadata plus its current content.
   *
   * @example
   * ```typescript
   * const result = await fileService.read({ path: "README.md" });
   * ```
   */
  read(params: FractalFileReadInput): Promise<ResponseWithCTA<{ file: FractalFile; content: string }>>;

  /**
   * Resolves a workspace file for HTTP content serving.
   *
   * @param params - Read options validated by {@link FractalFileReadSchema}.
   * @returns File metadata plus the absolute path used by the transport layer.
   */
  content(params: FractalFileReadInput): Promise<ResponseWithCTA<{ file: FractalFile; absolutePath: string }>>;

  /**
   * Persists UTF-8 content into an existing workspace file.
   *
   * @param params - Write options validated by {@link FractalFileWriteSchema}.
   * @returns The refreshed file metadata plus the saved content.
   *
   * @example
   * ```typescript
   * const result = await fileService.write({ path: "README.md", content: "# Hello" });
   * ```
   */
  write(params: FractalFileWriteInput): Promise<ResponseWithCTA<{ file: FractalFile; content: string }>>;

  /**
   * Creates a new file or directory inside the workspace.
   *
   * @param params - Creation options validated by {@link FractalFileCreateSchema}.
   * @returns The created file-system node.
   *
   * @example
   * ```typescript
   * const result = await fileService.create({ path: "src/features/file", type: "directory" });
   * ```
   */
  create(params: FractalFileCreateInput): Promise<ResponseWithCTA<{ file: FractalFile }>>;
}
