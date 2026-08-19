import type { NotificationPayload } from "@/core/builders/notification";
import { AosStore } from "@/app/builders/store";
import { api } from "@/lib/aos-facade";

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
    const activities = (response.data as { activities?: NotificationPayload[] } | undefined)?.activities ?? [];

    return {
      activities,
      unreadCount: activities.filter(
        (data: NotificationPayload) => data.read.length === 0,
      ).length,
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
    const activities = (response.data as { activities?: NotificationPayload[] } | undefined)?.activities ?? [];

    return ctx.state.set({
      activities,
      unreadCount: activities.filter(
        (data: NotificationPayload) => data.read.length === 0,
      ).length,
    });
  })
  .build();
