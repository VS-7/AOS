import * as React from "react";

import type { UpdateStatus } from "@/features/update/interfaces/update.interfaces";
import { followed, type Followed } from "./updates.helper";

/**
 * How often the status is read while a download or install runs that this
 * screen has no answer from. Short enough that the release shows up about
 * when it is staged; a status read changes nothing on the daemon.
 */
export const FOLLOW_EVERY_MS = 1500;

/**
 * How long a follow goes on hearing nothing at all before it gives up.
 *
 * The bound is on silence, not on the work: while status reads keep landing
 * and keep saying busy, the daemon is demonstrably alive and getting on with
 * a download that a slow link can make genuinely long, and cutting that off
 * at a fixed time would report a working install as lost.
 *
 * Hearing nothing is the other case, and it had no bound at all: the refusals
 * this screen follows on (AOS_DAEMON_TIMEOUT, AOS_DAEMON_ANSWER_LOST) are the
 * bridge saying it lost the connection, and a daemon that then stays
 * unreachable left "Downloading…" on the screen with every button disabled,
 * polling every 1.5 seconds, saying nothing, for ever.
 *
 * A minute is longer than the daemon takes to restart under an install, which
 * is the one time a status read is expected to fail for a while.
 */
export const FOLLOW_SILENCE_MS = 60_000;

/** How a followed call ended. `lost` is the silence bound above. */
export type FollowEnd = "staged" | "installed" | "ended" | "lost";

/** The part of the status query this follow reads. */
export interface StatusRead {
  status: UpdateStatus | null;
  /** When the last status read landed; 0 before any has. */
  dataUpdatedAt: number;
  refetch: (opts?: { cancelRefetch?: boolean }) => unknown;
}

export interface Follow {
  /** The call being followed, while one is. */
  following: Followed | null;
  /** Follow a download or install this screen lost the answer to. */
  follow: (call: Followed) => void;
  /** True while the status is being read again and again. */
  running: boolean;
}

/**
 * Following a download or install this screen has no answer from — its own
 * lost call, or one it opened on — until the status says how it ended.
 *
 * It is a hook rather than two effects in the panel because the decisions are
 * all in the effects: the guard that ignores a status read from before the
 * call began (without it, the status still on screen when the button was
 * clicked ends the follow immediately, as "ended", with an error toast about
 * a download that is running fine), the polling, and the bound on silence.
 * None of that was reachable from a test of the pure helpers.
 */
export function useFollow(read: StatusRead, onEnd: (call: Followed, end: FollowEnd) => void): Follow {
  // The call, and when this screen started following it: only a status read
  // after that moment can say how it ended.
  const [followed_, setFollowing] = React.useState<{ call: Followed; since: number } | null>(null);
  const follow = React.useCallback((call: Followed) => setFollowing({ call, since: Date.now() }), []);

  // Something runs that this screen has no answer from: its own lost call, or
  // one somebody else started that the status reports.
  const running = followed_ !== null || Boolean(read.status?.busy);

  const { refetch, status, dataUpdatedAt } = read;
  React.useEffect(() => {
    if (!running) return;
    const timer = window.setInterval(() => {
      // cancelRefetch: false, because v5 defaults it to true — a read still in
      // flight is cancelled and started again, so a status read slower than
      // the interval would be abandoned every time and never land at all.
      void refetch({ cancelRefetch: false });
    }, FOLLOW_EVERY_MS);
    return () => window.clearInterval(timer);
  }, [running, refetch]);

  // The call ends when a status read *from after it began* shows nothing
  // running, and it ends as whatever that status shows. A read that landed
  // before the click describes the world before it.
  const ended = React.useRef(onEnd);
  React.useEffect(() => {
    ended.current = onEnd;
  });
  React.useEffect(() => {
    if (!followed_ || dataUpdatedAt <= followed_.since) return;
    const where = followed(followed_.call, status);
    if (where === "running") return;
    setFollowing(null);
    ended.current(followed_.call, where);
  }, [followed_, status, dataUpdatedAt]);

  // Nothing heard at all since the call began, or since the last status read
  // that landed: the daemon is unreachable, and waiting on it in silence is
  // not a state this screen leaves somebody in.
  React.useEffect(() => {
    if (!followed_) return;
    const heard = Math.max(followed_.since, dataUpdatedAt);
    const timer = window.setTimeout(
      () => {
        setFollowing(null);
        ended.current(followed_.call, "lost");
      },
      Math.max(0, heard + FOLLOW_SILENCE_MS - Date.now()),
    );
    return () => window.clearTimeout(timer);
  }, [followed_, dataUpdatedAt]);

  return { following: followed_?.call ?? null, follow, running };
}
