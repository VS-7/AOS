import * as React from "react";
import { z } from "zod";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Field,
  FieldGroup,
  Form,
  FormControl,
  FormField,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { aos } from "@/app/aos";
import type { FileExplorerContext } from "@/features/file/interfaces/file.interfaces";
import { t } from "@/lib/i18n";
import {
  formatCreateDestinationPath,
  joinWorkspacePath,
} from "@/features/file/presentation/helpers/files-explorer.helper";

const createNodeSchema = z.object({
  name: z.string().min(1, "Name is required"),
  type: z.enum(["file", "directory"]),
});

export interface FilesCreateNodeDialogProps {
  open: boolean;
  parentPath: string;
  defaultType?: "file" | "directory";
  explorerContext: FileExplorerContext;
  onOpenChange: (open: boolean) => void;
  onCreated?: (path: string) => void;
}

export function FilesCreateNodeDialog({
  open,
  parentPath,
  defaultType = "file",
  explorerContext,
  onOpenChange,
  onCreated,
}: FilesCreateNodeDialogProps) {
  const form = aos.useForm({
    schema: createNodeSchema,
    values: {
      name: "",
      type: defaultType,
    },
    // Runs after validation, from the Create button or Enter in the name
    // field — <Form> routes both here. `createNode` is declared below; this
    // only runs on a submit, long after the render that declared it.
    onSubmit: (values) => {
      createNode({
        body: {
          path: joinWorkspacePath(parentPath, values.name.trim()),
          type: values.type,
          context: explorerContext,
        },
      });
    },
  });

  const watchedName = form.watch("name");
  const watchedType = form.watch("type");
  const destinationFolder = formatCreateDestinationPath(parentPath);
  const previewPath = watchedName?.trim()
    ? joinWorkspacePath(parentPath, watchedName.trim())
    : destinationFolder === "/"
      ? `/${watchedType === "directory" ? "folder-name" : "file-name"}`
      : joinWorkspacePath(
          parentPath,
          watchedType === "directory" ? "folder-name" : "file-name",
        );

  const { mutate: createNode, loading: isCreating } =
    aos.client.file.create.useMutation({
      onSuccess: (response) => {
        // `onSuccess` receives the full `Envelope` — see `aos-facade.ts`'s
        // `useMutation` doc comment.
        const createdPath = response?.data?.file?.path;

        toast.success(
          defaultType === "directory" ? "Folder created." : "File created.",
        );
        form.reset({ name: "", type: defaultType });
        onOpenChange(false);

        if (createdPath) {
          onCreated?.(createdPath);
        }
      },
      onError: (error: unknown) => {
        const message =
          typeof error === "object" &&
          error != null &&
          "error" in error &&
          typeof (error as { error?: { message?: string } }).error?.message ===
            "string"
            ? (error as { error?: { message?: string } }).error?.message
            : error instanceof Error
              ? error.message
              : "Unable to create item.";

        toast.error(message);
      },
    });

  React.useEffect(() => {
    if (!open) return;
    form.reset({ name: "", type: defaultType });
  }, [defaultType, form, open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {defaultType === "directory" ? "New Folder" : "New File"}
          </DialogTitle>
          <DialogDescription>
            {destinationFolder === "/"
              ? "Create at the workspace root."
              : `Create inside ${destinationFolder}.`}
          </DialogDescription>
        </DialogHeader>

        {/* One <form>, the one <Form> renders. This dialog used to put its
            own <form> inside it; Blink and WebKit stop a nested form's
            submit event at the outer one, so Create reloaded the app at
            /?name=<typed name> and created nothing. */}
        <Form form={form} className="flex flex-col gap-4">
          <FieldGroup>
            <div className="rounded-md border bg-muted/40 px-3 py-2">
              <p className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
                {t("Path")}
              </p>
              <p className="mt-1 break-all font-mono text-xs text-foreground">
                {previewPath}
              </p>
            </div>

            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <Field>
                  <Label htmlFor="files-create-name">{t("Name")}</Label>
                  <FormControl>
                    <Input
                      {...field}
                      id="files-create-name"
                      autoFocus
                      placeholder={
                        defaultType === "directory" ? "components" : "index.ts"
                      }
                    />
                  </FormControl>
                  <FormMessage />
                </Field>
              )}
            />

            <FormField
              control={form.control}
              name="type"
              render={({ field }) => (
                <Field>
                  <Label htmlFor="files-create-type">{t("Type")}</Label>
                  <Select value={field.value} onValueChange={field.onChange}>
                    <FormControl>
                      <SelectTrigger id="files-create-type">
                        <SelectValue placeholder={t("Select type")} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value="file">{t("File")}</SelectItem>
                      <SelectItem value="directory">{t("Folder")}</SelectItem>
                    </SelectContent>
                  </Select>
                </Field>
              )}
            />
          </FieldGroup>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              {t("Cancel")}
            </Button>
            <Button type="submit" disabled={isCreating}>
              {t("Create")}
            </Button>
          </DialogFooter>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
