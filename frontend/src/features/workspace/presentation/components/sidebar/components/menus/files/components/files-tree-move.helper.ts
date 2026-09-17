import { toast } from "sonner";
import { prepareFileTreeInput } from "@pierre/trees";
import { api } from "@/lib/aos-facade";
import { aos } from "@/app/aos";
import type {
  FileExplorerContext,
  FileExplorerSnapshot,
} from "@/features/file/interfaces/file.interfaces";
import { basenameOf } from "@/features/file/presentation/helpers/files-explorer.helper";
import type { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";
import { t } from "@/lib/i18n";

function tabFilePath(tab: ViewportTabState) {
  return typeof tab.metadata?.filePath === "string" ? tab.metadata.filePath : undefined;
}

/**
 * The paths as `@pierre/trees` takes them: a directory ends in `/`, which is
 * the only way the tree tells a folder from a file. The daemon names
 * directories without one.
 */
export function treePathsOf(snapshot: FileExplorerSnapshot): string[] {
  return snapshot.paths.map((entryPath) => {
    const indexed =
      snapshot.pathIndex[entryPath] ??
      snapshot.pathIndex[`${entryPath}/`] ??
      snapshot.pathIndex[entryPath.replace(/\/+$/, "")];
    if (indexed?.type === "directory" && !entryPath.endsWith("/")) {
      return `${entryPath}/`;
    }
    return entryPath;
  });
}

/** The part of the tree model a move is undone through. */
export interface TreeMoveModel {
  getItem(path: string): unknown;
  move(fromPath: string, toPath: string): unknown;
  resetPaths(options: { preparedInput: ReturnType<typeof prepareFileTreeInput> }): unknown;
}

/**
 * Puts a rename or drag the daemon refused back where it was.
 *
 * The tree applies the move to itself before asking, and a refetch could not
 * undo it: the answer is deep-equal to the last one, so React Query hands back
 * the same array and the effect that resets the tree never runs. A tree that
 * cannot be moved back is redrawn from the last snapshot instead.
 */
export function undoTreeMove(
  model: TreeMoveModel,
  snapshot: FileExplorerSnapshot | null | undefined,
  fromPath: string,
  toPath: string,
) {
  try {
    if (model.getItem(toPath) && !model.getItem(fromPath)) {
      model.move(toPath, fromPath);
    }
  } catch {
    if (snapshot?.paths?.length) {
      model.resetPaths({ preparedInput: prepareFileTreeInput(treePathsOf(snapshot)) });
    }
  }
}

/**
 * Asks the daemon to move `fromPath` to `toPath` after the tree already has.
 * A refusal is said, the tree put back and reread; a move that landed takes
 * the open tabs along and rereads the tree, whose index is keyed by path.
 */
export async function handleTreeMove(
  fromPath: string,
  toPath: string,
  explorerContext: FileExplorerContext,
  after: { revert: () => void; refresh: () => void },
) {
  if (!fromPath || !toPath || fromPath === toPath) return;

  const response = await api.file.move.mutate({
    body: {
      fromPath,
      toPath,
      context: explorerContext,
    },
  });

  if (response.error) {
    toast.error(
      (response.error as { message?: string })?.message ||
        t("Unable to move \"{{path}}\".", { path: fromPath }),
    );
    after.revert();
    after.refresh();
    return;
  }

  retargetOpenTabs(fromPath, toPath);
  toast.success(t("Moved."));
  // The index is keyed by path; until it is read again, the moved file is not
  // in it and a click on its new name would open nothing.
  after.refresh();
}

/**
 * Points the tabs open on a moved file — or on anything under a moved folder —
 * at where it went, so the next save lands on the file rather than recreating
 * it at its old name.
 */
export function retargetOpenTabs(fromPath: string, toPath: string) {
  const from = fromPath.replace(/\/+$/, "");
  const to = toPath.replace(/\/+$/, "");
  for (const tab of aos.stores.viewport.state.tabs.items) {
    const current = tabFilePath(tab);
    if (tab.type !== "file" || !current) continue;
    if (current !== from && !current.startsWith(`${from}/`)) continue;

    const next = `${to}${current.slice(from.length)}`;
    aos.stores.viewport.actions.updateTab(tab.id, {
      title: basenameOf(next),
      metadata: { ...tab.metadata, filePath: next, fileName: basenameOf(next) },
    });
  }
}
