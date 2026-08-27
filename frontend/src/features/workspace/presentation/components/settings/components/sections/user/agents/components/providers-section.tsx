"use client";

import * as React from "react";
import {
  ArrowRight01Icon,
  Cancel01Icon,
  MoreHorizontalIcon,
  PencilEdit01Icon,
  PlusSignIcon,
} from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";
import { useQueryClient } from "@tanstack/react-query";
import { useRouter } from "@tanstack/react-router";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import type { ModelProvider } from "@/features/model/interfaces/model.interfaces";
import {
  MODEL_DISCOVERY_KEY,
  disconnectModelProvider,
} from "@/features/model/services/model-provider.service";
import { ProviderUpsertDialog } from "./provider-upsert-dialog";
import { useProviderLogo } from "../hooks/use-provider-logo";
import { t } from "@/lib/i18n";

function isSubscriptionAuth(provider: ModelProvider) {
  return provider.auth.mode !== "api-key";
}

/**
 * What this connected provider is actually serving.
 *
 * The row used to say only how the provider authenticates, which is the one
 * thing about it that never changes. What a person needs to see here is
 * whether asking it worked: a count means the credential is good and the
 * catalogue is this account's, and the failure means the models listed in the
 * pickers are the static fallback rather than anything this provider said.
 * Without that line the two are indistinguishable, which is how a provider
 * that has been quietly refusing for a week goes unnoticed.
 */
function catalogueLabel(provider: ModelProvider): string {
  const auth = isSubscriptionAuth(provider) ? "Subscription" : "API Key";
  if (provider.modelsError) return `${auth} · could not read its models`;
  if (!provider.modelsDiscovered) return auth;
  const count = provider.models.length;
  return `${auth} · ${count} ${count === 1 ? "model" : "models"}`;
}

function ProviderLogo({ className, provider }: { className?: string, provider: ModelProvider }) {
  const src = useProviderLogo(provider);
  if (!src) return null;
  return (
    <img
      src={src}
      alt={`${provider.name} logo`}
      className={cn("size-5 shrink-0 rounded-md", className)}
    />
  );
}

interface ProvidersSectionProps {
  providers: ModelProvider[];
  onRefresh?: () => void;
}

export function ProvidersSection({ providers, onRefresh }: ProvidersSectionProps) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [connectOpen, setConnectOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<ModelProvider | null>(null);

  const connected = providers.filter((p) => p.configured);
  const available = providers.filter((p) => !p.configured);

  const subscriptionAvailable = available.filter(isSubscriptionAuth);
  const apiKeyAvailable = available.filter((p) => !isSubscriptionAuth(p));

  const handleDisconnect = async (provider: ModelProvider) => {
    try {
      await disconnectModelProvider(provider.id);
      // Same reason as connecting: the credential this catalogue was read
      // with is gone, so the catalogue is no longer this account's.
      await queryClient.invalidateQueries({ queryKey: MODEL_DISCOVERY_KEY });
      toast.success(`${provider.name} disconnected.`);
      onRefresh?.();
      router.invalidate();
    } catch (error) {
      console.error(error);
      toast.error(
        error instanceof Error ? error.message : "Failed to disconnect provider.",
      );
    }
  };

  return (
    <div className="rounded-xl border border-border bg-secondary/50 overflow-hidden divide-y divide-border">
      {connected.length === 0 && (
        <div className="flex items-center justify-between p-4 text-sm text-muted-foreground">
          <span>{t("No providers connected yet.")}</span>
        </div>
      )}

      {connected.map((provider) => (
        <div
          key={provider.id}
          className="flex items-center gap-3 p-4 min-h-16"
        >
          <ProviderLogo provider={provider} />
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium leading-tight">{provider.name}</p>
            <p
              className={cn(
                "text-xs leading-tight",
                provider.modelsError ? "text-destructive" : "text-muted-foreground",
              )}
              // The provider's own words, for whoever needs to act on them —
              // an expired key and a network that is down read identically
              // until you can see which one it was.
              title={provider.modelsError}
            >
              {catalogueLabel(provider)}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label={`Manage ${provider.name}`}
                >
                  <HugeiconsIcon
                    icon={MoreHorizontalIcon}
                    className="size-4"
                  />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" sideOffset={6} className="w-44">
                <DropdownMenuItem onSelect={() => setEditing(provider)}>
                  <HugeiconsIcon icon={PencilEdit01Icon} className="size-4" />
                  <span>{t("Edit")}</span>
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  variant="destructive"
                  onSelect={() => handleDisconnect(provider)}
                >
                  <HugeiconsIcon icon={Cancel01Icon} className="size-4" />
                  <span>{t("Disconnect")}</span>
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>
      ))}

      <div className="p-2">
        <DropdownMenu open={connectOpen} onOpenChange={setConnectOpen}>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="sm"
              className="w-full justify-start gap-2 text-muted-foreground"
            >
              <HugeiconsIcon icon={PlusSignIcon} className="size-4" />
              {t("Connect")}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" sideOffset={6} className="w-72 p-1">
            {subscriptionAvailable.length > 0 && (
              <>
                <DropdownMenuLabel className="px-2 py-1 text-[11px]">
                  {t("Subscription")}
                </DropdownMenuLabel>
                {subscriptionAvailable.map((provider) => (
                  <DropdownMenuItem
                    key={provider.id}
                    onSelect={() => {
                      setConnectOpen(false);
                      setEditing(provider);
                    }}
                    className="cursor-pointer"
                  >
                    <ProviderLogo className="size-3.5" provider={provider} />
                    <span className="flex-1">{provider.name}</span>
                    <HugeiconsIcon
                      icon={ArrowRight01Icon}
                      className="size-3.5 opacity-50"
                    />
                  </DropdownMenuItem>
                ))}
              </>
            )}

            {apiKeyAvailable.length > 0 && (
              <>
                {subscriptionAvailable.length > 0 && <DropdownMenuSeparator />}
                <DropdownMenuLabel className="px-2 py-1 text-[11px]">
                  API
                </DropdownMenuLabel>
                {apiKeyAvailable.map((provider) => (
                  <DropdownMenuItem
                    key={provider.id}
                    onSelect={() => {
                      setConnectOpen(false);
                      setEditing(provider);
                    }}
                    className="cursor-pointer"
                  >
                    <ProviderLogo className="size-3.5" provider={provider} />
                    <span className="flex-1">{provider.name}</span>
                    <HugeiconsIcon
                      icon={ArrowRight01Icon}
                      className="size-3.5 opacity-50"
                    />
                  </DropdownMenuItem>
                ))}
              </>
            )}

            {available.length === 0 && (
              <div className="px-2 py-3 text-center text-xs text-muted-foreground">
                {t("All providers are connected.")}
              </div>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <ProviderUpsertDialog
        open={editing !== null}
        onOpenChange={(open) => {
          if (!open) setEditing(null);
        }}
        provider={
          editing ?? {
            id: "",
            name: "",
            description: "",
            logo: { light: "", dark: "" },
            configured: false,
            default: false,
            auth: {
              mode: "api-key",
              connectionType: "external",
              label: "API Key",
              placeholder: "",
              description: "",
              required: true,
            },
            models: [],
          }
        }
        mode={editing?.configured ? "edit" : "create"}
        onSuccess={() => {
          onRefresh?.();
          router.invalidate();
        }}
      />
    </div>
  );
}

export default ProvidersSection;
