/**
 * The three file screens, assembled from what the daemon publishes.
 *
 * `file.explorer`, `file.changes` and `file.search` were all `null` in
 * `command-map.ts`, and the note there said the difference was "one of shape,
 * not capability — a candidate for an adapter". This is that adapter.
 *
 * What each one was doing before: the sidebar's file tree rendered no files,
 * the Changes panel rendered no changes, and the composer's `@`-mention picker
 * offered no files to attach. None of the three showed an error, because a
 * dormant call resolves to null rather than failing — so all three looked like
 * empty workspaces.
 *
 * `file.list` was mapped, and just as broken: it answered `fileApi.tree`'s own
 * `{path, nodes}` while all three of its call sites read `.files` off a
 * `WorkspaceFile[]`. It goes through the same adapter now.
 */
import { tree, changes as rawChanges, read as rawRead, diff as rawDiff, type FileNode } from "./file";
import { client } from "./client";
import { resolveFileViewer } from "@/features/file/presentation/helpers/file-viewer.helper";
import type {
  FileChangeEntry,
  FileChangesSummary,
  FileExplorerSnapshot,
  WorkspaceFile,
} from "@/features/file/interfaces/file.interfaces";

/**
 * One node, in the shape the ported screens read.
 *
 * The daemon answers `internal/domain/file.Node` — path, name, dir, size,
 * extension, media type, editable, modifiedAt. The interface was written
 * against a richer record, and the fields it has no source for are filled from
 * what is there rather than invented: `createdAt` repeats `modifiedAt` (a
 * filesystem birth time is not something the tree walk reads), and
 * `browserUrl` is empty because there is no static route serving workspace
 * files — the editor reads through `/api/file/read`, not a URL.
 */
export function toWorkspaceFile(node: FileNode): WorkspaceFile {
  const extension = (node.extension ?? "").replace(/^\./, "").toLowerCase();
  const parent = node.path.includes("/")
    ? node.path.slice(0, node.path.lastIndexOf("/"))
    : "";

  // The viewer comes from the same resolver the panel uses for a path it has
  // no node for. A second table here answered "text" for Markdown and JSON
  // and "binary" — not a viewer at all — for anything unlisted, so the same
  // file opened differently depending on which door it came in by.
  return {
    absolutePath: node.path,
    browserUrl: "",
    createdAt: node.modifiedAt,
    extension,
    isEditable: node.editable,
    mimeType: node.mediaType,
    name: node.name,
    path: node.path,
    parentPath: parent || undefined,
    size: node.size,
    type: node.dir ? "directory" : "file",
    updatedAt: node.modifiedAt,
    viewer: node.dir ? "other" : resolveFileViewer(node.name),
  } as WorkspaceFile;
}

/** `file.list`: the nodes under one directory, as the screens read them. */
export async function list(
  path: string,
  recursive: boolean,
): Promise<{ files: WorkspaceFile[] }> {
  const answered = await tree(path, recursive);
  return { files: (answered.nodes ?? []).map(toWorkspaceFile) };
}

/**
 * `file.search`: paths matching a query.
 *
 * The match is done here rather than by the daemon because the daemon has no
 * search: it has a recursive tree walk, and a workspace's tree is small enough
 * that filtering it in the window is both simpler and instant. A workspace
 * where that stops being true is one that needs a real index, not a bigger
 * loop — and the limit below is what keeps this honest in the meantime.
 */
export async function search(
  query: string,
  limit = 24,
): Promise<{ files: WorkspaceFile[] }> {
  const needle = query.trim().toLowerCase();
  if (!needle) return { files: [] };

  const answered = await tree("", true);
  const matched = (answered.nodes ?? [])
    .filter((node) => !node.dir && node.path.toLowerCase().includes(needle))
    // A match on the file's own name beats one buried in a directory further
    // up the path, which is what somebody typing a filename means.
    .sort((a, b) => {
      const an = a.name.toLowerCase().includes(needle) ? 0 : 1;
      const bn = b.name.toLowerCase().includes(needle) ? 0 : 1;
      return an - bn || a.path.length - b.path.length;
    })
    .slice(0, limit);

  return { files: matched.map(toWorkspaceFile) };
}

/**
 * `file.explorer`: the tree the sidebar draws, with the changed paths marked.
 *
 * The panel wants one snapshot rather than three requests it has to correlate:
 * every path, an index to look one up by, the tasks whose worktrees it can
 * switch to, and what git says has changed.
 */
export async function explorer(
  options: { includeContexts?: boolean } = {},
): Promise<{ snapshot: FileExplorerSnapshot }> {
  // Contexts are opt-in. The switcher that shows them is hidden until the
  // daemon's file API can resolve a task's worktree, and until then the list
  // is a request whose answer nothing renders.
  const [walked, changed, tasks] = await Promise.all([
    // Not caught. A tree that cannot be read is a failure the panel has to
    // show ("Unable to load files", with the reason), not an empty workspace:
    // swallowing it is what made a daemon outage read as "No files
    // available", and left that empty answer cached after the daemon came
    // back, since a query that succeeded has nothing to retry.
    tree("", true),
    // The change list is optional — a directory that is not a git repository
    // has none — so its failure still degrades to an empty one.
    changes().catch(() => ({
      snapshot: {
        paths: [],
        pathIndex: {},
        files: [],
        summary: summarize([]),
      },
    })),
    options.includeContexts === true ? listTasks() : Promise.resolve([]),
  ]);

  const nodes = walked.nodes ?? [];
  const paths = nodes.map((node) => node.path);
  // Each entry is the whole file record, not a summary of one. The tree's
  // click handler opens whatever it finds here, and an entry with no path is
  // a tab that says "No file selected" — which is what every click opened
  // while this held only {type, name, size, editable}.
  const pathIndex: FileExplorerSnapshot["pathIndex"] = {};
  for (const node of nodes) {
    pathIndex[node.path] = toWorkspaceFile(node);
  }

  const files = changed.snapshot.files ?? [];
  return {
    snapshot: {
      paths,
      pathIndex,
      tasks,
      files,
      summary: changed.snapshot.summary,
      readOnly: false,
      // The shape @pierre/trees' setGitStatus takes, straight through.
      gitStatus: files.map((file) => ({ path: file.path, status: file.status })),
    },
  };
}

