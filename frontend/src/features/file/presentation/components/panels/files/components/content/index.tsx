import * as React from "react"
import { LoaderCircle, RotateCcw, Save } from "lucide-react"
import { toast } from "sonner"
import { aos } from "@/app/aos"
import { useRealtime } from "@/hooks/use-realtime"
import { AnimatedEmptyState } from "@/components/ui/animated-empty-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { SplitPageLayout } from "@/components/ui/split-page-layout"
import type { WorkspaceFile } from "@/features/file/interfaces/file.interfaces"
import {
  isTextContentViewer,
  resolveFileViewer,
} from "@/features/file/presentation/helpers/file-viewer.helper"
import {
  explorerContextsEqual,
  parseExplorerContext,
} from "@/features/file/presentation/helpers/files-explorer.helper"
import { FilesExternalViewer } from "./files-external-viewer.component"
import { FilesImageViewer } from "./files-image-viewer.component"
import { FilesTextViewer } from "./files-text-viewer.component"
import { FilesVideoViewer } from "./files-video-viewer.component"

// Heavy editors/viewers stay out of the initial app.html bundle and load only
// when a matching file type is opened.
const FilesExcalidrawViewer = React.lazy(() =>
  import("./files-excalidraw-viewer.component").then((mod) => ({
    default: mod.FilesExcalidrawViewer,
  })),
)
const FilesJsonViewer = React.lazy(() =>
  import("./files-json-viewer.component").then((mod) => ({
    default: mod.FilesJsonViewer,
  })),
)
const FilesMarkdownViewer = React.lazy(() =>
  import("./files-markdown-viewer.component").then((mod) => ({
    default: mod.FilesMarkdownViewer,
  })),
)
const FilesPdfViewer = React.lazy(() =>
  import("./files-pdf-viewer.component").then((mod) => ({
    default: mod.FilesPdfViewer,
  })),
)

function FilesViewerFallback() {
  return (
    <div className="flex h-full items-center justify-center gap-2 text-sm text-muted-foreground">
      <LoaderCircle className="size-4 animate-spin" />
      Loading editor...
    </div>
  )
}

interface FilesContentProps {
  activeFilePath?: string
  activeFileTabId?: string
  activeFileTabMetadata?: Record<string, string | number | boolean>
}

