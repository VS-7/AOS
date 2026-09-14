import { describe, expect, it, vi } from "vitest";

vi.mock("@/lib/client", () => ({ getWorkspace: () => "vs" }));

const { RoutineWebhookHelper, pendingWebhookTokens } = await import("./routine-webhook.helper");

describe("RoutineWebhookHelper", () => {
  // The row read `routine.fireUrl`, which Go has never answered with, so a
  // webhook routine showed "Save this routine to generate the public fire
  // URL" forever.
  it("addresses the daemon's webhook route in this window's workspace", () => {
    expect(RoutineWebhookHelper.fireUrl("9ffdcf95-1360", "http://127.0.0.1:5326")).toBe(
      "http://127.0.0.1:5326/api/hooks/routines/9ffdcf95-1360?workspace=vs",
    );
  });

  it("sends the token as a bearer credential, never in the URL", () => {
    const curl = RoutineWebhookHelper.curlExample("http://h/api/hooks/routines/r?workspace=vs", "tok");
    expect(curl).toContain("Authorization: Bearer tok");
    expect(curl).not.toContain("token=");
  });

  it("hands a held token out once", () => {
    pendingWebhookTokens.hold("r-1", "tok");
    expect(pendingWebhookTokens.take("r-1")).toBe("tok");
    expect(pendingWebhookTokens.take("r-1")).toBeUndefined();
  });
});