/**
 * The aggregate the Changes header draws, folded out of the file list.
 *
 * The original server answers this beside the list rather than making the
 * panel count for itself, and the panel reads it straight through — so it is
 * computed once, here, and every snapshot this module builds carries one.
 * Omitting it is what made the header throw on a resolved snapshot.
 */
function summarize(files: FileChangeEntry[]): FileChangesSummary {
  return files.reduce<FileChangesSummary>(
    (summary, file) => ({
      fileCount: summary.fileCount + 1,
      additions: summary.additions + (file.additions ?? 0),
      deletions: summary.deletions + (file.deletions ?? 0),
    }),
    { fileCount: 0, additions: 0, deletions: 0 },
  );
}

/** `file.changes`: what the working tree differs from HEAD at. */
export async function changes(): Promise<{ snapshot: FileExplorerSnapshot }> {
  const answered = await rawChanges();
  const files = (answered.files ?? []) as FileChangeEntry[];
  return {
    snapshot: {
      paths: files.map((file) => file.path),
      pathIndex: {},
      files,
      summary: summarize(files),
    },
  };
}

/**
 * The tasks the explorer offers as contexts to switch to: the ones that have a
 * worktree, labelled by name.
 *
 * Go's task has `name` and no `title`, so the picker used to fall back to the
 * id and show UUIDs; and a task without a worktree has no tree of its own to
 * switch to.
 *
 * A failure here is not a failure of the file tree: the panel opens on the
 * live workspace, and the list of worktrees to switch to is the part that is
 * missing.
 */
export async function listTasks(): Promise<Array<{ id: string; title: string }>> {
  try {
    const answered = (await client.invoke("tasks_list", {
      _reasoning: "listing the task worktrees the file explorer can switch to",
      limit: 100,
    })) as
      | { tasks?: Array<{ id?: string; name?: string; worktree?: { enabled?: boolean } }> }
      | undefined;

    return (answered?.tasks ?? [])
      .filter((task) => task.id && task.worktree?.enabled === true)
      .map((task) => ({ id: String(task.id), title: String(task.name || task.id) }));
  } catch {
    return [];
  }
}

/** What the editor opens a file with. */
export interface EditorRead {
  /** The text to edit; empty for a binary, which has no text to show. */
  content: string;
  file: WorkspaceFile;
  truncated: boolean;
  /**
   * Whether saving this buffer back is safe. Not for a truncated read — the
   * save would cut the file at the read limit — and not for a binary, whose
   * bytes are not in `content` at all.
   */
  editable: boolean;
}

/**
 * `file.read`, in the shape the editor reads.
 *
 * The daemon answers `{path, mediaType, text | base64, size, truncated}`, and
 * the editor read `content` and `file` — so every file opened as an empty
 * buffer, and the first keystroke enabled a Save that wrote that emptiness
 * over the real file.
 */
export async function readForEditor(path: string): Promise<EditorRead> {
  const answered = await rawRead(path);
  const name = path.split("/").pop() || path;
  const extension = name.includes(".") ? (name.split(".").pop() ?? "").toLowerCase() : "";
  const binary = answered.base64 !== undefined && answered.text === undefined;
  const file = toWorkspaceFile({
    path: answered.path || path,
    name,
    dir: false,
    size: answered.size,
    extension,
    mediaType: answered.mediaType,
    editable: !binary,
    modifiedAt: "",
  });
  return {
    content: binary ? "" : (answered.text ?? ""),
    file,
    truncated: answered.truncated,
    editable: !binary && !answered.truncated,
  };
}

/** One side of a diff, as `@pierre/diffs` takes it. */
export interface DiffSide {
  name: string;
  contents: string;
}

/**
 * `file.diff`, in the shape the Changes panel reads: `snapshot.oldFile` and
 * `snapshot.newFile`, each present only when that side exists.
 *
 * The daemon answers `{status, isBinary, oldText, newText}` beside each other,
 * and the panel read a `snapshot` that never arrived — so every diff, however
 * plain the change, said the text diff "could not be rendered".
 */
export async function diffForPanel(path: string): Promise<{
  snapshot: { status: string; isBinary: boolean; oldFile?: DiffSide; newFile?: DiffSide };
}> {
  const answered = await rawDiff(path);
  const name = path.split("/").pop() || path;
  return {
    snapshot: {
      status: answered.status,
      isBinary: answered.isBinary,
      ...(answered.oldText !== undefined && answered.oldText !== null
        ? { oldFile: { name, contents: answered.oldText } }
        : {}),
      ...(answered.newText !== undefined && answered.newText !== null
        ? { newFile: { name, contents: answered.newText } }
        : {}),
    },
  };
}
