import * as React from "react";
import type { BaseComponentProps } from "@json-render/react";
import { MultiFileDiff } from "@pierre/diffs/react";
import type { FileContents } from "@pierre/diffs/react";
import { Badge } from "@/components/ui/badge";
import { Icon } from "@/components/ui/icon";
import { cn } from "@/lib/utils";
import { resolveLucideIcon } from "../shared/resolve-lucide-icon";

type IconProps = {
  name: string;
  size?: number | null;
  className?: string | null;
  fallback?: string | null;
};

/**
 * Dynamic Lucide icon by PascalCase name.
 */
export function IconComponent({ props }: BaseComponentProps<IconProps>) {
  return (
    <Icon
      value={props.name}
      fallback={props.fallback ?? "Circle"}
      size={props.size ?? 16}
      className={cn("shrink-0", props.className)}
    />
  );
}

type BadgeProps = {
  text: string;
  variant?: "default" | "secondary" | "destructive" | "outline" | null;
  size?: "sm" | "md" | "lg" | null;
  color?:
    | "gray"
    | "red"
    | "orange"
    | "amber"
    | "yellow"
    | "lime"
    | "green"
    | "emerald"
    | "teal"
    | "cyan"
    | "blue"
    | "indigo"
    | "violet"
    | "purple"
    | "fuchsia"
    | "pink"
    | "rose"
    | null;
  className?: string | null;
};

/**
 * AOS badge with extended color palette.
 */
export function BadgeComponent({ props }: BaseComponentProps<BadgeProps>) {
  return (
    <Badge
      variant={props.variant ?? "default"}
      size={props.size ?? "md"}
      color={props.color ?? "gray"}
      className={props.className ?? undefined}
    >
      {props.text}
    </Badge>
  );
}

type DiffStatsProps = {
  additions: number;
  deletions: number;
  size?: "sm" | "md" | null;
  className?: string | null;
};

/**
 * Presentational diff stats markup (shared by registry + sidebar item).
 */
export function DiffStatsInline({
  additions,
  deletions,
  size = "sm",
  className,
}: {
  additions: number;
  deletions: number;
  size?: "sm" | "md";
  className?: string;
}) {
  const textClass = size === "sm" ? "text-xs" : "text-sm";

  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center gap-2 font-mono tabular-nums",
        textClass,
        className,
      )}
    >
      {additions > 0 ? (
        <span className="text-emerald-600 dark:text-emerald-400">
          +{additions}
        </span>
      ) : null}
      {deletions > 0 ? (
        <span className="text-destructive">−{deletions}</span>
      ) : null}
      {additions === 0 && deletions === 0 ? (
        <span className="text-muted-foreground">—</span>
      ) : null}
    </span>
  );
}

/**
 * Colored +/− diff counter — matches Changes panel styling.
 */
export function DiffStatsComponent({
  props,
}: BaseComponentProps<DiffStatsProps>) {
  return (
    <DiffStatsInline
      additions={props.additions}
      deletions={props.deletions}
      size={props.size ?? "sm"}
      className={props.className ?? undefined}
    />
  );
}

type StatProps = {
  icon?: string | null;
  label: string;
  value: string;
  variant?: "inline" | "tile" | "strip" | "row" | null;
  className?: string | null;
};

/**
 * Metadata display — inline row, bordered tile, strip KPI, or Codex labeled row.
 */
