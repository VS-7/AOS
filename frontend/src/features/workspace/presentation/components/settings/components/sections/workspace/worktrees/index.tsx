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
import { FractalWorkspaceWorktreesSchema } from "@/features/workspace/schemas/workspace.schema";
import { toast } from "sonner";
import { FractalAppError } from "@/core/errors/fractal.error";

export function WorkspaceWorktreesSection() {
  const currentWorkspace = aos.stores.workspace.useState((state) => state.current);
  const router = useRouter();

  const form = aos.useForm({
    schema: FractalWorkspaceWorktreesSchema,
    mode: "onChange",
    mutation: "workspace.update",
    values: {
      deleteOldWorktrees: currentWorkspace?.worktrees?.deleteOldWorktrees ?? true,
      worktreeLimit: currentWorkspace?.worktrees?.worktreeLimit ?? 5,
      onCreateScript: currentWorkspace?.worktrees?.onCreateScript ?? "",
    },
    onSubmit: (values) => ({
      body: { worktrees: values },
      params: { id: currentWorkspace?.id },
    }),
    onResponse: ({ error }) => {
      if (error) {
        if (error instanceof FractalAppError) {
          toast.error(error.message);
          return;
        }

        console.error(error);
        toast.error(error.message || "Failed to update worktrees settings");
        return;
      }

      router.invalidate();
      toast.success("Worktrees settings updated successfully!");
    },
  });

  return (
    <Form form={form} className="flex h-full flex-1 flex-col overflow-y-auto">
      <SettingsSectionShell>
        <FormSection>
          <FormSectionHeader>
            <FormSectionTitle>Lifecycle Management</FormSectionTitle>
            <FormSectionDescription>
              Configure how isolated environments are cleaned up.
            </FormSectionDescription>
          </FormSectionHeader>
          <FormSectionContent className="divide-y divide-border">
            <FormField
              control={form.control}
              name="deleteOldWorktrees"
              render={({ field }) => (
                <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                  <div className="space-y-0.5">
                    <FormLabel>Automatically delete old worktrees</FormLabel>
                    <FormDescription>Clean up unused environments to save disk space.</FormDescription>
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
              render={({ field }) => (
                <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                  <div className="flex-1 space-y-0.5">
                    <FormLabel>Worktree Limit</FormLabel>
                    <FormDescription>Maximum number of active worktrees to keep.</FormDescription>
                  </div>
                  <FormControl>
                    <Input
                      type="number"
                      className="max-w-25"
                      {...field}
                      onChange={(event) => field.onChange(parseInt(event.target.value, 10))}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          </FormSectionContent>
        </FormSection>

        <FormSection>
          <FormSectionHeader>
            <FormSectionTitle>Initialization Script</FormSectionTitle>
            <FormSectionDescription>
              Optional Bash script to execute after worktree creation.
            </FormSectionDescription>
          </FormSectionHeader>
          <FormSectionContent className="p-4">
            <FormField
              control={form.control}
              name="onCreateScript"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormDescription>
                    This script runs with the worktree path as the current working directory. Useful for
                    copying `.env` or installing dependencies.
                  </FormDescription>
                  <FormControl>
                    <Textarea
                      placeholder="cp ../../../.env . && bun install"
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
