import * as React from "react";
import {
  BookOpen01Icon,
  Copy01Icon,
  Key01Icon,
  LinkSquare02Icon,
  RefreshIcon,
} from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";
import { toast } from "sonner";

import { aos } from "@/app/aos";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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

const API_DOCS_URL = "http://localhost:5326/api/docs";

interface DevelopersRestApiSectionProps {
  /** Full API token when revealed after regenerate. */
  apiToken: string | null;
  /** Masked token preview for display. */
  maskedToken: string | null;
  /** Whether the account has an API token configured. */
  hasToken: boolean;
  /** Called after generate/regenerate with the full token once. */
  onTokenRevealed: (fullToken: string, maskedToken: string) => void;
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
      toast.error("No token to copy");
      return;
    }
    try {
      await navigator.clipboard.writeText(apiToken ?? value);
      toast.success(
        apiToken ? "Full token copied to clipboard" : "Token preview copied",
      );
    } catch {
      toast.error("Failed to copy");
    }
  };

  const handleRegenerate = async () => {
    setIsRegenerating(true);
    try {
      const result = await aos.stores.auth.actions.regenerateToken();
      if (!result.success || !result.token) {
        toast.error(result.error?.message || "Failed to regenerate API token");
        return;
      }

      const masked = `aos_...${result.token.slice(-4)}`;
      onTokenRevealed(result.token, masked);
      toast.success("API token regenerated successfully!");
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Failed to regenerate API token";
      toast.error(message);
    } finally {
      setIsRegenerating(false);
    }
  };

  const displayToken = maskedToken ?? (hasToken ? "aos_..." : "No token configured");

  return (
    <FormSection>
      <FormSectionHeader>
        <FormSectionTitle>REST API</FormSectionTitle>
        <FormSectionDescription>
          Manage tokens for programmatic access to the AOS API.
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
              <p className="text-sm font-medium text-foreground">API Token</p>
              <p className="text-sm text-muted-foreground">
                {hasToken
                  ? "Use this token for REST and MCP authentication."
                  : "No token configured. Generate one to start using the API."}
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
                      aria-label={
                        hasToken ? "Regenerate API token" : "Generate API token"
                      }
                    >
                      <HugeiconsIcon
                        icon={RefreshIcon}
                        className={`size-3.5 ${isRegenerating ? "animate-spin" : ""}`}
                      />
                    </Button>
                  </AlertDialogTrigger>
                </TooltipTrigger>
                <TooltipContent>
                  {hasToken ? "Regenerate token" : "Generate token"}
                </TooltipContent>
              </Tooltip>

              <AlertDialogContent size="sm">
                <AlertDialogHeader>
                  <AlertDialogTitle>
                    {hasToken ? "Regenerate API Token?" : "Generate API Token?"}
                  </AlertDialogTitle>
                  <AlertDialogDescription>
                    {hasToken
                      ? "This will invalidate the current token immediately. Any integrations using it will stop working until updated."
                      : "This will create a new API token for programmatic access."}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancel</AlertDialogCancel>
                  <AlertDialogAction
                    variant="destructive"
                    disabled={isRegenerating}
                    onClick={() => void handleRegenerate()}
                  >
                    {isRegenerating
                      ? "Generating..."
                      : hasToken
                        ? "Regenerate"
                        : "Generate"}
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
                  disabled={!hasToken && !apiToken}
                  onClick={() => void handleCopyToken()}
                  aria-label="Copy API token"
                >
                  <HugeiconsIcon icon={Copy01Icon} className="size-3.5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Copy full token</TooltipContent>
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
                API Documentation
              </p>
              <p className="text-sm text-muted-foreground">
                Open the interactive OpenAPI docs for this AOS instance.
              </p>
            </div>
          </div>
          <Button type="button" variant="ghost" size="sm" asChild>
            <a href={API_DOCS_URL} target="_blank" rel="noopener noreferrer">
              Open docs
              <HugeiconsIcon icon={LinkSquare02Icon} className="size-3.5" />
            </a>
          </Button>
        </FormSectionItem>
      </FormSectionContent>
    </FormSection>
  );
}
