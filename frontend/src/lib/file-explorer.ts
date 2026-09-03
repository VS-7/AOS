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
import { tree, changes as rawChanges, type FileNode } from "./file";
import { client } from "./client";
import type {
  FileChangeEntry,
  FileChangesSummary,
  FileExplorerSnapshot,
  WorkspaceFile,
} from "@/features/file/interfaces/file.interfaces";

/** Extensions the in-app text editor opens. Everything else gets a viewer. */
const EDITABLE = new Set([
  "ts", "tsx", "js", "jsx", "mjs", "cjs", "go", "py", "rb", "rs", "java", "kt",
  "swift", "c", "h", "cc", "cpp", "hpp", "cs", "php", "sh", "bash", "zsh",
  "fish", "sql", "html", "css", "scss", "less", "json", "jsonc", "yaml", "yml",
  "toml", "ini", "cfg", "conf", "env", "md", "mdx", "txt", "csv", "tsv", "xml",
  "svg", "graphql", "gql", "proto", "dockerfile", "makefile", "gitignore",
]);

const IMAGE = new Set(["png", "jpg", "jpeg", "gif", "webp", "avif", "bmp", "ico"]);
const VIDEO = new Set(["mp4", "webm", "mov", "m4v"]);
const AUDIO = new Set(["mp3", "wav", "ogg", "m4a", "flac"]);

function viewerFor(extension: string, dir: boolean): string {
  if (dir) return "none";
  if (IMAGE.has(extension)) return "image";
  if (VIDEO.has(extension)) return "video";
  if (AUDIO.has(extension)) return "audio";
  if (extension === "pdf") return "pdf";
  if (EDITABLE.has(extension)) return "text";
  return "binary";
}

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
    viewer: viewerFor(extension, node.dir),
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
  const [walked, changed, tasks] = await Promise.all([
    tree("", true).catch(() => ({ path: "", nodes: [] as FileNode[] })),
    changes().catch(() => ({
      snapshot: {
        paths: [],
        pathIndex: {},
        files: [],
        summary: summarize([]),
      },
    })),
    options.includeContexts === false ? Promise.resolve([]) : listTasks(),
  ]);

  const nodes = walked.nodes ?? [];
  const paths = nodes.map((node) => node.path);
  const pathIndex: FileExplorerSnapshot["pathIndex"] = {};
  for (const node of nodes) {
    pathIndex[node.path] = {
      type: node.dir ? "directory" : "file",
      name: node.name,
      size: node.size,
      editable: node.editable,
    };
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
 * The tasks the explorer offers as contexts to switch to.
 *
 * A failure here is not a failure of the file tree: the panel opens on the
 * live workspace, and the list of worktrees to switch to is the part that is
 * missing.
 */
async function listTasks(): Promise<Array<{ id: string; title: string }>> {
  try {
    const answered = (await client.invoke("tasks_list", {
      _reasoning: "listing the task worktrees the file explorer can switch to",
      limit: 100,
    })) as { tasks?: Array<{ id?: string; title?: string }> } | undefined;

    return (answered?.tasks ?? [])
      .filter((task) => task.id)
      .map((task) => ({ id: String(task.id), title: String(task.title ?? task.id) }));
  } catch {
    return [];
  }
}
