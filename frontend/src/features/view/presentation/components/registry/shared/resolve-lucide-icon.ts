import { icons, type LucideIcon } from "lucide-react";

/**
 * Resolves a Lucide icon component by PascalCase name.
 */
export function resolveLucideIcon(
  name?: string | null,
  fallback = "Circle",
): LucideIcon | null {
  if (!name) return null;
  const resolved =
    (icons as Record<string, LucideIcon | undefined>)[name] ??
    (icons as Record<string, LucideIcon | undefined>)[fallback];
  return resolved ?? null;
}
