import type { JSX, ReactNode } from "react";
import { isCommandDormant, isDormant } from "@/lib/command-map";
import { t } from "@/lib/i18n";

/**
 * What a screen shows when its domain does not yet exist in Go.
 *
 * Without this, a dormant domain renders an empty screen — indistinguishable
 * from a bug to anyone who opens it. The panel makes the degradation honest:
 * it says the interface is ahead of the backend, which is true and is deliberate.
 *
 * M6 of the final review: this copy was the only Portuguese user-facing
 * string in an otherwise-English UI — and it's what all 14 dormant domains
 * show. Code comments across this port stay in whichever language they
 * already were; this is UI copy, and the UI is English, so it's translated.
 */
export function DormantDomain({ feature }: { feature: string }): JSX.Element {
  return (
    <div className="flex h-full min-h-64 w-full items-center justify-center p-8">
      <div className="max-w-md rounded-lg border border-dashed border-border bg-muted p-6 text-center">
        <p className="text-sm font-medium text-foreground">
          {t("Domain not available yet")}
        </p>
        <p className="mt-2 text-sm text-muted-foreground">
          {t("The")} <code className="font-mono">{feature}</code> {t("interface already exists, but the Go backend does not publish this domain yet. The screen lights up on its own once it is implemented.")}
        </p>
      </div>
    </div>
  );
}

export interface DormantGateProps {
  /** The domain label to show in the panel, and (absent `commands`) what decides gating. */
  feature: string;
  /**
   * C6 of the final review: gate on these specific `command-map.ts` paths
   * instead of the whole `feature` domain — for a section whose domain is
   * live but whose own dependencies are individually `null` (see
   * `isCommandDormant`'s own comment). Gated when every listed path is
   * dormant; a mix of live and dormant paths is not this section's
   * situation today and is treated as live (not gated) rather than guessed at.
   */
  commands?: string[];
  children: ReactNode;
}

/** Wraps a route or section: shows the panel if dormant, the content if not. */
export function DormantGate({ feature, commands, children }: DormantGateProps): JSX.Element {
  const dormant = commands ? commands.every(isCommandDormant) : isDormant(feature);
  if (dormant) return <DormantDomain feature={feature} />;
  return <>{children}</>;
}
