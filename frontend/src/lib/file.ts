import { DomainError, unwrap } from "./client";
import { getWorkspace, isDesktop } from "./client";

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
 * command group ([[File (Go)]]), so there is no DomainService.Invoke path
 * for it to ride, and no desktop binding has been built for it yet (see
 * SystemService for what that binding looks like once it exists). Inside
 * the desktop window this fetch reaches Wails' own asset host instead of the
 * daemon, the same way the command surface's fetch did before it grew a
 * Call.ByName transport — isNotYetAvailableInDesktop lets a caller show that
 * plainly instead of a confusing network failure.
 */
export const isNotYetAvailableInDesktop = isDesktop;

function headers(extra?: Record<string, string>): Record<string, string> {
  const ws = getWorkspace();
  return { ...(ws ? { "x-workspace-id": ws } : {}), ...extra };
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, init);
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
