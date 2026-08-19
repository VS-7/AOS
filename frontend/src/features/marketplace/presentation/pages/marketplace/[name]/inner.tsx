import * as React from "react";
import { Link, useNavigate, useRouter } from "@tanstack/react-router";
import {
  ArrowLeft01Icon,
  ArrowUpRight01Icon,
  Delete02Icon,
  EcoPowerIcon,
} from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";
import { toast } from "sonner";

import type {
  FractalMarketplaceInstalledSkill,
  FractalMarketplaceSkill,
  FractalMarketplaceSkillComponentItem,
  FractalMarketplaceSkillInventory,
  FractalMarketplaceSkillListing,
} from "@/features/marketplace/interfaces/marketplace.interfaces";
import { MarketplaceInstallButton } from "@/features/marketplace/presentation/components/marketplace-install-button.component";
import {
  MarketplaceRail,
  MarketplaceShell,
} from "@/features/marketplace/presentation/components/marketplace-shell.component";
import {
  PluginCard,
  PluginLogo,
} from "@/features/marketplace/presentation/components/plugin-card.component";
import { PluginDetailSection } from "@/features/marketplace/presentation/components/plugin-detail-section.component";
import { PluginDefaultPrompts } from "@/features/marketplace/presentation/components/plugin-default-prompts.component";
import { PluginInventoryItemSheet } from "@/features/marketplace/presentation/components/inventory/plugin-inventory-item-sheet.component";
import {
  MARKETPLACE_INVENTORY_FOLDERS,
  MARKETPLACE_INVENTORY_ORDER,
} from "@/features/marketplace/presentation/consts/marketplace";
import { buildGithubFolderUrl } from "@/features/marketplace/presentation/helpers/marketplace.helper";
import { openWorkspaceFileTab } from "@/features/file/presentation/helpers/open-file-tab.helper";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { aos } from "@/app/aos";

interface MarketplaceDetailsPageInnerProps {
  plugin: FractalMarketplaceSkill;
  inventory: FractalMarketplaceSkillInventory;
  sourceUrl: string;
  isInstalled: boolean;
  installedSkill?: FractalMarketplaceInstalledSkill;
  related: FractalMarketplaceSkillListing[];
  installedNames: string[];
}

