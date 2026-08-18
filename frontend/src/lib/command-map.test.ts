import { describe, expect, it } from "vitest";
import { COMMAND_MAP, DORMANT_DOMAINS, isDormant } from "./command-map";
import { COMMAND_KEYS } from "./schema";

describe("COMMAND_MAP", () => {
  it("covers the 113 calls the Fractal frontend makes", () => {
    expect(Object.keys(COMMAND_MAP)).toHaveLength(113);
  });

  it("points only at commands the Go side actually publishes", () => {
    const published = new Set<string>(COMMAND_KEYS);
    const broken = Object.entries(COMMAND_MAP)
      .filter(([, v]) => typeof v === "string" && !published.has(v))
      .map(([k, v]) => `${k} -> ${String(v)}`);
    expect(broken).toEqual([]);
  });

  it("maps the irregular renames, including the kebab-case ones", () => {
    expect(COMMAND_MAP["task.getById"]).toBe("tasks_get");
    expect(COMMAND_MAP["task.setStatus"]).toBe("tasks_set-status");
    expect(COMMAND_MAP["activity.markAsRead"]).toBe("activity_read");
    expect(COMMAND_MAP["activity.markAllAsRead"]).toBe("activity_read-all");
  });

  it("declares dormancy with null, never by omitting the key", () => {
    expect("collection.list" in COMMAND_MAP).toBe(true);
    expect(COMMAND_MAP["collection.list"]).toBeNull();
  });

  it("recognizes the 14 domains without a Go backend", () => {
    expect(DORMANT_DOMAINS.size).toBe(14);
    expect(isDormant("collection")).toBe(true);
    expect(isDormant("task")).toBe(false);
  });
});
