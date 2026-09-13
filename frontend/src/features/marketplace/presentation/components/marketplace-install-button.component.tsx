import * as React from "react";
import { useRouter } from "@tanstack/react-router";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { aos } from "@/app/aos";
import { errorMessage } from "@/lib/aos-facade";
import { useTranslation } from "@/lib/i18n";

interface MarketplaceInstallButtonProps {
  /** The listing's "owner/repo" — what marketplace_install fetches. */
  source: string;
  /** The registry that listed it, so the fetch goes to that one. */
  registry?: string;
  pluginName: string;
  className?: string;
}

/**
 * Installs a registry listing through marketplace_install.
 *
 * It used to call skills_install with source "aos/registry" — not a package
 * anywhere, so it always failed — and toasted "installed successfully" from
 * onSuccess regardless. The install still stops for the permissions consent
 * the daemon asks for; the button waits with it.
 */
export function MarketplaceInstallButton({
  source,
  registry,
  pluginName,
  className,
}: MarketplaceInstallButtonProps) {
  const { t } = useTranslation();
  const router = useRouter();

  const { mutate: installPlugin, loading: isInstalling } =
    aos.client.marketplace.install.useMutation({
      onSuccess: async () => {
        toast.success(t("{{name}} installed.", { name: pluginName }));
        await router.invalidate();
      },
      onError: (error: unknown) => {
        toast.error(t("Failed to install plugin"), { description: errorMessage(error) });
      },
    });

  function handleInstall(event: React.MouseEvent) {
    event.preventDefault();
    if (isInstalling) return;
    installPlugin({ body: { source, registry: registry || undefined } });
  }

  return (
    <Button
      onClick={handleInstall}
      disabled={isInstalling}
      aria-busy={isInstalling}
      className={className}
    >
      {isInstalling ? (
        <>
          <Spinner />
          {t("Installing...")}
        </>
      ) : (
        t("Install on your Workspace")
      )}
    </Button>
  );
}
