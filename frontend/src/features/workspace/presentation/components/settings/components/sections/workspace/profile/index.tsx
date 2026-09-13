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
import { t } from "@/lib/i18n";
import { errorMessage } from "@/lib/aos-facade";

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

      toast.success(t("Workspace profile updated successfully!"));
      void aos.stores.workspace.actions.refresh();
    },
  });

  const handleDelete = async () => {
    if (!currentWorkspace?.id) return;

    setDeleting(true);
    try {
      // The store answers a refusal as `{error}` rather than throwing, so
      // awaiting it inside this try was not enough: a refused delete was
      // announced as done and navigated away from a workspace still there.
      const { error } = await aos.stores.workspace.actions.deleteWorkspace(
        currentWorkspace.id,
      );
      if (error) {
        toast.error(t("Failed to delete workspace"), { description: errorMessage(error) });
        return;
      }
      toast.success(t("Workspace deleted successfully"));
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
            <FormSectionTitle>{t("Branding")}</FormSectionTitle>
            <FormSectionDescription>
              {t("Name, logo, and color for this workspace.")}
            </FormSectionDescription>
          </FormSectionHeader>
          <FormSectionContent className="divide-y divide-border">
            <FormField
              control={form.control}
              name="logo"
              render={({ field }) => (
                <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                  <div className="flex-1 space-y-0.5">
                    <FormLabel>{t("Logo")}</FormLabel>
                    <FormDescription>
                      {t("Workspace logo.")}
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
              render={({ field, fieldState }) => (
                <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                  <div className="flex-1 space-y-0.5">
                    <FormLabel>{t("Name")}</FormLabel>
                    <FormDescription>
                      {t("How this workspace is named.")}
                    </FormDescription>
                    {/* The reason the autosave did not happen. Rendered here
                        rather than through FormMessage, which shows the
                        schema's message untranslated. */}
                    {fieldState.error ? (
                      <p role="alert" className="text-sm text-destructive">
                        {t("The workspace needs a name.")}
                      </p>
                    ) : null}
                  </div>
                  <FormControl>
                    <Input
                      className="max-w-50"
                      placeholder={t("Workspace name")}
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
                    <FormLabel>{t("Accent color")}</FormLabel>
                    <FormDescription>
                      {t("Optional color accent.")}
                    </FormDescription>
                  </div>
                  <FormControl>
                    {/* Stored as hex, which is all the daemon accepts: the
                        format dropdown changes only how the channels are
                        shown. It used to rewrite the accent as rgb()/hsl()/
                        oklch(), and the autosave was refused. */}
                    <ColorPickerPopover
                      onTriggerRemove={() => field.onChange(null)}
                      value={field.value}
                      valueFormat="hex"
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
            <FormSectionTitle>{t("Danger Zone")}</FormSectionTitle>
            <FormSectionDescription>
              {t("Irreversible actions. Proceed with caution.")}
            </FormSectionDescription>
          </FormSectionHeader>
          <FormSectionContent>
            <div className="flex flex-row items-center justify-between gap-4 p-4">
              <div className="flex-1 space-y-0.5">
                <p className="text-sm font-medium">{t("Delete this workspace")}</p>
                <p className="text-sm text-muted-foreground">
                  {t("Permanently remove the workspace and all its configuration. This action cannot be undone.")}
                </p>
              </div>
              <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
                <AlertDialogTrigger asChild>
                  <Button variant="destructive" size="sm">
                    {t("Delete Workspace")}
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>{t("Delete Workspace")}</AlertDialogTitle>
                    <AlertDialogDescription>
                      {t("Are you sure you want to delete")}{" "}
                      <span className="font-semibold">
                        {currentWorkspace?.name}
                      </span>
                      {t("? This will permanently remove the workspace and all its configuration. This action cannot be undone.")}
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel disabled={deleting}>
                      {t("Cancel")}
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
