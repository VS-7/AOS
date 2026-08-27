import type { BaseComponentProps } from "@json-render/react";
import MarkdownContent from "@/components/ui/markdown-content";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";

type HeadingProps = {
  text: string;
  level?: "h1" | "h2" | "h3" | "h4" | null;
  className?: string | null;
  truncate?: boolean | null;
};

const LEVEL_CLASS: Record<string, string> = {
  h1: "text-2xl font-semibold tracking-tight",
  h2: "text-xl font-semibold tracking-tight",
  h3: "text-lg font-semibold",
  h4: "text-base font-semibold",
};

/**
 * Heading text (h1–h4).
 */
export function HeadingComponent({
  props,
}: BaseComponentProps<HeadingProps>) {
  const level = props.level ?? "h2";
  const Tag = level;
  return (
    <Tag
      className={cn(
        LEVEL_CLASS[level],
        props.truncate ? "truncate" : undefined,
        props.className,
      )}
    >
      {props.text}
    </Tag>
  );
}

type TextProps = {
  text: string;
  variant?: "body" | "caption" | "muted" | "lead" | "code" | null;
  weight?: "normal" | "medium" | "semibold" | null;
  align?: "left" | "center" | "right" | null;
  truncate?: boolean | null;
  lines?: number | null;
  className?: string | null;
};

const VARIANT_CLASS: Record<string, string> = {
  body: "text-sm",
  caption: "text-xs text-muted-foreground",
  muted: "text-sm text-muted-foreground",
  lead: "text-base text-muted-foreground",
  code: "font-mono text-sm",
};

/**
 * Paragraph / inline text with AOS typography tokens.
 */
export function TextComponent({ props }: BaseComponentProps<TextProps>) {
  const variant = props.variant ?? "body";
  const weight =
    props.weight === "semibold"
      ? "font-semibold"
      : props.weight === "medium"
        ? "font-medium"
        : undefined;
  const align =
    props.align === "center"
      ? "text-center"
      : props.align === "right"
        ? "text-right"
        : undefined;

  return (
    <p
      className={cn(
        VARIANT_CLASS[variant],
        weight,
        align,
        props.truncate ? "truncate" : undefined,
        props.lines
          ? `line-clamp-${props.lines}`
          : undefined,
        "whitespace-pre-wrap",
        props.className,
      )}
    >
      {props.text}
    </p>
  );
}

type LinkProps = {
  label: string;
  href: string;
  external?: boolean | null;
  variant?: "default" | "muted" | null;
  className?: string | null;
};

/**
 * Anchor link with optional external target.
 */
export function LinkComponent({
  props,
  on,
}: BaseComponentProps<LinkProps>) {
  const press = on("press");

  return (
    <a
      href={props.href}
      target={props.external ? "_blank" : undefined}
      rel={props.external ? "noopener noreferrer" : undefined}
      className={cn(
        "text-sm underline-offset-4 hover:underline",
        props.variant === "muted"
          ? "text-muted-foreground"
          : "text-primary",
        props.className,
      )}
      onClick={(event) => {
        if (press.bound) {
          event.preventDefault();
          press.emit();
        }
      }}
    >
      {props.label}
    </a>
  );
}

type MarkdownContentProps = {
  content: string;
  isUserMessage?: boolean | null;
  className?: string | null;
};

/**
 * AOS markdown renderer — same component used in tasks, chat, and comments.
 */
export function MarkdownContentComponent({
  props,
}: BaseComponentProps<MarkdownContentProps>) {
  const content = props.content?.trim() ?? "";

  if (!content) {
    return (
      <p className="text-sm text-muted-foreground">{t("No content to display.")}</p>
    );
  }

  return (
    <div className={cn("min-w-0", props.className)}>
      <MarkdownContent
        content={content}
        isUserMessage={props.isUserMessage ?? false}
      />
    </div>
  );
}
