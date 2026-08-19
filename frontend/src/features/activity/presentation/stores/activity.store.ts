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
    const response = await api.activity.list.query();

    return {
      activities: response.data || [],
      unreadCount:
        response.data?.filter(
          (data: NotificationPayload) => data.read.length === 0,
        ).length || 0,
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
    const response = await api.activity.list.query();

    return ctx.state.set({
      activities: response.data || [],
      unreadCount:
        response.data?.filter(
          (data: NotificationPayload) => data.read.length === 0,
        ).length || 0,
    });
  })
  .build();