export function StatComponent({ props }: BaseComponentProps<StatProps>) {
  const variant = props.variant ?? "inline";

  if (variant === "row") {
    return (
      <div
        className={cn(
          "flex min-w-0 items-center gap-3 py-1.5 text-[13px]",
          props.className,
        )}
      >
        {props.icon ? (
          <Icon
            value={props.icon}
            fallback="Circle"
            size={14}
            className="shrink-0 text-muted-foreground"
          />
        ) : (
          <span className="size-3.5 shrink-0" />
        )}
        <span className="w-[6.5rem] shrink-0 text-muted-foreground">
          {props.label}
        </span>
        <span className="min-w-0 flex-1 truncate text-foreground">
          {props.value}
        </span>
      </div>
    );
  }

  if (variant === "strip") {
    return (
      <div
        className={cn(
          "flex min-w-0 flex-col gap-1",
          props.className,
        )}
      >
        <span className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          {props.label}
        </span>
        <div className="flex min-w-0 items-center gap-1.5">
          {props.icon ? (
            <Icon
              value={props.icon}
              fallback="Circle"
              size={14}
              className="shrink-0 text-muted-foreground"
            />
          ) : null}
          <span className="truncate text-sm font-semibold tabular-nums tracking-tight text-foreground">
            {props.value}
          </span>
        </div>
      </div>
    );
  }

  if (variant === "tile") {
    return (
      <div
        className={cn(
          "flex min-w-0 flex-col gap-1.5 rounded-md border border-border/60 bg-muted/15 px-3 py-2.5",
          props.className,
        )}
      >
        <div className="flex items-center gap-1.5">
          {props.icon ? (
            <Icon
              value={props.icon}
              fallback="Circle"
              size={14}
              className="shrink-0 text-muted-foreground"
            />
          ) : null}
          <span className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
            {props.label}
          </span>
        </div>
        <span className="truncate text-sm font-semibold tabular-nums leading-none tracking-tight text-foreground">
          {props.value}
        </span>
      </div>
    );
  }

  return (
    <div
      className={cn(
        "flex min-w-0 items-center gap-2 text-sm text-muted-foreground",
        props.className,
      )}
    >
      {props.icon ? (
        <Icon
          value={props.icon}
          fallback="Circle"
          size={14}
          className="shrink-0 text-muted-foreground"
        />
      ) : null}
      <span className="shrink-0 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
        {props.label}
      </span>
      <span className="min-w-0 truncate text-foreground/90">{props.value}</span>
    </div>
  );
}

type DiffViewProps = {
  oldContent: string;
  newContent: string;
  fileName?: string | null;
  diffStyle?: "unified" | "split" | null;
  wordWrap?: boolean | null;
  className?: string | null;
};

function useThemeType(): "light" | "dark" {
  const [themeType, setThemeType] = React.useState<"light" | "dark">("light");

  React.useEffect(() => {
    const root = document.documentElement;
    const read = () =>
      setThemeType(root.classList.contains("dark") ? "dark" : "light");
    read();
    const observer = new MutationObserver(read);
    observer.observe(root, { attributes: true, attributeFilter: ["class"] });
    return () => observer.disconnect();
  }, []);

  return themeType;
}

/**
 * Full text diff viewer — same @pierre/diffs engine as the Changes panel.
 */
export function DiffViewComponent({ props }: BaseComponentProps<DiffViewProps>) {
  const themeType = useThemeType();
  const fileName = props.fileName ?? "diff.txt";

  const oldFile = React.useMemo<FileContents>(
    () => ({
      name: fileName,
      contents: props.oldContent,
      cacheKey: `old:${fileName}:${props.oldContent.length}`,
    }),
    [fileName, props.oldContent],
  );

  const newFile = React.useMemo<FileContents>(
    () => ({
      name: fileName,
      contents: props.newContent,
      cacheKey: `new:${fileName}:${props.newContent.length}`,
    }),
    [fileName, props.newContent],
  );

  return (
    <div
      className={cn(
        "min-h-0 w-full overflow-auto rounded-md border bg-background",
        props.className,
      )}
    >
      <MultiFileDiff
        oldFile={oldFile}
        newFile={newFile}
        options={{
          theme: { dark: "pierre-dark", light: "pierre-light" },
          themeType,
          diffStyle: props.diffStyle ?? "unified",
          overflow: props.wordWrap ? "wrap" : "scroll",
          disableFileHeader: true,
          stickyHeader: false,
        }}
      />
    </div>
  );
}