export function FilesContent({
  activeFilePath,
  activeFileTabId,
  activeFileTabMetadata,
}: FilesContentProps) {
  const explorerContext = React.useMemo(
    () => parseExplorerContext(activeFileTabMetadata?.fileExplorerContext),
    [activeFileTabMetadata?.fileExplorerContext],
  )
  const isReadOnly = Boolean(activeFileTabMetadata?.fileReadOnly)

  const hintedViewer = activeFileTabMetadata?.fileViewer as WorkspaceFile["viewer"] | undefined
  const fileName = activeFilePath?.split("/").pop() ?? activeFilePath ?? ""
  const resolvedViewer =
    hintedViewer ?? (fileName ? resolveFileViewer(fileName) : undefined)
  const shouldReadTextContent = Boolean(
    activeFilePath && resolvedViewer && isTextContentViewer(resolvedViewer),
  )

  const readQuery = aos.client.file.read.useQuery({
    query: {
      path: activeFilePath || "__aos_sidebar_placeholder__",
      context: JSON.stringify(explorerContext),
    },
    enabled: Boolean(activeFilePath) && shouldReadTextContent,
  })

  const [draft, setDraft] = React.useState("")
  const [savedContent, setSavedContent] = React.useState("")

  const { mutate: saveFile, loading: isSaving } = aos.client.file.write.useMutation({
    onSuccess: (response) => {
      // `onSuccess` receives the full `Envelope` — see `aos-facade.ts`'s
      // `useMutation` doc comment.
      const nextContent = response?.data?.content ?? draft
      setDraft(nextContent)
      setSavedContent(nextContent)
      if (activeFilePath) {
        aos.stores.files.actions.clearDraft(activeFilePath)
      }
      syncDirty(false)
      toast.success("File saved.")
    },
    onError: (error: any) => {
      toast.error(error?.error?.message || error?.message || "Unable to save file.")
    },
  })

  const syncDirty = React.useCallback(
    (fileDirty: boolean) => {
      if (!activeFileTabId) return
      const tab = aos.stores.viewport.state.tabs.items.find(
        (item) => item.id === activeFileTabId,
      )
      if (!tab || tab.type !== "file") return
      if (Boolean(tab.metadata?.fileDirty) === fileDirty) return

      aos.stores.viewport.actions.updateTab(activeFileTabId, {
        metadata: {
          ...tab.metadata,
          fileDirty,
        },
      })
    },
    [activeFileTabId],
  )

  React.useEffect(() => {
    if (!shouldReadTextContent) return

    const content = readQuery.data?.content ?? ""
    setDraft(content)
    setSavedContent(content)
    if (activeFilePath) {
      aos.stores.files.actions.setDraft(activeFilePath, content)
    }
    syncDirty(false)
  }, [activeFilePath, readQuery.data?.content, shouldReadTextContent, syncDirty])

  const hasDraftChanges = draft !== savedContent

  React.useEffect(() => {
    if (!activeFilePath || !shouldReadTextContent) return
    aos.stores.files.actions.setDraft(activeFilePath, draft)
    syncDirty(hasDraftChanges)
  }, [activeFilePath, draft, hasDraftChanges, shouldReadTextContent, syncDirty])

  useRealtime(
    "files:changed",
    (payload) => {
      if (!activeFilePath || !shouldReadTextContent) return
      if (!explorerContextsEqual(payload.context, explorerContext)) return

      const touched = payload.changes.some(
        (change: any) =>
          change.path === activeFilePath ||
          activeFilePath.startsWith(`${change.path}/`),
      )
      if (!touched) return

      if (hasDraftChanges) {
        toast.message("File changed on disk", {
          description: "Your local edits were kept. Revert or save to reconcile.",
        })
        return
      }

      void readQuery.refetch()
    },
    [activeFilePath, explorerContext, hasDraftChanges, shouldReadTextContent],
  )

  const fallbackFile = React.useMemo(() => {
    if (!activeFilePath) return null

    return {
      absolutePath: String(activeFileTabMetadata?.fileAbsolutePath ?? activeFilePath),
      browserUrl: String(activeFileTabMetadata?.fileBrowserUrl ?? ""),
      createdAt: new Date(0).toISOString(),
      extension: String(activeFileTabMetadata?.fileExtension ?? ""),
      isEditable: Boolean(activeFileTabMetadata?.fileIsEditable),
      name: String(activeFileTabMetadata?.fileName ?? activeFilePath.split("/").pop() ?? activeFilePath),
      path: activeFilePath,
      parentPath: activeFilePath.includes("/") ? activeFilePath.split("/").slice(0, -1).join("/") : undefined,
      size: 0,
      type: "file" as const,
      updatedAt: new Date(0).toISOString(),
      viewer: (activeFileTabMetadata?.fileViewer as WorkspaceFile["viewer"]) ?? "other",
    } satisfies WorkspaceFile
  }, [activeFilePath, activeFileTabMetadata])

  const file = (readQuery.data?.file as WorkspaceFile | undefined) ?? fallbackFile
  const isLoading = Boolean(activeFilePath) && shouldReadTextContent && readQuery.isLoading

  function handleSave() {
    if (!activeFilePath || isReadOnly) return

    saveFile({
      body: {
        path: activeFilePath,
        content: draft,
        context: explorerContext,
      },
    })
  }

  function handleDiscard() {
    setDraft(savedContent)
    if (activeFilePath) {
      aos.stores.files.actions.setDraft(activeFilePath, savedContent)
    }
    syncDirty(false)
  }

  function handleOpenExternal() {
    if (!file?.browserUrl) return

    const tabId = aos.stores.viewport.actions.createTab({
      type: "browser",
      title: file.name,
      url: file.browserUrl,
      closable: true,
    })

    if (tabId) {
      aos.stores.viewport.actions.setActiveTab(tabId)
    }
  }

  if (!activeFilePath || !file) {
    return (
      <div className="flex h-full items-center justify-center p-8">
        <AnimatedEmptyState className="border-none shadow-none">
          <AnimatedEmptyState.Content>
            <AnimatedEmptyState.Title>No file selected</AnimatedEmptyState.Title>
            <AnimatedEmptyState.Description>
              Choose a file in the sidebar explorer to inspect its content.
            </AnimatedEmptyState.Description>
          </AnimatedEmptyState.Content>
        </AnimatedEmptyState>
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center gap-2 text-sm text-muted-foreground">
        <LoaderCircle className="size-4 animate-spin" />
        Loading file content...
      </div>
    )
  }

  if (file.viewer !== "text" && file.viewer !== "markdown" && file.viewer !== "json") {
    if (file.viewer === "image") {
      return <FilesImageViewer file={file} />
    }

    if (file.viewer === "pdf") {
      return (
        <React.Suspense fallback={<FilesViewerFallback />}>
          <FilesPdfViewer file={file} onOpenExternal={handleOpenExternal} />
        </React.Suspense>
      )
    }

    if (file.viewer === "video") {
      return <FilesVideoViewer file={file} />
    }

    if (file.viewer === "excalidraw") {
      return (
        <React.Suspense fallback={<FilesViewerFallback />}>
          <FilesExcalidrawViewer file={file} content={draft} />
        </React.Suspense>
      )
    }

    return <FilesExternalViewer file={file} onOpenExternal={handleOpenExternal} />
  }

  return (
    <div className="grid h-full min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden">
      <SplitPageLayout.ContentHeader>
        <SplitPageLayout.ContentHeaderMain>
          <SplitPageLayout.ContentTitle className="flex items-center gap-2">
            {file.path}
          </SplitPageLayout.ContentTitle>
        </SplitPageLayout.ContentHeaderMain>

        <SplitPageLayout.ContentHeaderActions>
          {isReadOnly ? <Badge variant="secondary">Read-only</Badge> : null}
          {readQuery.isError ? <Badge variant="destructive">Load failed</Badge> : null}

          <Button
            size="icon"
            variant="ghost"
            onClick={handleDiscard}
            disabled={!hasDraftChanges || isSaving || isReadOnly}
          >
            <RotateCcw data-icon="inline-start" />
          </Button>
          <Button
            size="icon"
            variant="ghost"
            onClick={handleSave}
            disabled={!hasDraftChanges || isSaving || isReadOnly}
          >
            {isSaving ? (
              <LoaderCircle data-icon="inline-start" className="animate-spin" />
            ) : (
              <Save data-icon="inline-start" />
            )}
          </Button>
        </SplitPageLayout.ContentHeaderActions>
      </SplitPageLayout.ContentHeader>

      <SplitPageLayout.ContentBody
        className={
          file.viewer === "json"
            ? "min-h-0 overflow-hidden"
            : "min-h-0 overflow-y-auto"
        }
      >
        <React.Suspense fallback={<FilesViewerFallback />}>
          {file.viewer === "markdown" ? (
            <FilesMarkdownViewer
              content={draft}
              readOnly={isReadOnly}
              onChange={setDraft}
            />
          ) : file.viewer === "json" ? (
            <FilesJsonViewer
              content={draft}
              file={file}
              readOnly={isReadOnly}
              onChange={setDraft}
            />
          ) : (
            <FilesTextViewer
              content={draft}
              file={file}
              isLoading={readQuery.isFetching && !readQuery.data}
              readOnly={isReadOnly}
              onChange={setDraft}
            />
          )}
        </React.Suspense>
      </SplitPageLayout.ContentBody>
    </div>
  )
}
