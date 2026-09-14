import { describe, expect, it } from "vitest";
import { setLocale } from "@/lib/i18n";
import { GoalHelper } from "./goal.helper";

// The Goals list wrote a deadline with a hard-coded en-US locale in local
// time: a goal due 2026-09-20 read "Sep 19, 2026" west of UTC, and stayed in
// English inside the Portuguese interface.
describe("GoalHelper.formatDeadline", () => {
  it("writes the picked day, in the interface's language", () => {
    setLocale("en");
    expect(GoalHelper.formatDeadline("2026-09-20T00:00:00Z")).toBe("Sep 20, 2026");
    setLocale("pt-BR");
    expect(GoalHelper.formatDeadline("2026-10-01T00:00:00Z")).toBe("1 de out. de 2026");
    setLocale("en");
  });
});
