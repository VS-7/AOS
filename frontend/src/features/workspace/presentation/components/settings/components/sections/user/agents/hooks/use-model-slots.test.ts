import { describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useModelSlots } from "./use-model-slots";

/**
 * R-A2: the slot picker set its own copy of the value before saving and never
 * put it back when the daemon refused, and the effect that re-synced from the
 * configuration did not run because the configuration had not changed. The
 * row went on showing a model that was never saved.
 */

const saved = { default: { provider: "openai", model: "gpt-5.1", reasoning: "medium" as const } };

describe("useModelSlots", () => {
  it("shows the pick while it saves, and the saved value again when the save is refused", async () => {
    let refuse!: (error: Error) => void;
    const persist = vi.fn(() => new Promise<void>((_, reject) => (refuse = reject)));
    const { result } = renderHook(() => useModelSlots(saved, persist));

    let pending!: Promise<void>;
    act(() => {
      pending = result.current.change("default", { provider: "anthropic", model: "claude-opus-5" });
    });
    expect(result.current.value.default).toEqual({ provider: "anthropic", model: "claude-opus-5" });

    await act(async () => {
      refuse(new Error("AOS_CONFIG_SAVE_FAILED"));
      await pending.catch(() => {});
    });
    expect(result.current.value.default).toEqual(saved.default);
  });

  it("keeps the pick until the saved configuration catches up", async () => {
    const persist = vi.fn(async () => {});
    const { result, rerender } = renderHook(({ models }) => useModelSlots(models, persist), {
      initialProps: { models: saved as Record<string, unknown> },
    });

    await act(async () => {
      await result.current.change("default", { provider: "anthropic", model: "claude-opus-5" });
    });
    // Saved, but the route context has not been re-read yet.
    expect(result.current.value.default).toEqual({ provider: "anthropic", model: "claude-opus-5" });

    const next = { default: { provider: "anthropic", model: "claude-opus-5", reasoning: "medium" } };
    rerender({ models: next });
    expect(result.current.value.default).toEqual(next.default);
  });
});
