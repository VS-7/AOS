import * as React from "react";
import { FilePlus, FolderPlus, GitCompareArrows, LoaderCircle, RefreshCw } from "lucide-react";
import { toast } from "sonner";
import { prepareFileTreeInput } from "@pierre/trees";
import { FileTree, useFileTree } from "@pierre/trees/react";
import type {
  FileTreeBuiltInIconSet,
  FileTreeDirectoryHandle,
} from "@pierre/trees";
import { AnimatedEmptyState } from "@/components/ui/animated-empty-state";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  SidebarGroup,
  SidebarGroupContent,
} from "@/components/ui/sidebar";
import { api } from "@/lib/aos-facade";
import { aos } from "@/app/aos";
import { useRealtime } from "@/hooks/use-realtime";
import type {
  FractalFile,
  FractalFileExplorerContext,
  FractalFileExplorerSnapshot,
} from "@/features/file/interfaces/file.interfaces";
import { FilesCreateNodeDialog } from "@/features/file/presentation/components/panels/files/components/dialogs/files-create-node.dialog";
import {
  buildFileTabMetadata,
  basenameOf,
  explorerContextsEqual,
  FILES_TREE_THEME_STYLE,
  formatExplorerContextLabel,
  getAncestorPaths,
  joinWorkspacePath,
  lookupPathIndex,
  parseExplorerContext,
  resolveCreateParentPath,
  serializeExplorerContext,
  synthesizeFileFromPath,
} from "@/features/file/presentation/helpers/files-explorer.helper";
import type { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";
import { SidebarActionButton } from "./sidebar-action-button";
import {
  FilesCreateNodeRequest,
  FilesTreeContextMenu,
} from "./files-tree-context-menu";
import { openChangesTab } from "@/features/file/presentation/helpers/open-changes-tab.helper";

function getTabFilePath(tab: ViewportTabState) {
  return typeof tab.metadata?.filePath === "string" ? tab.metadata.filePath : undefined;
}

function toTreeIconSet(set: string): FileTreeBuiltInIconSet | "none" {
  if (set === "none") return "none";
  if (set === "minimal" || set === "standard" || set === "complete") {
    return set;
  }
  return "complete";
}

function toTreeIcons(set: string, colored: boolean) {
  return {
    set: toTreeIconSet(set),
    colored,
  };
}

function normalizeExplorerSnapshot(
  data: unknown,
): FractalFileExplorerSnapshot | undefined {
  if (!data || typeof data !== "object") {
    return undefined;
  }

  const candidate =
    "paths" in data
      ? data
      : "data" in data &&
          typeof (data as { data?: unknown }).data === "object" &&
          (data as { data?: unknown }).data !== null &&
          "paths" in ((data as { data: object }).data)
        ? (data as { data: object }).data
        : null;

  if (!candidate || !("paths" in candidate)) {
    return undefined;
  }

  return candidate as FractalFileExplorerSnapshot;
}

export function FilesExplorerGroup() {
  // useFileTree only reads some options on first model creation — remount when toggled.
  return <FilesExplorerGroupInner key="files-explorer-collapsed" />;
}

function FilesExplorerGroupInner() {
  const explorerContext = aos.stores.files.useState((state) =>
    parseExplorerContext(state.explorerContext),
  );
  const clipboard = aos.stores.files.useState((state) => state.clipboard);
  const themeIcons = aos.stores.theme.useState((state) => state.icons ?? { set: "complete", colored: true });
  const viewportTabs = aos.stores.viewport.useState((state) => state.tabs);

  const activeFilePath = aos.stores.viewport.useState((state) => {
    const activeTab = state.tabs.items.find((tab) => tab.id === state.tabs.current);
    return activeTab?.type === "file" ? getTabFilePath(activeTab) : undefined;
  });

  const [createRequest, setCreateRequest] = React.useState<FilesCreateNodeRequest | null>(null);
  const [emptyMenuAnchor, setEmptyMenuAnchor] = React.useState<{ x: number; y: number } | null>(null);
  const refetchTimerRef = React.useRef<number | undefined>(undefined);
  const treeResizeObserverRef = React.useRef<ResizeObserver | null>(null);
  const [treeHostHeight, setTreeHostHeight] = React.useState(0);

  const setTreeHostNode = React.useCallback((node: HTMLDivElement | null) => {
    if (treeResizeObserverRef.current) {
      treeResizeObserverRef.current.disconnect();
      treeResizeObserverRef.current = null;
    }

    if (!node) {
      setTreeHostHeight(0);
      return;
    }

    const observer = new ResizeObserver((entries) => {
      const nextHeight = Math.floor(entries[0]?.contentRect.height ?? 0);
      setTreeHostHeight((current) => (current === nextHeight ? current : nextHeight));
    });

    observer.observe(node);
    treeResizeObserverRef.current = observer;
    setTreeHostHeight(Math.floor(node.getBoundingClientRect().height));
  }, []);

  const explorerQuery = aos.client.file.explorer.useQuery({
    query: {
      context: serializeExplorerContext(explorerContext),
      includeIgnored: false,
      includeContexts: true,
    },
  });

  const snapshot = normalizeExplorerSnapshot(explorerQuery.data?.snapshot);
  const snapshotRef = React.useRef(snapshot);
  snapshotRef.current = snapshot;

  const explorerContextRef = React.useRef(explorerContext);
  explorerContextRef.current = explorerContext;

  const readOnlyRef = React.useRef(snapshot?.readOnly ?? false);
  readOnlyRef.current = snapshot?.readOnly ?? false;

  const openFileTabRef = React.useRef<(file: FractalFile) => void>(() => {});

  const { model } = useFileTree({
    paths: [],
    search: false,
    density: "compact",
    initialExpansion: "closed",
    icons: toTreeIcons(themeIcons.set, themeIcons.colored),
    unsafeCSS: `
      [data-file-tree-search-container] {
        display: none !important;
      }
    `,
    composition: {
      contextMenu: {
        enabled: true,
        onOpen: () => {
          setEmptyMenuAnchor(null);
        },
      },
    },
    dragAndDrop: {
      canDrag: () => !readOnlyRef.current,
      canDrop: () => !readOnlyRef.current,
      onDropComplete: (event: any) => {
        const targetDir = event.target.directoryPath ?? "";

        for (const fromPath of event.draggedPaths) {
          const toPath = joinWorkspacePath(targetDir, basenameOf(fromPath));
          void handleTreeMove(
            fromPath,
            toPath,
            explorerContextRef.current,
            () => explorerQuery.refetch(),
          );
        }
      },
      onDropError: (error: any) => toast.error(error),
    },
    renaming: {
      canRename: () => !readOnlyRef.current,
      onRename: (event: any) => {
        void handleTreeMove(
          event.sourcePath,
          event.destinationPath,
          explorerContextRef.current,
          () => explorerQuery.refetch(),
        );
      },
      onError: (error: any) => toast.error(error),
    },
    onSelectionChange: (selectedPaths: any) => {
      const path = selectedPaths.at(-1);
      if (!path) return;

      const indexed = lookupPathIndex(snapshotRef.current?.pathIndex, path);
      const file =
        indexed ??
        synthesizeFileFromPath(path, snapshotRef.current?.paths ?? []);
      if (!file || file.type !== "file") return;

      // `indexed` (from `pathIndex`, loosely typed by design — see
      // `file.interfaces.ts`'s `FractalFileExplorerSnapshot` doc comment)
      // doesn't structurally match `FractalFile` even after the `type`
      // narrowing above.
      openFileTabRef.current(file as any);
    },
  });

  function openFileTab(file: FractalFile) {
    const readOnly = snapshotRef.current?.readOnly ?? false;
    const metadata = buildFileTabMetadata(file, explorerContextRef.current, {
      fileReadOnly: readOnly,
    });

    const existingTab = viewportTabs.items.find(
      (tab) => tab.type === "file" && getTabFilePath(tab) === file.path,
    );

    if (existingTab) {
      aos.stores.viewport.actions.setActiveTab(existingTab.id);
      return;
    }

    const placeholderTab = viewportTabs.items.find(
      (tab) => tab.type === "file" && !getTabFilePath(tab),
    );

    if (placeholderTab) {
      aos.stores.viewport.actions.updateTab(placeholderTab.id, {
        title: file.name,
        metadata: {
          ...placeholderTab.metadata,
          ...metadata,
        },
      });
      aos.stores.viewport.actions.setActiveTab(placeholderTab.id);
      return;
    }

    const createdTabId = aos.stores.viewport.actions.createTab({
      type: "file",
      title: file.name,
      closable: true,
      metadata,
    });

    if (createdTabId) {
      aos.stores.viewport.actions.setActiveTab(createdTabId);
    }
  }

  openFileTabRef.current = openFileTab;

  React.useEffect(() => {
    model.closeSearch();
  }, [model]);

  React.useEffect(() => {
    model.setIcons(toTreeIcons(themeIcons.set, themeIcons.colored));
  }, [model, themeIcons.colored, themeIcons.set]);

  React.useEffect(() => {
    if (!snapshot?.paths?.length) return;

    // Pierre marks directories only when paths end with `/`. Backend should emit
    // that, but normalize from pathIndex so folders never render as file+dir.
    const pathsForTree = snapshot.paths.map((entryPath) => {
      const indexed =
        snapshot.pathIndex[entryPath] ??
        snapshot.pathIndex[`${entryPath}/`] ??
        snapshot.pathIndex[entryPath.replace(/\/+$/, "")];
      if (indexed?.type === "directory" && !entryPath.endsWith("/")) {
        return `${entryPath}/`;
      }
      return entryPath;
    });

    const preparedInput = prepareFileTreeInput(pathsForTree);

    model.resetPaths({
      preparedInput,
      initialExpandedPaths: activeFilePath
        ? getAncestorPaths(activeFilePath)
        : undefined,
    });
    // Only reset when the explorer snapshot identity changes — not on every tab switch.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- activeFilePath used for initial expand only
  }, [model, snapshot?.paths]);

  React.useEffect(() => {
    // Pierre docs: update Git signals via setGitStatus after mount / path resets.
    model.setGitStatus(snapshot?.gitStatus ?? []);
  }, [model, snapshot?.gitStatus]);

  React.useEffect(() => {
    if (!activeFilePath) return;

    for (const ancestor of getAncestorPaths(activeFilePath)) {
      const handle = model.getItem(ancestor);
      if (!handle?.isDirectory()) continue;

      const directory = handle as FileTreeDirectoryHandle;
      if (!directory.isExpanded()) {
        directory.expand();
      }
    }

    model.scrollToPath(activeFilePath, { focus: false });
  }, [activeFilePath, model, snapshot?.paths]);

  useRealtime(
    "files:changed",
    (payload) => {
      if (
        payload.context &&
        !explorerContextsEqual(payload.context, explorerContextRef.current)
      ) {
        return;
      }

      window.clearTimeout(refetchTimerRef.current);
      refetchTimerRef.current = window.setTimeout(() => {
        void explorerQuery.refetch();
      }, 350);
    },
    [serializeExplorerContext(explorerContext)],
  );

  React.useEffect(() => {
    if (!emptyMenuAnchor) return;

    function handlePointerDown(event: MouseEvent) {
      const clickedMenu = event.composedPath().some(
        (node) =>
          node instanceof Element &&
          node.getAttribute("data-file-tree-context-menu-root") === "true",
      );
      if (clickedMenu) return;
      setEmptyMenuAnchor(null);
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setEmptyMenuAnchor(null);
      }
    }

    window.addEventListener("pointerdown", handlePointerDown);
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("pointerdown", handlePointerDown);
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [emptyMenuAnchor]);

  const isInitialLoading = explorerQuery.isLoading && !snapshot;
  const hasError = explorerQuery.isError;
  const isEmpty = (snapshot?.paths.length ?? 0) === 0;

  async function handleRefresh() {
    await explorerQuery.refetch();
  }

  function handleContextChange(value: string) {
    aos.stores.files.actions.setExplorerContext(parseExplorerContext(value));
  }

  function handleCreateNode(request: FilesCreateNodeRequest) {
    setCreateRequest(request);
    setEmptyMenuAnchor(null);
  }

  function handleHeaderCreate(type: "file" | "directory") {
    if (snapshot?.readOnly) return;

    const parentPath = resolveCreateParentPath({
      focusedPath: model.getFocusedPath(),
      selectedPaths: model.getSelectedPaths(),
      pathIndex: snapshot?.pathIndex,
      isDirectoryPath: (path) => model.getItem(path)?.isDirectory() === true,
    });

    handleCreateNode({ parentPath, type });
  }

  function handleFilesChanged() {
    void explorerQuery.refetch();
  }

  return (
    <SidebarGroup className="flex h-full min-h-0 flex-1 flex-col overflow-hidden p-0 px-3 pt-2 pb-0">
      <div className="flex shrink-0 items-center gap-2 pl-2 pb-2">
        <Select
          value={serializeExplorerContext(explorerContext)}
          onValueChange={handleContextChange}
        >
          <SelectTrigger
            size="sm"
            className="h-auto w-fit max-w-36 shrink-0 border-0 bg-transparent px-0 py-0 text-[11px] font-medium tracking-wide text-sidebar-foreground/70 shadow-none focus-visible:border-0 focus-visible:ring-0 data-[size=sm]:h-auto"
          >
            <SelectValue placeholder="main">
              {formatExplorerContextLabel(explorerContext, snapshot)}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              <SelectLabel>Workspace</SelectLabel>
              <SelectItem value={serializeExplorerContext({ type: "main" })}>
                main
              </SelectItem>
            </SelectGroup>
            {(snapshot?.tasks?.length ?? 0) > 0 ? (
              <SelectGroup>
                <SelectLabel>Tasks</SelectLabel>
                {snapshot?.tasks?.map((task) => (
                  <SelectItem
                    key={task.id}
                    value={serializeExplorerContext({
                      type: "task",
                      taskId: task.id,
                    })}
                  >
                    {task.title || task.id}
                  </SelectItem>
                ))}
              </SelectGroup>
            ) : null}
            {(snapshot?.branches?.length ?? 0) > 0 ? (
              <SelectGroup>
                <SelectLabel>Branches</SelectLabel>
                {snapshot?.branches?.map((branch) => (
                  <SelectItem
                    key={branch}
                    value={serializeExplorerContext({ type: "branch", branch })}
                  >
                    {branch}
                  </SelectItem>
                ))}
              </SelectGroup>
            ) : null}
          </SelectContent>
        </Select>
        <div className="ml-auto flex items-center gap-0.5">
          <SidebarActionButton
            icon={FilePlus}
            label="New file"
            disabled={snapshot?.readOnly}
            onClick={() => handleHeaderCreate("file")}
          />
          <SidebarActionButton
            icon={FolderPlus}
            label="New folder"
            disabled={snapshot?.readOnly}
            onClick={() => handleHeaderCreate("directory")}
          />
          <SidebarActionButton
            icon={GitCompareArrows}
            label="Open changes"
            onClick={() => openChangesTab(explorerContext)}
          />
          <SidebarActionButton
            icon={explorerQuery.isFetching ? LoaderCircle : RefreshCw}
            label="Refresh files"
            onClick={() => void handleRefresh()}
          />
        </div>
      </div>

      <SidebarGroupContent className="flex min-h-0 flex-1 flex-col overflow-hidden px-0 pb-0">
        {isInitialLoading ? (
          <div className="flex items-center gap-2 px-2 py-3 text-sm text-muted-foreground">
            <LoaderCircle className="size-4 animate-spin" />
            Loading files...
          </div>
        ) : hasError && !snapshot ? (
          <AnimatedEmptyState className="border-none shadow-none">
            <AnimatedEmptyState.Content>
              <AnimatedEmptyState.Title>Unable to load files</AnimatedEmptyState.Title>
              <AnimatedEmptyState.Description>
                The sidebar could not resolve the workspace tree from the backend.
                {explorerQuery.error ? (
                  <>
                    {" "}
                    {(explorerQuery.error as { message?: string })?.message ||
                      String(explorerQuery.error)}
                  </>
                ) : null}
              </AnimatedEmptyState.Description>
            </AnimatedEmptyState.Content>
          </AnimatedEmptyState>
        ) : isEmpty ? (
          <AnimatedEmptyState className="border-none shadow-none">
            <AnimatedEmptyState.Content>
              <AnimatedEmptyState.Title>No files available</AnimatedEmptyState.Title>
              <AnimatedEmptyState.Description>
                This explorer context is empty right now.
              </AnimatedEmptyState.Description>
            </AnimatedEmptyState.Content>
          </AnimatedEmptyState>
        ) : (
          <>
          <div
            ref={setTreeHostNode}
            className="relative min-h-0 flex-1 overflow-hidden"
            onContextMenu={(event) => {
              const composedPath = event.nativeEvent.composedPath();
              const clickedMenu = composedPath.some(
                (node) =>
                  node instanceof Element &&
                  node.getAttribute("data-file-tree-context-menu-root") ===
                    "true",
              );
              if (clickedMenu) return;

              // Pierre owns item/folder context menus. Only handle true empty space.
              const clickedTreeItem = composedPath.some(
                (node) =>
                  node instanceof Element &&
                  node.getAttribute("data-type") === "item",
              );
              if (clickedTreeItem) {
                setEmptyMenuAnchor(null);
                return;
              }

              event.preventDefault();
              event.stopPropagation();
              setEmptyMenuAnchor({
                x: event.clientX,
                y: event.clientY,
              });
            }}
          >
            {treeHostHeight > 0 ? (
              <FileTree
                model={model}
                className="block w-full"
                style={{
                  ...FILES_TREE_THEME_STYLE,
                  height: `${treeHostHeight}px`,
                }}
                renderContextMenu={(item: any, context: any) => {
                  queueMicrotask(() => setEmptyMenuAnchor(null));
                  return (
                    <FilesTreeContextMenu
                      item={item}
                      context={context}
                      model={model}
                      snapshot={snapshot}
                      explorerContext={explorerContext}
                      clipboard={clipboard}
                      onCreateNode={handleCreateNode}
                      onClipboardChange={
                        aos.stores.files.actions.setClipboard
                      }
                      onFilesChanged={handleFilesChanged}
                    />
                  );
                }}
              />
            ) : null}
          </div>
          {emptyMenuAnchor ? (
            <FilesTreeContextMenu
              context={{
                anchorElement: document.body,
                anchorRect: {
                  top: emptyMenuAnchor.y,
                  right: emptyMenuAnchor.x,
                  bottom: emptyMenuAnchor.y,
                  left: emptyMenuAnchor.x,
                  width: 0,
                  height: 0,
                  x: emptyMenuAnchor.x,
                  y: emptyMenuAnchor.y,
                },
                close: () => setEmptyMenuAnchor(null),
                restoreFocus: () => undefined,
              }}
              model={model}
              snapshot={snapshot}
              explorerContext={explorerContext}
              clipboard={clipboard}
              onCreateNode={handleCreateNode}
              onClipboardChange={aos.stores.files.actions.setClipboard}
              onFilesChanged={handleFilesChanged}
              anchor={emptyMenuAnchor}
            />
          ) : null}
          </>
        )}
      </SidebarGroupContent>

      <FilesCreateNodeDialog
        open={createRequest != null}
        parentPath={createRequest?.parentPath ?? ""}
        defaultType={createRequest?.type ?? "file"}
        explorerContext={explorerContext}
        onOpenChange={(open) => {
          if (!open) {
            setCreateRequest(null);
          }
        }}
        onCreated={() => {
          void explorerQuery.refetch();
        }}
      />
    </SidebarGroup>
  );
}

async function handleTreeMove(
  fromPath: string,
  toPath: string,
  explorerContext: FractalFileExplorerContext,
  resync: () => void,
) {
  if (!fromPath || !toPath || fromPath === toPath) return;

  const response = await api.file.move.mutate({
    body: {
      fromPath,
      toPath,
      context: explorerContext,
    },
  });

  if (response.error) {
    toast.error(
      (response.error as { message?: string })?.message ||
        `Unable to move "${fromPath}".`,
    );
    resync();
    return;
  }

  toast.success("Moved.");
}
