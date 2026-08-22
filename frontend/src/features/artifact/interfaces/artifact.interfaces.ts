/**
 * Re-verified against the real Go backend — `internal/domain/artifact`
 * (`entity.go`, `service.go`) — now that it exists; this used to be
 * recovered from the original product's TS server instead (see git history
 * for that version's own doc comment), per its own instruction to do
 * exactly this once a Go backend grew.
 *
 * Three fields the original had no longer apply: `icon` and `serve` were
 * never added to this rebuild's entity (`ArtifactHelper.getIcon` already
 * treats a missing icon as the normal case, falling back to a default), and
 * the password is never sent to a client at all — `PasswordHash` is
 * `json:"-"` on the Go side (`entity.go`), by design; there is no `secret`
 * field on the wire to read.
 *
 * `urls` is real and always present, computed by the service on every read
 * (`Service.urlsFor`, `service.go`) rather than persisted — `.local` is
 * `internal/transport/artifactapi`'s route for this artifact
 * (`/v/artifacts/{id}/`), which `ArtifactHelper.openInBrowserTab` opens
 * directly. `.tunnel` is on the wire, matching the original's own shape,
 * but this build does not yet compute one — always `null` today (see
 * `urlsFor`'s own comment on why).
 */
export type ArtifactVisibility = "private" | "workspace" | "by_password";

export interface Artifact {
  id: string;
  name: string;
  skill?: string;
  description?: string;
  entrypoint: string;
  visibility: ArtifactVisibility;
  createdAt: string;
  updatedAt: string;
  urls: ArtifactUrls;
}

export interface ArtifactUrls {
  local: string;
  tunnel: string | null;
}

/**
 * Historically `Artifact & { urls }` — the Go entity carries `urls` itself
 * now (see the module doc above), so this is just an alias kept for every
 * existing call site that names it.
 */
export type ArtifactListItem = Artifact;
