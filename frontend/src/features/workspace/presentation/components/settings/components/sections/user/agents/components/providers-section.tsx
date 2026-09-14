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
import { Badge } from "@/components/ui/badge";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { aos } from "@/app/aos";
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
  const auth = isSubscriptionAuth(provider) ? t("Subscription") : t("API key");
  if (provider.modelsError) return t("{{auth}} · could not read its models", { auth });
  if (!provider.modelsDiscovered) return auth;
  const count = provider.models.length;
  return count === 1
    ? t("{{auth}} · 1 model", { auth })
    : t("{{auth}} · {{count}} models", { auth, count });
}

const SLOT_NAMES: Record<string, () => string> = {
  default: () => t("Default"),
  subconscious: () => t("Subconscious"),
  realtime: () => t("Realtime"),
  voice: () => t("Voice"),
  image: () => t("Image"),
  video: () => t("Video"),
};

function ProviderLogo({ className, provider }: { className?: string, provider: ModelProvider }) {
  const src = useProviderLogo(provider);
  if (!src) return null;
  return (
    <img
      src={src}
      alt={t("{{provider}} logo", { provider: provider.name })}
      className={cn("size-5 shrink-0 rounded-md", className)}
    />
  );
}

interface ProvidersSectionProps {
  providers: ModelProvider[];
  /** The saved model slots, to say which ones a disconnect would clear. */
  models?: Record<string, { provider?: string } | undefined>;
  onRefresh?: () => void;
}

export function ProvidersSection({ providers, models, onRefresh }: ProvidersSectionProps) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [connectOpen, setConnectOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<ModelProvider | null>(null);
  const [disconnecting, setDisconnecting] = React.useState<ModelProvider | null>(null);
  const agents = aos.stores.agent.useState((state) => state.items);

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
      toast.success(t("{{provider}} disconnected.", { provider: provider.name }));
      onRefresh?.();
      router.invalidate();
    } catch (error) {
      console.error(error);
      toast.error(
        error instanceof Error ? error.message : t("Failed to disconnect provider."),
      );
    }
  };

  // What a disconnect takes with it, named before it happens: the slots it
  // clears, and the agents that name this provider and will stop answering.
  const disconnectImpact = React.useMemo(() => {
    if (!disconnecting) return { slots: [] as string[], agents: [] as string[] };
    const slots = Object.entries(models ?? {})
      .filter(([, slot]) => slot?.provider === disconnecting.id)
      .map(([name]) => SLOT_NAMES[name]?.() ?? name);
    const users = agents
      .filter((agent) => agent.provider === disconnecting.id)
      .map((agent) => agent.name || agent.id);
    return { slots, agents: users };
  }, [disconnecting, models, agents]);

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
            {provider.modelsError ? (
              // On the row, not only in a tooltip nobody hovers: the reason
              // is the part that says whether signing in again can help, and
              // the first action is what to do instead when it cannot.
              <div className="mt-1 space-y-0.5 text-xs leading-tight text-muted-foreground">
                {/* Whole, not clamped: the part that says why comes last,
                    after the path of the file that could not be renewed. */}
                <p className="wrap-anywhere">{provider.modelsError}</p>
                {provider.modelsErrorActions?.[0] ? (
                  <p className="break-words text-foreground/80">→ {provider.modelsErrorActions[0]}</p>
                ) : null}
              </div>
            ) : null}
          </div>
          <div className="flex items-center gap-2">
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label={t("Manage {{provider}}", { provider: provider.name })}
                >
                  <HugeiconsIcon
                    icon={MoreHorizontalIcon}
                    className="size-4"
                  />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" sideOffset={6} className="w-44">
                {provider.auth.mode === "api-key" ? (
                  // A login-file provider has nothing to edit: its
                  // credential is the other tool's file.
                  <>
                    <DropdownMenuItem onSelect={() => setEditing(provider)}>
                      <HugeiconsIcon icon={PencilEdit01Icon} className="size-4" />
                      <span>{t("Edit")}</span>
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                  </>
                ) : null}
                <DropdownMenuItem
                  variant="destructive"
                  onSelect={() => setDisconnecting(provider)}
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
                  <ConnectMenuItem
                    key={provider.id}
                    provider={provider}
                    onSelect={() => {
                      setConnectOpen(false);
                      setEditing(provider);
                    }}
                  />
                ))}
              </>
            )}

            {apiKeyAvailable.length > 0 && (
              <>
                {subscriptionAvailable.length > 0 && <DropdownMenuSeparator />}
                <DropdownMenuLabel className="px-2 py-1 text-[11px]">
                  {t("API key")}
                </DropdownMenuLabel>
                {apiKeyAvailable.map((provider) => (
                  <ConnectMenuItem
                    key={provider.id}
                    provider={provider}
                    onSelect={() => {
                      setConnectOpen(false);
                      setEditing(provider);
                    }}
                  />
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

      <AlertDialog
        open={disconnecting !== null}
        onOpenChange={(open) => {
          if (!open) setDisconnecting(null);
        }}
      >
        <AlertDialogContent size="sm">
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("Disconnect {{provider}}?", { provider: disconnecting?.name ?? "" })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("{{provider}} is removed from this installation's connected providers. Nothing else on this machine is touched.", {
                provider: disconnecting?.name ?? "",
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          {disconnectImpact.slots.length > 0 || disconnectImpact.agents.length > 0 ? (
            <ul className="space-y-1 text-sm text-muted-foreground">
              {disconnectImpact.slots.length > 0 ? (
                <li>
                  {t("Model slots cleared: {{slots}}", { slots: disconnectImpact.slots.join(", ") })}
                </li>
              ) : null}
              {disconnectImpact.agents.length > 0 ? (
                <li>
                  {disconnecting?.auth.mode === "api-key" && disconnecting.auth.required
                    ? t("Agents that use it and will stop answering: {{agents}}", {
                        agents: disconnectImpact.agents.join(", "),
                      })
                    : // A login-file or optional-key provider is still
                      // reachable without an entry, so "will stop
                      // answering" would not be true of it.
                      t("Agents that name it: {{agents}}", {
                        agents: disconnectImpact.agents.join(", "),
                      })}
                </li>
              ) : null}
            </ul>
          ) : null}
          <AlertDialogFooter>
            <AlertDialogCancel>{t("Cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={() => {
                const provider = disconnecting;
                setDisconnecting(null);
                if (provider) void handleDisconnect(provider);
              }}
            >
              {t("Disconnect")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

/**
 * One provider in the Connect menu, with the catalogue's own description.
 *
 * The menu listed names only, so "Gemini (CLI login)" was offered as if it
 * still worked while its catalogue entry said it was retired — text nothing
 * ever rendered.
 */
function ConnectMenuItem({ provider, onSelect }: { provider: ModelProvider; onSelect: () => void }) {
  return (
    <DropdownMenuItem
      onSelect={onSelect}
      disabled={provider.retired}
      className="cursor-pointer items-start"
    >
      <ProviderLogo className="mt-0.5 size-3.5" provider={provider} />
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="flex items-center gap-1.5">
          {provider.name}
          {provider.retired ? (
            <Badge variant="outline" className="h-4 px-1 text-[10px]">
              {t("Retired")}
            </Badge>
          ) : null}
        </span>
        <span className="line-clamp-2 text-[11px] leading-snug text-muted-foreground">
          {t(provider.description)}
        </span>
      </span>
      <HugeiconsIcon icon={ArrowRight01Icon} className="mt-0.5 size-3.5 opacity-50" />
    </DropdownMenuItem>
  );
}

export default ProvidersSection;
