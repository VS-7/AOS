import { describe, expect, it } from "vitest";
import { AosStore } from "./store";
import { replacing } from "./replace-state";

describe("replacing", () => {
  it("makes a store take a record whole, dropping what the new one no longer has", async () => {
    const store = AosStore.create("replace-state-test")
      .withState({
        current: { git: { branchPrefix: "feat/", commitInstructions: "Conventional." } } as Record<string, unknown>,
      })
      .addAction("take", (ctx) => (next: Record<string, unknown>) => {
        ctx.state.set({ current: replacing(ctx.state.get().current, next) });
      })
      .build();
    await store.init();

    // A daemon answer omits an emptied field (`omitempty`). Merged as a patch,
    // the store kept the old value, and the form reopened with the text the
    // person had just deleted.
    store.actions.take({ git: { branchPrefix: "feat/" } });

    expect(store.state.current).toEqual({ git: { branchPrefix: "feat/", commitInstructions: undefined } });
  });

  it("replaces lists outright and leaves scalars as given", () => {
    expect(replacing({ tasks: [1, 2, 3], name: "a" }, { tasks: [1], name: "b" })).toEqual({ tasks: [1], name: "b" });
    expect(replacing(undefined, { name: "b" })).toEqual({ name: "b" });
    expect(replacing({ name: "a" }, null)).toBeNull();
  });
});
