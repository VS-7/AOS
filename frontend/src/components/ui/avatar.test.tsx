import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

/**
 * #185: the same agent had two generated faces — one seeded by its id, one by
 * its lowercased display name — depending on which screen drew it.
 */

const seen = vi.hoisted(() => [] as string[]);

vi.mock("hashvatar/react", () => ({
  Hashvatar: ({ hash }: { hash: string }) => {
    seen.push(hash);
    return <span data-hash={hash} />;
  },
}));

vi.mock("@/lib/client", () => ({
  client: {
    invoke: async () => ({ agents: [{ id: "api-builder", name: "API Builder" }] }),
  },
}));

const { AvatarAgentFallback } = await import("./avatar");

afterEach(() => {
  cleanup();
  seen.length = 0;
});

function draw(name: string) {
  const client = new QueryClient();
  return render(
    <QueryClientProvider client={client}>
      <AvatarAgentFallback name={name} />
    </QueryClientProvider>,
  );
}

describe("AvatarAgentFallback", () => {
  it("draws one face for an agent whether it is named by id or by display name", async () => {
    const byId = draw("api-builder");
    await waitFor(() => expect(byId.container.querySelector("[data-hash]")?.getAttribute("data-hash")).toBe("api-builder"));
    cleanup();

    const byName = draw("api builder");
    await waitFor(() => expect(byName.container.querySelector("[data-hash]")?.getAttribute("data-hash")).toBe("api-builder"));
  });
});
