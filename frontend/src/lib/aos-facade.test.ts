import { describe, expect, it } from "vitest";
import { flattenArgs } from "./aos-facade";

describe("flattenArgs", () => {
  it("funde params, query e body numa carga única", () => {
    expect(flattenArgs({ params: { task: "t-1" }, query: { limit: 10 }, body: { title: "x" } }))
      .toEqual({ task: "t-1", limit: 10, title: "x" });
  });

  it("não deixa `enabled` vazar para a carga", () => {
    expect(flattenArgs({ query: { limit: 5 }, enabled: false })).toEqual({ limit: 5 });
  });

  it("devolve objeto vazio quando não há argumentos", () => {
    expect(flattenArgs()).toEqual({});
  });
});
