/**
 * A composed view, in the shape the renderer reads.
 *
 * This is the fourth translation point, and the largest, because the two sides
 * describe a screen at different levels. Go stores a *nested* tree of catalog
 * components with data bindings left unresolved, and answers `views_render`
 * with that tree beside the records it selected — the composition and the data,
 * separately (`internal/domain/view.Rendered`). `@json-render` renders a *flat*
 * spec: `{root, elements, state}`, where every element is keyed, children are
 * named by key, and a prop is either a literal or a `{$state}` pointer into the
 * state model.
 *
 * Nothing translated one into the other, so `ViewDataHelper.getSpec` read a
 * `spec` field that never arrived and the whole screen rendered its "this view
 * has no renderable json-render spec" panel — for every view, always. The
 * renderer, the component registry and the action bridge were all finished and
 * correct; they were being handed the wrong shape.
 *
 * **The one rule about data.** Go's `bind` says "this prop comes from this
 * field of the source record", and a view has many records, so some part of
 * the tree has to appear once per record. That part is the lowest node that
 * contains every binding — the lowest common ancestor of the bound nodes.
 *
 * Not each bound leaf: repeating those interleaves the fields of every record
 * into one flat run, which is how a list of five deals renders as fifteen
 * unattributed lines. Not the whole tree either: a view whose sidebar is fixed
 * and whose content binds would render the sidebar once per record. The LCA is
 * the node that holds one record's worth of screen, which for the scaffold's
 * own composition is exactly the Stack it puts the field nodes in.
 *
 * Bound props become `{$state: "/records/<i>/data/<field>"}` rather than the
 * value itself, so that an action's `updates` — which arrive as JSON pointers
 * and are applied to the state — actually re-render what they changed.
 *
 * `Table` is the exception, and it is Go's exception rather than this file's:
 * it is the one catalog component that takes many records as a prop, and the
 * scaffold emits it with `columns` filled and `rows` deliberately empty
 * (`internal/domain/view/scaffold.go`). Empty rows is a placeholder for the
 * side holding the data, which is this one.
 */

import type { Spec } from "@json-render/core";

/** One record as `views_render` answers it (`collection.Record`). */
export interface RenderedRecord {
  id?: string;
  collection?: string;
  data?: Record<string, unknown>;
  content?: string;
  createdAt?: string;
  updatedAt?: string;
}

/** A button a node offers (`view.Action`). */
export interface RenderedAction {
  label?: string;
  command?: string;
  input?: Record<string, unknown>;
  confirm?: boolean;
}

/** One node of the composed tree (`view.Node`). */
export interface RenderedNode {
  component?: string;
  props?: Record<string, unknown>;
  bind?: Record<string, string>;
  children?: RenderedNode[];
  actions?: RenderedAction[];
}

/** What `views_render` answers (`view.Rendered`). */
export interface RenderedView {
  view?: {
    id?: string;
    name?: string;
    title?: string;
    description?: string;
    tree?: RenderedNode;
    [key: string]: unknown;
  };
  records?: RenderedRecord[];
  renderedAt?: string;
}

/** Whether this node reads anything from a record. */
function binds(node: RenderedNode): boolean {
  return Boolean(node.bind && Object.keys(node.bind).length > 0);
}

/** The path, as child indices from the root, of every node that binds. */
function bindingPaths(node: RenderedNode, at: number[] = []): number[][] {
  const found: number[][] = binds(node) ? [at] : [];
  (node.children ?? []).forEach((child, index) => {
    found.push(...bindingPaths(child, [...at, index]));
  });
  return found;
}

/**
 * The path of the node that repeats: the lowest one containing every binding.
 *
 * Null when nothing binds — the tree is a screen rather than a list, and
 * renders once.
 */
function repeatPath(tree: RenderedNode): number[] | null {
  const paths = bindingPaths(tree);
  if (paths.length === 0) return null;

  let common = paths[0]!;
  for (const path of paths.slice(1)) {
    let shared = 0;
    while (
      shared < common.length &&
      shared < path.length &&
      common[shared] === path[shared]
    ) {
      shared++;
    }
    common = common.slice(0, shared);
  }
  return common;
}

/**
 * The buttons of a whole tree, keyed by label.
 *
 * The page builds its handler map from `view.actions`, and Go keeps actions on
 * the nodes that offer them — which is the right place for them to live and the
 * wrong place for the page to look. Label is the key because `views_execute-
 * action` resolves an action by label too: an index would shift the moment the
 * tree is edited.
 */
