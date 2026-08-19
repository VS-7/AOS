import { cn, timeAgoCompact } from "@/lib/utils";

interface ChatActivityStampProps {
  at?: string | Date | null;
  className?: string;
}

/**
 * Compact last-activity stamp for sidebar chat rows (`5m`, `1h`, `4d`).
 */
export function ChatActivityStamp({ at, className }: ChatActivityStampProps) {
  if (!at) {
    return null;
  }

  const label = timeAgoCompact(at);
  const absolute =
    at instanceof Date ? at.toLocaleString() : new Date(at).toLocaleString();

  return (
    <span
      title={absolute}
      className={cn(
        "shrink-0 rounded-md px-1 py-0.5 text-[10px] tabular-nums leading-none",
        "text-muted-foreground/55",
        "transition-colors duration-150",
        "group-hover/chat-row:bg-muted/55 group-hover/chat-row:text-muted-foreground",
        "group-data-[active=true]/chat-row:bg-muted/40 group-data-[active=true]/chat-row:text-muted-foreground/80",
        className,
      )}
    >
      {label}
    </span>
  );
}
