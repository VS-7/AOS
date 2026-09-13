import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { TODO_TRANSITIONS } from "./todo";

const GO_NAMES: Record<string, string> = {
  Pending: "pending",
  InProgress: "in_progress",
  Blocked: "blocked",
  Finished: "finished",
  Skipped: "skipped",
};

// The todo status menu offers only the moves the daemon accepts. Its table is
// a copy, so it is pinned to the Go one it copies.
describe("TODO_TRANSITIONS", () => {
  it("is the daemon's todo lifecycle", () => {
    const source = readFileSync(resolve(process.cwd(), "../internal/domain/todo/entity.go"), "utf8");
    const block = source.slice(source.indexOf("var transitions = map[Status][]Status{"));
    const body = block.slice(0, block.indexOf("\n}"));
    const parsed: Record<string, string[]> = {};
    for (const match of body.matchAll(/^\s*(\w+):\s*\{([^}]*)\}/gm)) {
      parsed[GO_NAMES[match[1]]] = match[2].split(",").map((name) => GO_NAMES[name.trim()]).filter(Boolean);
    }
    expect(Object.keys(parsed)).toHaveLength(5);
    expect(TODO_TRANSITIONS).toEqual(parsed);
  });
});
