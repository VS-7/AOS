import React from "react";
import { z } from "zod";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { FolderInput } from "@/components/ui/folder-input";
import { aos } from "@/app/aos";
import { Form, FormField, FormItem, FormControl, FormMessage } from "@/components/ui/form";
import { toast } from "sonner";
import { ColorPickerPopover } from "@/components/ui/color-picker";
import { t } from "@/lib/i18n";

interface CreateWorkspaceDialogProps {
  trigger?: React.ReactNode;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  onSuccess?: (workspaceId: string) => void;
}

const formSchema = z.object({
  name: z.string().min(1, "Name is required"),
  path: z.string().min(1, "Path is required"),
  color: z.string(),
});

export function CreateWorkspaceDialog({ trigger, open, onOpenChange, onSuccess }: CreateWorkspaceDialogProps) {
  const form = aos.useForm({
    schema: formSchema,
    mutation: "workspace.create",
    values: {
      name: "",
      path: "",
      color: "#6366f1",
    },
    onSubmit: (values) => ({
      body: values,
    }),
    onResponse: async ({ data: rawData, error }) => {
      // `aos.useForm`'s `onResponse` data param doesn't carry the mutation's
      // real response shape here — same generic-inference gap as
      // `useFieldArray` in `tasks/index.tsx`. Cast, not a real type.
      const data = rawData as any;
      if (error) {
        toast.error(error.message || "Failed to create workspace");
        return;
      }

      // `data.workspace.id`, not `data.data.id`. `useForm` already unwraps
      // the envelope before it gets here (`app/builders/app.tsx`), so the
      // second `.data` was reaching into the workspace object for a key it
      // does not have: the condition was never true, and creating a
      // workspace ended in a dialog that stayed open having said nothing.
      const created = data?.workspace?.id ?? data?.id;
      if (!created) {
        toast.error(t("The workspace was created but the daemon did not say which one."));
        return;
      }

      // Refresh before switching: `switch` only accepts an id it can see in
      // `options`, and the list it reads was fetched before this workspace
      // existed.
      await aos.stores.workspace.actions.refresh();

      const result = await aos.stores.workspace.actions.switch(created);
      if (result.error) {
        toast.error(result.error.message || "Failed to switch to the new workspace");
        return;
      }

      toast.success(t("Workspace created."));
      onSuccess?.(created);
    }
  });

  const currentName = form.watch("name");
  const currentColor = form.watch("color");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {trigger && <DialogTrigger asChild>{trigger}</DialogTrigger>}
      <DialogContent className="sm:max-w-106.25">
        <DialogHeader>
          <DialogTitle>{t("New Workspace")}</DialogTitle>
          <DialogDescription>
            {t("Create a new workspace to organize your projects, skills, and agents.")}
          </DialogDescription>
        </DialogHeader>
        <Form form={form} className="grid gap-4 py-4">

          <div className="flex flex-col items-center justify-center mb-4">
            <div
              className="flex size-16 shrink-0 items-center justify-center rounded-xl shadow-sm border mb-2 text-white font-bold text-2xl"
              style={{ backgroundColor: currentColor }}
            >
              {currentName ? currentName.charAt(0).toUpperCase() : "?"}
            </div>
            <span className="text-xs text-muted-foreground">{t("Preview")}</span>
          </div>

          <FormField
            control={form.control}
            name="name"
            render={({ field }) => (
              <FormItem className="grid gap-2">
                <Label htmlFor="name">{t("Workspace Name")}</Label>
                <FormControl>
                  <Input id="name" placeholder={t("e.g. Acme Corp")} {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="path"
            render={({ field }) => (
              <FormItem className="grid gap-2">
                <Label htmlFor="path">{t("Directory Path")}</Label>
                <FormControl>
                  <FolderInput placeholder={t("Absolute path on your machine")} value={field.value} onChange={field.onChange} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="color"
            render={({ field }) => (
              <FormItem className="grid gap-2">
                <Label htmlFor="color">{t("Brand Color")}</Label>
                <div className="flex gap-2">
                  <ColorPickerPopover
                    triggerShowRemove
                    onTriggerRemove={() => field.onChange(null)}
                    value={field.value}
                    onValueChange={(v) => field.onChange(v)}
                  />
                </div>
                <FormMessage />
              </FormItem>
            )}
          />

          <DialogFooter className="mt-4">
            <Button type="submit" disabled={form.isLoading}>
              {form.isLoading ? "Creating..." : "Create Workspace"}
            </Button>
          </DialogFooter>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
