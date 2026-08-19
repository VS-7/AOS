import { Copy, FileText, Save, Trash2 } from "lucide-react";
import { AnimatedEmptyState } from "@/components/ui/animated-empty-state";
import { Skeleton } from "@/components/ui/skeleton";
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
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { MarkdownEditor } from "@/components/ui/markdown-editor";
import { Textarea } from "@/components/ui/textarea";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { SettingsContentContainer } from "../../../../../content-container";
import { useInstructions } from "../../contexts/instructions.context";

export function SelectedInstructionContent() {
  const {
    selectedInstruction,
    isCreateMode,
    isLoadingContent,
    isDeleting,
    form,
    deleteSelectedInstruction,
  } = useInstructions();

  if (!selectedInstruction && !isCreateMode) {
    return (
      <div className="flex h-full items-center justify-center p-8">
        <AnimatedEmptyState className="border-none shadow-none">
          <AnimatedEmptyState.Carousel>
            <div className="flex items-center gap-3">
              <div className="flex size-8 items-center justify-center rounded-md bg-muted/50">
                <FileText className="size-4 text-muted-foreground" />
              </div>
              <div className="flex flex-col gap-0.5">
                <div className="h-2 w-24 rounded-md bg-muted" />
                <div className="h-2 w-16 rounded-md bg-muted/50" />
              </div>
            </div>
          </AnimatedEmptyState.Carousel>
          <AnimatedEmptyState.Content>
            <AnimatedEmptyState.Title>
              No Instruction Selected
            </AnimatedEmptyState.Title>
            <AnimatedEmptyState.Description>
              Select an instruction from the list or create a new one.
            </AnimatedEmptyState.Description>
          </AnimatedEmptyState.Content>
        </AnimatedEmptyState>
      </div>
    );
  }

  const title = form.watch("name");

  async function handleCopyId() {
    if (!selectedInstruction?.id) return;
    await navigator.clipboard.writeText(selectedInstruction.id);
  }

  return (
    <div className="grid h-full grid-rows-[auto_1fr]">
      <SplitPageLayout.ContentHeader>
        <SplitPageLayout.ContentHeaderMain>
          <SplitPageLayout.ContentTitle>
            {title?.trim() ||
              (isCreateMode ? "New Instruction" : selectedInstruction?.name)}
          </SplitPageLayout.ContentTitle>
        </SplitPageLayout.ContentHeaderMain>

        <SplitPageLayout.ContentHeaderActions>
          <div className="flex items-center gap-2">
            {!isCreateMode && selectedInstruction ? (
              <>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="rounded-md"
                  onClick={() => void handleCopyId()}
                >
                  <Copy />
                  <span className="sr-only">Copy instruction ID</span>
                </Button>

                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="rounded-md"
                      disabled={isDeleting}
                    >
                      <Trash2 />
                      <span className="sr-only">Delete instruction</span>
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent size="sm">
                    <AlertDialogHeader>
                      <AlertDialogTitle>
                        Delete this instruction?
                      </AlertDialogTitle>
                      <AlertDialogDescription>
                        This action removes{" "}
                        <strong>{selectedInstruction.name}</strong> permanently.
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel disabled={isDeleting}>
                        Cancel
                      </AlertDialogCancel>
                      <AlertDialogAction
                        variant="destructive"
                        disabled={isDeleting}
                        onClick={deleteSelectedInstruction}
                      >
                        {isDeleting ? "Deleting..." : "Delete instruction"}
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </>
            ) : null}

            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => void form.submit()}
              disabled={form.isLoading}
            >
              <Save />
              {form.isLoading
                ? "Saving..."
                : isCreateMode
                  ? "Create instruction"
                  : "Save changes"}
            </Button>
          </div>
        </SplitPageLayout.ContentHeaderActions>
      </SplitPageLayout.ContentHeader>

      <SplitPageLayout.ContentBody>
        <SettingsContentContainer className="pb-10">
          {isLoadingContent && !isCreateMode ? (
            <div className="space-y-3">
              <Skeleton className="h-10 w-1/2" />
              <Skeleton className="h-16 w-full" />
              <Skeleton className="h-56 w-full" />
            </div>
          ) : (
            <div className="container mx-auto max-w-3xl space-y-6 py-6">
              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <FormItem className="space-y-2">
                    <FormLabel className="opacity-60">Name</FormLabel>
                    <FormControl>
                      <Input
                        placeholder="Frontend Instructions"
                        className="h-auto rounded-none border-0 bg-transparent px-0 py-0 text-2xl font-semibold shadow-none focus-visible:ring-0"
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="description"
                render={({ field }) => (
                  <FormItem className="space-y-2">
                    <FormLabel className="opacity-60">Description</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder="Summarize what this instruction enforces and when it applies."
                        className="min-h-10 max-h-48 resize-none rounded-none border-0 bg-transparent px-0 py-0 text-sm shadow-none focus-visible:ring-0"
                        {...field}
                        value={field.value ?? ""}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="content"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel className="opacity-60">Content</FormLabel>
                    <FormControl>
                      <MarkdownEditor
                        value={field.value ?? ""}
                        onValueChange={field.onChange}
                        placeholder="Write the full markdown instruction body..."
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          )}
        </SettingsContentContainer>
      </SplitPageLayout.ContentBody>
    </div>
  );
}
