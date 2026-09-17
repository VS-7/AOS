import { toast } from "sonner";
import { api } from "@/lib/aos-facade";
import { t } from "@/lib/i18n";
import type { FileExplorerContext } from "@/features/file/interfaces/file.interfaces";
import {
  basenameOf,
  copyDestinationPath,
  joinWorkspacePath,
} from "@/features/file/presentation/helpers/files-explorer.helper";
import type { FilesClipboardState } from "@/features/file/presentation/stores/files.store";

function refusal(error: unknown, fallback: string): Error {
  const message = (error as { message?: string } | undefined)?.message;
  return new Error(message || fallback);
}

/**
 * Pastes clipboard paths into a target directory using move (cut) or copy
 * semantics.
 *
 * A copy is one request, made by the daemon, whether the source is a file or
 * a whole folder. It used to be done here: read the file through the JSON
 * API, then write back a `content` field the answer never had — so every copy
 * came out empty, and a copy pasted beside its original was written over the
 * original. Reading and writing back could not have worked for a picture
 * either, which the API carries as Base64.
 *
 * The copy never replaces anything: it takes the first free "name copy" name
 * the explorer knows of, and the daemon refuses a destination that is taken
 * anyway, for the one that appeared since the tree was last read.
 */
export async function pasteFilesClipboard(params: {
  clipboard: FilesClipboardState;
  targetParentPath: string;
  explorerContext: FileExplorerContext;
  pathIndex?: Record<string, { type: "file" | "directory" }>;
}): Promise<void> {
  const { clipboard, targetParentPath, explorerContext, pathIndex } = params;
  const taken = new Set(Object.keys(pathIndex ?? {}));

  for (const sourcePath of clipboard.paths) {
    const source = sourcePath.replace(/\/+$/, "");
    const destination = joinWorkspacePath(targetParentPath, basenameOf(source));

    if (clipboard.mode === "cut") {
      // Pasting a cut item back where it is has nothing to move, and the
      // daemon would refuse it as a destination that already exists.
      if (destination === source) continue;

      const moveResponse = await api.file.move.mutate({
        body: { fromPath: source, toPath: destination, context: explorerContext },
      });
      if (moveResponse.error) {
        throw refusal(moveResponse.error, t("Unable to move \"{{path}}\".", { path: source }));
      }
      continue;
    }

    const target = copyDestinationPath(destination, taken);
    const copyResponse = await api.file.copy.mutate({
      body: { fromPath: source, toPath: target, context: explorerContext },
    });
    if (copyResponse.error) {
      throw refusal(copyResponse.error, t("Unable to copy \"{{path}}\".", { path: source }));
    }
    // A second item with the same name in one paste must not pick the name
    // the first one just took.
    taken.add(target);
  }

  toast.success(
    clipboard.mode === "cut" ? t("Moved to destination.") : t("Pasted to destination."),
  );
}
