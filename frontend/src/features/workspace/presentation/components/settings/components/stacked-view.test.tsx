import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { SettingsStackedView } from "./stacked-view";

/**
 * Closed, this view is transparent and shifted aside — still in the document.
 * Without `inert` its fields kept their turn in the tab order, so a keyboard
 * could land on an invisible button and press it (#223). The title is the
 * other half: it was accepted and never drawn, so creating a task type and
 * editing one looked the same.
 */
describe("SettingsStackedView", () => {
  afterEach(cleanup);

  it("takes itself out of the page while it is closed", () => {
    const { container } = render(
      <SettingsStackedView open={false} title="New task type" onBack={vi.fn()}>
        <button type="button">Create task type</button>
      </SettingsStackedView>,
    );

    const panel = container.firstElementChild as HTMLElement;
    expect(panel.hasAttribute("inert")).toBe(true);
    expect(panel.getAttribute("aria-hidden")).toBe("true");
  });

  it("is reachable again once it is open", () => {
    const { container } = render(
      <SettingsStackedView open title="New task type" onBack={vi.fn()}>
        <button type="button">Create task type</button>
      </SettingsStackedView>,
    );

    const panel = container.firstElementChild as HTMLElement;
    expect(panel.hasAttribute("inert")).toBe(false);
    expect(panel.getAttribute("aria-hidden")).toBe("false");
  });

  it("draws the title and description it is given", () => {
    render(
      <SettingsStackedView open title="Edit bug" description="How bugs are tracked" onBack={vi.fn()}>
        <span>fields</span>
      </SettingsStackedView>,
    );

    expect(screen.getByRole("heading", { name: "Edit bug" })).toBeTruthy();
    expect(screen.getByText("How bugs are tracked")).toBeTruthy();
  });
});
