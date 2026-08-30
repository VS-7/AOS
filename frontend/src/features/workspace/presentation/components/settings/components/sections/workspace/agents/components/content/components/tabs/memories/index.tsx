import { Suspense, lazy, useEffect, useMemo, useRef, useState } from "react";
import { BrainCircuit } from "lucide-react";

/**
 * The 3D memory graph, loaded when this tab is actually rendering one.
 *
 * `react-force-graph-3d` brings all of three.js with it — 4.4 MB of source,
 * the single largest thing in the application and about a seventh of the
 * whole bundle. Imported at the top of this file, as it was, every one of
 * those bytes was downloaded, parsed and compiled during startup for a screen
 * behind Settings -> Agents -> a specific agent -> the Memories tab, which
 * most sessions never open. That is most of what made the window slow to
 * become interactive.
 *
 * It is loaded lazily and only below the "does this agent have any memories"
 * check, so an agent with an empty graph never pays for it either.
 */
const ForceGraph3D = lazy(() => import("react-force-graph-3d"));
import { AnimatedEmptyState } from "@/components/ui/animated-empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { aos } from "@/app/aos";
import type { Agent } from "@/features/agent/interfaces/agent.interfaces";
import type { MemoryGraph } from "@/features/memory/interfaces/memory.interfaces";
import { MemoryGraphHelper } from "./helpers/memory-graph.helper";
import { MemoryList } from "@/features/memory/presentation/components/memory-list";
import { Button } from "@/components/ui/button";
import { t } from "@/lib/i18n";

interface AgentMemoriesTabProps {
  agent: Agent;
}

