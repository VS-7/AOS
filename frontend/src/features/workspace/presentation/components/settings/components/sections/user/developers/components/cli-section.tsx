import {
  ComputerTerminalIcon,
  Copy01Icon,
  Plug01Icon,
  Rocket01Icon,
  StartUp01Icon,
} from "@hugeicons/core-free-icons";
import { HugeiconsIcon, type IconSvgElement } from "@hugeicons/react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  FormSection,
  FormSectionContent,
  FormSectionHeader,
  FormSectionItem,
  FormSectionTitle,
} from "@/components/ui/form-section";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";

// No npm distribution (see docs/08 - Entrega/Build e Cross-Compile.md's "Sem
// distribuição via npm" decision) — the original's Node shim existed only to
// ship binaries through a JS package manager, which a native Go binary
// doesn't need. `go install` is the one install path that needs no download
// host of its own.
const CLI_INSTALL_COMMAND = "go install github.com/OWNER/aos/cmd/aos@latest";
const CLI_VERSION_LABEL = "AOS CLI, installed with Go.";

const CLI_CAPABILITIES: {
  icon: IconSvgElement;
  title: string;
  command: string;
  description: string;
}[] = [
  {
    icon: Plug01Icon,
    title: "MCP Server",
    command: "aos --mcp",
    description:
      "Connect AOS to your agent or coding tool as an MCP server.",
  },
  {
    icon: Rocket01Icon,
    title: "Gateway",
    command: "aos gateway start",
    description: "Start the local AOS gateway from your terminal.",
  },
  {
    icon: StartUp01Icon,
    title: "Onboarding",
    command: "aos onboarding",
    description: "Run the AOS onboarding flow to set up a workspace.",
  },
];

/**
 * Copies text to the clipboard and shows toast feedback.
 *
 * @param text - Text to copy.
 * @param successMessage - Toast shown on success.
 */
async function copyText(text: string, successMessage: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(text);
    toast.success(successMessage);
  } catch {
    toast.error(t("Failed to copy"));
  }
}

/**
 * Copyable command pill matched to Button `size="sm"` (`h-7` + same radius/padding).
 */
function CliCommandInput({
  command,
  className,
  onCopy,
}: {
  command: string;
  className?: string;
  onCopy?: () => void;
}) {
  return (
    <div
      className={cn(
        // Mirror Button size="sm": h-7, radius, px-2.5, text-[0.8rem]
        "box-border inline-flex h-7 min-h-7 max-h-7 shrink-0 items-center gap-1.5 rounded-[min(var(--radius-md),12px)] border border-border bg-background px-2.5 font-mono text-[0.8rem] leading-none text-foreground",
        className,
      )}
    >
      {onCopy ? (
        <button
          type="button"
          className="inline-flex size-3.5 shrink-0 items-center justify-center text-muted-foreground hover:text-foreground"
          onClick={onCopy}
          aria-label={t("Copy command")}
        >
          <HugeiconsIcon icon={Copy01Icon} className="size-3.5" />
        </button>
      ) : null}
      <span className="truncate leading-none">{command}</span>
    </div>
  );
}

/**
 * Stylized dual-terminal preview used in the CLI hero row.
 */
function CliHeroPreview() {
  return (
    <div className="relative size-[132px] shrink-0">
      <div className="absolute left-0 top-3 size-[102px] rotate-[-6deg] overflow-hidden rounded-lg border border-border bg-background shadow-sm">
        <div className="flex items-center gap-1.5 border-b border-border bg-muted/60 px-2.5 py-1.5">
          <span className="size-2 rounded-md bg-muted-foreground/40" />
          <span className="size-2 rounded-md bg-muted-foreground/40" />
          <span className="size-2 rounded-md bg-muted-foreground/40" />
        </div>
        <div className="space-y-1.5 p-3">
          <div className="h-2 w-[60px] rounded bg-muted-foreground/25" />
          <div className="h-2 w-[84px] rounded bg-muted-foreground/15" />
          <div className="h-2 w-12 rounded bg-muted-foreground/20" />
        </div>
      </div>
      <div className="absolute bottom-0 right-0 size-[102px] rotate-[4deg] overflow-hidden rounded-lg border border-border bg-background shadow-md">
        <div className="flex items-center gap-1.5 border-b border-border bg-muted/60 px-2.5 py-1.5">
          <span className="size-2 rounded-md bg-muted-foreground/40" />
          <span className="size-2 rounded-md bg-muted-foreground/40" />
          <span className="size-2 rounded-md bg-muted-foreground/40" />
        </div>
        <div className="flex flex-col gap-1.5 p-3">
          <div className="flex items-center gap-1.5">
            <HugeiconsIcon
              icon={ComputerTerminalIcon}
              className="size-3.5 text-muted-foreground"
            />
            <div className="h-2 w-[72px] rounded bg-muted-foreground/30" />
          </div>
          <div className="h-2 w-[60px] rounded bg-muted-foreground/15" />
          <div className="h-2 w-[84px] rounded bg-muted-foreground/20" />
        </div>
      </div>
    </div>
  );
}

/**
 * CLI install hero + capability rows for the Developers settings page.
 */
export function DevelopersCliSection() {
  return (
    <FormSection>
      <FormSectionHeader>
        <FormSectionTitle>CLI</FormSectionTitle>
      </FormSectionHeader>

      <FormSectionContent className="divide-y-0">
        <div className="flex flex-col gap-4 p-4 sm:flex-row sm:items-center">
          <CliHeroPreview />

          <div className="flex min-w-0 flex-1 flex-col gap-3">
            <p className="text-sm text-muted-foreground">
              {t("Browse and run AOS from the command line. Works directly with Codex, Claude Code, and more.")}
            </p>
            <p className="text-sm font-semibold text-foreground">
              {CLI_VERSION_LABEL}
            </p>
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
              <CliCommandInput
                command={CLI_INSTALL_COMMAND}
                className="min-w-0 w-full flex-1 sm:w-auto"
                onCopy={() =>
                  void copyText(CLI_INSTALL_COMMAND, "Install command copied")
                }
              />
              <Button
                type="button"
                size="sm"
                className="h-7 shrink-0"
                onClick={() =>
                  void copyText(CLI_INSTALL_COMMAND, "Install command copied")
                }
              >
                {t("Install")}
              </Button>
            </div>
          </div>
        </div>
      </FormSectionContent>

      <FormSectionContent className="mt-3">
        {CLI_CAPABILITIES.map((capability) => (
          <FormSectionItem key={capability.command}>
            <div className="flex min-w-0 flex-1 items-center gap-3">
              <div className="flex size-9 shrink-0 items-center justify-center rounded-lg border border-border bg-background">
                <HugeiconsIcon
                  icon={capability.icon}
                  className="size-4 text-muted-foreground"
                />
              </div>
              <div className="min-w-0">
                <p className="text-sm font-medium text-foreground">
                  {capability.title}
                </p>
                <p className="text-sm text-muted-foreground">
                  {capability.description}
                </p>
              </div>
            </div>
            <CliCommandInput
              command={capability.command}
              className="w-fit max-w-full"
              onCopy={() =>
                void copyText(capability.command, "Command copied")
              }
            />
          </FormSectionItem>
        ))}
      </FormSectionContent>
    </FormSection>
  );
}
