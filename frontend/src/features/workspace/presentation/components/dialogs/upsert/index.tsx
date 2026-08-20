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

      if (data && data.data) {
        const result = await aos.stores.workspace.actions.switch(data.data.id);

        if (result.error) {
          toast.error(result.error.message || "Failed to switch to the new workspace");
          return;
        }

        if (onSuccess) {
          onSuccess(data.data.id);
        }
      }
    }
  });

  const currentName = form.watch("name");
  const currentColor = form.watch("color");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {trigger && <DialogTrigger asChild>{trigger}</DialogTrigger>}
      <DialogContent className="sm:max-w-106.25">
        <DialogHeader>
          <DialogTitle>New Workspace</DialogTitle>
          <DialogDescription>
            Create a new workspace to organize your projects, skills, and agents.
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
            <span className="text-xs text-muted-foreground">Preview</span>
          </div>

          <FormField
            control={form.control}
            name="name"
            render={({ field }) => (
              <FormItem className="grid gap-2">
                <Label htmlFor="name">Workspace Name</Label>
                <FormControl>
                  <Input id="name" placeholder="e.g. Acme Corp" {...field} />
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
                <Label htmlFor="path">Directory Path</Label>
                <FormControl>
                  <FolderInput placeholder="Absolute path on your machine" value={field.value} onChange={field.onChange} />
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
                <Label htmlFor="color">Brand Color</Label>
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
