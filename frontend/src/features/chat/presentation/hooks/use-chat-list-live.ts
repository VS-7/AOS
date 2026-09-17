import * as React from "react";
import { aos } from "@/app/aos";
import { useRealtime } from "@/hooks/use-realtime";

/**
 * How long a burst of chat writes is given to finish before the roster is
 * read again. One send is several writes — the question, the run being
 * marked, the answer — and reading the list once per write would be three
 * reads of the most expensive listing a sidebar makes, for one change.
 */
const REFRESH_DELAY_MS = 300;

/**
 * Keeps the chat roster (`aos.stores.chat`) following the workspace.
 *
 * The roster is a preloaded store, and the realtime channel only invalidates
 * react-query caches, which the store never reads. So it was refreshed on
 * mount and after a rename or delete in this window, and at no other time: a
 * channel created from the sidebar said "Channel created." beside a list still
 * reading "No channels yet", a DM opened from Team or search never appeared,
 * and last-activity stamps froze at whatever they were when the sidebar
 * mounted.
 *
 * Every chat write publishes `collection.changed` for `chats`, whoever made
 * it — this window, another one, an agent's turn — so that is the one signal
 * followed. `chat.done` is followed too: it is the moment a turn's answer is
 * stored, and it costs nothing when the write already scheduled the read.
 */
export function useChatListLive(): void {
  const timer = React.useRef<ReturnType<typeof setTimeout> | null>(null);

  const schedule = React.useCallback(() => {
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      timer.current = null;
      void aos.stores.chat.actions.refresh();
    }, REFRESH_DELAY_MS);
  }, []);

  React.useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
      timer.current = null;
    },
    [],
  );

  useRealtime(
    "records:changed",
    (payload: { collection?: string }) => {
      if (payload?.collection === "chats") schedule();
    },
    [schedule],
  );
  useRealtime("chat:end-processing", schedule, [schedule]);
}
