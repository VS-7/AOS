import { afterEach, describe, expect, it } from "vitest";
import { setLocale } from "@/lib/i18n";
import { timeAgo } from "./utils";

// The comment stamps on the task page read "2w ago" and "just now" in every
// language, next to bodies and buttons that translate.
describe("timeAgo", () => {
  afterEach(() => setLocale("en"));

  const at = (seconds: number) => new Date(Date.now() - seconds * 1000);

  it("reads in English", () => {
    setLocale("en");
    expect(timeAgo(at(10))).toBe("just now");
    expect(timeAgo(at(5 * 60))).toBe("5m ago");
    expect(timeAgo(at(3 * 3600))).toBe("3h ago");
    expect(timeAgo(at(2 * 86400))).toBe("2d ago");
    expect(timeAgo(at(14 * 86400))).toBe("2w ago");
  });

  it("reads in the language the person chose", () => {
    setLocale("pt-BR");
    expect(timeAgo(at(10))).toBe("agora mesmo");
    expect(timeAgo(at(5 * 60))).toBe("há 5min");
    expect(timeAgo(at(3 * 3600))).toBe("há 3h");
    expect(timeAgo(at(2 * 86400))).toBe("há 2d");
    expect(timeAgo(at(14 * 86400))).toBe("há 2sem");
    expect(timeAgo(at(60 * 86400))).toBe("há 2m");
    expect(timeAgo(at(400 * 86400))).toBe("há 1a");
  });
});
