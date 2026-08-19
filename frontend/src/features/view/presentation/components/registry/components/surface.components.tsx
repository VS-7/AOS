import type { BaseComponentProps } from "@json-render/react";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";
import { resolveLucideIcon } from "../shared/resolve-lucide-icon";

type CardProps = {
  title?: string | null;
  description?: string | null;
  maxWidth?: "sm" | "md" | "lg" | "full" | null;
  centered?: boolean | null;
  className?: string | null;
  padding?: "none" | "sm" | "md" | null;
  density?: "default" | "compact" | null;
};

const MAX_WIDTH_CLASS: Record<string, string> = {
  sm: "max-w-sm",
  md: "max-w-md",
  lg: "max-w-lg",
  full: "max-w-full",
};

/**
 * Card container with optional title/description header.
 */
export function CardComponent({
  props,
  children,
}: BaseComponentProps<CardProps>) {
  const padding = props.padding ?? "md";
  const compact = props.density === "compact";
  const contentClass =
    padding === "none" ? "p-0" : padding === "sm" ? "px-4 py-3" : undefined;

  return (
    <Card
      className={cn(
        compact && "gap-3 py-4",
        props.maxWidth ? MAX_WIDTH_CLASS[props.maxWidth] : undefined,
        props.centered ? "mx-auto" : undefined,
        props.className,
      )}
    >
      {props.title || props.description ? (
        <CardHeader>
          {props.title ? <CardTitle>{props.title}</CardTitle> : null}
          {props.description ? (
            <CardDescription>{props.description}</CardDescription>
          ) : null}
        </CardHeader>
      ) : null}
      <CardContent className={contentClass}>{children}</CardContent>
    </Card>
  );
}

type ScrollAreaProps = {
  className?: string | null;
  orientation?: "vertical" | "horizontal" | "both" | null;
};

/**
 * Scrollable region for sidebars and long detail panes.
 */
export function ScrollAreaComponent({
  props,
  children,
}: BaseComponentProps<ScrollAreaProps>) {
  return (
    <ScrollArea className={cn("min-h-0", props.className)}>
      <div
        className={cn(
          props.orientation === "horizontal" && "flex w-max",
          props.orientation === "both" && "min-w-full",
        )}
      >
        {children}
      </div>
    </ScrollArea>
  );
}

type SeparatorProps = {
  orientation?: "horizontal" | "vertical" | null;
  className?: string | null;
};

/**
 * Visual separator line.
 */
export function SeparatorComponent({
  props,
}: BaseComponentProps<SeparatorProps>) {
  return (
    <Separator
      orientation={props.orientation ?? "horizontal"}
      className={cn(props.className)}
    />
  );
}

type DetailSectionProps = {
  title: string;
  value?: string | null;
  defaultOpen?: boolean | null;
  className?: string | null;
};

/**
 * Linear-style collapsible detail section (chevron + quiet title + border).
 */
export function DetailSectionComponent({
  props,
  children,
}: BaseComponentProps<DetailSectionProps>) {
  const value = props.value ?? props.title.toLowerCase().replace(/\s+/g, "-");
  const open = props.defaultOpen !== false;

  return (
    <Accordion
      type="multiple"
      defaultValue={open ? [value] : []}
      className={cn("w-full", props.className)}
    >
      <AccordionItem value={value} className="border-0">
        <AccordionTrigger
          className={cn(
            "flex-row-reverse justify-end gap-2 px-6 py-3 hover:bg-transparent",
            "text-[13px] font-medium tracking-tight text-muted-foreground data-[state=open]:text-foreground",
          )}
        >
          {props.title}
        </AccordionTrigger>
        <AccordionContent className="px-6 pb-5 pt-0">
          {children}
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  );
}

type ActivityItemProps = {
  icon?: string | null;
  tone?: "success" | "warning" | "danger" | "muted" | null;
  title: string;
  meta?: string | null;
  status?: string | null;
  variant?: "plain" | "pill" | null;
  className?: string | null;
};

const ACTIVITY_TONE: Record<
  NonNullable<ActivityItemProps["tone"]>,
  string
> = {
  success: "text-emerald-500",
  warning: "text-amber-500",
  danger: "text-red-500",
  muted: "text-muted-foreground",
};

/**
 * Quiet activity / check row — Codex pill: icon · title · trailing meta.
 */
export function ActivityItemComponent({
  props,
}: BaseComponentProps<ActivityItemProps>) {
  const Icon = props.icon
    ? resolveLucideIcon(props.icon, "Circle")
    : null;
  const tone = props.tone ?? "muted";
  const pill = (props.variant ?? "pill") === "pill";

  return (
    <div
      className={cn(
        "flex min-w-0 items-center gap-2.5 text-[13px]",
        pill
          ? "rounded-full border border-border/50 bg-muted/40 px-3.5 py-2"
          : "py-2",
        props.className,
      )}
    >
      {Icon ? (
        <span
          className={cn(
            "flex size-6 shrink-0 items-center justify-center rounded-full",
            pill && "bg-background/70",
          )}
        >
          <Icon
            className={cn("size-3.5", ACTIVITY_TONE[tone as keyof typeof ACTIVITY_TONE])}
            strokeWidth={1.75}
          />
        </span>
      ) : (
        <span className="size-1.5 shrink-0 rounded-full bg-border" />
      )}
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="truncate text-foreground/90">{props.title}</span>
        {props.meta ? (
          <span className="truncate text-[11px] text-muted-foreground">
            {props.meta}
          </span>
        ) : null}
      </div>
      {props.status ? (
        <span
          className={cn(
            "shrink-0 text-[12px] tabular-nums text-muted-foreground",
            tone !== "muted" && ACTIVITY_TONE[tone as keyof typeof ACTIVITY_TONE],
          )}
        >
          {props.status}
        </span>
      ) : null}
    </div>
  );
}

type ActivityListItem = {
  icon?: string | null;
  tone?: "success" | "warning" | "danger" | "muted" | null;
  title: string;
  meta?: string | null;
  status?: string | null;
};

type ActivityListProps = {
  items?: ActivityListItem[] | null;
  emptyText?: string | null;
  className?: string | null;
};

/**
 * Stack of Codex-style pill activity rows (test cases, checks, timeline).
 */
export function ActivityListComponent({
  props,
}: BaseComponentProps<ActivityListProps>) {
  const items = Array.isArray(props.items) ? props.items : [];

  if (items.length === 0) {
    return (
      <p className="px-1 py-2 text-[13px] text-muted-foreground">
        {props.emptyText ?? "Nothing here yet."}
      </p>
    );
  }

  return (
    <div className={cn("flex flex-col gap-1.5", props.className)}>
      {items.map((item: any, index: number) => {
        const tone = item.tone ?? "muted";
        const Icon = item.icon
          ? resolveLucideIcon(item.icon, "Circle")
          : null;
        return (
          <div
            key={`${item.title}-${index}`}
            className="flex min-w-0 items-center gap-2.5 rounded-full border border-border/50 bg-muted/40 px-3.5 py-2 text-[13px]"
          >
            {Icon ? (
              <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-background/70">
                <Icon
                  className={cn("size-3.5", ACTIVITY_TONE[tone as keyof typeof ACTIVITY_TONE])}
                  strokeWidth={1.75}
                />
              </span>
            ) : null}
            <span className="min-w-0 flex-1 truncate text-foreground/90">
              {item.title}
            </span>
            {item.status ? (
              <span className="shrink-0 text-[12px] tabular-nums text-muted-foreground">
                {item.status}
              </span>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}