export function MarketplaceDetailsPageInner({
  plugin,
  inventory,
  sourceUrl,
  isInstalled,
  installedSkill,
  related,
  installedNames,
}: MarketplaceDetailsPageInnerProps) {
  const router = useRouter();
  const navigate = useNavigate();
  const [selectedItem, setSelectedItem] =
    React.useState<FractalMarketplaceSkillComponentItem | null>(null);
  const [sheetOpen, setSheetOpen] = React.useState(false);

  const installedNameSet = React.useMemo(
    () => new Set(installedNames),
    [installedNames],
  );
  const description =
    plugin.interface.longDescription ??
    plugin.description ??
    plugin.interface.shortDescription;

  const isActive = installedSkill?.active !== false;

  const { mutate: updatePlugin, loading: isUpdating } =
    aos.client.skill.update.useMutation({
      onSuccess: async () => {
        toast.success(isActive ? "Plugin disabled" : "Plugin enabled");
        await router.invalidate();
      },
      onError: (error: unknown) => {
        toast.error(
          error instanceof Error ? error.message : "Failed to update plugin",
        );
      },
    });

  const { mutate: deletePlugin, loading: isDeleting } =
    aos.client.skill.delete.useMutation({
      onSuccess: async () => {
        toast.success("Plugin uninstalled");
        await navigate({ to: "/marketplace" });
      },
      onError: (error: unknown) => {
        toast.error(
          error instanceof Error ? error.message : "Failed to uninstall plugin",
        );
      },
    });

  function handleToolsetClick(item: FractalMarketplaceSkillComponentItem) {
    if (item.kind !== "toolsets") return;
    setSelectedItem(item);
    setSheetOpen(true);
  }

  return (
    <MarketplaceShell
      rail={
        <MarketplaceRail>
          <div className="flex flex-col gap-5 text-[13px] leading-5">
            <nav
              aria-label="Breadcrumb"
              className="flex flex-wrap items-center gap-2 text-muted-foreground"
            >
              <Link
                to="/marketplace"
                className="transition-colors hover:text-foreground"
              >
                Marketplace
              </Link>
              <span aria-hidden>/</span>
              <span className="text-foreground">
                {plugin.interface.category}
              </span>
            </nav>

            <div className="flex flex-col gap-2 text-muted-foreground">
              <p>
                Created by{" "}
                <span className="font-medium text-foreground">
                  {plugin.author.name}
                </span>
              </p>
              <p>
                Verified by{" "}
                <span className="font-medium text-foreground">Fractal</span>
              </p>
              {sourceUrl ? (
                <a
                  href={sourceUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 font-medium text-foreground transition-colors hover:text-foreground/80"
                >
                  View Source
                  <HugeiconsIcon icon={ArrowUpRight01Icon} className="size-3.5" />
                </a>
              ) : null}
              {isInstalled && installedSkill?.skillMdPath ? (
                <button
                  type="button"
                  onClick={() =>
                    openWorkspaceFileTab(installedSkill.skillMdPath, {
                      title: "SKILL.md",
                    })
                  }
                  className="inline-flex items-center gap-1 font-medium text-foreground transition-colors hover:text-foreground/80"
                >
                  SKILL.md
                  <HugeiconsIcon icon={ArrowUpRight01Icon} className="size-3.5" />
                </button>
              ) : null}
              {isInstalled && installedSkill?.manifestPath ? (
                <button
                  type="button"
                  onClick={() =>
                    openWorkspaceFileTab(installedSkill.manifestPath!, {
                      title: "manifest.json",
                    })
                  }
                  className="inline-flex items-center gap-1 font-medium text-foreground transition-colors hover:text-foreground/80"
                >
                  Manifest
                  <HugeiconsIcon icon={ArrowUpRight01Icon} className="size-3.5" />
                </button>
              ) : null}
            </div>
          </div>
        </MarketplaceRail>
      }
    >
      <Link
        to="/marketplace"
        className="inline-flex items-center gap-1.5 text-[13px] text-muted-foreground transition-colors hover:text-foreground"
      >
        <HugeiconsIcon icon={ArrowLeft01Icon} className="size-4" />
        Back
      </Link>

      <div className="flex flex-col gap-6 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-4">
          <PluginLogo
            listing={{
              name: plugin.name,
              displayName: plugin.interface.displayName,
              logo: plugin.interface.logo ?? null,
              brandColor: plugin.interface.brandColor ?? null,
            }}
            size={56}
          />
          <div className="min-w-0">
            <h1 className="text-3xl font-medium tracking-tight text-foreground md:text-[2rem] md:leading-none">
              {plugin.interface.displayName}
            </h1>
          </div>
        </div>

        {isInstalled && installedSkill ? (
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex items-center gap-2 rounded-md border border-border px-3 py-1.5">
              <HugeiconsIcon
                icon={EcoPowerIcon}
                className="size-3.5 text-muted-foreground"
              />
              <Label
                htmlFor="plugin-active"
                className="text-[12px] text-muted-foreground"
              >
                {isActive ? "Enabled" : "Disabled"}
              </Label>
              <Switch
                id="plugin-active"
                checked={isActive}
                disabled={isUpdating}
                onCheckedChange={(active) =>
                  updatePlugin({
                    params: { skill: installedSkill.id },
                    body: { active },
                  })
                }
              />
            </div>

            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button
                  variant="destructive"
                  size="sm"
                  className="rounded-md"
                  disabled={isDeleting}
                >
                  <HugeiconsIcon icon={Delete02Icon} className="size-3.5" />
                  Uninstall
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent size="sm">
                <AlertDialogHeader>
                  <AlertDialogTitle>Uninstall this plugin?</AlertDialogTitle>
                  <AlertDialogDescription>
                    Removes{" "}
                    <strong>{plugin.interface.displayName}</strong> and its
                    local files from this workspace.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancel</AlertDialogCancel>
                  <AlertDialogAction
                    variant="destructive"
                    onClick={() =>
                      deletePlugin({ params: { skill: installedSkill.id } })
                    }
                  >
                    Uninstall
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        ) : (
          <MarketplaceInstallButton
            pluginName={plugin.name}
            isInstalled={isInstalled}
            className="h-10 shrink-0 rounded-md px-5 text-[13px] font-medium"
          />
        )}
      </div>

      <div className="flex flex-col gap-6">
        {plugin.interface.defaultPrompt &&
        plugin.interface.defaultPrompt.length > 0 ? (
          <PluginDefaultPrompts
            pluginName={plugin.name}
            displayName={plugin.interface.displayName}
            prompts={plugin.interface.defaultPrompt}
            logo={plugin.interface.logo}
            brandColor={plugin.interface.brandColor}
          />
        ) : null}

        <p className="max-w-3xl text-[15px] leading-7 text-muted-foreground">
          {description}
        </p>
      </div>

      <div className="flex flex-col gap-8">
        {MARKETPLACE_INVENTORY_ORDER.map((kind) => (
          <PluginDetailSection
            key={kind}
            kind={kind}
            items={inventory[kind] ?? []}
            folderUrl={
              isInstalled
                ? undefined
                : buildGithubFolderUrl(
                    plugin.name,
                    MARKETPLACE_INVENTORY_FOLDERS[kind],
                  )
            }
            interactive={isInstalled}
            onToolsetClick={isInstalled ? handleToolsetClick : undefined}
          />
        ))}
      </div>

      {related.length > 0 ? (
        <section className="flex flex-col gap-5 pt-4">
          <h2 className="text-xl font-medium tracking-tight text-foreground md:text-2xl">
            Related plugins
          </h2>
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            {related.map((listing) => (
              <PluginCard
                key={listing.name}
                listing={listing}
                isInstalled={installedNameSet.has(listing.name)}
              />
            ))}
          </div>
        </section>
      ) : null}

      <PluginInventoryItemSheet
        item={selectedItem}
        open={sheetOpen}
        onOpenChange={setSheetOpen}
      />
    </MarketplaceShell>
  );
}
