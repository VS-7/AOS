import * as React from "react";
import { ArrowDown01Icon, Tick01Icon } from "@hugeicons/core-free-icons";
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
import { isDesktop, system, type SkillTarget } from "@/lib/client";
import { t } from "@/lib/i18n";

/**
 * The agents the skill can go into, in menu order. The ids are the ones
 * `aos self skill install --to <id>` takes and SystemService.InstallSkill
 * resolves — pkg/skill's own list — so the desktop and the terminal name the
 * same things. The paths shown are the conventional ones; the desktop reads
 * the real, resolved ones from SkillTargets when it can.
 */
const SKILL_TARGETS: readonly {
  id: string;
  label: string;
  path: string | null;
  logo: { light: string; dark: string } | null;
}[] = [
  { id: "all", label: "All detected agents", path: null, logo: null },
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
    id: "codex",
    label: "Codex",
    path: "~/.codex/skills",
    logo: {
      light: "https://svgl.app/library/codex_light.svg",
      dark: "https://svgl.app/library/codex_dark.svg",
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
    id: "gemini",
    label: "Gemini CLI",
    path: "~/.gemini/skills",
    logo: {
      light: "https://svgl.app/library/gemini.svg",
      dark: "https://svgl.app/library/gemini.svg",
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
  {
    id: "agents",
    label: "Any agent (.agents convention)",
    path: "~/.agents/skills",
    logo: null,
  },
];

/** The terminal equivalent of a menu entry, for the browser and for copying. */
function installCommand(id: string): string {
  return id === "all" ? "aos self skill install --all" : `aos self skill install --to ${id}`;
}

/**
 * Installs the AOS skill into a coding agent's skills directory.
 *
 * Inside the desktop window the application writes the files itself — the
 * skill is compiled into it — and says where they went. In a browser tab
 * (the server flavour) nothing can reach the person's home directory, so
 * the menu copies the `aos self skill install` line that does the same from
 * a terminal.
 */
export function AddSkillDropdown() {
  const themeMode = useActiveThemeMode();
  const [targets, setTargets] = React.useState<Record<string, SkillTarget>>({});
  const [busy, setBusy] = React.useState<string | null>(null);

  const refresh = React.useCallback(async () => {
    if (!isDesktop()) return;
    try {
      const found = await system.skillTargets();
      setTargets(Object.fromEntries(found.map((target) => [target.id, target])));
    } catch {
      // Not the desktop after all, or the bridge is not up yet: the menu
      // falls back to the conventional paths and the copy behaviour.
    }
  }, []);

  const handleSelect = async (id: string, label: string) => {
    setBusy(id);
    try {
      if (isDesktop()) {
        const result = await system.installSkill(id);
        const installed = result.installed ?? [];
        const skipped = Object.entries(result.skipped ?? {});
        if (installed.length > 0) {
          toast.success(t("Skill installed"), { description: installed.join("\n") });
        }
        for (const [dir, reason] of skipped) {
          toast.warning(t("Skipped {{dir}}", { dir }), { description: reason });
        }
        await refresh();
        return;
      }
      await navigator.clipboard.writeText(installCommand(id));
      toast.success(t("Install command copied"), {
        description: t("Run it in a terminal where the aos command is installed."),
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      toast.error(t("Could not install the skill for {{agent}}", { agent: label }), {
        description: message,
      });
    } finally {
      setBusy(null);
    }
  };

  return (
    <DropdownMenu onOpenChange={(open) => open && void refresh()}>
      <DropdownMenuTrigger asChild>
        <Button type="button" variant="outline" size="sm" className="gap-1.5" disabled={busy !== null}>
          {t("Add skill to...")}
          <HugeiconsIcon icon={ArrowDown01Icon} className="size-3.5 opacity-60" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" sideOffset={6} className="w-80">
        {SKILL_TARGETS.map((target, index) => {
          const logoSrc = target.logo
            ? themeMode === "dark"
              ? target.logo.dark
              : target.logo.light
            : null;
          const known = targets[target.id];
          const path = known?.dir ?? target.path;

          return (
            <div key={target.id}>
              {index === 1 ? <DropdownMenuSeparator /> : null}
              <DropdownMenuItem
                className="gap-3 py-2"
                onSelect={() => void handleSelect(target.id, target.label)}
              >
                {logoSrc ? (
                  <img src={logoSrc} alt="" className="size-5 shrink-0 object-contain" />
                ) : (
                  <span className="size-5 shrink-0" aria-hidden />
                )}
                <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="text-sm font-medium text-foreground">{target.label}</span>
                  {path ? (
                    <span className="truncate font-mono text-xs text-muted-foreground">
                      {path}
                    </span>
                  ) : null}
                  {known && !known.present ? (
                    <span className="text-xs text-muted-foreground">
                      {t("Not detected on this machine")}
                    </span>
                  ) : null}
                </div>
                {known?.installed ? (
                  <HugeiconsIcon
                    icon={Tick01Icon}
                    className="size-3.5 shrink-0 text-muted-foreground"
                    aria-label={t("Installed")}
                  />
                ) : null}
              </DropdownMenuItem>
            </div>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
