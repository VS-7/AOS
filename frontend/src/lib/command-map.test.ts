import { describe, expect, it } from "vitest";
import { COMMAND_MAP, DORMANT_DOMAINS, isDormant } from "./command-map";
import { COMMAND_KEYS } from "./schema";

describe("COMMAND_MAP", () => {
  it("cobre as 113 chamadas que o front do Fractal faz", () => {
    expect(Object.keys(COMMAND_MAP)).toHaveLength(113);
  });

  it("aponta apenas para comandos que o Go publica de fato", () => {
    const publicados = new Set<string>(COMMAND_KEYS);
    const quebrados = Object.entries(COMMAND_MAP)
      .filter(([, v]) => typeof v === "string" && !publicados.has(v))
      .map(([k, v]) => `${k} -> ${String(v)}`);
    expect(quebrados).toEqual([]);
  });

  it("mapeia os renomes irregulares, inclusive os kebab-case", () => {
    expect(COMMAND_MAP["task.getById"]).toBe("tasks_get");
    expect(COMMAND_MAP["task.setStatus"]).toBe("tasks_set-status");
    expect(COMMAND_MAP["activity.markAsRead"]).toBe("activity_read");
    expect(COMMAND_MAP["activity.markAllAsRead"]).toBe("activity_read-all");
  });

  it("declara dormência com null, nunca por omissão da chave", () => {
    expect("collection.list" in COMMAND_MAP).toBe(true);
    expect(COMMAND_MAP["collection.list"]).toBeNull();
  });

  it("reconhece os 14 domínios sem backend Go", () => {
    expect(DORMANT_DOMAINS.size).toBe(14);
    expect(isDormant("collection")).toBe(true);
    expect(isDormant("task")).toBe(false);
  });
});
