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
import { WorkspaceGitSchema } from "@/features/workspace/schemas/workspace.schema";
import { toast } from "sonner";
import { AppError } from "@/core/errors/aos.error";
import { t } from "@/lib/i18n";
import { changedSettings } from "../../../../helpers/changed-settings";
import { saveWorkspaceSettings } from "../../../../helpers/save-workspace-settings";

type WorkspaceSnapshot = typeof aos.stores.workspace.state.current;

/** What the form shows for a workspace — also what a change is measured against. */
function gitFormValues(workspace: WorkspaceSnapshot) {
  return {
    branchPrefix: workspace?.git?.branchPrefix || "",
    forcePush: workspace?.git?.forcePush || false,
    commitInstructions: workspace?.git?.commitInstructions || "",
    prInstructions: workspace?.git?.prInstructions || "",
  };
}

export function WorkspaceGitSection() {
  // The store, not the route context: the context is a copy taken when the
  // route loaded, and a save never reached it — returning to this section
  // showed the values from before the save.
  const currentWorkspace = aos.stores.workspace.useState((state) => state.current);

  const form = aos.useForm({
    schema: WorkspaceGitSchema,
    mode: "onChange",
    values: gitFormValues(currentWorkspace),
    onSubmit: async (values) => {
      const saved = aos.stores.workspace.state.current;
      if (await saveWorkspaceSettings(saved?.id, changedSettings("git", values, gitFormValues(saved)))) {
        toast.success(t("Git settings updated successfully!"));
      }
      return values;
    },
    onResponse: ({ error }) => {
      if (!error) return;
      if (error instanceof AppError) {
        toast.error(error.message);
        return;
      }
      toast.error(error.message || t("Failed to update git settings"));
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
            <FormSectionTitle>{t("Workflow")}</FormSectionTitle>
            <FormSectionDescription>{t("Basic git automation behavior.")}</FormSectionDescription>
          </FormSectionHeader>
          <FormSectionContent className="divide-y divide-border">
            <FormField
              control={form.control}
              name="branchPrefix"
              render={({ field }) => (
                <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                  <div className="flex-1 space-y-0.5">
                    <FormLabel>{t("Branch prefix")}</FormLabel>
                    <FormDescription>{t("Prefix used when creating new branches.")}</FormDescription>
                  </div>
                  <FormControl>
                    <Input className="max-w-50" placeholder={t("e.g. agent/")} {...field} />
                  </FormControl>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="forcePush"
              render={({ field }) => (
                <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                  <div className="space-y-0.5">
                    <FormLabel>{t("Always force push")}</FormLabel>
                    <FormDescription>{t("Enable force pushing by default on agent branches.")}</FormDescription>
                  </div>
                  <FormControl>
                    <Switch checked={field.value} onCheckedChange={field.onChange} />
                  </FormControl>
                </FormItem>
              )}
            />
          </FormSectionContent>
        </FormSection>

        <FormSection>
          <FormSectionHeader>
            <FormSectionTitle>{t("Instructions")}</FormSectionTitle>
            <FormSectionDescription>{t("Tell agents how to format git messages.")}</FormSectionDescription>
          </FormSectionHeader>
          <FormSectionContent className="divide-y divide-border">
            <FormField
              control={form.control}
              name="commitInstructions"
              render={({ field }) => (
                <FormItem className="gap-4 p-4">
                  <div className="mb-4 space-y-0.5">
                    <FormLabel>{t("Commit instructions")}</FormLabel>
                    <FormDescription>{t("Custom rules for writing commit messages.")}</FormDescription>
                  </div>
                  <FormControl>
                    <Textarea placeholder={t("Instructions here...")} className="min-h-25" {...field} />
                  </FormControl>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="prInstructions"
              render={({ field }) => (
                <FormItem className="gap-4 p-4">
                  <div className="mb-4 space-y-0.5">
                    <FormLabel>{t("Pull request instructions")}</FormLabel>
                    <FormDescription>{t("Custom rules for writing PR descriptions.")}</FormDescription>
                  </div>
                  <FormControl>
                    <Textarea placeholder={t("Instructions here...")} className="min-h-25" {...field} />
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
