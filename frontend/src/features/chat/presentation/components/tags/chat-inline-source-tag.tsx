import { FileTextIcon } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * An inline citation chip: what was consulted, and the kind of thing it was.
 *
 * Minimal — ported ahead of the rest of the chat feature; purely
 * presentational, so there is nothing feature-specific left to wire in later.
 */
export function ChatInlineSourceTag({
  name,
  path,
  sourceType,
  className,
}: {
  name: string;
  path: string;
  sourceType: string;
  className?: string;
}) {
  return (
    <span
      title={path || undefined}
      data-source-type={sourceType}
      className={cn(
        "inline-flex items-center gap-1 rounded-md border border-border bg-muted px-1.5 py-0.5 text-xs text-muted-foreground",
        className,
      )}
    >
      <FileTextIcon className="size-3" />
      {name}
    </span>
  );
}
