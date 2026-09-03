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
  const types = Array.isArray(descriptor.type) ? descriptor.type : [descriptor.type];
  return onRealtimeEvent((raw) => {
    if (!types.includes(raw.type)) return;
    callback(descriptor.adapt ? descriptor.adapt(raw) : raw.data);
  });
}

describe("REALTIME_EVENT_MAP, exercised through the real delivery pipeline", () => {
  // `replace: true` is the point of this one. At the end of a turn the stored
  // transcript is authoritative and the local one is not — a partial answer
  // from a turn that failed mid-stream, or a snapshot the merge preferred
  // because it had more parts, stayed on screen otherwise.
  it("chat:refresh asks for a verbatim replace on the daemon's chat.done", () => {
    const payloads: any[] = [];
    const unsubscribe = subscribeEvent("chat:refresh", (p) => payloads.push(p));

    const daemonEvent: RealtimeEvent = { type: "chat.done", data: { chat: "c-42", usage: { tokens: 10 } } };
    deliver(fakeQueryClient() as any, daemonEvent);

    expect(payloads).toEqual([{ chatId: "c-42", replace: true }]);
    unsubscribe();
  });

  it("chat:refresh carries a translated snapshot on chat.message, which is what renders a turn as it happens", () => {
    const payloads: any[] = [];
    const unsubscribe = subscribeEvent("chat:refresh", (p) => payloads.push(p));

    const message = {
      id: "m-1",
      role: "assistant",
      author: { type: "agent", id: "atlas" },
      parts: [{ type: "tool-call", toolName: "memories_recall", toolCallId: "c-1" }],
      createdAt: "2026-09-01T10:00:00Z",
    };
    deliver(fakeQueryClient() as any, { type: "chat.message", data: { chat: "c-42", message } });

    // The snapshot goes through the same translator a stored message does. It
    // used to be passed through raw, so the answer being written had no
    // metadata: attributed to the chat's title rather than the agent, with no
    // timestamp, and with each tool call rendered twice and never as running.
    expect(payloads).toHaveLength(1);
    expect(payloads[0].chatId).toBe("c-42");
    expect(payloads[0].message.metadata.type).toBe("agent");
    expect(payloads[0].message.metadata.execution.status).toBe("running");
    expect(payloads[0].message.parts).toEqual([
      {
        type: "tool-memories_recall",
        toolCallId: "c-1",
        toolName: "memories_recall",
        input: undefined,
        state: "input-available",
      },
    ]);
    unsubscribe();
  });

  it("chat:refresh does not fire on a chat.delta (text-only; the snapshot event carries the whole message)", () => {
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

  // These two were `null` while the daemon had no event naming *which* agent
  // was working — `layout/index.tsx` feeds both straight into
  // `setProcessing(chatId, agentId, ...)`, and an occupancy map keyed by a
  // fabricated id is worse than an indicator that stays dark. The daemon now
  // states both facts, so the pair drives "Atlas is working…" for real.
  it("chat:start-processing carries the agent, so the indicator knows who is working", () => {
    const payloads: any[] = [];
    const unsubscribe = subscribeEvent("chat:start-processing", (p) => payloads.push(p));

    deliver(fakeQueryClient() as any, { type: "chat.started", data: { chat: "c-1", agent: "atlas" } });

    expect(payloads).toEqual([{ chatId: "c-1", agentId: "atlas" }]);
    unsubscribe();
  });

  it("chat:end-processing carries the same agent, so the indicator clears the one that finished", () => {
    const payloads: any[] = [];
    const unsubscribe = subscribeEvent("chat:end-processing", (p) => payloads.push(p));

    deliver(fakeQueryClient() as any, {
      type: "chat.done",
      data: { chat: "c-1", agent: "atlas", usage: { total: 5 } },
    });

    expect(payloads).toEqual([{ chatId: "c-1", agentId: "atlas" }]);
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
