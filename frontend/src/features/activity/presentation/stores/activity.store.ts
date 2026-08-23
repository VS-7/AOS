import type { NotificationPayload } from "@/core/builders/notification";
import { AosStore } from "@/app/builders/store";
import { api } from "@/lib/aos-facade";

/**
 * What `activity_list` actually answers with
 * (`internal/domain/activity/schema.go`'s `ListOutput`).
 *
 * `unread` is a *count* the daemon computes for the calling actor, not
 * something derivable from the entries: Go's `Activity`
 * (`internal/domain/activity/entity.go`) has no `read` field at all —
 * read state is per-actor and lives server-side, reachable only through
 * `activity_read`/`activity_read-all`. The ported code counted
 * `entry.read.length === 0` instead, which is the *original* app's shape
 * (`NotificationPayload.read`, a receipt array — see that interface's own
 * doc comment on how it was reconstructed). Against this backend every
 * entry's `read` is `undefined`, so the count threw `Cannot read
 * properties of undefined (reading 'length')` on the very first entry —
 * inside a preload whose only error handling is a `console.error`
 * (`app/builders/store.ts`'s `_hydrateActiveNamespace`). The store kept
 * its empty default state, so the Activities screen rendered no entries
 * and the sidebar's unread badge never appeared, silently, on every boot.
 */
interface ActivityListOutput {
  activities?: NotificationPayload[];
  total?: number;
  unread?: number;
  actor?: string;
}

export const ActivityStore = AosStore.create("activities")
  .withState({
    activities: [] as NotificationPayload[],
    unreadCount: 0
  })
  .withPersistence({
    enabled: false,
  })
  .withNamespace({
    resolver: ({ namespaces }) => namespaces.workspaceId,
    strategy: "memory-partition",
  })
  .withPreload(async (ctx) => {
    // task-12 disclosed divergence: `activity_list` (Go's
    // `internal/domain/activity/schema.go` `ListOutput`) answers
    // `{ activities, total, unread, actor }`, not a bare array — reading
    // `response.data` directly (as this originally did) hands `.filter` a
    // plain object and throws `response.data.filter is not a function` on
    // every load. `command-map.ts` has no mechanism for unwrapping a named
    // `Output` struct's field the way `wrapOut` nests a bare entity (that
    // only adds a layer, it doesn't remove one), so this is a call-site
    // read fix, not a map entry.
    const response = await api.activity.list.query();
    const output = response.data as ActivityListOutput | undefined;
    const activities = output?.activities ?? [];

    return {
      activities,
      unreadCount: output?.unread ?? 0,
    };
  })
  .addAction("markAsRead", (ctx) => async (activityId: string) => {
    await api.activity.markAsRead.mutate({
      params: {
        activity: activityId
      }
    })
  })
  .addAction("markAllAsRead", (ctx) => async () => {
    await api.activity.markAllAsRead.mutate();
    return ctx.state.set((prev) => ({
      ...prev,
      unreadCount: 0,
    }));
  })
  .addAction("refresh", (ctx) => async () => {
    // Same shape fix as `withPreload` above.
    const response = await api.activity.list.query();
    const output = response.data as ActivityListOutput | undefined;
    const activities = output?.activities ?? [];

    return ctx.state.set({
      activities,
      unreadCount: output?.unread ?? 0,
    });
  })
  .build();
