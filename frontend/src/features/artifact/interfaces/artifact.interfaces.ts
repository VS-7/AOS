/**
 * There is no AOS Go backend for this domain yet, but a real, checkable
 * declaration exists in the old AOS server:
 * `v401/server/src/features/artifact/schemas/artifact.schema.ts`
 * (`ArtifactSchema`, the entity) and
 * `.../services/artifact/artifact.service.ts` (the exact construction site
 * for `ArtifactListItem` and `ArtifactUrls` — the type-only
 * `interfaces/artifact.interfaces.ts` that would normally declare these
 * was itself erased by the bundler, same as everywhere else, so the
 * schema + the literal object construction are the best available ground
 * truth). Recovered, not guessed. When AOS grows a Go backend for this,
 * re-verify against that instead.
 *
 * `ArtifactListItem` is `{ ...artifact, urls }`
 * (`artifact.service.ts`'s `list()`, `sorted.map(async (artifact) => ({
 * ...artifact, urls: await this._get_artifact_urls(artifact.id) }))`) —
 * every field of the full entity, not just the four the frontend's ported
 * UI (`artifact.store.ts`, `use-artifacts.ts`, `artifact.helper.ts`,
 * `artifact.trigger.ts`) happens to read. `ArtifactUrls` comes from
 * `_get_artifact_urls`: `{ local, tunnel }` — the frontend only ever reads
 * `.local`, but `.tunnel` is a real field on the wire (nullable, not
 * optional — always computed, just `null` when no tunnel is configured).
 */
export type ArtifactVisibility = "private" | "workspace" | "by_password";

export interface Artifact {
  id: string;
  name: string;
  icon?: string;
  skill?: string;
  description?: string;
  entrypoint: string;
  serve?: string;
  visibility: ArtifactVisibility;
  /** Hashed password when `visibility` is `"by_password"`. Present on the wire as-is — the server does not strip it for list/get responses. */
  secret?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ArtifactUrls {
  local: string;
  tunnel: string | null;
}

export type ArtifactListItem = Artifact & {
  urls: ArtifactUrls;
};
