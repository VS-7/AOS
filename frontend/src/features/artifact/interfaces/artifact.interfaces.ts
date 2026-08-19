/**
 * Reconstructed from usage: the file was type-only and the bundler erased
 * it. There is no Go backend for this domain yet — when there is, this
 * contract becomes verifiable and should be checked against it.
 *
 * Usage sites (v401/web/src): `features/artifact/presentation/{stores/
 * artifact.store.ts,hooks/use-artifacts.ts,helpers/artifact.helper.ts,
 * triggers/artifact.trigger.ts}` read `id`, `name`, `icon`, and
 * `urls.local`. `artifact.helper.ts`'s `openInBrowserTab` is the only
 * consumer of `urls`, and only ever reads `.local` — no other key on it is
 * touched anywhere in the extraction.
 */
export interface FractalArtifactUrls {
  local: string;
}

export interface FractalArtifactListItem {
  id: string;
  name: string;
  icon?: string;
  urls: FractalArtifactUrls;
}
