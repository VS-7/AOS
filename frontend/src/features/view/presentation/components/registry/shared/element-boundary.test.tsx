import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

const toastError = vi.fn();
vi.mock("sonner", () => ({ toast: { error: (...args: unknown[]) => toastError(...args) } }));

const { withElementBoundaries } = await import("./element-boundary");

// A registry component the way the real Button and Link are: it fires `press`
// and never looks at what firing it returns. The test keeps that return value
// to see whether anything is left to reject.
let fired: unknown[] = [];
const Pressable = ({ props, emit, on }: any) => (
  <>
    <button type="button" onClick={() => fired.push(emit("press"))}>{props.label}</button>
    <button type="button" onClick={() => fired.push(on("press").emit())}>{props.label} (handle)</button>
  </>
);
const { Pressable: Wrapped } = withElementBoundaries({ Pressable });

function renderWith(emit: (event: string) => unknown) {
  return render(
    <Wrapped
      props={{ label: "Bad action" }}
      emit={emit}
      on={(event: string) => ({ emit: () => emit(event), shouldPreventDefault: false, bound: true })}
    />,
  );
}

afterEach(() => {
  cleanup();
  fired = [];
  toastError.mockClear();
});

describe("an element's emitted events", () => {
  // json-render rejects the press with Error("Action cancelled") when the
  // person answers Cancel to an action's confirmation. Nothing awaited it, so
  // every Cancel was an uncaught exception.
  it("settle quietly when a confirmation is cancelled", async () => {
    const emit = vi.fn(() => Promise.reject(new Error("Action cancelled")));
    renderWith(emit);
    fireEvent.click(screen.getByText("Bad action"));
    fireEvent.click(screen.getByText("Bad action (handle)"));

    expect(emit).toHaveBeenCalledTimes(2);
    for (const result of fired) await expect(result).resolves.toBeUndefined();
    expect(toastError).not.toHaveBeenCalled();
  });

  it("say so when an action fails for any other reason", async () => {
    const emit = vi.fn(() => Promise.reject(new Error("the daemon is not answering")));
    renderWith(emit);
    fireEvent.click(screen.getByText("Bad action"));

    await expect(fired[0]).resolves.toBeUndefined();
    expect(toastError).toHaveBeenCalledWith("The action failed", { description: "the daemon is not answering" });
  });
});
