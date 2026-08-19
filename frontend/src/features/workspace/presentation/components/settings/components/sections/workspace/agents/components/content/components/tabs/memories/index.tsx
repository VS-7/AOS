import { useEffect, useMemo, useRef, useState } from "react";
import { BrainCircuit } from "lucide-react";
import ForceGraph3D from "react-force-graph-3d";
import { AnimatedEmptyState } from "@/components/ui/animated-empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { aos } from "@/app/aos";
import type { FractalAgent } from "@/features/agent/interfaces/agent.interfaces";
import type { FractalMemoryGraph } from "@/features/memory/interfaces/memory.interfaces";
import { MemoryGraphHelper } from "./helpers/memory-graph.helper";

interface AgentMemoriesTabProps {
  agent: FractalAgent;
}

export function AgentMemoriesTab({ agent }: AgentMemoriesTabProps) {
  const [graph, setGraph] = useState<FractalMemoryGraph>({ nodes: [], links: [] });
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
    <div className="h-full w-full p-3">
      {!isLoading && graph.nodes.length === 0 && (
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
            <AnimatedEmptyState.Title>No memory graph yet</AnimatedEmptyState.Title>
            <AnimatedEmptyState.Description>
              This agent doesn&apos;t have indexed memory nodes yet.
            </AnimatedEmptyState.Description>
          </AnimatedEmptyState.Content>
        </AnimatedEmptyState>
      )}

      {!isLoading && graph.nodes.length > 0 && (
        <div ref={hostRef} className="h-full w-full min-h-130 overflow-hidden">
          {size.width > 0 && size.height > 0 && (
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
          )}
        </div>
      )}
    </div>
  );
}
