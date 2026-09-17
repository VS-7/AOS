import { DAEMON_UNREACHABLE_CODE, DomainError, bridgeFetch, unwrap } from "./client";
import { getWorkspace, isDesktop } from "./client";
import { daemonURL } from "./daemon-origin";
import { isDesktopWindow } from "./wails";

/** Mirrors internal/domain/file.Node. */
export interface FileNode {
  path: string;
  name: string;
  dir: boolean;
  size: number;
  extension?: string;
  mediaType?: string;
  editable: boolean;
  modifiedAt: string;
}

/** Mirrors internal/domain/file.Tree. */
export interface FileTree {
  path: string;
  nodes: FileNode[];
}

/** Mirrors internal/domain/file.Content. */
export interface FileContent {
  path: string;
  mediaType: string;
  text?: string;
  base64?: string;
  size: number;
  truncated: boolean;
}

/** Mirrors internal/domain/file.Diff. */
export interface FileDiff {
  path: string;
  status: string;
  isBinary: boolean;
  oldText?: string;
  newText?: string;
}

/**
 * The file explorer's own transport.
 *
 * Unlike lib/client.ts's Client, this is HTTP only — the file domain has no
 * command group ([[File (Go)]]), so there is no DomainService.Invoke path for
 * it to ride, and no desktop binding has been built for it.
 *
 * That used to mean it did not work in the desktop window at all: a relative
 * `/api/file/...` there reaches Wails' own asset host, which has no such route
 * and answers with the interface's index.html — a 200 with HTML in it, so
 * nothing threw and the file tree, the editor and every diff simply stayed
 * empty. `daemonURL` addresses the daemon directly instead, which needs no
 * binding: the window knows the address, and CORS does not apply to a request
 * the application makes to its own local daemon.
 */

/**
 * Kept for the one thing it is still true of: whether this page is the desktop
 * window. It no longer means "the file API is unreachable here" — that is what
 * `daemonURL` fixed — and it has no callers; a screen that wants to know where
 * it is running should read `isDesktopWindow` from `lib/native.ts`, which
 * answers synchronously rather than after the first bridge call.
 */
export const isNotYetAvailableInDesktop = isDesktop;

function headers(extra?: Record<string, string>): Record<string, string> {
  const ws = getWorkspace();
  return { ...(ws ? { "x-workspace-id": ws } : {}), ...extra };
}

/**
 * One call to /api/file, over the transport this page has.
 *
 * The bridge inside the desktop window, and it is not an optimisation: a plain
 * fetch to the daemon from there is cross-origin, carries no cookie and no
 * bearer, and is refused — so the file tree, the editor and every diff were
 * empty there. The bridge attaches the credential the window already holds
 * (internal/transport/wailsvc.DomainService.Fetch). In a browser tab the
 * daemon serves the page, the request is same-origin and the session cookie
 * goes with it.
 *
 * Never one after the other. This used to fall back to a bare fetch whenever
 * the bridge rejected, which inside the window meant an anonymous request
 * after every Go-side failure: a daemon that was down came back as a refused
 * connection with no code, rather than as the AOS_DAEMON_UNREACHABLE the
 * bridge had already answered.
 */
async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const method = init?.method ?? "GET";
  const headers = (init?.headers ?? {}) as Record<string, string>;
  const body = typeof init?.body === "string" ? init.body : "";

  if (isDesktopWindow) {
    const answer = await bridgeFetch(method, path, headers["content-type"] ?? "", body);
    return unwrap(parseBody(answer.body, answer.status));
  }

  let response: Response;
  try {
    response = await fetch(daemonURL(path), init);
  } catch {
    // A refused connection rejects with a bare TypeError ("Load failed"),
    // which is what every file screen showed after the daemon stopped.
    throw new DomainError({
      code: DAEMON_UNREACHABLE_CODE,
      message: "the daemon is not answering",
      status: 503,
    });
  }
  let payload: unknown;
  try {
    payload = await response.json();
  } catch {
    throw new DomainError({
      code: "TRANSPORT_UNREADABLE",
      message: `the daemon answered ${response.status} with something that is not JSON`,
      status: response.status,
    });
  }
  return unwrap(payload);
}

