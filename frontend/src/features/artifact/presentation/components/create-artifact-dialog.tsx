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

const VISIBILITY_OPTIONS: Array<{
  value: ArtifactVisibility;
  label: string;
  description: string;
}> = [
  {
    value: "private",
    label: "Private",
    description: "Only you can open it.",
  },
  {
    value: "workspace",
    label: "Workspace",
    description: "Any authenticated member of this workspace can open it.",
  },
  {
    value: "by_password",
    label: "By password",
    description: "Anyone holding the password can open it — set one after creating.",
  },
];

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

  const { mutate: createArtifact, loading: isCreating } =
    aos.client.artifact.create.useMutation({
      onSuccess: async (response) => {
        const created = response?.data as ArtifactListItem | undefined;
        if (!created) {
          toast.error("Unable to create artifact.");
          return;
        }
        await ArtifactStore.actions.refresh();
        resetAndClose();
        toast.success(`Created "${created.name}".`);
        ArtifactHelper.openInBrowserTab(created);
      },
      onError: (error: any) => {
        toast.error(
          error?.error?.message || error?.message || "Unable to create artifact.",
        );
      },
    });

  function resetAndClose() {
    setOpen(false);
    setName("");
    setDescription("");
    setVisibility("private");
  }

  function handleOpenChange(nextOpen: boolean) {
    setOpen(nextOpen);
    if (!nextOpen) {
      setName("");
      setDescription("");
      setVisibility("private");
    }
  }

  function handleSubmit(event?: React.FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    const trimmed = name.trim();
    if (!trimmed) return;

    createArtifact({
      body: {
        name: trimmed,
        description: description.trim() || undefined,
        visibility,
      },
    });
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>{children}</DialogTrigger>

      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>New artifact</DialogTitle>
          <DialogDescription>
            A static web app registered in this workspace and served by the
            daemon — a dashboard, a report, a landing page. Starts as a
            minimal placeholder page you (or an agent) can fill in.
          </DialogDescription>
        </DialogHeader>

        <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
          <div className="flex flex-col gap-2">
            <Label htmlFor="artifact-name">Name</Label>
            <Input
              id="artifact-name"
              autoFocus
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="Sales dashboard"
              disabled={isCreating}
              maxLength={120}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="artifact-description">Description (optional)</Label>
            <Textarea
              id="artifact-description"
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder="What this artifact is, for whoever finds it later."
              disabled={isCreating}
              rows={2}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="artifact-visibility">Visibility</Label>
            <Select
              value={visibility}
              onValueChange={(value) => setVisibility(value as ArtifactVisibility)}
              disabled={isCreating}
            >
              <SelectTrigger id="artifact-visibility" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {VISIBILITY_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">
              {VISIBILITY_OPTIONS.find((o) => o.value === visibility)?.description}
            </p>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => handleOpenChange(false)}
            >
              Cancel
            </Button>
            <Button type="submit" size="sm" disabled={isCreating || !name.trim()}>
              {isCreating ? (
                <HugeiconsIcon icon={Loading03Icon} className="size-4 animate-spin" />
              ) : (
                <HugeiconsIcon icon={WindowsNewIcon} className="size-4" />
              )}
              Create
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
