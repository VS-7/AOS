import type { JSX, ReactNode } from "react";
import { isDormant } from "@/lib/command-map";

/**
 * What a screen shows when its domain does not yet exist in Go.
 *
 * Without this, a dormant domain renders an empty screen — indistinguishable
 * from a bug to anyone who opens it. The panel makes the degradation honest:
 * it says the interface is ahead of the backend, which is true and is deliberate.
 */
export function DormantDomain({ feature }: { feature: string }): JSX.Element {
  return (
    <div className="flex h-full min-h-64 w-full items-center justify-center p-8">
      <div className="max-w-md rounded-lg border border-dashed border-border bg-muted p-6 text-center">
        <p className="text-sm font-medium text-foreground">
          Domínio ainda não disponível
        </p>
        <p className="mt-2 text-sm text-muted-foreground">
          A interface de <code className="font-mono">{feature}</code> já existe, mas o
          backend Go ainda não publica esse domínio. A tela acende sozinha quando ele for
          implementado.
        </p>
      </div>
    </div>
  );
}

/** Wraps a route: shows the panel if the domain is dormant, the content if not. */
export function DormantGate({ feature, children }: { feature: string; children: ReactNode }): JSX.Element {
  if (isDormant(feature)) return <DormantDomain feature={feature} />;
  return <>{children}</>;
}
