import { useRouter } from "@tanstack/react-router";
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

export function WorkspaceGitSection() {
  const router = useRouter();
  
  // `aos.useContext()` is AOS's global route context (`withContext(...)`),
  // which this port's `app/aos.tsx` never wires -- `DefaultContext` (`app/
  // builders/types.ts`) is deliberately loose (`Record<string, any>`) for
  // exactly this unset case, so no per-call-site cast is needed here.
  const context = aos.useContext();
  const currentWorkspace = context.workspaces?.current;

  const form = aos.useForm({
    schema: WorkspaceGitSchema,
    mode: "onChange",
    mutation: "workspace.update",
    values: {
      branchPrefix: currentWorkspace?.git?.branchPrefix || "",
      forcePush: currentWorkspace?.git?.forcePush || false,
      commitInstructions: currentWorkspace?.git?.commitInstructions || "",
      prInstructions: currentWorkspace?.git?.prInstructions || "",
    },
    onSubmit: (values) => ({
      body: { git: values },
      params: { id: currentWorkspace?.id },
    }),
    onResponse: ({ error }) => {
      if (error) {
        if (error instanceof AppError) {
          toast.error(error.message);
          return;
        }

        console.error(error);
        toast.error(error.message || "Failed to update git settings");
        return;
      }

      toast.success(t("Git settings updated successfully!"));
      router.invalidate();
    },
  });

  return (
    <Form form={form} className="flex h-full flex-1 flex-col overflow-y-auto">
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
