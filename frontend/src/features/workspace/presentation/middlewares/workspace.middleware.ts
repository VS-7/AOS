// Neighbouring-feature stub, same spirit as the goal/project/chat type
// stubs this task creates — but `workspace` isn't one of the three the
// brief named, so this doesn't carry that convention's exact removal
// marker; flagged separately in the task-6 report for the controller to
// fold into whichever task ports `workspace` for real.
//
// Fractal's original redirects to /login or /onboarding based on
// `stores.auth.state` — a store this vertical slice's `aos.tsx` does not
// wire (Task 10). It's also redundant here: AOS already gates the entire
// router behind real auth/onboarding via `<AuthGate>` in `App.tsx`, before
// `RouterProvider` (and therefore any page, including this one) ever
// mounts. So this middleware has nothing left to do — a no-op loader that
// satisfies `AosPage.use(...)`'s shape (see `app/builders/page.ts`: it
// calls `proc.loader(...)` and merges a truthy return into route context).
export function WorkspacePageMiddleware() {
  return {
    loader: async () => ({}),
  };
}
