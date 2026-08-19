import { AosStore } from "@/app/builders/store";
import type { FractalFileExplorerContext } from "@/features/file/interfaces/file.interfaces";

export type FilesClipboardMode = "cut" | "copy";

export interface FilesClipboardState {
  mode: FilesClipboardMode;
  paths: string[];
  context: FractalFileExplorerContext;
}

export const FilesStore = AosStore.create("files")
  .withState({
    explorerContext: { type: "main" } as FractalFileExplorerContext,
    clipboard: null as FilesClipboardState | null,
    draftsByPath: {} as Record<string, string>,
  })
  .withNamespace({
    resolver: ({ namespaces }) => namespaces.workspaceId,
    strategy: "memory-partition",
  })
  .withPersistence({
    enabled: true,
    storage: "localstorage",
    pick: (state) => ({ explorerContext: state.explorerContext }),
  })
  .addAction("setExplorerContext", (ctx) => (explorerContext: FractalFileExplorerContext) => {
    ctx.state.set({ explorerContext, clipboard: null });
  })
  .addAction(
    "setClipboard",
    (ctx) =>
      (clipboard: FilesClipboardState | null) => {
        ctx.state.set({ clipboard });
      },
  )
  .addAction("clearClipboard", (ctx) => () => {
    ctx.state.set({ clipboard: null });
  })
  .addAction(
    "setDraft",
    (ctx) =>
      (path: string, content: string) => {
        ctx.state.set((state) => ({
          draftsByPath: {
            ...state.draftsByPath,
            [path]: content,
          },
        }));
      },
  )
  .addAction("clearDraft", (ctx) => (path: string) => {
    ctx.state.set((state) => {
      const next = { ...state.draftsByPath };
      delete next[path];
      return { draftsByPath: next };
    });
  })
  .build();
