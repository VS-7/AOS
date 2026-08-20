import * as React from "react";
import type { BaseComponentProps } from "@json-render/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { Badge } from "@/components/ui/badge";
import { useStateStore } from "@json-render/react";
import { cn } from "@/lib/utils";
import { useBoundValue } from "../shared/use-bound-value";
import { listJsonRenderChildren } from "../shared/unwrap-json-render-child";
import { resolveLucideIcon } from "../shared/resolve-lucide-icon";
import { DiffStatsInline } from "./data.components";

type ButtonProps = {
  label: string;
  variant?:
  | "default"
  | "outline"
  | "secondary"
  | "ghost"
  | "destructive"
  | "link"
  | "primary"
  | "danger"
  | null;
  size?:
  | "xs"
  | "sm"
  | "default"
  | "lg"
  | "icon"
  | "icon-sm"
  | "icon-lg"
  | null;
  disabled?: boolean | null;
  className?: string | null;
};

function mapButtonVariant(
  variant?: ButtonProps["variant"],
): "default" | "outline" | "secondary" | "ghost" | "destructive" | "link" {
  if (variant === "primary") return "default";
  if (variant === "danger") return "destructive";
  return variant ?? "default";
}

/**
 * AOS button with full shadcn variant support.
 */
export function ButtonComponent({
  props,
  emit,
}: BaseComponentProps<ButtonProps>) {
  return (
    <Button
      type="button"
      variant={mapButtonVariant(props.variant)}
      size={props.size ?? "default"}
      disabled={props.disabled ?? false}
      className={cn(props.className)}
      onClick={() => emit("press")}
    >
      {props.label}
    </Button>
  );
}

type SearchInputProps = {
  placeholder?: string | null;
  value?: string | null;
  name?: string | null;
  className?: string | null;
};

/**
 * Sidebar search input — uses SplitPageLayout.SearchInput styling.
 */
export function SearchInputComponent({
  props,
  bindings,
  emit,
}: BaseComponentProps<SearchInputProps>) {
  const [value, setValue] = useBoundValue(bindings, "value", props.value ?? "");

  return (
    <SplitPageLayout.SearchInput
      name={props.name ?? "search"}
      placeholder={props.placeholder ?? "Search…"}
      value={value}
      className={cn(props.className)}
      onChange={(event) => {
        setValue(event.target.value);
        emit("change");
      }}
    />
  );
}

type InputProps = {
  label: string;
  name: string;
  type?: "text" | "email" | "password" | "number" | null;
  placeholder?: string | null;
  value?: string | null;
  className?: string | null;
};

/**
 * Labeled text input with optional state binding.
 */
export function InputComponent({
  props,
  bindings,
}: BaseComponentProps<InputProps>) {
  const [value, setValue] = useBoundValue(bindings, "value", props.value ?? "");

  return (
    <div className={cn("grid w-full gap-2", props.className)}>
      <Label htmlFor={props.name}>{props.label}</Label>
      <Input
        id={props.name}
        name={props.name}
        type={props.type ?? "text"}
        placeholder={props.placeholder ?? undefined}
        value={value}
        onChange={(event) => setValue(event.target.value)}
      />
    </div>
  );
}

// ─── SplitPageLayout family ───────────────────────────────────────────────────

type SplitPageLayoutProps = {
  variant?: "default" | "stacked" | null;
  activeItemId?: string | null;
  className?: string | null;
};

type SplitPageSlotProps = {
  children?: React.ReactNode;
  className?: string | null;
};

/**
 * json-render registry wrappers are not `SplitPageLayout.Sidebar` / `.Content`,
 * so the slot-based root cannot find them. Re-mount children under real slots.
 */
function partitionSplitPageSlots(children: React.ReactNode): {
  sidebar: React.ReactElement<SplitPageSlotProps> | null;
  content: React.ReactElement<SplitPageSlotProps> | null;
} {
  const items = listJsonRenderChildren(children);
  const bySlotKey = (key: string) =>
    items.find(
      (item) =>
        (item.type as { slotKey?: string }).slotKey === key,
    ) ?? null;

  return {
    sidebar:
      (bySlotKey("split-page-layout.sidebar") as React.ReactElement<SplitPageSlotProps> | null) ??
      (items[0] as React.ReactElement<SplitPageSlotProps> | undefined) ??
      null,
    content:
      (bySlotKey("split-page-layout.content") as React.ReactElement<SplitPageSlotProps> | null) ??
      (items[1] as React.ReactElement<SplitPageSlotProps> | undefined) ??
      null,
  };
}

export function SplitPageLayoutComponent({
  props,
  children,
}: BaseComponentProps<SplitPageLayoutProps>) {
  const { sidebar, content } = React.useMemo(
    () => partitionSplitPageSlots(children),
    [children],
  );

  // json-render children are registry wrappers — render them whole (not props.children).
  return (
    <div
      className={cn(
        "flex h-full min-h-0 w-full flex-1 overflow-hidden",
        props.className,
      )}
    >
      {sidebar ? (
        <div className="h-full w-88 shrink-0 overflow-hidden border-r border-border">
          {sidebar}
        </div>
      ) : null}
      {content ? (
        <div className="min-h-0 min-w-0 flex-1 overflow-hidden">{content}</div>
      ) : null}
    </div>
  );
}

