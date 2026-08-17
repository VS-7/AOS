import { useQuery } from "@tanstack/react-query";
import { AtSignIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { client } from "@/lib/client";

interface AgentSummary {
  id: string;
  name: string;
}

/**
 * An inline @mention chip, resolving an agent id to its display name.
 *
 * Minimal — ported ahead of the rest of the chat feature because
 * markdown-content.tsx (the shared renderer) needs it to render an inline
 * `<chat-mention>` tag. Fills in once the chat feature's own components land.
 */
export function ChatInlineMentionTag({ id, className }: { id: string; className?: string }) {
  const agents = useQuery({
    queryKey: ["agents"],
    queryFn: async () =>
      (await client.invoke("agents_list", { _reasoning: "resolving an inline mention" })) as {
        agents: AgentSummary[];
      },
  });

  const name = agents.data?.agents.find((a) => a.id === id)?.name ?? id;

  return (
    <span
      className={cn(
        "inline-flex items-center gap-0.5 rounded-md bg-accent px-1.5 py-0.5 text-xs font-medium text-accent-foreground",
        className,
      )}
    >
      <AtSignIcon className="size-3" />
      {name}
    </span>
  );
}
