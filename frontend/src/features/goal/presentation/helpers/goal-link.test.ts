import { describe, expect, it } from "vitest";
import { shareableLink } from "./goal-link";

describe("shareableLink", () => {
  it("copies an address a browser can open when the page has one", () => {
    const link = shareableLink("/goals/launch", "http://127.0.0.1:5326", false);
    expect(link.value).toBe("http://127.0.0.1:5326/goals/launch");
    expect(link.label).toBe("Copy link");
  });

  it("copies an address on a tunnel's https origin", () => {
    expect(shareableLink("/projects/aos", "https://aos.example.com", false).value).toBe(
      "https://aos.example.com/projects/aos",
    );
  });

  // The desktop window's origin is the webview's asset host: a link on it
  // opens nothing outside AOS. macOS and Linux serve it as wails://localhost,
  // Windows as http://wails.localhost — an http origin that still is not an
  // address anybody else can open.
  it.each(["wails://localhost", "http://wails.localhost", "https://wails.localhost"])(
    "copies the path, and says so, on the desktop asset origin %s",
    (origin) => {
      const link = shareableLink("/goals/launch", origin, false);
      expect(link.value).toBe("/goals/launch");
      expect(link.label).toBe("Copy path");
      expect(link.copied).toBe("Path copied");
    },
  );

  it("copies the path in the desktop window whatever origin it was served from", () => {
    const link = shareableLink("/goals/launch", "http://127.0.0.1:34115", true);
    expect(link.value).toBe("/goals/launch");
    expect(link.label).toBe("Copy path");
  });

  it("copies the path when there is no origin at all", () => {
    expect(shareableLink("/goals/launch", "", false).value).toBe("/goals/launch");
  });
});
