import * as React from "react";
import {
  BookOpen01Icon,
  Copy01Icon,
  Key01Icon,
  RefreshIcon,
} from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";
import { toast } from "sonner";

import { aos } from "@/app/aos";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { t } from "@/lib/i18n";
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
import {
  FormSection,
  FormSectionContent,
  FormSectionDescription,
  FormSectionHeader,
  FormSectionItem,
  FormSectionTitle,
} from "@/components/ui/form-section";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

// The reference a person can actually open. This linked
// http://localhost:5326/api/docs, which answers 404: the daemon mounts its
// docs only outside production, behind a bearer an external browser does not
// have, and not necessarily on that port. The terminal prints the same surface
// — every command, its documentation and its input schema — wherever the
// daemon runs.
const API_REFERENCE_COMMAND = "aos self llms";

interface DevelopersRestApiSectionProps {
  /** Full API token when revealed after regenerate. */
  apiToken: string | null;
  /** Masked token preview for display. */
  maskedToken: string | null;
  /** Whether the account has an API token configured. */
  hasToken: boolean;
  /** Called after generate/regenerate with the full token, once. */
  onTokenRevealed: (fullToken: string) => void;
}

/**
 * REST API token management and documentation for Developers settings.
 */
export function DevelopersRestApiSection({
  apiToken,
  maskedToken,
  hasToken,
  onTokenRevealed,
}: DevelopersRestApiSectionProps) {
  const [isRegenerating, setIsRegenerating] = React.useState(false);

  const handleCopyToken = async () => {
    const value = apiToken ?? maskedToken;
    if (!value) {
      toast.error(t("No token to copy"));
      return;
    }
    try {
      await navigator.clipboard.writeText(apiToken ?? value);
      toast.success(apiToken ? t("Full token copied to clipboard") : t("Token preview copied"));
    } catch {
      toast.error(t("Failed to copy"));
    }
  };

  const handleRegenerate = async () => {
    setIsRegenerating(true);
    try {
      const result = await aos.stores.auth.actions.regenerateToken();
      if (!result.success || !result.token) {
        toast.error(result.error?.message || t("Failed to regenerate API token"));
        return;
      }

      onTokenRevealed(result.token);
      toast.success(t("API token generated. Copy it now: it is not shown again."));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("Failed to regenerate API token"));
    } finally {
      setIsRegenerating(false);
    }
  };

  // The full value only right after it was generated; afterwards, the prefix.
  const displayToken = apiToken ?? maskedToken ?? t("No token configured");

  const handleCopyReference = async () => {
    try {
      await navigator.clipboard.writeText(API_REFERENCE_COMMAND);
      toast.success(t("Command copied"));
    } catch {
      toast.error(t("Failed to copy"));
    }
  };

  return (
    <FormSection>
      <FormSectionHeader>
        <FormSectionTitle>{t("REST API")}</FormSectionTitle>
        <FormSectionDescription>
          {t("Manage tokens for programmatic access to the AOS API.")}
        </FormSectionDescription>
      </FormSectionHeader>

      <FormSectionContent>
        <FormSectionItem className="flex-wrap sm:flex-nowrap">
          <div className="flex min-w-0 flex-1 items-center gap-3">
            <div className="flex size-9 shrink-0 items-center justify-center rounded-lg border border-border bg-background">
              <HugeiconsIcon
                icon={Key01Icon}
                className="size-4 text-muted-foreground"
              />
            </div>
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground">{t("API Token")}</p>
              <p className="text-sm text-muted-foreground">
                {hasToken
                  ? t("Use this token for REST and MCP authentication.")
                  : t("No token configured. Generate one to start using the API.")}
              </p>
            </div>
          </div>

          <div className="flex shrink-0 items-center gap-1.5">
            <Input
              readOnly
              value={displayToken}
              className="h-8 w-[160px] font-mono text-xs text-muted-foreground"
            />

            <AlertDialog>
              <Tooltip>
                <TooltipTrigger asChild>
                  <AlertDialogTrigger asChild>
                    <Button
                      type="button"
                      variant="outline"
                      size="icon"
                      className="size-8 shrink-0"
                      disabled={isRegenerating}
                      aria-label={hasToken ? t("Regenerate API token") : t("Generate API token")}
                    >
                      <HugeiconsIcon
                        icon={RefreshIcon}
                        className={`size-3.5 ${isRegenerating ? "animate-spin" : ""}`}
                      />
                    </Button>
                  </AlertDialogTrigger>
                </TooltipTrigger>
                <TooltipContent>
                  {hasToken ? t("Regenerate token") : t("Generate token")}
                </TooltipContent>
              </Tooltip>

              <AlertDialogContent size="sm">
                <AlertDialogHeader>
                  <AlertDialogTitle>
                    {hasToken ? t("Regenerate API Token?") : t("Generate API Token?")}
                  </AlertDialogTitle>
                  <AlertDialogDescription>
                    {hasToken
                      ? t("This will invalidate the current token immediately. Any integrations using it will stop working until updated.")
                      : t("This will create a new API token for programmatic access.")}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t("Cancel")}</AlertDialogCancel>
                  <AlertDialogAction
                    variant={hasToken ? "destructive" : "default"}
                    disabled={isRegenerating}
                    onClick={() => void handleRegenerate()}
                  >
                    {isRegenerating
                      ? t("Generating...")
                      : hasToken
                        ? t("Regenerate")
                        : t("Generate")}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>

            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  type="button"
                  variant="outline"
                  size="icon"
                  className="size-8 shrink-0"
                  disabled={!apiToken && !maskedToken}
                  onClick={() => void handleCopyToken()}
                  aria-label={t("Copy API token")}
                >
                  <HugeiconsIcon icon={Copy01Icon} className="size-3.5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{apiToken ? t("Copy full token") : t("Copy token preview")}</TooltipContent>
            </Tooltip>
          </div>
        </FormSectionItem>

        <FormSectionItem>
          <div className="flex min-w-0 flex-1 items-center gap-3">
            <div className="flex size-9 shrink-0 items-center justify-center rounded-lg border border-border bg-background">
              <HugeiconsIcon
                icon={BookOpen01Icon}
                className="size-4 text-muted-foreground"
              />
            </div>
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground">
                {t("API Documentation")}
              </p>
              <p className="text-sm text-muted-foreground">
                {t("Every command, with its documentation and input schema, printed by the terminal.")}
              </p>
            </div>
          </div>
          <Button type="button" variant="outline" size="sm" onClick={() => void handleCopyReference()}>
            <HugeiconsIcon icon={Copy01Icon} className="size-3.5" />
            <span className="font-mono text-xs">{API_REFERENCE_COMMAND}</span>
          </Button>
        </FormSectionItem>
      </FormSectionContent>
    </FormSection>
  );
}