type SplitPageClassProps = { className?: string | null };

export function SplitPageSidebarComponent({
  props,
  children,
}: BaseComponentProps<SplitPageClassProps>) {
  return (
    <SplitPageLayout.Sidebar className={cn(props.className)}>
      {children}
    </SplitPageLayout.Sidebar>
  );
}

export function SplitPageSidebarHeaderComponent({
  props,
  children,
}: BaseComponentProps<SplitPageClassProps>) {
  return (
    <SplitPageLayout.SidebarHeader className={cn(props.className)}>
      {children}
    </SplitPageLayout.SidebarHeader>
  );
}

export function SplitPageSidebarContentComponent({
  props,
  children,
}: BaseComponentProps<SplitPageClassProps>) {
  return (
    <SplitPageLayout.SidebarContent className={cn(props.className, 'p-4')}>
      {children}
    </SplitPageLayout.SidebarContent>
  );
}

type SplitPageSidebarItemProps = {
  itemIndex: number;
  title: string;
  meta?: string | null;
  badge?: string | null;
  statusIcon?: string | null;
  statusTone?: "success" | "warning" | "danger" | "muted" | null;
  additions?: number | null;
  deletions?: number | null;
  className?: string | null;
};

const STATUS_TONE_CLASS: Record<
  NonNullable<SplitPageSidebarItemProps["statusTone"]>,
  string
> = {
  success: "text-emerald-500",
  warning: "text-amber-500",
  danger: "text-red-500",
  muted: "text-muted-foreground",
};

export function SplitPageSidebarItemComponent({
  props,
  emit,
}: BaseComponentProps<SplitPageSidebarItemProps>) {
  const { get } = useStateStore();
  const selected = get("/ui/selected");
  const search = String(get("/ui/search") ?? "").trim().toLowerCase();
  const isActive = String(selected) === String(props.itemIndex);
  const StatusIcon = props.statusIcon
    ? resolveLucideIcon(props.statusIcon, "Circle")
    : null;
  const toneClass = props.statusTone
    ? STATUS_TONE_CLASS[props.statusTone as keyof typeof STATUS_TONE_CLASS]
    : "text-muted-foreground";

  if (search) {
    const haystack = `${props.title ?? ""} ${props.meta ?? ""}`.toLowerCase();
    if (!haystack.includes(search)) return null;
  }

  const hasDiff = props.additions != null || props.deletions != null;

  return (
    <SplitPageLayout.SidebarItemCard
      isActive={isActive}
      className={cn(props.className)}
      onClick={() => emit("press")}
    >
      {StatusIcon ? (
        <StatusIcon
          className={cn("mt-0.5 size-3.5 shrink-0", toneClass)}
          strokeWidth={1.75}
        />
      ) : null}
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <div className="flex min-w-0 items-baseline gap-2">
          <span className="min-w-0 flex-1 truncate font-medium leading-snug text-foreground">
            {props.title}
          </span>
          {props.badge ? (
            <span className="shrink-0 text-[11px] tabular-nums text-muted-foreground">
              {props.badge}
            </span>
          ) : null}
        </div>
        {(props.meta || hasDiff) && (
          <div className="flex min-w-0 items-center gap-2">
            {props.meta ? (
              <span className="min-w-0 flex-1 truncate text-[11px] text-muted-foreground">
                {props.meta}
              </span>
            ) : (
              <span className="flex-1" />
            )}
            {hasDiff ? (
              <DiffStatsInline
                additions={props.additions ?? 0}
                deletions={props.deletions ?? 0}
                size="sm"
                className="shrink-0"
              />
            ) : null}
          </div>
        )}
      </div>
    </SplitPageLayout.SidebarItemCard>
  );
}

export function SplitPageContentComponent({
  props,
  children,
}: BaseComponentProps<SplitPageClassProps>) {
  return (
    <SplitPageLayout.Content className={cn(props.className)}>
      {children}
    </SplitPageLayout.Content>
  );
}

export function SplitPageContentHeaderComponent({
  props,
  children,
}: BaseComponentProps<SplitPageClassProps>) {
  return (
    <SplitPageLayout.ContentHeader className={cn(props.className)}>
      {children}
    </SplitPageLayout.ContentHeader>
  );
}

export function SplitPageContentBodyComponent({
  props,
  children,
}: BaseComponentProps<SplitPageClassProps>) {
  return (
    <SplitPageLayout.ContentBody className={cn(props.className)}>
      {children}
    </SplitPageLayout.ContentBody>
  );
}

// slotKey helps partitionSplitPageSlots when json-render child order differs.
(SplitPageSidebarComponent as { slotKey?: string }).slotKey =
  "split-page-layout.sidebar";
(SplitPageContentComponent as { slotKey?: string }).slotKey =
  "split-page-layout.content";
