import { describe, expect, it, vi } from "vitest";
import { deliver, onRealtimeEvent, type RealtimeEvent } from "./realtime";

/** Just enough of `QueryClient` for `deliver`/`dispatch` to call — no React needed. */
function fakeQueryClient() {
  return {
    invalidateQueries: vi.fn(),
    setQueryData: vi.fn(),
  };
}

describe("dispatch, via deliver (B1(c): query keys migrated to the facade's shape)", () => {
  it("chat.done invalidates the facade's chat.getById key for the chat that finished, not the old two-element key", () => {
    const qc = fakeQueryClient();
    const event: RealtimeEvent = { type: "chat.done", data: { chat: "c-1", usage: {} } };

    deliver(qc as any, event);

    expect(qc.invalidateQueries).toHaveBeenCalledTimes(1);
    const call = qc.invalidateQueries.mock.calls[0][0];
    expect(typeof call.predicate).toBe("function");

    // The facade's real key shape: [feature, action, flattenArgs(opts)].
    expect(call.predicate({ queryKey: ["chat", "getById", { chat: "c-1" }] })).toBe(true);
    // A different open chat's getById query must not be swept up.
    expect(call.predicate({ queryKey: ["chat", "getById", { chat: "c-2" }] })).toBe(false);
    // The old, never-matching two-element shape this replaced.
    expect(call.predicate({ queryKey: ["chat", "c-1"] })).toBe(false);
    // A different action on the same feature.
    expect(call.predicate({ queryKey: ["chat", "list", {}] })).toBe(false);
  });

  it("activity invalidates the facade-shaped ['activity'] prefix and the dynamic namespace prefix", () => {
    const qc = fakeQueryClient();
    const event: RealtimeEvent = { type: "activity", data: { namespace: "task", event: "status_changed" } };

    deliver(qc as any, event);

    expect(qc.invalidateQueries).toHaveBeenCalledWith({ queryKey: ["activity"] });
    expect(qc.invalidateQueries).toHaveBeenCalledWith({ queryKey: ["task"] });
  });

  it("approval.request invalidates ['approvals'], matching ApprovalModal's own key", () => {
    const qc = fakeQueryClient();
    deliver(qc as any, { type: "approval.request", data: {} });
    expect(qc.invalidateQueries).toHaveBeenCalledWith({ queryKey: ["approvals"] });
  });
});

describe("onRealtimeEvent (B1(b): the one shared subscription point)", () => {
  it("delivers a live event to every raw subscriber", () => {
    const received: RealtimeEvent[] = [];
    const unsubscribe = onRealtimeEvent((e) => received.push(e));

    const event: RealtimeEvent = { type: "activity", data: { namespace: "task" } };
    deliver(fakeQueryClient() as any, event);

    expect(received).toEqual([event]);
    unsubscribe();
  });

  it("stops delivering once unsubscribed", () => {
    const received: RealtimeEvent[] = [];
    const unsubscribe = onRealtimeEvent((e) => received.push(e));
    unsubscribe();

    deliver(fakeQueryClient() as any, { type: "activity", data: {} });

    expect(received).toEqual([]);
  });

  it("delivers to every subscriber independently, and one unsubscribing doesn't affect the other", () => {
    const a: RealtimeEvent[] = [];
    const b: RealtimeEvent[] = [];
    const unsubA = onRealtimeEvent((e) => a.push(e));
    const unsubB = onRealtimeEvent((e) => b.push(e));

    const event: RealtimeEvent = { type: "chat.done", data: { chat: "c-1" } };
    deliver(fakeQueryClient() as any, event);
    unsubA();
    deliver(fakeQueryClient() as any, event);

    expect(a).toEqual([event]);
    expect(b).toEqual([event, event]);
    unsubB();
  });
});
