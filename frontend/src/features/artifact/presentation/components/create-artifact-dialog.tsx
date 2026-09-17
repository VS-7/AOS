import * as React from "react";
import { HugeiconsIcon } from "@hugeicons/react";
import { WindowsNewIcon, Loading03Icon } from "@hugeicons/core-free-icons";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { aos } from "@/app/aos";
import type {
  ArtifactListItem,
  ArtifactVisibility,
} from "@/features/artifact/interfaces/artifact.interfaces";
import { ArtifactHelper } from "@/features/artifact/presentation/helpers/artifact.helper";
import { ArtifactStore } from "@/features/artifact/presentation/stores/artifact.store";
import { t } from "@/lib/i18n";
import { MIN_ARTIFACT_PASSWORD_LENGTH, isArtifactPasswordRefused } from "../helpers/artifact-access";

// A function, so the labels are in the language the person has when the
// dialog opens rather than the one this module was loaded in.
function visibilityOptions(): Array<{
  value: ArtifactVisibility;
  label: string;
  description: string;
}> {
  return [
    {
      value: "private",
      label: t("Private"),
      description: t("Only you can open it."),
    },
    {
      value: "workspace",
      label: t("Workspace"),
      description: t("Any authenticated member of this workspace can open it."),
    },
    {
      value: "by_password",
      label: t("By password"),
      // Set here, with the artifact. "Set one after creating" pointed at a
      // screen that does not exist, and an artifact with no password refuses
      // everybody.
      description: t("Anyone holding the password can open it."),
    },
  ];
}

/** The shortest password the dialog accepts — the daemon's own minimum. */
const MIN_PASSWORD_LENGTH = MIN_ARTIFACT_PASSWORD_LENGTH;

interface CreateArtifactDialogProps {
  children: React.ReactNode;
}

/**
 * Registers a new artifact and, on success, opens it right away — the
 * fastest way to see whether what an agent (or a person, from here) just
 * scaffolded actually renders. `artifacts_create` (internal/domain/artifact/
 * service.go's Create) answers a bare *Artifact*, urls included (see
 * Service.urlsFor) — command-map.ts's "artifact.create" needs no wrapOut,
 * unlike "artifact.list".
 */
export function CreateArtifactDialog({ children }: CreateArtifactDialogProps) {
  const [open, setOpen] = React.useState(false);
  const [name, setName] = React.useState("");
  const [description, setDescription] = React.useState("");
  const [visibility, setVisibility] = React.useState<ArtifactVisibility>("private");
  const [password, setPassword] = React.useState("");
  const options = visibilityOptions();
  const needsPassword = visibility === "by_password";
  const passwordTooShort = needsPassword && isArtifactPasswordRefused(password);

  const { mutate: createArtifact, loading: isCreating } =
    aos.client.artifact.create.useMutation({
      onSuccess: async (response, variables) => {
        const created = response?.data as ArtifactListItem | undefined;
        if (!created) {
          toast.error(t("Unable to create artifact."));
          return;
        }
        // A by_password artifact asks everybody for its password, its creator
        // included, and this is the one moment the dialog already has it: the
        // one it was just created with.
        const typed = (variables as { body?: { password?: string } } | undefined)?.body?.password;
        await ArtifactStore.actions.refresh();
        resetAndClose();
        if (created.visibility === "by_password") {
          toast.success(t("Created \"{{name}}\".", { name: created.name }), {
            description: t("Share its address together with the password."),
          });
          ArtifactHelper.openInBrowserTab(created, { password: typed });
          return;
        }
        toast.success(t("Created \"{{name}}\".", { name: created.name }));
        ArtifactHelper.openInBrowserTab(created);
      },
      onError: (error: any) => {
        toast.error(
          error?.error?.message || error?.message || t("Unable to create artifact."),
        );
      },
    });

  function resetAndClose() {
    setOpen(false);
    setName("");
    setDescription("");
    setVisibility("private");
    setPassword("");
  }

  function handleOpenChange(nextOpen: boolean) {
    setOpen(nextOpen);
    if (!nextOpen) {
      setName("");
      setDescription("");
      setVisibility("private");
      setPassword("");
    }
  }

  function handleSubmit(event?: React.FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    const trimmed = name.trim();
    if (!trimmed || passwordTooShort) return;

    createArtifact({
      body: {
        name: trimmed,
        description: description.trim() || undefined,
        visibility,
        // Stored with the artifact in the same call, so it is never reachable
        // half-configured.
        ...(needsPassword ? { password } : {}),
      },
    });
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>{children}</DialogTrigger>

      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("New artifact")}</DialogTitle>
          <DialogDescription>
            {t("A static web app registered in this workspace and served by the daemon — a dashboard, a report, a landing page. Starts as a minimal placeholder page you (or an agent) can fill in.")}
          </DialogDescription>
        </DialogHeader>

        <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
          <div className="flex flex-col gap-2">
            <Label htmlFor="artifact-name">{t("Name")}</Label>
            <Input
              id="artifact-name"
              autoFocus
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder={t("Sales dashboard")}
              disabled={isCreating}
              maxLength={120}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="artifact-description">{t("Description (optional)")}</Label>
            <Textarea
              id="artifact-description"
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder={t("What this artifact is, for whoever finds it later.")}
              disabled={isCreating}
              rows={2}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="artifact-visibility">{t("Visibility")}</Label>
            <Select
              value={visibility}
              onValueChange={(value) => setVisibility(value as ArtifactVisibility)}
              disabled={isCreating}
            >
              <SelectTrigger id="artifact-visibility" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {options.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">
              {options.find((o) => o.value === visibility)?.description}
            </p>
          </div>

          {needsPassword ? (
            <div className="flex flex-col gap-2">
              <Label htmlFor="artifact-password">{t("Password")}</Label>
              <Input
                id="artifact-password"
                type="password"
                autoComplete="new-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                disabled={isCreating}
              />
              <p className="text-xs text-muted-foreground">
                {t("At least {{count}} characters. Share it only with who should open the artifact.", {
                  count: MIN_PASSWORD_LENGTH,
                })}
              </p>
            </div>
          ) : null}

          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => handleOpenChange(false)}
            >
              {t("Cancel")}
            </Button>
            <Button type="submit" size="sm" disabled={isCreating || !name.trim() || passwordTooShort}>
              {isCreating ? (
                <HugeiconsIcon icon={Loading03Icon} className="size-4 animate-spin" />
              ) : (
                <HugeiconsIcon icon={WindowsNewIcon} className="size-4" />
              )}
              {t("Create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
