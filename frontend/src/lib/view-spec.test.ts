import { describe, expect, it } from "vitest";
import { actionsOf, formatViewValue, toSpec, type RenderedView } from "./view-spec";

/**
 * The fixtures are what `views_render` actually answers: `view.Rendered`, with
 * the composed tree nested and unresolved beside the records it selected. The
 * scaffold's own two shapes (`internal/domain/view/scaffold.go`) are the ones
 * that have to work, because they are the only trees the system composes on
 * its own.
 */

const scaffoldedDetail: RenderedView = {
  view: {
    id: "deals-detail",
    title: "Deals",
    tree: {
      component: "Stack",
      children: [
        { component: "Text", bind: { text: "name" } },
        { component: "Badge", bind: { text: "stage" } },
        { component: "Stat", props: { label: "amount" }, bind: { value: "amount" } },
      ],
    },
  },
  records: [
    { id: "d-1", data: { name: "Acme", stage: "won", amount: 1200 } },
    { id: "d-2", data: { name: "Globex", stage: "open", amount: 800 } },
  ],
  renderedAt: "2026-09-03T10:00:00Z",
};

const scaffoldedTable: RenderedView = {
  view: {
    id: "deals-table",
    title: "Deals",
    tree: { component: "Table", props: { columns: ["name", "stage"], rows: [] } },
  },
  records: [
    { id: "d-1", data: { name: "Acme", stage: "won" } },
    { id: "d-2", data: { name: "Globex", stage: "open" } },
  ],
};

describe("toSpec", () => {
  it("gives the renderer a root that names a real element", () => {
    const spec = toSpec(scaffoldedDetail)!;
    // The exact check `ViewRenderer` makes before it refuses to render.
    expect(typeof spec.root).toBe("string");
    expect(spec.elements[spec.root]).toBeDefined();
    expect(typeof (spec.elements[spec.root] as { type: string }).type).toBe("string");
  });

  it("repeats one record's worth of screen, once per record", () => {
    const spec = toSpec(scaffoldedDetail)!;
    const stacks = Object.values(spec.elements).filter((el) => (el as { type: string }).type === "Stack");
    // Two records, plus the synthetic root the copies hang from.
    expect(stacks).toHaveLength(3);

    const texts = Object.values(spec.elements).filter((el) => (el as { type: string }).type === "Text");
    expect(texts).toHaveLength(2);
    expect((texts[0] as { props: Record<string, unknown> }).props["text"]).toEqual({
      $state: "/records/0/data/name",
    });
    expect((texts[1] as { props: Record<string, unknown> }).props["text"]).toEqual({
      $state: "/records/1/data/name",
    });
  });

  it("keeps a bound node's literal props beside its bindings", () => {
    const spec = toSpec(scaffoldedDetail)!;
    const stat = Object.values(spec.elements).find((el) => (el as { type: string }).type === "Stat") as {
      props: Record<string, unknown>;
    };
    expect(stat.props["label"]).toBe("amount");
    expect(stat.props["value"]).toEqual({ $state: "/records/0/data/amount" });
  });

  // The state is what an action's `updates` are applied to, by JSON pointer.
  // Bound props point into it, so a patched field re-renders.
  it("seeds the state with the records the bindings point at", () => {
    const spec = toSpec(scaffoldedDetail)!;
    expect((spec.state as { records: unknown[] }).records).toHaveLength(2);
  });

  // The catalog's Table takes `rows: string[][]` in column order and calls
  // `row.map` on each; rows built as objects threw "row.map is not a
  // function", and json-render swallowed it into a blank page.
  it("fills a table's rows from the records, as cell strings in column order", () => {
    const spec = toSpec(scaffoldedTable)!;
    const table = spec.elements[spec.root] as { type: string; props: Record<string, unknown> };
    expect(table.type).toBe("Table");
    expect(table.props["rows"]).toEqual([
      ["Acme", "won"],
      ["Globex", "open"],
    ]);
    // A table renders once whatever the record count: it takes the rows itself.
    expect(Object.keys(spec.elements)).toHaveLength(1);
  });

  // A fixed sidebar beside bound content must not be drawn once per record.
  it("repeats the lowest node holding every binding, not the whole tree", () => {
    const spec = toSpec({
      view: {
        tree: {
          component: "SplitPageLayout",
          children: [
            { component: "SplitPageSidebar", props: { title: "Filters" } },
            {
              component: "SplitPageContent",
              children: [{ component: "Card", bind: { title: "name" } }],
            },
          ],
        },
      },
      records: [{ id: "a", data: { name: "One" } }, { id: "b", data: { name: "Two" } }],
    })!;

    const kinds = Object.values(spec.elements).map((el) => (el as { type: string }).type);
    expect(kinds.filter((type) => type === "SplitPageSidebar")).toHaveLength(1);
    expect(kinds.filter((type) => type === "SplitPageContent")).toHaveLength(1);
    expect(kinds.filter((type) => type === "Card")).toHaveLength(2);
  });

  it("renders a tree that binds nothing exactly once", () => {
    const spec = toSpec({
      view: { tree: { component: "Heading", props: { text: "Nothing here yet" } } },
      records: [{ id: "a" }, { id: "b" }],
    })!;
    expect(Object.keys(spec.elements)).toHaveLength(1);
  });

  // An empty collection is a screen with nothing in it, not a screen full of
  // the word "undefined".
  it("drops bindings when there is no record to resolve them against", () => {
    const spec = toSpec({ ...scaffoldedDetail, records: [] })!;
    const texts = Object.values(spec.elements).filter((el) => (el as { type: string }).type === "Text");
    expect(texts).toHaveLength(0);
    expect(spec.elements[spec.root]).toBeDefined();
  });

  it("turns a node's buttons into event bindings the renderer can fire", () => {
    const spec = toSpec({
      view: {
        tree: {
          component: "Button",
          props: { label: "Close" },
          actions: [{ label: "Close deal", command: "tasks_set-status", input: { status: "done" }, confirm: true }],
        },
      },
      records: [],
    })!;

    // `press` is the event the registry's Button and Link emit. Bindings keyed
    // under `click` never matched, so every button in a view did nothing.
    const button = spec.elements[spec.root] as { on?: Record<string, any> };
    expect(button.on?.["click"]).toBeUndefined();
    expect(button.on?.["press"].action).toBe("Close deal");
    expect(button.on?.["press"].params).toEqual({ status: "done" });
    expect(button.on?.["press"].confirm).toBeDefined();
  });

  it("has nothing to render without a tree", () => {
    expect(toSpec(null)).toBeNull();
    expect(toSpec({ view: {}, records: [] })).toBeNull();
  });
});

