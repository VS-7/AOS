import { AosStore } from "@/app/builders/store";
import { api } from "@/lib/aos-facade";
import type { ActivityEntry, ActivityList } from "@/features/activity/interfaces/activity.interfaces";

/**
 * The inbox, as `activity_list` answers it
 * (`internal/domain/activity/schema.go`'s `ListOutput`): a page of entries,
 * each saying whether the reader has seen it, the total that matched, and the
 * unread count for all of them.
 *
 * `unread` is a count the daemon computes for the calling actor. Each entry's
 * own `read` is the same overlay applied line by line; without it every line
 * showed as unread forever, whatever "mark all as read" had recorded.
 */

/** One page of the inbox. The daemon's default, named so paging agrees with it. */
const PAGE_SIZE = 50;

/** A page, or the refusal: a list that could not be read is not an empty one. */
async function fetchPage(offset: number, limit: number): Promise<ActivityList> {
  const response = await api.activity.list.query({ query: { offset, limit } });
  if (response.error) throw response.error;
  const output = response.data as Partial<ActivityList> | undefined;
  return {
    activities: output?.activities ?? [],
    total: output?.total ?? 0,
    unread: output?.unread ?? 0,
    actor: output?.actor ?? "",
  };
}

export const ActivityStore = AosStore.create("activities")
  .withState({
    activities: [] as ActivityEntry[],
    /** How many entries matched in all, loaded or not — what decides "Load more". */
    total: 0,
    unreadCount: 0,
  })
  .withPersistence({
    enabled: false,
  })
  .withNamespace({
    resolver: ({ namespaces }) => namespaces.workspaceId,
    strategy: "memory-partition",
  })
  .withPreload(async () => {
    const page = await fetchPage(0, PAGE_SIZE);
    return { activities: page.activities, total: page.total, unreadCount: page.unread };
  })
  // `mutateOrThrow`: `mutate` resolves a refusal as a value, and these
  // awaited it and carried on — so a daemon that refused, or was not there
  // at all, still zeroed the unread badge and let the caller announce
  // "All activities marked as read". The state changes only after the daemon
  // has said yes, and a refusal reaches the caller as the rejection it is.
  .addAction("markAsRead", (ctx) => async (activityId: string) => {
    const entry = ctx.state.get().activities.find((item) => item.id === activityId);
    if (entry?.read) return;
    await api.activity.markAsRead.mutateOrThrow({
      params: {
        activity: activityId,
      },
    });
    ctx.state.set((prev) => ({
      ...prev,
      activities: prev.activities.map((item) => (item.id === activityId ? { ...item, read: true } : item)),
      unreadCount: entry ? Math.max(0, prev.unreadCount - 1) : prev.unreadCount,
    }));
  })
  .addAction("markAllAsRead", (ctx) => async () => {
    await api.activity.markAllAsRead.mutateOrThrow();
    return ctx.state.set((prev) => ({
      ...prev,
      activities: prev.activities.map((item) => ({ ...item, read: true })),
      unreadCount: 0,
    }));
  })
  // The next page, after what is already on screen. The list used to stop at
  // the daemon's first 50 and then claim there were no more.
  .addAction("loadMore", (ctx) => async () => {
    const loaded = ctx.state.get().activities;
    const page = await fetchPage(loaded.length, PAGE_SIZE);
    const seen = new Set(loaded.map((item) => item.id));
    ctx.state.set((prev) => ({
      ...prev,
      activities: [...prev.activities, ...page.activities.filter((item) => !seen.has(item.id))],
      total: page.total,
      unreadCount: page.unread,
    }));
  })
  // As many entries as are already loaded, so a new activity arriving does
  // not snap a long list the person paged through back to its first page.
  //
  // A refresh that fails keeps what is on screen: it runs on every realtime
  // activity, unattended, and wiping the inbox because one read failed would
  // be worse than showing it a moment stale.
  .addAction("refresh", (ctx) => async () => {
    const loaded = ctx.state.get().activities.length;
    let page: ActivityList;
    try {
      page = await fetchPage(0, Math.max(PAGE_SIZE, loaded));
    } catch (error) {
      console.error("[activity] the inbox could not be refreshed", error);
      return;
    }
    return ctx.state.set({
      activities: page.activities,
      total: page.total,
      unreadCount: page.unread,
    });
  })
  .build();