export function AgentMemoriesTab({ agent }: AgentMemoriesTabProps) {
  // The list first, the graph second.
  //
  // The graph was the only thing this tab had, and the only memory surface the
  // desktop had at all — which meant an agent's knowledge was visible as dots
  // and unreadable as text. The graph answers "how is this connected"; the
  // list answers "what does it know", which is the question people actually
  // arrive with, so it is what opens.
  const [view, setView] = useState<"list" | "graph">("list");
  const [graph, setGraph] = useState<MemoryGraph>({ nodes: [], links: [] });
  const [isLoading, setIsLoading] = useState(false);
  const [hoverNodeId, setHoverNodeId] = useState<string | null>(null);
  const hostRef = useRef<HTMLDivElement | null>(null);
  const graphRef = useRef<any>(null);
  const [size, setSize] = useState({ width: 0, height: 0 });

  useEffect(() => {
    const element = hostRef.current;
    if (!element) return;

    const updateSize = () => {
      const nextWidth = Math.max(320, element.clientWidth);
      const nextHeight = Math.max(320, element.clientHeight);
      setSize({ width: nextWidth, height: nextHeight });
    };

    const observer = new ResizeObserver(() => {
      updateSize();
    });

    observer.observe(element);
    updateSize();

    return () => observer.disconnect();
  }, [graph.nodes.length]);

  useEffect(() => {
    let isMounted = true;
    setIsLoading(true);

    aos.client.memory.graph
      .query({ query: { agent: agent.id.toLowerCase() } })
      .then((response) => {
        if (!isMounted) return;
        // `command-map.ts`'s `memory.graph` entry: Go's `memories_graph`
        // answers a bare `{nodes, edges, health, counts}` (`internal/
        // domain/memory/schema.go`'s `Graph`), not the `{nodes, links}`
        // shape this 3D renderer wants — disclosed there, adapted here.
        // `Node.title`/`.category` become the renderer's `.label`/`.group`;
        // `Edge.From`/`.To` become `.source`/`.target`. There is no Go
        // equivalent of the renderer's per-node `.val` (relative size) —
        // `Node.confidence` (0..1) is the closest available signal, made
        // visible as the substitute rather than left at a silent default.
        const raw = response.data as
          | { nodes?: unknown[]; edges?: unknown[] }
          | null
          | undefined;
        const rawNodes = Array.isArray(raw?.nodes) ? raw.nodes : [];
        const rawEdges = Array.isArray(raw?.edges) ? raw.edges : [];

        setGraph({
          nodes: rawNodes.map((n: any) => ({
            id: n.id,
            label: n.title,
            category: n.category,
            status: n.status,
            group: n.category,
            val: typeof n.confidence === "number" ? n.confidence : undefined,
          })),
          links: rawEdges.map((e: any) => ({
            source: e.from,
            target: e.to,
            type: e.type,
          })),
        });
      })
      .catch((error) => {
        // No daemon call is unmapped here anymore (`memory.graph` is
        // registered above), but the facade can still reject on a network
        // failure — surface it instead of leaving the graph stuck loading.
        console.error("[AgentMemoriesTab] failed to load memory graph", error);
      })
      .finally(() => {
        if (!isMounted) return;
        setIsLoading(false);
      });

    return () => {
      isMounted = false;
    };
  }, [agent.id]);

  const hoveredNode = useMemo(
    () => graph.nodes.find((node) => node.id === hoverNodeId),
    [graph.nodes, hoverNodeId],
  );

  useEffect(() => {
    if (!graphRef.current || graph.nodes.length === 0 || size.width === 0) return;

    const id = window.setTimeout(() => {
      try {
        graphRef.current.d3ReheatSimulation();
        graphRef.current.zoomToFit(600, 48);
      } catch {
        // No-op: if renderer is still initializing, next render tick will retry naturally.
      }
    }, 120);

    return () => window.clearTimeout(id);
  }, [graph.nodes.length, size.width, size.height]);

  return (
    <div className="flex h-full w-full flex-col gap-3 p-3">
      <div className="flex items-center gap-1">
        <Button
          size="sm"
          variant={view === "list" ? "secondary" : "ghost"}
          onClick={() => setView("list")}
        >
          {t("List")}
        </Button>
        <Button
          size="sm"
          variant={view === "graph" ? "secondary" : "ghost"}
          onClick={() => setView("graph")}
        >
          {t("Graph")}
        </Button>
      </div>

      {view === "list" && (
        <div className="min-h-0 flex-1">
          <MemoryList agentId={agent.id.toLowerCase()} />
        </div>
      )}

      {view === "graph" && !isLoading && graph.nodes.length === 0 && (
        <AnimatedEmptyState className="border-none shadow-none py-12">
          <AnimatedEmptyState.Carousel>
            <div className="flex items-center gap-3">
              <div className="flex size-8 items-center justify-center rounded-md bg-muted/50">
                <BrainCircuit className="size-4 text-muted-foreground" />
              </div>
              <div className="flex flex-col gap-0.5">
                <div className="h-2 w-24 rounded-md bg-muted" />
                <div className="h-2 w-16 rounded-md bg-muted/50" />
              </div>
            </div>
          </AnimatedEmptyState.Carousel>
          <AnimatedEmptyState.Content>
            <AnimatedEmptyState.Title>{t("No memory graph yet")}</AnimatedEmptyState.Title>
            <AnimatedEmptyState.Description>
              {t("This agent doesn't have indexed memory nodes yet.")}
            </AnimatedEmptyState.Description>
          </AnimatedEmptyState.Content>
        </AnimatedEmptyState>
      )}

      {view === "graph" && !isLoading && graph.nodes.length > 0 && (
        <div ref={hostRef} className="h-full w-full min-h-130 overflow-hidden">
          {size.width > 0 && size.height > 0 && (
            <Suspense fallback={<Skeleton className="h-full w-full" />}>
            <ForceGraph3D
              ref={graphRef}
              width={size.width}
              height={size.height}
              graphData={graph}
              nodeLabel={(node: any) => `${node.label} (${node.category})`}
              nodeColor={(node: any) => MemoryGraphHelper.nodeColor(node)}
              nodeVal={(node: any) => MemoryGraphHelper.nodeSize(node)}
              linkColor={(link: any) => MemoryGraphHelper.linkColor(link)}
              linkWidth={(link: any) => MemoryGraphHelper.linkWidth(link)}
              linkOpacity={0.75}
              backgroundColor="rgba(0,0,0,0)"
              onNodeHover={(node: any) => setHoverNodeId(node?.id ?? null)}
              enableNodeDrag
              enablePointerInteraction
              showNavInfo={false}
              warmupTicks={60}
              cooldownTicks={120}
            />
            </Suspense>
          )}
        </div>
      )}
    </div>
  );
}
