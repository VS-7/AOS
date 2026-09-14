import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import type { ModelProvider } from "@/features/model/interfaces/model.interfaces";

vi.mock("@tanstack/react-router", () => ({ useRouter: () => ({ invalidate: vi.fn() }) }));
vi.mock("@tanstack/react-query", () => ({ useQueryClient: () => ({ invalidateQueries: vi.fn() }) }));
vi.mock("@/app/aos", () => ({
  aos: { stores: { agent: { useState: (select: (s: unknown) => unknown) => select({ items: [] }) } } },
}));
vi.mock("@/features/model/services/model-provider.service", () => ({
  MODEL_DISCOVERY_KEY: ["models"],
  disconnectModelProvider: vi.fn(),
}));
vi.mock("../hooks/use-provider-logo", () => ({ useProviderLogo: () => "" }));
vi.mock("./provider-upsert-dialog", () => ({ ProviderUpsertDialog: () => null }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import { ProvidersSection } from "./providers-section";

beforeAll(() => {
  // Radix measures what it positions; jsdom has none of these.
  globalThis.ResizeObserver ??= class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as never;
  Element.prototype.scrollIntoView ??= () => {};
  Element.prototype.hasPointerCapture ??= () => false;
});
afterEach(cleanup);

const anthropic = {
  id: "anthropic",
  name: "Anthropic",
  description: "",
  logo: { light: "", dark: "" },
  configured: true,
  default: false,
  auth: { mode: "api-key", connectionType: "external", label: "API Key", placeholder: "", description: "", required: true },
  models: [],
} as unknown as ModelProvider;

describe("the disconnect confirmation", () => {
  // It said nothing else on the machine is touched, and the next line listed
  // the model slots the same write clears.
  it("does not say nothing else changes above the slots it clears", () => {
    render(
      <ProvidersSection providers={[anthropic]} models={{ default: { provider: "anthropic" } }} />,
    );
    fireEvent.keyDown(screen.getByRole("button", { name: "Manage Anthropic" }), { key: "Enter" });
    fireEvent.click(screen.getByRole("menuitem", { name: "Disconnect" }));

    const dialog = screen.getByRole("alertdialog");
    expect(dialog.textContent).toContain("Model slots cleared: Default");
    expect(dialog.textContent).not.toMatch(/Nothing else/);
    expect(dialog.textContent).toContain(
      "Anthropic is removed from this installation's connected providers. A login it has outside AOS, such as a CLI's, is left as it is.",
    );
  });
});
