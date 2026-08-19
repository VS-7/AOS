import type {
  FractalMemoryGraphLink,
  FractalMemoryGraphNode,
} from "@/features/memory/interfaces/memory.interfaces";

export class MemoryGraphHelper {
  public static nodeColor(node: FractalMemoryGraphNode) {
    if (node.status !== "active") return "rgba(120, 120, 130, 0.85)";

    const palette: Record<string, string> = {
      preference: "#86efac",
      architecture: "#60a5fa",
      workflow: "#f59e0b",
      context: "#a78bfa",
      lesson: "#34d399",
      constraint: "#f87171",
      tooling: "#22d3ee",
      security: "#fb7185",
      reference: "#c4b5fd",
    };

    return palette[node.category] ?? "#94a3b8";
  }

  public static linkColor(link: FractalMemoryGraphLink) {
    return link.type === "supersedes" ? "rgba(248, 113, 113, 0.55)" : "rgba(148, 163, 184, 0.45)";
  }

  public static linkWidth(link: FractalMemoryGraphLink) {
    return link.type === "supersedes" ? 1.8 : 1;
  }

  public static nodeSize(node: FractalMemoryGraphNode) {
    return Math.max(4, (node.val ?? 1) * 7);
  }
}
