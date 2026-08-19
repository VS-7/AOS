/**
 * Adapter gap, not a Task-9 stub: `chat` is not a deferred neighbouring
 * feature (it already has a real, hand-built implementation in AOS — see
 * `features/chat/presentation/`). Fractal's original opened a chat inside a
 * multi-tab side panel; AOS's chat is a single routed page
 * (`/chat/$chatId`, see `router.tsx`), which has no equivalent "open as a
 * tab" affordance yet.
 *
 * Wiring this to `router.navigate(...)` would import `router.tsx` from
 * inside `features/chat`, and `router.tsx` imports the ported `task` pages
 * that call this helper — a real circular import, not just a smell. Left as
 * an inert no-op until AOS's chat UX grows a place for this to open into.
 */
export function openChatTab(_args: { chatId: string; title?: string }): void {
  // Intentionally inert — see file header.
}
