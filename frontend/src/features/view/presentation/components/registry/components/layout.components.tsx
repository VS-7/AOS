import type { BaseComponentProps } from "@json-render/react";
import { cn } from "@/lib/utils";
import {
  ALIGN_CLASS,
  GAP_CLASS,
  JUSTIFY_CLASS,
  layoutClass,
  mapToken,
} from "../shared/cn-maps";

type StackProps = {
  direction?: "horizontal" | "vertical" | null;
  gap?: keyof typeof GAP_CLASS | null;
  align?: keyof typeof ALIGN_CLASS | null;
  justify?: keyof typeof JUSTIFY_CLASS | null;
  wrap?: boolean | null;
  className?: string | null;
};

/**
 * Flex stack layout — AOS registry implementation.
 */
export function StackComponent({
  props,
  children,
}: BaseComponentProps<StackProps>) {
  const direction = props.direction ?? "vertical";
  const isHorizontal = direction === "horizontal";

  return (
    <div
      className={layoutClass(
        [
          "flex min-w-0",
          isHorizontal ? "flex-row" : "flex-col",
          mapToken(GAP_CLASS, props.gap, GAP_CLASS.md),
          mapToken(ALIGN_CLASS, props.align, ALIGN_CLASS.stretch),
          mapToken(JUSTIFY_CLASS, props.justify, JUSTIFY_CLASS.start),
          props.wrap ? "flex-wrap" : undefined,
        ],
        props.className,
      )}
    >
      {children}
    </div>
  );
}

type GridProps = {
  columns?: number | null;
  gap?: keyof typeof GAP_CLASS | null;
  className?: string | null;
};

/**
 * CSS grid layout — AOS registry implementation.
 */
export function GridComponent({
  props,
  children,
}: BaseComponentProps<GridProps>) {
  const columns = props.columns ?? 1;

  return (
    <div
      className={layoutClass(
        [
          "grid min-w-0",
          mapToken(GAP_CLASS, props.gap, GAP_CLASS.md),
          columns === 1
            ? "grid-cols-1"
            : columns === 2
              ? "grid-cols-2"
              : columns === 3
                ? "grid-cols-3"
                : columns === 4
                  ? "grid-cols-4"
                  : columns === 5
                    ? "grid-cols-5"
                    : columns >= 6
                      ? "grid-cols-6"
                      : "grid-cols-1",
        ],
        props.className,
      )}
    >
      {children}
    </div>
  );
}

type BoxProps = {
  as?: "div" | "section" | "article" | "main" | null;
  className?: string | null;
};

/**
 * Generic container with full className control.
 */
export function BoxComponent({ props, children }: BaseComponentProps<BoxProps>) {
  const Tag = props.as ?? "div";
  return <Tag className={cn(props.className)}>{children}</Tag>;
}