describe("actionsOf", () => {
  // Go keeps actions on the nodes that offer them; the page looks for them on
  // the view, keyed by the label `views_execute-action` resolves them by.
  it("collects every button in the tree, keyed by label", () => {
    expect(
      actionsOf({
        component: "Stack",
        children: [
          { component: "Button", actions: [{ label: "Archive", command: "tasks_delete" }] },
          {
            component: "Box",
            children: [{ component: "Button", actions: [{ label: "Reopen", command: "tasks_set-status" }] }],
          },
        ],
      }),
    ).toEqual({
      Archive: { description: "tasks_delete" },
      Reopen: { description: "tasks_set-status" },
    });
  });

  it("answers nothing for a tree with no buttons", () => {
    expect(actionsOf({ component: "Text" })).toEqual({});
    expect(actionsOf(undefined)).toEqual({});
  });
});

describe("formatViewValue", () => {
  // A bound value reached Text and Badge raw: a boolean rendered as an empty
  // badge, a list as its items run together ("mathpoet"), a date as ISO.
  it("gives every JSON type a readable form", () => {
    expect(formatViewValue(true)).toBe("Yes");
    expect(formatViewValue(false)).toBe("No");
    expect(formatViewValue(["math", "poet"])).toBe("math, poet");
    expect(formatViewValue(null)).toBe("");
    expect(formatViewValue(undefined)).toBe("");
    expect(formatViewValue("plain")).toBe("plain");
    expect(formatViewValue({ a: 1 })).toBe('{"a":1}');
    expect(formatViewValue(1234.5)).toMatch(/1.?234/);
  });

  it("shows a midnight UTC date as the calendar day it names, not the day before", () => {
    const shown = formatViewValue("1815-12-10T00:00:00Z");
    expect(shown).not.toContain("T00:00");
    expect(shown).toMatch(/10/);
    expect(shown).toMatch(/1815/);
  });
});
