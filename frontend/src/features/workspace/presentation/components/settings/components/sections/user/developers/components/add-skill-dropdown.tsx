import { ArrowDown01Icon } from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useActiveThemeMode } from "@/features/theme/presentation/hooks/use-active-theme-mode";
import { t } from "@/lib/i18n";

/**
 * Skill install targets with SVGL logos (light/dark variants from svgl.app).
 */
const SKILL_TARGETS = [
  {
    id: "all",
    label: "All agents",
    path: null,
    logo: null,
  },
  {
    id: "codex",
    label: "Codex",
    path: "~/.agents/skills",
    logo: {
      light: "https://svgl.app/library/codex_light.svg",
      dark: "https://svgl.app/library/codex_dark.svg",
    },
  },
  {
    id: "claude-code",
    label: "Claude Code",
    path: "~/.claude/skills",
    logo: {
      light: "https://svgl.app/library/claude-ai-icon.svg",
      dark: "https://svgl.app/library/claude-ai-icon.svg",
    },
  },
  {
    id: "cursor",
    label: "Cursor",
    path: "~/.cursor/skills",
    logo: {
      light: "https://svgl.app/library/cursor_light.svg",
      dark: "https://svgl.app/library/cursor_dark.svg",
    },
  },
  {
    id: "opencode",
    label: "OpenCode",
    path: "~/.config/opencode/skills",
    logo: {
      light: "https://svgl.app/library/opencode.svg",
      dark: "https://svgl.app/library/opencode-dark.svg",
    },
  },
] as const;

/**
 * Stub dropdown for installing AOS skills into external agent skill directories.
 * Agent logos resolve from SVGL based on the active theme mode.
 */
export function AddSkillDropdown() {
  const themeMode = useActiveThemeMode();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button type="button" variant="outline" size="sm" className="gap-1.5">
          {t("Add skill to...")}
          <HugeiconsIcon
            icon={ArrowDown01Icon}
            className="size-3.5 opacity-60"
          />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" sideOffset={6} className="w-72">
        {SKILL_TARGETS.map((target, index) => {
          const logoSrc = target.logo
            ? themeMode === "dark"
              ? target.logo.dark
              : target.logo.light
            : null;

          return (
            <div key={target.id}>
              {index === 1 ? <DropdownMenuSeparator /> : null}
              <DropdownMenuItem
                className="gap-3 py-2"
                onSelect={() => {
                  toast.message(t("Coming soon"), {
                    description: `Skill install for ${target.label} will be available in a future update.`,
                  });
                }}
              >
                {logoSrc ? (
                  <img
                    src={logoSrc}
                    alt=""
                    className="size-5 shrink-0 object-contain"
                  />
                ) : (
                  <span className="size-5 shrink-0" aria-hidden />
                )}
                <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="text-sm font-medium text-foreground">
                    {target.label}
                  </span>
                  {target.path ? (
                    <span className="font-mono text-xs text-muted-foreground">
                      {target.path}
                    </span>
                  ) : null}
                </div>
              </DropdownMenuItem>
            </div>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
