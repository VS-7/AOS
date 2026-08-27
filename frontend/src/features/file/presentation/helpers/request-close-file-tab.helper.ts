import { toast } from "sonner";
import { api } from "@/lib/aos-facade";
import { stores } from "@/app/lib/stores";
import { parseExplorerContext } from "@/features/file/presentation/helpers/files-explorer.helper";
import { t } from "@/lib/i18n";

type PromptUnsaved = (options?: {
  title?: string;
  description?: string;
  saveText?: string;
  discardText?: string;
  cancelText?: string;
}) => Promise<"save" | "discard" | "cancel">;

/**
 * Closes a viewport tab, prompting Save / Don't Save / Cancel when the file is dirty.
 */
export async function requestCloseFileTab(
  tabId: string,
  promptUnsaved: PromptUnsaved,
): Promise<boolean> {
  const tab = stores.viewport.state.tabs.items.find((item) => item.id === tabId);
  if (!tab) return false;

  if (tab.type !== "file" || !tab.metadata?.fileDirty) {
    stores.viewport.actions.closeTab(tabId);
    return true;
  }

  const filePath =
    typeof tab.metadata.filePath === "string" ? tab.metadata.filePath : null;
  const fileName =
    typeof tab.metadata.fileName === "string"
      ? tab.metadata.fileName
      : filePath ?? "file";

  const choice = await promptUnsaved({
    title: "Unsaved changes",
    description: `Save changes to "${fileName}" before closing?`,
  });

  if (choice === "cancel") return false;

  if (choice === "save") {
    if (!filePath) {
      toast.error(t("Unable to save: missing file path."));
      return false;
    }

    const draft = stores.files.state.draftsByPath[filePath];
    if (typeof draft !== "string") {
      toast.error(t("Unable to save: draft content is missing."));
      return false;
    }

    const context = parseExplorerContext(tab.metadata.fileExplorerContext);

    try {
      const response = await api.file.write.mutate({
        body: {
          path: filePath,
          content: draft,
          context,
        },
      });

      if (response.error) {
        toast.error(
          (response.error as { message?: string })?.message ||
            "Unable to save file.",
        );
        return false;
      }

      stores.files.actions.clearDraft(filePath);
      stores.viewport.actions.updateTab(tabId, {
        metadata: {
          ...tab.metadata,
          fileDirty: false,
        },
      });
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : "Unable to save file.",
      );
      return false;
    }
  }

  if (filePath) {
    stores.files.actions.clearDraft(filePath);
  }

  stores.viewport.actions.closeTab(tabId);
  return true;
}
