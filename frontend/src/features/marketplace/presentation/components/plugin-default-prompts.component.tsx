import { ArrowRight01Icon } from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";

import { PluginLogo } from "@/features/marketplace/presentation/components/plugin-card.component";
import promptsBgLight from "@/public/marketplace-prompts-bg-light.jpg";
import promptsBgDark from "@/public/marketplace-prompts-bg-dark.jpg";
import { cn } from "@/lib/utils";

interface PluginDefaultPromptsProps {
  displayName: string;
  prompts: string[];
  logo?: string | null;
  brandColor?: string | null;
  pluginName: string;
  className?: string;
}

/**
 * Decorative example prompts banner for marketplace plugin detail pages.
 * Purely illustrative — no click handlers.
 */
export function PluginDefaultPrompts({
  displayName,
  prompts,
  logo,
  brandColor,
  pluginName,
  className,
}: PluginDefaultPromptsProps) {
  if (prompts.length === 0) return null;

  return (
    <div
      className={cn(
        "relative overflow-hidden rounded-2xl border border-border",
        className,
      )}
    >
      <img
        src={promptsBgLight}
        alt=""
        aria-hidden
        className="absolute inset-0 size-full object-cover dark:hidden"
      />
      <img
        src={promptsBgDark}
        alt=""
        aria-hidden
        className="absolute inset-0 hidden size-full object-cover dark:block"
      />
      <div className="absolute inset-0 bg-background/15 dark:bg-background/40" />

      <div className="relative flex flex-col items-center gap-2.5 px-5 py-8 sm:px-10 sm:py-10">
        {prompts.slice(0, 3).map((prompt) => (
          <div
            key={prompt}
            className="flex w-full max-w-xl items-center gap-2.5 rounded-md border border-border/60 bg-card/95 px-3 py-2 shadow-sm backdrop-blur-sm sm:gap-3 sm:px-4 sm:py-2.5"
          >
            <PluginLogo
              listing={{
                name: pluginName,
                displayName,
                logo: logo ?? null,
                brandColor: brandColor ?? null,
              }}
              size={28}
            />
            <p className="min-w-0 flex-1 truncate text-[13px] leading-5 sm:text-[14px]">
              <span
                className="mr-1.5 font-medium text-foreground"
                style={brandColor ? { color: brandColor } : undefined}
              >
                {displayName}
              </span>
              <span className="text-foreground/85">{prompt}</span>
            </p>
            <span className="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
              <HugeiconsIcon icon={ArrowRight01Icon} className="size-3.5" />
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
