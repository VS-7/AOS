"use client";

import * as React from "react";
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
import type { ModelProvider, ModelProviderAuth } from "@/features/model/interfaces/model.interfaces";
import { setModelProviderKey } from "@/features/model/services/model-provider.service";
import { useProviderLogo } from "../hooks/use-provider-logo";

interface ProviderUpsertDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  provider: ModelProvider;
  /** When provided, indicates an edit flow (the key will be pre-filled if known). */
  mode?: "create" | "edit";
  onSuccess?: () => void;
}

function isOAuthLike(auth: ModelProviderAuth) {
  return auth.mode !== "api-key";
}

function ProviderLogoBadge({ provider }: { provider: ModelProvider }) {
  const src = useProviderLogo(provider);
  if (!src) return null;
  return (
    <img
      src={src}
      alt={`${provider.name} logo`}
      className="size-6 rounded-md"
    />
  );
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
      return;
    }
    // In edit mode, providers don't expose the existing key (it's masked).
    // The user enters a new key only if they want to rotate it.
  }, [open]);

  const requiresKey = provider.auth.required && provider.auth.mode === "api-key";
  const oauthLike = isOAuthLike(provider.auth);

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (requiresKey && !value.trim()) {
      toast.error("API key is required.");
      return;
    }

    setIsSubmitting(true);
    try {
      await setModelProviderKey(provider.id, value);

      toast.success(
        mode === "edit"
          ? `${provider.name} updated.`
          : `${provider.name} connected.`,
      );
      onOpenChange(false);
      onSuccess?.();
    } catch (error) {
      console.error(error);
      toast.error(
        error instanceof Error ? error.message : "Failed to save provider.",
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const description = oauthLike
    ? provider.auth.description
    : `Visit ${provider.name} to get your API key.`;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader className="gap-3">
          <div className="flex items-center gap-2">
            <ProviderLogoBadge provider={provider} />
            <DialogTitle>
              {mode === "edit" ? "Edit" : "Connect with"} {provider.name}
            </DialogTitle>
          </div>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor={`provider-${provider.id}-key`}>
              {provider.auth.label}
            </Label>
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
                Optional — leave empty to use the default discovery.
              </p>
            )}
          </div>

          <DialogFooter className="gap-2 sm:gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isSubmitting}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? "Submitting…" : "Submit"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export default ProviderUpsertDialog;
