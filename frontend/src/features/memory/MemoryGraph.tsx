import { useMemo, useState } from "react";
import type { JSX } from "react";
import { useQuery } from "@tanstack/react-query";
import { client } from "@/lib/client";
import { Failure } from "@/features/chat/ChatScreen";

interface GraphNode {
  id: string;
  title: string;
  category: string;
  confidence: number;
  status: string;
  links?: string[];
  supersedes?: Array<{ id: string; reason: string }>;
}

interface GraphOutput {
  nodes?: GraphNode[];
  memories?: GraphNode[];
  hubs?: string[];
  isolated?: string[];
  averageConfidence?: number;
}

/** Where a node sits, after the layout has settled. */
interface Placed extends GraphNode {
  x: number;
  y: number;
}

const SIZE = 720;

/**
 * The cognitive graph.
 *
 * Force-directed, laid out here rather than by a library: the layout is two
 * forces and a loop, and a dependency would cost more to keep than to replace.
 * Colour comes from the category token, which is why the graph stays readable
 * when somebody switches from Dracula to Solarized Light.
 */
export function MemoryGraph({ agent }: { agent: string }): JSX.Element {
  const [selected, setSelected] = useState<string | null>(null);

  const graph = useQuery({
    queryKey: ["memory", agent],
    queryFn: async () =>
      (await client.invoke("memories_graph", {
        agent,
        _reasoning: "the memory graph is open",
      })) as GraphOutput,
  });

  const nodes = useMemo(() => graph.data?.nodes ?? graph.data?.memories ?? [], [graph.data]);
  const placed = useMemo(() => layout(nodes), [nodes]);
  const byID = useMemo(() => new Map(placed.map((n) => [n.id, n])), [placed]);

  if (graph.isLoading) return <p className="empty">Reading the graph…</p>;
  if (graph.error) return <Failure error={graph.error} />;
  if (placed.length === 0) {
    return <p className="empty">This agent has not formed a memory yet.</p>;
  }

  const chosen = selected ? byID.get(selected) : undefined;
  const categories = [...new Set(placed.map((n) => n.category))].sort();

  return (
    <div className="graph">
      <svg viewBox={`0 0 ${SIZE} ${SIZE}`} role="img" aria-label="Memory graph">
        {placed.flatMap((node) =>
          [
            ...(node.links ?? []).map((to) => ({ to, kind: "link" as const })),
            ...(node.supersedes ?? []).map((s) => ({ to: s.id, kind: "supersedes" as const })),
          ].flatMap(({ to, kind }) => {
            const other = byID.get(to);
            if (!other) return [];
            return [
              <line
                key={`${node.id}-${to}-${kind}`}
                className="edge"
                data-kind={kind}
                x1={node.x}
                y1={node.y}
                x2={other.x}
                y2={other.y}
              />,
            ];
          }),
        )}

        {placed.map((node) => (
          <circle
            key={node.id}
            className="node"
            tabIndex={0}
            role="button"
            aria-label={`${node.title} — ${node.category}`}
            cx={node.x}
            cy={node.y}
            // Confidence is the radius, so an honest low number is visible as
            // a small node rather than hidden in a tooltip.
            r={6 + node.confidence * 8}
            fill={`var(--category-${node.category}, var(--primary))`}
            opacity={node.status === "active" ? 1 : 0.35}
            onClick={() => setSelected(node.id)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                setSelected(node.id);
              }
            }}
          />
        ))}
      </svg>

      <aside>
        <div className="legend">
          {categories.map((category) => (
            <span key={category}>
              <i style={{ background: `var(--category-${category}, var(--primary))` }} />
              {category}
            </span>
          ))}
        </div>

        {chosen ? (
          <div className="card">
            <h4>{chosen.title}</h4>
            <p className="meta">
              {chosen.category} · confidence {chosen.confidence.toFixed(2)} · {chosen.status}
            </p>
            {chosen.supersedes && chosen.supersedes.length > 0 && (
              <>
                <p className="meta">Replaces</p>
                {chosen.supersedes.map((s) => (
                  <p key={s.id} className="meta">
                    {byID.get(s.id)?.title ?? s.id} — {s.reason}
                  </p>
                ))}
              </>
            )}
          </div>
        ) : (
          <p className="empty">Pick a node to read it.</p>
        )}

        {graph.data?.isolated && graph.data.isolated.length > 0 && (
          <div className="card">
            <h4>Islands</h4>
            <p className="meta">
              {graph.data.isolated.length} memories link to nothing. An unlinked trace is the one
              nobody finds again.
            </p>
          </div>
        )}
      </aside>
    </div>
  );
}

