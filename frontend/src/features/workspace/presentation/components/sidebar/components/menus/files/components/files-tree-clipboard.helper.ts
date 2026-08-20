import { toast } from "sonner";
import { api } from "@/lib/aos-facade";
import type { FileExplorerContext } from "@/features/file/interfaces/file.interfaces";
import {
  basenameOf,
  joinWorkspacePath,
  lookupPathIndex,
} from "@/features/file/presentation/helpers/files-explorer.helper";
import type { FilesClipboardState } from "@/features/file/presentation/stores/files.store";

async function copyFileNode(
  sourcePath: string,
  targetPath: string,
  context: FileExplorerContext,
): Promise<void> {
  const readResponse = await api.file.read.query({
    query: { path: sourcePath.replace(/\/+$/, ""), context },
  });

  if (readResponse.error || !readResponse.data) {
    throw new Error(
      (readResponse.error as { message?: string })?.message ||
        `Unable to read "${sourcePath}".`,
    );
  }

  const createResponse = await api.file.create.mutate({
    body: {
      path: targetPath,
      type: "file",
      content: readResponse.data.content,
      context,
    },
  });

  if (createResponse.error) {
    throw new Error(
      (createResponse.error as { message?: string })?.message ||
        `Unable to create "${targetPath}".`,
    );
  }
}

async function copyDirectoryRecursive(
  sourcePath: string,
  targetPath: string,
  context: FileExplorerContext,
): Promise<void> {
  const createDirResponse = await api.file.create.mutate({
    body: {
      path: targetPath.replace(/\/+$/, ""),
      type: "directory",
      context,
    },
  });

  if (createDirResponse.error) {
    throw new Error(
      (createDirResponse.error as { message?: string })?.message ||
        `Unable to create "${targetPath}".`,
    );
  }

  const listResponse = await api.file.list.query({
    query: {
      path: sourcePath.replace(/\/+$/, ""),
      recursive: false,
      includeIgnored: false,
      context,
    },
  });

  if (listResponse.error || !listResponse.data) {
    throw new Error(
      (listResponse.error as { message?: string })?.message ||
        `Unable to list "${sourcePath}".`,
    );
  }

  for (const entry of listResponse.data.files) {
    const childTarget = joinWorkspacePath(targetPath, entry.name);

    if (entry.type === "directory") {
      await copyDirectoryRecursive(entry.path, childTarget, context);
      continue;
    }

    await copyFileNode(entry.path, childTarget, context);
  }
}

/**
 * Pastes clipboard paths into a target directory using move (cut) or copy semantics.
 */
export async function pasteFilesClipboard(params: {
  clipboard: FilesClipboardState;
  targetParentPath: string;
  explorerContext: FileExplorerContext;
  pathIndex?: Record<string, { type: "file" | "directory" }>;
}): Promise<void> {
  const { clipboard, targetParentPath, explorerContext, pathIndex } = params;

  for (const sourcePath of clipboard.paths) {
    const name = basenameOf(sourcePath);
    const destinationPath = joinWorkspacePath(targetParentPath, name);

    if (clipboard.mode === "cut") {
      const moveResponse = await api.file.move.mutate({
        body: {
          fromPath: sourcePath,
          toPath: destinationPath,
          context: explorerContext,
        },
      });

      if (moveResponse.error) {
        throw new Error(
          (moveResponse.error as { message?: string })?.message ||
            `Unable to move "${sourcePath}".`,
        );
      }

      continue;
    }

    const indexed = lookupPathIndex(pathIndex, sourcePath);
    const isDirectory =
      indexed?.type === "directory" || /\/$/.test(sourcePath);

    if (isDirectory) {
      await copyDirectoryRecursive(sourcePath, destinationPath, explorerContext);
      continue;
    }

    await copyFileNode(sourcePath, destinationPath, explorerContext);
  }

  toast.success(
    clipboard.mode === "cut"
      ? "Moved to destination."
      : "Pasted to destination.",
  );
}
