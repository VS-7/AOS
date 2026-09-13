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
  MarketplacePluginDetail,
  MarketplaceSkillComponentItem,
  MarketplaceSkillListing,
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
import { PluginInventoryItemSheet } from "@/features/marketplace/presentation/components/inventory/plugin-inventory-item-sheet.component";
import { MarketplaceUnavailable } from "@/features/marketplace/presentation/components/marketplace-unavailable.component";
import { MARKETPLACE_INVENTORY_ORDER } from "@/features/marketplace/presentation/consts/marketplace";
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
import { errorMessage } from "@/lib/aos-facade";
import { useTranslation } from "@/lib/i18n";

interface MarketplaceDetailsPageInnerProps {
  detail: MarketplacePluginDetail | null;
  loadError: { code: string; message: string } | null;
  related: MarketplaceSkillListing[];
  installedNames: string[];
}

/** The permission lists a manifest declares, in the order a reader weighs them. */
const PERMISSION_KEYS = ["network", "exec", "toolsets", "hooks", "agents", "collections", "routines"] as const;

function permissionLabel(key: (typeof PERMISSION_KEYS)[number], t: (key: string) => string): string {
  switch (key) {
    case "network":
      return t("Network");
    case "exec":
      return t("Programs");
    case "toolsets":
      return t("Toolsets");
    case "hooks":
      return t("Hooks");
    case "agents":
      return t("Agents");
    case "collections":
      return t("Collections");
    case "routines":
      return t("Routines");
  }
}

function permissionValues(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.map((entry) =>
      typeof entry === "string"
        ? entry
        : entry && typeof entry === "object"
          ? String((entry as { command?: string; baseUrl?: string; type?: string }).command ??
              (entry as { baseUrl?: string }).baseUrl ??
              (entry as { type?: string }).type ??
              "")
          : String(entry),
    ).filter(Boolean);
  }
  if (typeof value === "number" && value > 0) return [String(value)];
  return [];
}

