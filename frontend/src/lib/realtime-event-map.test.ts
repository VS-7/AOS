import { describe, expect, it, vi } from "vitest";
import { deliver, onRealtimeEvent, type RealtimeEvent } from "./realtime";
import { REALTIME_EVENT_MAP } from "./realtime-event-map";

/**
 * Exercises the exact subscribe/resolve/adapt sequence `hooks/use-
 * realtime.ts` runs, end to end through the real `deliver`/`onRealtimeEvent`
 * pipeline (the same one `ws.onmessage` calls) — not just asserting the map
 * object's shape. B1 of the final review specifically calls out that a
 * mapped event whose invalidation/payload still doesn't match is the same
 * dead path in a new costume; this is the test that would have caught that.
 */
function fakeQueryClient() {
  return { invalidateQueries: vi.fn(), setQueryData: vi.fn() };
}

/** The subscribe/resolve/adapt sequence `hooks/use-realtime.ts` runs. */
function subscribeEvent(name: string, callback: (payload: unknown) => void): () => void {
  const entry = REALTIME_EVENT_MAP[name];
  if (!entry) return () => {};
  const descriptor = typeof entry === "string" ? { type: entry } : entry;
  return onRealtimeEvent((raw) => {
    if (raw.type !== descriptor.type) return;
    callback(descriptor.adapt ? descriptor.adapt(raw) : raw.data);
  });
}

describe("REALTIME_EVENT_MAP, exercised through the real delivery pipeline", () => {
  it("chat:refresh fires on the daemon's chat.done, with chatId lifted from event.data.chat", () => {
    const payloads: any[] = [];
    const unsubscribe = subscribeEvent("chat:refresh", (p) => payloads.push(p));

    const daemonEvent: RealtimeEvent = { type: "chat.done", data: { chat: "c-42", usage: { tokens: 10 } } };
    deliver(fakeQueryClient() as any, daemonEvent);

    expect(payloads).toEqual([{ chatId: "c-42" }]);
    unsubscribe();
  });

  it("chat:refresh does not fire on a chat.delta (the daemon has no per-message signal, only end-of-turn)", () => {
    const payloads: any[] = [];
    const unsubscribe = subscribeEvent("chat:refresh", (p) => payloads.push(p));

    deliver(fakeQueryClient() as any, { type: "chat.delta", data: { chat: "c-42", text: "hi" } });

    expect(payloads).toEqual([]);
    unsubscribe();
  });

  it("activity:created fires on the daemon's activity, reshaped into NotificationPayload", () => {
    const payloads: any[] = [];
    const unsubscribe = subscribeEvent("activity:created", (p) => payloads.push(p));

    const daemonEvent: RealtimeEvent = {
      type: "activity",
      workspace: "ws-1",
      data: {
        id: "a-1",
        namespace: "task",
        event: "status_changed",
        title: "Task moved",
        body: "",
        actor: "atlas",
        actorType: "agent",
        createdAt: "2026-08-19T00:00:00Z",
      },
    };
    deliver(fakeQueryClient() as any, daemonEvent);

    expect(payloads).toEqual([
      {
        id: "a-1",
        namespace: "task",
        event: "status_changed",
        title: "Task moved",
        body: "",
        actor: "atlas",
        actorType: "agent",
        createdAt: "2026-08-19T00:00:00Z",
        workspaceId: "ws-1",
      },
    ]);
    unsubscribe();
  });

  it("chat:start-processing and chat:end-processing are explicit nulls — no daemon counterpart, never a silent no-op", () => {
    expect(REALTIME_EVENT_MAP["chat:start-processing"]).toBeNull();
    expect(REALTIME_EVENT_MAP["chat:end-processing"]).toBeNull();
    // Subscribing to a null entry must not throw and must simply never call
    // back — verified, not assumed.
    const payloads: any[] = [];
    const unsubscribe = subscribeEvent("chat:start-processing", (p) => payloads.push(p));
    deliver(fakeQueryClient() as any, { type: "chat.delta", data: { chat: "c-1" } });
    expect(payloads).toEqual([]);
    unsubscribe();
  });

  it("files:changed fires on collection.changed and produces a changes array with the path", () => {
    const payloads: any[] = [];
    const unsubscribe = subscribeEvent("files:changed", (p) => payloads.push(p));

    deliver(fakeQueryClient() as any, { type: "collection.changed", data: { path: "notes/todo.md", op: "update" } });

    expect(payloads).toEqual([{ context: undefined, changes: [{ path: "notes/todo.md", op: "update" }] }]);
    unsubscribe();
  });

  it("every entry in the map is either a known-good string/descriptor or an explicit null — no undefined slips in", () => {
    for (const [name, entry] of Object.entries(REALTIME_EVENT_MAP)) {
      expect(entry === null || typeof entry === "string" || typeof entry === "object", name).toBe(true);
    }
  });
});
