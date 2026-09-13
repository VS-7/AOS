import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";

const openSettings = vi.hoisted(() => vi.fn());
vi.mock("@/app/aos", () => ({
  aos: { stores: { viewport: { actions: { openSettings } } } },
}));

import { ChatTurnFailure, settingsSectionFor } from "./turn-failure";

beforeEach(() => {
  cleanup();
  openSettings.mockReset();
});

// The refusal the user's Luara DM recorded: the provider stopped serving the
// model the Default slot names. The card said only "luara could not answer",
// the message and the code — nothing about where the model is chosen.
const refused = {
  agentId: "luara",
  status: "error",
  error: {
    code: "AOS_AGENT_MODEL_UNAVAILABLE",
    message: "the codex provider refused the model \"gpt-5.4-mini\": The 'gpt-5.4-mini' model is not supported when using Codex with a ChatGPT account.",
    cta: [
      { label: "choose another model for the Default slot in Settings › AI Providers › Models", tool: "config_update" },
      { label: "choose a model this account can use", tool: "models_list", input: { provider: "codex" } },
    ],
  },
};

describe("ChatTurnFailure", () => {
  it("says what to do, and opens the place to do it", () => {
    render(<ChatTurnFailure run={refused} />);

    expect(screen.getByText(/luara could not answer/i)).toBeTruthy();
    expect(screen.getByText(/Default slot in Settings/)).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: /AI Providers/i }));
    expect(openSettings).toHaveBeenCalledWith("user.agents");
  });

  it("opens the agent's own settings when the model is the agent's", () => {
    render(
      <ChatTurnFailure
        run={{ ...refused, error: { ...refused.error, cta: [{ label: "choose another model in API Builder's own settings", tool: "agents_update", input: { id: "api-builder" } }] } }}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /agent settings/i }));
    expect(openSettings).toHaveBeenCalledWith("workspace.agents");
  });

  it("offers no button for a failure with nowhere to go", () => {
    render(<ChatTurnFailure run={{ agentId: "luara", status: "error", error: { code: "AOS_AGENT_PROVIDER_FAILED", message: "timeout" } }} />);
    expect(screen.queryByRole("button")).toBeNull();
  });
});

describe("settingsSectionFor", () => {
  it("maps the tools this window has a screen for", () => {
    expect(settingsSectionFor({ tool: "config_update" })).toBe("user.agents");
    expect(settingsSectionFor({ tool: "models_list" })).toBe("user.agents");
    expect(settingsSectionFor({ tool: "agents_update" })).toBe("workspace.agents");
    expect(settingsSectionFor({ tool: "tasks_branch" })).toBeNull();
    expect(settingsSectionFor({})).toBeNull();
  });
});
