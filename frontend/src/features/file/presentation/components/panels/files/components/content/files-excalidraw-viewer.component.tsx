import React from "react"
import { LoaderCircle, Save } from "lucide-react"
import { toast } from "sonner"
import { aos } from "@/app/aos"
import { Button } from "@/components/ui/button"
import type { WorkspaceFile } from "@/features/file/interfaces/file.interfaces"
import { Excalidraw } from "@excalidraw/excalidraw";
import { t } from "@/lib/i18n";

interface FilesExcalidrawViewerProps {
  file: WorkspaceFile
  content?: string
}

const DEFAULT_SCENE = {
  type: "excalidraw",
  version: 2,
  source: "aos",
  elements: [],
  appState: {
    collaborators: new Map(),
  },
  files: {},
}

function normalizeAppStateForEditor(appState: any) {
  return {
    ...(appState ?? {}),
    collaborators: new Map(),
  }
}

function normalizeAppStateForSave(appState: any) {
  const next = { ...(appState ?? {}) }
  delete next.collaborators
  return next
}

export function FilesExcalidrawViewer({ file, content }: FilesExcalidrawViewerProps) {
  const [initialData, setInitialData] = React.useState<any>(DEFAULT_SCENE)
  const latestSceneRef = React.useRef<any>(DEFAULT_SCENE)
  const themeState = aos.stores.theme.useState()

  const excalidrawTheme = React.useMemo<"light" | "dark">(() => {
    if (themeState.mode === "dark") return "dark"
    if (themeState.mode === "light") return "light"
    if (typeof window !== "undefined" && window.matchMedia("(prefers-color-scheme: dark)").matches) {
      return "dark"
    }
    return "light"
  }, [themeState.mode])

  const { mutate: saveFile, loading: isSaving } = aos.client.file.write.useMutation({
    onSuccess: () => {
      toast.success(t("Drawing saved."))
    },
    onError: (error: any) => {
      toast.error(error?.error?.message || error?.message || "Unable to save drawing.")
    },
  })

  React.useEffect(() => {
    if (!content?.trim()) {
      setInitialData(DEFAULT_SCENE)
      latestSceneRef.current = DEFAULT_SCENE
      return
    }

    try {
      const parsed = JSON.parse(content)
      const nextScene = {
        ...DEFAULT_SCENE,
        ...parsed,
        elements: Array.isArray(parsed?.elements) ? parsed.elements : [],
        appState: normalizeAppStateForEditor(parsed?.appState),
        files: parsed?.files ?? {},
      }
      setInitialData(nextScene)
      latestSceneRef.current = nextScene
    } catch {
      setInitialData(DEFAULT_SCENE)
      latestSceneRef.current = DEFAULT_SCENE
    }
  }, [content, file.path])

  function handleSave(nextScene: any) {
    saveFile({
      body: {
        path: file.path,
        content: JSON.stringify(
          {
            type: "excalidraw",
            version: 2,
            source: "aos",
            elements: nextScene.elements ?? [],
            appState: normalizeAppStateForSave(nextScene.appState),
            files: nextScene.files ?? {},
          },
          null,
          2,
        ),
      },
    })
  }

  return (
    <div className="grid h-full grid-rows-[auto_1fr]">
      <div className="flex items-center justify-end gap-2 border-b px-3 py-2">
        <Button size="sm" onClick={() => handleSave(latestSceneRef.current)} disabled={isSaving}>
          {isSaving ? (
            <LoaderCircle data-icon="inline-start" className="size-4 animate-spin" />
          ) : (
            <Save data-icon="inline-start" className="size-4" />
          )}
          {t("Save drawing")}
        </Button>
      </div>

      <div className="h-full">
        <Excalidraw
          key={file.path}
          initialData={initialData}
          theme={excalidrawTheme}
          onChange={(elements: readonly any[], appState: any, files: any) => {
            latestSceneRef.current = {
              ...latestSceneRef.current,
              elements,
              appState,
              files,
            }
          }}
        />
      </div>
    </div>
  )
}
