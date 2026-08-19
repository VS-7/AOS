import * as React from "react";
import { useRouter } from "@tanstack/react-router";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { aos } from "@/app/aos";

interface MarketplaceInstallButtonProps {
  pluginName: string;
  isInstalled: boolean;
  className?: string;
}

export function MarketplaceInstallButton({
  pluginName,
  isInstalled,
  className,
}: MarketplaceInstallButtonProps) {
  const router = useRouter();

  const { mutate: installPlugin, loading: isInstalling } =
    aos.client.skill.install.useMutation({
      onSuccess: async () => {
        toast.success(`Plugin "${pluginName}" installed successfully!`);
        await router.invalidate();
      },
      onError: (error: unknown) => {
        const message =
          error instanceof Error ? error.message : "Failed to install plugin.";
        toast.error(message);
      },
    });

  function handleInstall(event: React.MouseEvent) {
    event.preventDefault();
    if (isInstalled || isInstalling) return;

    installPlugin({
      body: {
        source: "tryfractal/registry",
        skill: pluginName,
      },
    });
  }

  return (
    <Button
      onClick={handleInstall}
      disabled={isInstalled || isInstalling}
      className={className}
    >
      {isInstalling ? (
        <>
          <Spinner />
          Installing...
        </>
      ) : isInstalled ? (
        "Installed"
      ) : (
        "Install on your Workspace"
      )}
    </Button>
  );
}
