import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { aos } from "@/app/aos";
import { SettingsSectionShell } from "../../../section-shell";
import {
  FormSection,
  FormSectionContent,
  FormSectionDescription,
  FormSectionHeader,
  FormSectionTitle,
} from "@/components/ui/form-section";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
} from "@/components/ui/form";
import { ImageUpload } from "@/components/ui/image-upload";
import { Input } from "@/components/ui/input";
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
import { WorkspaceUpdateInputSchema } from "@/features/workspace/schemas/workspace.schema";
import { AppError } from "@/core/errors/aos.error";
import { toast } from "sonner";
import { ColorPickerPopover } from "@/components/ui/color-picker";

/**
 * Workspace branding (name, logo, color) and danger-zone delete.
 */
export function WorkspaceProfileSection() {
  const currentWorkspace = aos.stores.workspace.useState(
    (state) => state.current,
  );
  const navigate = useNavigate();
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const form = aos.useForm({
    schema: WorkspaceUpdateInputSchema,
    mode: "onChange",
    mutation: "workspace.update",
    values: {
      name: currentWorkspace?.name || "",
      logo: currentWorkspace?.logo || "",
      color: currentWorkspace?.color || "",
    },
    // task-12 disclosed divergence: Go's `workspace_update` (`UpdateInput`,
    // `internal/domain/workspace/schema.go`) takes a single dotted-path
    // `set: map[string]any`, not top-level `name`/`logo`/`color` fields.
    // `command-map.ts`'s `coerceIn` can't build this up across three
    // independent scalar fields in one call (each field's transform result
    // gets shallow-merged — see that file's `workspace.update` comment) —
    // this form is the one place the dotted `set` object is built directly.
    onSubmit: (values) => ({
      body: {
        set: {
          name: values.name,
          logo: values.logo,
          color: values.color,
        },
      },
      params: { id: currentWorkspace?.id },
    }),
    onResponse: ({ error }) => {
      if (error) {
        if (error instanceof AppError) {
          toast.error(error.message);
          return;
        }

        console.error(error);
        toast.error(error.message || "Failed to update workspace profile");
        return;
      }

      toast.success("Workspace profile updated successfully!");
      void aos.stores.workspace.actions.refresh();
    },
  });

  const handleDelete = async () => {
    if (!currentWorkspace?.id) return;

    setDeleting(true);
    try {
      await aos.stores.workspace.actions.deleteWorkspace(
        currentWorkspace.id,
      );
      toast.success("Workspace deleted successfully");
      navigate({ to: "/" });
    } catch (error) {
      const message =
        error instanceof AppError
          ? error.message
          : error instanceof Error
            ? error.message
            : "Failed to delete workspace";
      toast.error(message);
    } finally {
      setDeleting(false);
      setDeleteOpen(false);
    }
  };

  return (
    <Form form={form} className="flex h-full flex-1 flex-col overflow-y-auto">
      <SettingsSectionShell>
        <FormSection>
            <FormSectionHeader>
            <FormSectionTitle>Branding</FormSectionTitle>
            <FormSectionDescription>
              Name, logo, and color for this workspace.
            </FormSectionDescription>
          </FormSectionHeader>
          <FormSectionContent className="divide-y divide-border">
            <FormField
              control={form.control}
              name="logo"
              render={({ field }) => (
                <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                  <div className="flex-1 space-y-0.5">
                    <FormLabel>Logo</FormLabel>
                    <FormDescription>
                      Workspace logo.
                    </FormDescription>
                  </div>
                  <FormControl>
                    <ImageUpload
                      value={field.value}
                      fallback={currentWorkspace?.name || "W"}
                      onChange={field.onChange}
                      onRemove={() => field.onChange("")}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                  <div className="flex-1 space-y-0.5">
                    <FormLabel>Name</FormLabel>
                    <FormDescription>
                      How this workspace is named.
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Input
                      className="max-w-50"
                      placeholder="Workspace name"
                      {...field}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="color"
              render={({ field }) => (
                <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                  <div className="flex-1 space-y-0.5">
                    <FormLabel>Accent color</FormLabel>
                    <FormDescription>
                      Optional color accent.
                    </FormDescription>
                  </div>
                  <FormControl>
                    <ColorPickerPopover
                      onTriggerRemove={() => field.onChange(null)}
                      value={field.value}
                      onValueChange={(v) => field.onChange(v)}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          </FormSectionContent>
        </FormSection>

        <FormSection>
          <FormSectionHeader>
            <FormSectionTitle>Danger Zone</FormSectionTitle>
            <FormSectionDescription>
              Irreversible actions. Proceed with caution.
            </FormSectionDescription>
          </FormSectionHeader>
          <FormSectionContent>
            <div className="flex flex-row items-center justify-between gap-4 p-4">
              <div className="flex-1 space-y-0.5">
                <p className="text-sm font-medium">Delete this workspace</p>
                <p className="text-sm text-muted-foreground">
                  Permanently remove the workspace and all its configuration.
                  This action cannot be undone.
                </p>
              </div>
              <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
                <AlertDialogTrigger asChild>
                  <Button variant="destructive" size="sm">
                    Delete Workspace
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>Delete Workspace</AlertDialogTitle>
                    <AlertDialogDescription>
                      Are you sure you want to delete{" "}
                      <span className="font-semibold">
                        {currentWorkspace?.name}
                      </span>
                      ? This will permanently remove the workspace and all its
                      configuration. This action cannot be undone.
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel disabled={deleting}>
                      Cancel
                    </AlertDialogCancel>
                    <AlertDialogAction
                      disabled={deleting}
                      onClick={handleDelete}
                      className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                    >
                      {deleting ? "Deleting..." : "Delete"}
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </div>
          </FormSectionContent>
        </FormSection>
      </SettingsSectionShell>
    </Form>
  );
}
