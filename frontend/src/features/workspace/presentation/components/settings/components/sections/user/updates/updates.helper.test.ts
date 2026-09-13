import { describe, expect, it } from "vitest";

import type { CheckResult, Release, Staged, UpdateStatus } from "@/features/update/interfaces/update.interfaces";
import {
  canCheck,
  checkLine,
  messageOf,
  offerLine,
  offerOf,
  refusalOf,
  statusLine,
} from "./updates.helper";

const release: Release = {
  version: "v0.16.0",
  channel: "stable",
  checksumsUrl: "https://example.test/checksums.txt",
  signatureUrl: "https://example.test/checksums.txt.sig",
  publishedAt: "2026-09-10T00:00:00Z",
  assets: [],
};

const status = (over: Partial<UpdateStatus> = {}): UpdateStatus => ({
  current: "v0.15.2",
  channel: "stable",
  configured: true,
  ...over,
});

const check = (over: Partial<CheckResult>): CheckResult => ({
  state: "up-to-date",
  upToDate: false,
  current: "v0.15.2",
  channel: "stable",
  ...over,
});

describe("a mutation's refusal", () => {
  // The facade resolves refusals instead of rejecting them, so onSuccess ran
  // for a refused download and said "Downloaded and verified."
  it("is read out of a resolved envelope", () => {
    const refused = { data: undefined, error: { code: "AOS_UPDATE_SIGNATURE_MISSING", message: "no signature" } };
    expect(refusalOf(refused)).toBe(refused.error);
    expect(messageOf(refusalOf(refused), "fallback")).toBe("no signature");
  });

  it("is nothing on a real success", () => {
    expect(refusalOf({ data: { staged: {} }, error: undefined })).toBeNull();
    expect(refusalOf(undefined)).toBeNull();
    expect(refusalOf({ data: 1 })).toBeNull();
  });

  it("speaks in the daemon's words, whatever shape carries them", () => {
    expect(messageOf(new Error("thrown"), "fallback")).toBe("thrown");
    expect(messageOf({ error: { message: "nested" } }, "fallback")).toBe("nested");
    expect(messageOf({ message: "  " }, "fallback")).toBe("fallback");
    expect(messageOf(null, "fallback")).toBe("fallback");
  });
});

describe("the status line", () => {
  // A build with no feed said "not checked yet" beside a button that then
  // answered "You are on the newest release.".
  it("says a build with no release feed cannot update itself", () => {
    const line = statusLine(status({ configured: false }));
    expect(line).toContain("not enabled in this build");
    expect(canCheck(status({ configured: false }))).toBe(false);
  });

  // It said "not checked yet" right under the answer of the check just made.
  it("says when the installation was last checked", () => {
    const line = statusLine(status({ checkedAt: "2026-09-12T10:00:00Z" }));
    expect(line).toMatch(/^Last checked .+\.$/);
    expect(line).not.toContain("{{when}}");
  });

  it("says nothing was checked only when nothing was", () => {
    expect(statusLine(status())).toContain("has not been checked");
    expect(canCheck(status())).toBe(true);
    expect(canCheck(null)).toBe(true);
  });
});

describe("a check's answer", () => {
  it("is 'newest release' only when the channel says so", () => {
    expect(checkLine(check({ state: "up-to-date", upToDate: true }), status())).toBe("You are on the newest release.");
    expect(checkLine(check({ state: "not-configured" }), null)).toContain("not enabled in this build");
    expect(checkLine(check({ state: "not-configured" }), status({ configured: false }))).toBeNull();
    expect(checkLine(check({ state: "available", release }), status())).toBeNull();
    expect(checkLine(null, status())).toBeNull();
  });

  it("tells a development build apart from an up-to-date one", () => {
    expect(checkLine(check({ state: "developer-build", current: "dev", release }), status())).toContain(
      "development build (dev)",
    );
    expect(checkLine(check({ state: "developer-build", current: "dev", release }), status())).toContain("v0.16.0");
    expect(checkLine(check({ state: "developer-build", current: "dev" }), status())).not.toContain("newest");
  });
});

describe("an offered release", () => {
  const staged: Staged = { version: "v0.16.0", dir: "/state/update/staged", binaries: { aosd: "/state/update/staged/aosd" } };

  it("is only offered when the channel has something newer", () => {
    expect(offerOf(check({ state: "up-to-date" }), null, status())).toBeNull();
    expect(offerOf(check({ state: "available" }), null, status())).toBeNull();
  });

  it("knows what the daemon already staged for it, and nothing staged for another version", () => {
    const available = check({ state: "available", release, install: { method: "here" } });
    expect(offerOf(available, null, status({ staged }))?.staged).toEqual(staged);
    expect(offerOf(available, staged, status())?.staged).toEqual(staged);
    expect(offerOf(available, null, status({ staged: { ...staged, version: "v0.15.9" } }))?.staged).toBeNull();
  });

  it("explains how it installs here", () => {
    const offer = (method: "here" | "terminal" | "reinstall", withStaged: boolean) =>
      offerOf(check({ state: "available", release, install: { method } }), withStaged ? staged : null, status())!;

    expect(offerLine(offer("reinstall", false))).toContain("application bundle");
    expect(offerLine(offer("terminal", true))).toContain("from a terminal");
    expect(offerLine(offer("terminal", false))).toContain("Nothing is installed");
    expect(offerLine(offer("here", true))).toContain("Installing restarts the daemon");
  });
});
