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
import { changedSettings } from "../../../../helpers/changed-settings";
import { saveWorkspaceSettings } from "../../../../helpers/save-workspace-settings";

type WorkspaceSnapshot = typeof aos.stores.workspace.state.current;

/** What the form shows for a workspace — also what a change is measured against. */
function profileFormValues(workspace: WorkspaceSnapshot) {
  return {
    name: workspace?.name || "",
    logo: workspace?.logo || "",
    color: workspace?.color || "",
  };
}

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
    values: profileFormValues(currentWorkspace),
    // Go's `workspace_update` takes one dotted-path `set`, so the three
    // top-level fields go in as their own paths — only those that changed,
    // measured against the snapshot the save answers with (see
    // `saveWorkspaceSettings`).
    onSubmit: async (values) => {
      const saved = aos.stores.workspace.state.current;
      if (await saveWorkspaceSettings(saved?.id, changedSettings("", values, profileFormValues(saved)))) {
        toast.success(t("Workspace profile updated successfully!"));
      }
      return values;
    },
    onResponse: ({ error }) => {
      if (!error) return;
      if (error instanceof AppError) {
        toast.error(error.message);
        return;
      }
      toast.error(error.message || t("Failed to update workspace profile"));
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

  // `disableLoadingState`: this form saves itself while the person is still
  // typing, and disabling its fields for each save took the focus out of the
  // one being typed in — the keystrokes after it went nowhere.
  return (
    <Form form={form} disableLoadingState className="flex h-full flex-1 flex-col overflow-y-auto">
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
                      {t("Are you sure you want to delete {{name}}? This will permanently remove the workspace and all its configuration. This action cannot be undone.", {
                        name: currentWorkspace?.name ?? "",
                      })}
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel disabled={deleting}>
                      {t("Cancel")}
                    </AlertDialogCancel>
                    <AlertDialogAction
                      variant="destructive"
                      disabled={deleting}
                      onClick={handleDelete}
                    >
                      {deleting ? t("Deleting...") : t("Delete")}
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
