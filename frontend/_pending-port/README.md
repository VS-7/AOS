# Pending port

Ported from the original, adapted (branding, strict-mode nulls, Igniter
seams removed) — but held outside `src/` because each still imports one
thing that doesn't exist yet, and nothing in `src/` currently imports any of
these, so nothing is lost by keeping them out of the compiled tree:

- `components-ui/page.tsx`, `components-ui/split-page-layout.tsx`,
  `hooks/use-notification.ts` — import `@tanstack/react-router`. Move back
  into `src/` once the router stage installs it and ports the route tree.
- `lib/tabs.ts` — imports `@/features/workspace/presentation/stores/viewport.store`.
  Move back once the workspace feature is ported (it'll need TanStack Query
  or `lib/app-state.tsx`, not a store file, per the architecture — see
  `docs/06 - Frontend/React 19 e Bindings.md`).
- `components-ui/split-page-layout.tsx` additionally imports
  `@/components/ui/resizable`, which doesn't exist anywhere in
  `_extracted/webui/src` either — genuinely missing from the reconstructed
  source, not merely unported. Reconstruct it the same way
  `data-table-toolbar-component.tsx` was (a small, faithful composition of
  what's already there — react-resizable-panels wrapped in the shadcn
  pattern) when this file gets un-staged.

Move a file back with `git mv`, not `cp` — this directory is scratch, not a
second copy to keep in sync.
