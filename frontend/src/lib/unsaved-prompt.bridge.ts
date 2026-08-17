type UnsavedPromptResult = "save" | "discard" | "cancel";

type PromptUnsaved = (options?: {
  title?: string;
  description?: string;
  saveText?: string;
  discardText?: string;
  cancelText?: string;
}) => Promise<UnsavedPromptResult>;

let unsavedPromptHandler: PromptUnsaved | null = null;

export function setUnsavedPromptHandler(handler: PromptUnsaved | null) {
  unsavedPromptHandler = handler;
}

export function getUnsavedPromptHandler(): PromptUnsaved | null {
  return unsavedPromptHandler;
}
