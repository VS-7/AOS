import { aos } from "@/app/aos";
import { SettingsSectionShell } from "../../../section-shell";
import {
  FormSection,
  FormSectionContent,
  FormSectionDescription,
  FormSectionHeader,
  FormSectionTitle,
} from "@/components/ui/form-section";
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel } from "@/components/ui/form";
import { Switch } from "@/components/ui/switch";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { WorkspaceWorktreesSchema } from "@/features/workspace/schemas/workspace.schema";
import { toast } from "sonner";
import { AppError } from "@/core/errors/aos.error";
import { t } from "@/lib/i18n";
import { changedSettings } from "../../../../helpers/changed-settings";
import { saveWorkspaceSettings } from "../../../../helpers/save-workspace-settings";

type WorkspaceSnapshot = typeof aos.stores.workspace.state.current;

/** What the form shows for a workspace — also what a change is measured against. */
function worktreesFormValues(workspace: WorkspaceSnapshot) {
  return {
    deleteOldWorktrees: workspace?.worktrees?.deleteOldWorktrees ?? true,
    worktreeLimit: workspace?.worktrees?.worktreeLimit ?? 5,
    onCreateScript: workspace?.worktrees?.onCreateScript ?? "",
  };
}

export function WorkspaceWorktreesSection() {
  const currentWorkspace = aos.stores.workspace.useState((state) => state.current);

  const form = aos.useForm({
    schema: WorkspaceWorktreesSchema,
    mode: "onChange",
    values: worktreesFormValues(currentWorkspace),
    // Only what changed, and the answer becomes the snapshot: the whole group
    // used to be resent from the values this page opened with, so a limit
    // saved a moment earlier came back on the next edit.
    onSubmit: async (values) => {
      const saved = aos.stores.workspace.state.current;
      if (await saveWorkspaceSettings(saved?.id, changedSettings("worktrees", values, worktreesFormValues(saved)))) {
        toast.success(t("Worktrees settings updated successfully!"));
      }
      return values;
    },
    onResponse: ({ error }) => {
      if (!error) return;
      if (error instanceof AppError) {
        toast.error(error.message);
        return;
      }
      toast.error(error.message || t("Failed to update worktrees settings"));
    },
  });

  // `disableLoadingState`: this form saves itself while the person is still
  // typing, and disabling its fields for each save took the focus out of the
  // one being typed in — the keystrokes after it went nowhere.
  return (
    <Form form={form} disableLoadingState className="flex h-full flex-1 flex-col overflow-y-auto">
      <SettingsSectionShell>
        <FormSection>
          <FormSectionHeader>
            <FormSectionTitle>{t("Lifecycle Management")}</FormSectionTitle>
            <FormSectionDescription>
              {t("Configure how isolated environments are cleaned up.")}
            </FormSectionDescription>
          </FormSectionHeader>
          <FormSectionContent className="divide-y divide-border">
            <FormField
              control={form.control}
              name="deleteOldWorktrees"
              render={({ field }) => (
                <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                  <div className="space-y-0.5">
                    <FormLabel>{t("Automatically delete old worktrees")}</FormLabel>
                    <FormDescription>{t("Clean up unused environments to save disk space.")}</FormDescription>
                  </div>
                  <FormControl>
                    <Switch checked={field.value} onCheckedChange={field.onChange} />
                  </FormControl>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="worktreeLimit"
              render={({ field, fieldState }) => (
                <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                  <div className="flex-1 space-y-0.5">
                    <FormLabel>{t("Worktree Limit")}</FormLabel>
                    <FormDescription>{t("Maximum number of active worktrees to keep.")}</FormDescription>
                    {/* The reason the autosave did not happen: a limit
                        outside 1-50, or an empty field, was simply never
                        saved, with nothing on screen to say so. */}
                    {fieldState.error ? (
                      <p role="alert" className="text-sm text-destructive">
                        {t("Choose a limit from 1 to 50.")}
                      </p>
                    ) : null}
                  </div>
                  <FormControl>
                    <Input
                      type="number"
                      min={1}
                      max={50}
                      className="max-w-25"
                      {...field}
                      // An emptied field holds NaN, which the schema refuses
                      // like a limit out of range — but it is drawn as an
                      // empty box: React refused NaN as the input's value and
                      // logged it. (Undefined would not do: the field would
                      // fall back to showing the saved limit over what the
                      // person had just erased.)
                      value={Number.isFinite(field.value) ? field.value : ""}
                      onChange={(event) => field.onChange(event.target.valueAsNumber)}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          </FormSectionContent>
        </FormSection>

        <FormSection>
          <FormSectionHeader>
            <FormSectionTitle>{t("Initialization Script")}</FormSectionTitle>
            <FormSectionDescription>
              {t("Optional Bash script to execute after worktree creation.")}
            </FormSectionDescription>
          </FormSectionHeader>
          <FormSectionContent className="p-4">
            <FormField
              control={form.control}
              name="onCreateScript"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormDescription>
                    {t("This script runs with the worktree path as the current working directory. Useful for copying `.env` or installing dependencies.")}
                  </FormDescription>
                  <FormControl>
                    <Textarea
                      placeholder={t("cp ../../../.env . && bun install")}
                      className="min-h-32 font-mono text-xs"
                      {...field}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          </FormSectionContent>
        </FormSection>
      </SettingsSectionShell>
    </Form>
  );
}
