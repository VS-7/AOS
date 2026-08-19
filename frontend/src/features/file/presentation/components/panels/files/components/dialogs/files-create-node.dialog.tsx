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
import type { FractalFileExplorerContext } from "@/features/file/interfaces/file.interfaces";
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
  explorerContext: FractalFileExplorerContext;
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

  function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    void form.handleSubmit((values) => {
      createNode({
        body: {
          path: joinWorkspacePath(parentPath, values.name.trim()),
          type: values.type,
          context: explorerContext,
        },
      });
    })();
  }

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

        <Form form={form}>
          <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
            <FieldGroup>
              <div className="rounded-md border bg-muted/40 px-3 py-2">
                <p className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
                  Path
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
                    <Label htmlFor="files-create-name">Name</Label>
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
                  </Field>
                )}
              />

              <FormField
                control={form.control}
                name="type"
                render={({ field }) => (
                  <Field>
                    <Label htmlFor="files-create-type">Type</Label>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger id="files-create-type">
                          <SelectValue placeholder="Select type" />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value="file">File</SelectItem>
                        <SelectItem value="directory">Folder</SelectItem>
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
                Cancel
              </Button>
              <Button type="submit" disabled={isCreating}>
                Create
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