/**
 * A small force-directed layout: repulsion between every pair, attraction along
 * every edge, run for a fixed number of steps.
 *
 * Fixed steps rather than an animation loop, and a seeded starting circle rather
 * than random positions: the same graph should look the same twice, or a person
 * cannot learn its shape.
 */
function layout(nodes: GraphNode[]): Placed[] {
  const centre = SIZE / 2;
  const placed: Placed[] = nodes.map((node, index) => {
    const angle = (index / Math.max(nodes.length, 1)) * Math.PI * 2;
    return { ...node, x: centre + Math.cos(angle) * 220, y: centre + Math.sin(angle) * 220 };
  });
  if (placed.length < 2) return placed;

  const index = new Map(placed.map((n, i) => [n.id, i]));
  const edges: Array<[number, number]> = [];
  for (const node of placed) {
    const from = index.get(node.id);
    if (from === undefined) continue;
    for (const to of [...(node.links ?? []), ...(node.supersedes ?? []).map((s) => s.id)]) {
      const target = index.get(to);
      if (target !== undefined) edges.push([from, target]);
    }
  }

  for (let step = 0; step < 220; step++) {
    const fx = new Array<number>(placed.length).fill(0);
    const fy = new Array<number>(placed.length).fill(0);

    for (let i = 0; i < placed.length; i++) {
      for (let j = i + 1; j < placed.length; j++) {
        const a = placed[i]!;
        const b = placed[j]!;
        let dx = a.x - b.x;
        let dy = a.y - b.y;
        let distance = Math.hypot(dx, dy);
        if (distance < 0.01) {
          // Two nodes exactly on top of each other have no direction to push
          // apart in. Nudging beats dividing by zero.
          dx = (i % 2 === 0 ? 1 : -1) * 0.5;
          dy = 0.5;
          distance = 0.7;
        }
        const force = 9000 / (distance * distance);
        fx[i]! += (dx / distance) * force;
        fy[i]! += (dy / distance) * force;
        fx[j]! -= (dx / distance) * force;
        fy[j]! -= (dy / distance) * force;
      }
    }

    for (const [from, to] of edges) {
      const a = placed[from]!;
      const b = placed[to]!;
      const dx = b.x - a.x;
      const dy = b.y - a.y;
      const distance = Math.max(Math.hypot(dx, dy), 0.01);
      const force = (distance - 120) * 0.02;
      fx[from]! += (dx / distance) * force;
      fy[from]! += (dy / distance) * force;
      fx[to]! -= (dx / distance) * force;
      fy[to]! -= (dy / distance) * force;
    }

    const cooling = 1 - step / 260;
    for (let i = 0; i < placed.length; i++) {
      const node = placed[i]!;
      // Pulled gently to the middle, so a disconnected node drifts to the edge
      // of the frame rather than out of it.
      node.x += (fx[i]! + (centre - node.x) * 0.01) * 0.5 * cooling;
      node.y += (fy[i]! + (centre - node.y) * 0.01) * 0.5 * cooling;
      node.x = Math.min(SIZE - 20, Math.max(20, node.x));
      node.y = Math.min(SIZE - 20, Math.max(20, node.y));
    }
  }
  return placed;
}
