import { cn } from "@/lib/utils";

/** Canonical border radius — matches AOS theme. */
export const RADIUS_MD = "rounded-md";

/** Gap tokens used by Stack / Grid registry components. */
export const GAP_CLASS: Record<string, string> = {
  none: "gap-0",
  sm: "gap-2",
  md: "gap-4",
  lg: "gap-6",
  xl: "gap-8",
};

/** Flex alignment tokens for Stack. */
export const ALIGN_CLASS: Record<string, string> = {
  start: "items-start",
  center: "items-center",
  end: "items-end",
  stretch: "items-stretch",
};

/** Flex justification tokens for Stack. */
export const JUSTIFY_CLASS: Record<string, string> = {
  start: "justify-start",
  center: "justify-center",
  end: "justify-end",
  between: "justify-between",
  around: "justify-around",
};

/**
 * Resolves a nullable class token map value with an optional fallback.
 */
export function mapToken(
  map: Record<string, string>,
  value: string | null | undefined,
  fallback: string,
): string {
  if (!value) return fallback;
  return map[value] ?? fallback;
}

/**
 * Merges registry className with computed layout classes.
 */
export function layoutClass(
  parts: Array<string | null | undefined | false>,
  className?: string | null,
): string {
  return cn(...parts, className ?? undefined);
}