export function actionsOf(
  node: RenderedNode | undefined,
): Record<string, { description?: string }> {
  if (!node) return {};
  const out: Record<string, { description?: string }> = {};
  const walk = (current: RenderedNode) => {
    for (const action of current.actions ?? []) {
      if (!action.label) continue;
      out[action.label] = { description: action.command };
    }
    for (const child of current.children ?? []) walk(child);
  };
  walk(node);
  return out;
}

/** The event bindings for one node's buttons. */
function eventsOf(node: RenderedNode): Record<string, unknown> | undefined {
  const bindings = (node.actions ?? [])
    .filter((action) => Boolean(action.label))
    .map((action) => ({
      action: action.label,
      ...(action.input ? { params: action.input } : {}),
      ...(action.confirm
        ? {
            confirm: {
              title: action.label,
              message: `Run ${action.command ?? action.label}?`,
            },
          }
        : {}),
    }));
  if (bindings.length === 0) return undefined;
  return { click: bindings.length === 1 ? bindings[0] : bindings };
}

/** The props of one node, with its bindings pointed at one record. */
function propsOf(
  node: RenderedNode,
  index: number | null,
): Record<string, unknown> {
  const props: Record<string, unknown> = { ...(node.props ?? {}) };
  if (!node.bind) return props;
  for (const [prop, field] of Object.entries(node.bind)) {
    // A bound prop outside any record — a tree that binds with nothing
    // selected — is dropped rather than pointed at a record that is not
    // there: `{$state: "/records/0/..."}` on an empty collection resolves to
    // undefined and renders as the string "undefined".
    if (index === null) continue;
    props[prop] = { $state: `/records/${index}/data/${field}` };
  }
  return props;
}

/** Fills a Table's rows from the records, projected onto its own columns. */
function tableProps(
  props: Record<string, unknown>,
  records: RenderedRecord[],
): Record<string, unknown> {
  const declared = Array.isArray(props["columns"])
    ? (props["columns"] as unknown[])
    : [];
  const columns = declared.map((column) =>
    typeof column === "string"
      ? column
      : String((column as { key?: unknown })?.key ?? ""),
  );
  const rows = records.map((record) => {
    const data = record.data ?? {};
    const row: Record<string, unknown> = { id: record.id };
    for (const column of columns) {
      if (column) row[column] = data[column];
    }
    return row;
  });
  return { ...props, rows };
}

/**
 * Turns what `views_render` answers into a spec the renderer can render.
 *
 * Returns null when there is no tree at all — a view whose composition is
 * missing is not a view with an empty screen, and the renderer's own empty
 * state says so better than a spec with no elements would.
 */
export function toSpec(rendered: RenderedView | null | undefined): Spec | null {
  const tree = rendered?.view?.tree;
  if (!tree?.component) return null;

  const records = rendered?.records ?? [];
  const elements: Record<string, unknown> = {};
  let next = 0;
  const key = () => `e${next++}`;

  const repeated = repeatPath(tree);

  // `index` is the record this subtree is rendering, or null above the repeat.
  // `at` is where this node sits, so the walk can recognise the repeat point.
  const emit = (
    node: RenderedNode,
    index: number | null,
    at: number[],
  ): string => {
    const id = key();
    const isTable = node.component === "Table";
    const base = propsOf(node, index);
    const element: Record<string, unknown> = {
      type: node.component ?? "Box",
      props: isTable ? tableProps(base, records) : base,
    };

    const on = eventsOf(node);
    if (on) element["on"] = on;

    const children = node.children ?? [];
    if (children.length > 0) {
      element["children"] = children.flatMap((child, position) => {
        const childAt = [...at, position];
        // The repeat point: this child is one record's worth of screen, so it
        // is emitted once per record, as siblings in its parent.
        if (repeated !== null && samePath(childAt, repeated)) {
          return records.map((_, record) => emit(child, record, childAt));
        }
        return [emit(child, index, childAt)];
      });
    }

    elements[id] = element;
    return id;
  };

  // Nothing binds, or the whole tree is one record's worth of screen.
  if (repeated === null) {
    const root = emit(tree, null, []);
    return { root, elements: elements as Spec["elements"], state: { records } };
  }

  if (repeated.length === 0) {
    // The root itself is the repeat point, so the copies need something to
    // hang from. A Stack is the catalog's plain vertical container — the same
    // one the scaffold reaches for.
    const copies = records.map((_, index) => emit(tree, index, []));
    const root = key();
    elements[root] = { type: "Stack", props: { gap: "md" }, children: copies };
    return { root, elements: elements as Spec["elements"], state: { records } };
  }

  const root = emit(tree, null, []);
  return { root, elements: elements as Spec["elements"], state: { records } };
}

/** Whether two child-index paths name the same node. */
function samePath(a: number[], b: number[]): boolean {
  return a.length === b.length && a.every((step, index) => step === b[index]);
}
