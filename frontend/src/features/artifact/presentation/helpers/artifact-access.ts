import * as React from "react";
import type { ArtifactListItem } from "@/features/artifact/interfaces/artifact.interfaces";

/**
 * A request for an artifact's password: to open it (`open`), or to set or
 * change it from its row menu (`set`).
 *
 * Kept outside React so `ArtifactHelper.openInBrowserTab` — called from the
 * sidebar, the palette, Home and the marketplace alike — can ask for it, and
 * the one `ArtifactAccessDialog` the workspace layout mounts answers.
 */
export interface ArtifactAccessRequest {
  artifact: ArtifactListItem;
  mode: "open" | "set";
}

let current: ArtifactAccessRequest | null = null;
const listeners = new Set<() => void>();

function publish(next: ArtifactAccessRequest | null) {
  current = next;
  for (const listener of listeners) listener();
}

export function requestArtifactAccess(artifact: ArtifactListItem, mode: ArtifactAccessRequest["mode"] = "open") {
  publish({ artifact, mode });
}

export function closeArtifactAccess() {
  publish(null);
}

export function currentArtifactAccess(): ArtifactAccessRequest | null {
  return current;
}

export function useArtifactAccessRequest(): ArtifactAccessRequest | null {
  return React.useSyncExternalStore(
    (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    currentArtifactAccess,
    currentArtifactAccess,
  );
}

/**
 * An artifact's address with its password.
 *
 * The daemon reads a by_password artifact's password from the query string,
 * and only there (internal/transport/artifactapi): the password is the link's
 * own shareable secret, unlike the session, which never goes in a URL.
 */
export function addressWithPassword(url: string, password: string | undefined): string {
  if (!password) return url;
  return `${url}${url.includes("?") ? "&" : "?"}password=${encodeURIComponent(password)}`;
}

/**
 * Whether the daemon accepts `password` for the artifact at `url`: `true`,
 * `false` for a refusal, or the daemon's reason when it answered anything else.
 *
 * Asked before a tab opens, so a wrong password is said where it was typed
 * rather than as the daemon's JSON inside the tab.
 */
export async function checkArtifactPassword(url: string, password: string): Promise<true | false | string> {
  const response = await fetch(addressWithPassword(url, password), {
    method: "GET",
    headers: { Accept: "text/html" },
  });
  if (response.ok) return true;
  let code = "";
  let message = "";
  try {
    const body = (await response.json()) as { error?: { code?: string; message?: string } };
    code = body.error?.code ?? "";
    message = body.error?.message ?? "";
  } catch {
    // Not the daemon's error envelope; the status says enough.
  }
  if (response.status === 403 || code === "AOS_ARTIFACT_UNAUTHORIZED") return false;
  return message || `${response.status} ${response.statusText}`.trim();
}
