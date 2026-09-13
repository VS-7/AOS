import { describe, expect, it } from "vitest";
import { shareableLink } from "./goal-link";

describe("shareableLink", () => {
  it("copies an address a browser can open when the page has one", () => {
    expect(shareableLink("/goals/launch", "http://127.0.0.1:5326").value).toBe("http://127.0.0.1:5326/goals/launch");
  });

  // In the desktop window the origin is the webview's asset scheme: a link on
  // it opens nothing outside AOS.
  it("copies the path, and says so, when the origin is not an address", () => {
    const link = shareableLink("/goals/launch", "wails://wails");
    expect(link.value).toBe("/goals/launch");
    expect(link.label).toBe("Copy path");
  });
});