export function MarketplaceDetailsPageInner({
  detail,
  loadError,
  related,
  installedNames,
}: MarketplaceDetailsPageInnerProps) {
  const { t } = useTranslation();
  const router = useRouter();
  const navigate = useNavigate();
  const [selectedItem, setSelectedItem] =
    React.useState<MarketplaceSkillComponentItem | null>(null);
  const [sheetOpen, setSheetOpen] = React.useState(false);

  const installedNameSet = React.useMemo(
    () => new Set(installedNames),
    [installedNames],
  );

  const installedSkill = detail?.installedSkill;
  const isActive = installedSkill?.active !== false;

  // `mutateOrThrow`-backed hooks reach onError on a refusal; the toasts say
  // what the daemon said, under a sentence of our own.
  const { mutate: updatePlugin, loading: isUpdating } =
    aos.client.skill.update.useMutation({
      onSuccess: async () => {
        toast.success(isActive ? t("Plugin disabled") : t("Plugin enabled"));
        await router.invalidate();
      },
      onError: (error: unknown) => {
        toast.error(t("Failed to update plugin"), { description: errorMessage(error) });
      },
    });

  const { mutate: deletePlugin, loading: isDeleting } =
    aos.client.skill.delete.useMutation({
      onSuccess: async () => {
        toast.success(t("Plugin uninstalled"));
        await navigate({ to: "/marketplace" });
      },
      onError: (error: unknown) => {
        toast.error(t("Failed to uninstall plugin"), { description: errorMessage(error) });
      },
    });

  function handleToolsetClick(item: MarketplaceSkillComponentItem) {
    if (item.kind !== "toolsets") return;
    setSelectedItem(item);
    setSheetOpen(true);
  }

  const backLink = (
    <Link
      to="/marketplace"
      className="inline-flex items-center gap-1.5 text-[13px] text-muted-foreground transition-colors hover:text-foreground"
    >
      <HugeiconsIcon icon={ArrowLeft01Icon} className="size-4" />
      {t("Back")}
    </Link>
  );

  if (!detail) {
    return (
      <MarketplaceShell rail={<MarketplaceRail>{null}</MarketplaceRail>}>
        {backLink}
        <MarketplaceUnavailable code={loadError?.code ?? ""} message={loadError?.message ?? ""} />
      </MarketplaceShell>
    );
  }

  const { plugin, inventory, isInstalled } = detail;
  const permissions = PERMISSION_KEYS.map((key) => ({
    key,
    values: permissionValues(plugin.permissions?.[key]),
  })).filter((entry) => entry.values.length > 0);

  return (
    <MarketplaceShell
      rail={
        <MarketplaceRail>
          <div className="flex flex-col gap-5 text-[13px] leading-5">
            <nav
              aria-label={t("Breadcrumb")}
              className="flex flex-wrap items-center gap-2 text-muted-foreground"
            >
              <Link
                to="/marketplace"
                className="transition-colors hover:text-foreground"
              >
                {t("Marketplace")}
              </Link>
              <span aria-hidden>/</span>
              <span className="text-foreground">{t(plugin.category)}</span>
            </nav>

            <div className="flex flex-col gap-2 text-muted-foreground">
              {plugin.author ? (
                <p>
                  {t("Created by")}{" "}
                  <span className="font-medium text-foreground">{plugin.author}</span>
                </p>
              ) : null}
              {plugin.version ? (
                <p>
                  {t("Version")}{" "}
                  <span className="font-medium text-foreground">{plugin.version}</span>
                </p>
              ) : null}
              {plugin.source ? (
                <p className="break-all">
                  {t("Source")}{" "}
                  <span className="font-mono text-[12px] text-foreground">{plugin.source}</span>
                </p>
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
                  {t("SKILL.md")}
                  <HugeiconsIcon icon={ArrowUpRight01Icon} className="size-3.5" />
                </button>
              ) : null}
            </div>
          </div>
        </MarketplaceRail>
      }
    >
      {backLink}

      <div className="flex flex-col gap-6 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-4">
          <PluginLogo
            listing={{
              name: plugin.name,
              displayName: plugin.displayName,
              logo: null,
              brandColor: null,
            }}
            size={56}
          />
          <div className="min-w-0">
            <h1 className="text-3xl font-medium tracking-tight text-foreground md:text-[2rem] md:leading-none">
              {plugin.displayName}
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
                {isActive ? t("Enabled") : t("Disabled")}
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
                  {t("Uninstall")}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent size="sm">
                <AlertDialogHeader>
                  <AlertDialogTitle>{t("Uninstall this plugin?")}</AlertDialogTitle>
                  <AlertDialogDescription>
                    {t("Removes {{name}} and its local files from this workspace.", { name: plugin.displayName })}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t("Cancel")}</AlertDialogCancel>
                  <AlertDialogAction
                    variant="destructive"
                    onClick={() =>
                      deletePlugin({ params: { skill: installedSkill.id } })
                    }
                  >
                    {t("Uninstall")}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        ) : plugin.source ? (
          <MarketplaceInstallButton
            source={plugin.source}
            registry={plugin.registry}
            pluginName={plugin.displayName}
            className="h-10 shrink-0 rounded-md px-5 text-[13px] font-medium"
          />
        ) : null}
      </div>

      {plugin.description ? (
        <p className="max-w-3xl text-[15px] leading-7 text-muted-foreground">
          {plugin.description}
        </p>
      ) : null}

      {/* What installing it would allow, before any of it is fetched
          (ADR-0015) — the one thing a listing can say about its contents. */}
      {!isInstalled && permissions.length > 0 ? (
        <section className="flex flex-col gap-3">
          <h2 className="text-lg font-medium tracking-tight text-foreground">{t("Permissions it asks for")}</h2>
          <dl className="grid gap-2 text-[13px]">
            {permissions.map((entry) => (
              <div key={entry.key} className="flex flex-wrap gap-2">
                <dt className="w-24 shrink-0 text-muted-foreground">{permissionLabel(entry.key, t)}</dt>
                <dd className="font-mono text-[12px] text-foreground">{entry.values.join(", ")}</dd>
              </div>
            ))}
          </dl>
        </section>
      ) : null}

      {isInstalled ? (
        <div className="flex flex-col gap-8">
          {MARKETPLACE_INVENTORY_ORDER.map((kind) => (
            <PluginDetailSection
              key={kind}
              kind={kind}
              items={inventory[kind] ?? []}
              interactive
              onToolsetClick={handleToolsetClick}
            />
          ))}
        </div>
      ) : null}

      {related.length > 0 ? (
        <section className="flex flex-col gap-5 pt-4">
          <h2 className="text-xl font-medium tracking-tight text-foreground md:text-2xl">
            {t("Related plugins")}
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
