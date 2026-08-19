import { Bot, Copy, Save, Trash2 } from "lucide-react";
import { AnimatedEmptyState } from "@/components/ui/animated-empty-state";
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
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { AgentContentTabs } from "./components/tabs";
import { AvatarAgentFallback } from "@/components/ui/avatar";
import { SettingsContentContainer } from "../../../../../content-container";
import { useAgents } from "../../contexts/agents.context";

export function SelectedAgentContent() {
  const {
    selectedAgent,
    isCreateMode,
    isLoadingContent,
    isDeleting,
    form,
    deleteSelectedAgent,
  } = useAgents();

  if (!selectedAgent && !isCreateMode) {
    return (
      <div className="flex h-full items-center justify-center p-8">
        <AnimatedEmptyState className="border-none shadow-none">
          <AnimatedEmptyState.Carousel>
            <div className="flex items-center gap-3">
              <div className="flex size-8 items-center justify-center rounded-md bg-muted/50">
                <Bot className="size-4 text-muted-foreground" />
              </div>
              <div className="flex flex-col gap-0.5">
                <div className="h-2 w-24 rounded-md bg-muted" />
                <div className="h-2 w-16 rounded-md bg-muted/50" />
              </div>
            </div>
          </AnimatedEmptyState.Carousel>
          <AnimatedEmptyState.Content>
            <AnimatedEmptyState.Title>
              No Agent Selected
            </AnimatedEmptyState.Title>
            <AnimatedEmptyState.Description>
              Select an agent from the list or create a new one.
            </AnimatedEmptyState.Description>
          </AnimatedEmptyState.Content>
        </AnimatedEmptyState>
      </div>
    );
  }

  const title = form.watch("name");
  const image = form.watch("image");

  async function handleCopyId() {
    if (!selectedAgent?.id) return;
    await navigator.clipboard.writeText(selectedAgent.id);
  }

  return (
    <div className="grid h-full grid-rows-[auto_1fr]">
      <SplitPageLayout.ContentHeader>
        <SplitPageLayout.ContentHeaderMain>
          <SplitPageLayout.ContentTitle className="flex items-center gap-2">
            <AvatarAgentFallback
              name={(title || selectedAgent?.id || "agent").toLowerCase()}
              image={image}
            />
            {title?.trim() ||
              (isCreateMode ? "New Agent" : selectedAgent?.name)}
          </SplitPageLayout.ContentTitle>
        </SplitPageLayout.ContentHeaderMain>

        <SplitPageLayout.ContentHeaderActions>
          <div className="flex items-center gap-2">
            {!isCreateMode && selectedAgent ? (
              <>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="rounded-md"
                  onClick={() => void handleCopyId()}
                >
                  <Copy />
                  <span className="sr-only">Copy agent ID</span>
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
                      <span className="sr-only">Delete agent</span>
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent size="sm">
                    <AlertDialogHeader>
                      <AlertDialogTitle>Delete this agent?</AlertDialogTitle>
                      <AlertDialogDescription>
                        This action removes{" "}
                        <strong>{selectedAgent.name}</strong> permanently.
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel disabled={isDeleting}>
                        Cancel
                      </AlertDialogCancel>
                      <AlertDialogAction
                        variant="destructive"
                        disabled={isDeleting}
                        onClick={deleteSelectedAgent}
                      >
                        {isDeleting ? "Deleting..." : "Delete agent"}
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
                  ? "Create agent"
                  : "Save changes"}
            </Button>
          </div>
        </SplitPageLayout.ContentHeaderActions>
      </SplitPageLayout.ContentHeader>

      <SplitPageLayout.ContentBody>
        <SettingsContentContainer>
          <AgentContentTabs
            agent={selectedAgent}
            form={form}
            isCreateMode={isCreateMode}
            isLoadingInstructions={isLoadingContent}
          />
        </SettingsContentContainer>
      </SplitPageLayout.ContentBody>
    </div>
  );
}
