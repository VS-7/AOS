import { cn } from "@/lib/utils";

/**
 * An inline rich-text fragment embedded inside a `<chat-inline-rich-text>`
 * tag. Minimal — renders the text as-is. What "rich" meant for this original
 * tag (bold/italic/code spans, or a nested render of the same markdown
 * pipeline) isn't verifiable from the reconstructed source alone; rather than
 * guess at formatting behaviour, this stays a plain, honest span until the
 * chat feature's own port pins down the real contract.
 */
export function ChatInlineRichText({
  text,
  className,
}: {
  text: string;
  className?: string;
  inlineTagClassName?: string;
}) {
  return <span className={cn("align-middle", className)}>{text}</span>;
}
