"use client";

import * as React from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import type { ModelProvider } from "@/features/model/interfaces/model.interfaces";
import {
  MODEL_DISCOVERY_KEY,
  setModelProviderKey,
} from "@/features/model/services/model-provider.service";
import { useProviderLogo } from "../hooks/use-provider-logo";
import { t } from "@/lib/i18n";

interface ProviderUpsertDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  provider: ModelProvider;
  /** When provided, indicates an edit flow (the key will be pre-filled if known). */
  mode?: "create" | "edit";
  onSuccess?: () => void;
}

function ProviderLogoBadge({ provider }: { provider: ModelProvider }) {
  const src = useProviderLogo(provider);
  if (!src) return null;
  return (
    <img
      src={src}
      alt={t("{{provider}} logo", { provider: provider.name })}
      className="size-6 rounded-md"
    />
  );
}

/**
 * What the dialog tells a person about the credential.
 *
 * The catalogue's own guidance, translated. An API-key dialog used to replace
 * it with "Visit <name> to get your API key." — which told somebody connecting
 * opencode Zen to go and get a key that provider does not need.
 */
export function providerGuidance(provider: ModelProvider): string {
  const { auth } = provider;
  if (auth.mode === "oauth-file" && auth.login) {
    return t(auth.description, { tool: auth.login.tool, path: auth.login.path });
  }
  return t(auth.description);
}

export function ProviderUpsertDialog({
  open,
  onOpenChange,
  provider,
  mode = "create",
  onSuccess,
}: ProviderUpsertDialogProps) {
  const [value, setValue] = React.useState("");
  const [isSubmitting, setIsSubmitting] = React.useState(false);

  React.useEffect(() => {
    if (!open) {
      setValue("");
      setIsSubmitting(false);
    }
    // In edit mode, providers don't expose the existing key (it's masked).
    // The user enters a new key only if they want to rotate it.
  }, [open]);

  const takesKey = provider.auth.mode === "api-key";
  const requiresKey = takesKey && provider.auth.required;

  const queryClient = useQueryClient();

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (requiresKey && !value.trim()) {
      toast.error(t("API key is required."));
      return;
    }

    setIsSubmitting(true);
    try {
      const outcome = await setModelProviderKey(provider.id, takesKey ? value : "");
      // The catalogue is cached for five minutes on both sides, which is
      // right for a screen being re-rendered and wrong for the one moment a
      // person has just changed the credential the catalogue is read with.
      await queryClient.invalidateQueries({ queryKey: MODEL_DISCOVERY_KEY });

      if (outcome.error) {
        // Saved, but the provider did not accept it. Saying "connected."
        // here is how a refused key looked ready until the first chat.
        toast.warning(t("{{provider}} is saved, but it did not answer.", { provider: provider.name }), {
          description: [outcome.error, outcome.actions?.[0]].filter(Boolean).join("\n→ "),
          duration: 12000,
        });
      } else {
        toast.success(
          mode === "edit"
            ? t("{{provider}} updated.", { provider: provider.name })
            : t("{{provider}} connected.", { provider: provider.name }),
        );
      }
      onOpenChange(false);
      onSuccess?.();
    } catch (error) {
      console.error(error);
      toast.error(error instanceof Error ? error.message : t("Failed to save provider."));
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader className="gap-3">
          <div className="flex items-center gap-2">
            <ProviderLogoBadge provider={provider} />
            <DialogTitle>
              {mode === "edit"
                ? t("Edit {{provider}}", { provider: provider.name })
                : t("Connect {{provider}}", { provider: provider.name })}
            </DialogTitle>
          </div>
          <DialogDescription>{providerGuidance(provider)}</DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          {takesKey ? (
            <div className="space-y-2">
              <Label htmlFor={`provider-${provider.id}-key`}>{t("API key")}</Label>
              <Input
                id={`provider-${provider.id}-key`}
                autoFocus
                autoComplete="off"
                type={provider.auth.masked ? "password" : "text"}
                value={value}
                onChange={(e) => setValue(e.target.value)}
                placeholder={provider.auth.placeholder}
                required={requiresKey}
                disabled={isSubmitting}
              />
              {!provider.auth.required && (
                <p className="text-xs text-muted-foreground">
                  {t("Optional — leave empty to use the default discovery.")}
                </p>
              )}
            </div>
          ) : null}

          <DialogFooter className="gap-2 sm:gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isSubmitting}
            >
              {t("Cancel")}
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? t("Connecting…") : mode === "edit" ? t("Save") : t("Connect")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export default ProviderUpsertDialog;
