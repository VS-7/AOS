import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";

import type { UpdateStatus } from "@/features/update/interfaces/update.interfaces";
import { FOLLOW_EVERY_MS, FOLLOW_SILENCE_MS, useFollow, type FollowEnd, type StatusRead } from "./updates.follow";
import type { Followed } from "./updates.helper";

/**
 * W3-15: only the pure helpers were tested, and every decision that matters
 * is in the effects — the guard that ignores a status read from before the
 * call began, the polling, and what happens when the daemon says nothing at
 * all. Deleting the guard passed the whole suite, and a daemon that went
 * unreachable left the screen following it for ever.
 */

const ended: [Followed, FollowEnd][] = [];
const refetch = vi.fn();
const call: Followed = { kind: "download", version: "v0.10.0" };

/** The status query, as the panel hands it over. */
function read(status: UpdateStatus | null, dataUpdatedAt: number): StatusRead {
  return { status, dataUpdatedAt, refetch };
}

function follow(initial: StatusRead) {
  return renderHook((props: StatusRead) => useFollow(props, (c, end) => ended.push([c, end])), {
    initialProps: initial,
  });
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-08-15T12:00:00Z"));
  ended.length = 0;
  refetch.mockClear();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("useFollow", () => {
  it("ignores the status that was already on screen when the call began", () => {
    // The status read that landed before the click describes the world before
    // it: nothing is busy and nothing is staged, which is exactly what a
    // download that has just started looks like from before it started.
    const stale = Date.now() - 5000;
    const { result, rerender } = follow(read({ current: "v0.9.0", channel: "stable" }, stale));

    act(() => result.current.follow(call));
    rerender(read({ current: "v0.9.0", channel: "stable" }, stale));
    expect(ended).toEqual([]);
    expect(result.current.following).toEqual(call);

    // A read from after it began is the one that decides.
    rerender(read({ current: "v0.9.0", channel: "stable" }, Date.now() + 1));
    expect(ended).toEqual([[call, "ended"]]);
    expect(result.current.following).toBeNull();
  });

  it("ends as staged, installed or ended, by what the status shows", () => {
    const cases: [UpdateStatus, Followed, FollowEnd][] = [
      [
        { current: "v0.9.0", channel: "stable", staged: { version: "v0.10.0", dir: "/tmp", binaries: {} } },
        call,
        "staged",
      ],
      [{ current: "v0.10.0", channel: "stable" }, { kind: "install", version: "v0.10.0" }, "installed"],
      [{ current: "v0.9.0", channel: "stable" }, { kind: "install", version: "v0.10.0" }, "ended"],
    ];
    for (const [status, followedCall, end] of cases) {
      ended.length = 0;
      const { result, rerender } = follow(read(null, 0));
      act(() => result.current.follow(followedCall));
      rerender(read(status, Date.now() + 1));
      expect(ended).toEqual([[followedCall, end]]);
    }
  });

  it("keeps following while the installation is busy, and reads the status again without cancelling one in flight", () => {
    const { result, rerender } = follow(read(null, 0));
    act(() => result.current.follow(call));

    rerender(read({ current: "v0.9.0", channel: "stable", busy: true }, Date.now() + 1));
    expect(ended).toEqual([]);

    act(() => void vi.advanceTimersByTime(FOLLOW_EVERY_MS * 2));
    expect(refetch).toHaveBeenCalledTimes(2);
    // v5 cancels a read still in flight by default, so a status read slower
    // than the interval would be started and dropped, over and over, and
    // nothing would ever land for the follow to read.
    expect(refetch).toHaveBeenLastCalledWith({ cancelRefetch: false });
  });

  it("reads the status for a download somebody else started, and stops when it ends", () => {
    const { rerender } = follow(read({ current: "v0.9.0", channel: "stable", busy: true }, Date.now()));
    act(() => void vi.advanceTimersByTime(FOLLOW_EVERY_MS));
    expect(refetch).toHaveBeenCalledTimes(1);

    rerender(read({ current: "v0.9.0", channel: "stable" }, Date.now() + 1));
    act(() => void vi.advanceTimersByTime(FOLLOW_EVERY_MS * 3));
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it("gives up on a daemon that says nothing at all, rather than following it for ever", () => {
    const { result, rerender } = follow(read(null, 0));
    act(() => result.current.follow(call));

    // Reads that keep landing keep the follow alive: this is a slow download,
    // not a daemon that went away.
    for (let i = 1; i <= 3; i++) {
      act(() => void vi.advanceTimersByTime(FOLLOW_SILENCE_MS - 1000));
      rerender(read({ current: "v0.9.0", channel: "stable", busy: true }, Date.now()));
    }
    expect(ended).toEqual([]);
    expect(result.current.following).toEqual(call);

    // Then the daemon stops answering altogether: no status read lands, and
    // the screen says so instead of leaving "Downloading…" up for ever.
    act(() => void vi.advanceTimersByTime(FOLLOW_SILENCE_MS));
    expect(ended).toEqual([[call, "lost"]]);
    expect(result.current.following).toBeNull();

    // Reading goes on, because the last thing the daemon said was that it was
    // busy: that is how the screen picks the story back up when it returns.
    // It stops once a read lands saying the installation is idle.
    refetch.mockClear();
    act(() => void vi.advanceTimersByTime(FOLLOW_EVERY_MS));
    expect(refetch).toHaveBeenCalledTimes(1);

    rerender(read({ current: "v0.9.0", channel: "stable" }, Date.now()));
    refetch.mockClear();
    act(() => void vi.advanceTimersByTime(FOLLOW_EVERY_MS * 3));
    expect(refetch).not.toHaveBeenCalled();
    expect(ended).toEqual([[call, "lost"]]);
  });
});