/** Reads a bridged body, which arrives as the daemon's own JSON text. */
function parseBody(body: string, status: number): unknown {
  try {
    return JSON.parse(body);
  } catch {
    throw new DomainError({
      code: "TRANSPORT_UNREADABLE",
      message: `the daemon answered ${status} with something that is not JSON`,
      status,
    });
  }
}

/**
 * A URL for one file's own bytes, for an element that takes a `src`.
 *
 * Deliberately relative, and it works in both modes for two different
 * reasons. In a browser the daemon serves this page, so the request is
 * same-origin and the session cookie goes with it. In the desktop window it
 * reaches the application's own asset host, where a middleware forwards it to
 * the daemon with the token this process holds (`cmd/aos-desktop`'s
 * bridgeContent) — an `<img>` cannot carry a bearer, and the Wails bridge
 * answers a JSON string, which is not a picture.
 *
 * The viewers used to build `/api/files/content` by hand: plural, a path no
 * daemon has ever served, and in the window resolved against `wails://` where
 * nothing would have answered anyway. Every image, PDF and video in the Files
 * panel failed to load.
 */
export function contentURL(path: string): string {
  return `/api/file/content?path=${encodeURIComponent(path)}`;
}

export async function tree(path: string, recursive = false): Promise<FileTree> {
  const qs = new URLSearchParams({ path, recursive: String(recursive) });
  return request(`/api/file/tree?${qs}`, { headers: headers() });
}

export async function read(path: string): Promise<FileContent> {
  const qs = new URLSearchParams({ path });
  return request(`/api/file/read?${qs}`, { headers: headers() });
}

export async function write(path: string, content: string): Promise<{ path: string }> {
  return request(`/api/file/write`, {
    method: "PUT",
    headers: headers({ "content-type": "application/json" }),
    body: JSON.stringify({ path, content }),
  });
}

/**
 * A new file, refused (409) when anything is already at `path` — the
 * explorer's "New File". `write` overwrites on purpose, because it is the
 * editor's save; creating through it is how a paste used to empty a file.
 */
export async function create(path: string, content: string): Promise<{ path: string }> {
  return request(`/api/file/create`, {
    method: "PUT",
    headers: headers({ "content-type": "application/json" }),
    body: JSON.stringify({ path, content }),
  });
}

/** A new, empty directory, refused when `path` is taken. */
export async function mkdir(path: string): Promise<{ path: string }> {
  return request(`/api/file/mkdir`, {
    method: "PUT",
    headers: headers({ "content-type": "application/json" }),
    body: JSON.stringify({ path }),
  });
}

/**
 * A copy of a file or a whole directory, made by the daemon and refused when
 * `to` is taken. Copying here instead of reading and writing back keeps a
 * binary intact and a large file whole.
 */
export async function copy(from: string, to: string): Promise<{ path: string }> {
  return request(`/api/file/copy`, {
    method: "PUT",
    headers: headers({ "content-type": "application/json" }),
    body: JSON.stringify({ from, to }),
  });
}

export async function move(from: string, to: string): Promise<{ path: string }> {
  return request(`/api/file/move`, {
    method: "PUT",
    headers: headers({ "content-type": "application/json" }),
    body: JSON.stringify({ from, to }),
  });
}

export async function remove(path: string): Promise<{ path: string }> {
  const qs = new URLSearchParams({ path });
  return request(`/api/file/delete?${qs}`, { method: "DELETE", headers: headers() });
}

export async function diff(path: string): Promise<FileDiff> {
  const qs = new URLSearchParams({ path });
  return request(`/api/file/diff?${qs}`, { headers: headers() });
}

/** Mirrors internal/domain/file.Change. */
export interface FileChange {
  path: string;
  status: "added" | "modified" | "deleted" | "renamed" | "untracked";
  oldPath?: string;
}

/**
 * Every path the working tree differs from HEAD at.
 *
 * `diff` answers one path's two versions, for a file somebody has opened. This
 * is the list of them, which the Changes panel needs before anybody opens
 * anything — and which cannot be built out of `diff` without walking the whole
 * repository and shelling out once per file.
 */
export async function changes(): Promise<{ files: FileChange[]; total: number }> {
  return request(`/api/file/changes`, { headers: headers() });
}
