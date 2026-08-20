import React from "react"
import { ExternalLink, Search, ZoomIn, ZoomOut } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import type { WorkspaceFile } from "@/features/file/interfaces/file.interfaces"

interface FilesPdfViewerProps {
  file: WorkspaceFile
  onOpenExternal: () => void
}

const MIN_ZOOM = 0.5
const MAX_ZOOM = 3
const ZOOM_STEP = 0.1

export function FilesPdfViewer({ file, onOpenExternal }: FilesPdfViewerProps) {
  const [zoom, setZoom] = React.useState(1)
  const [search, setSearch] = React.useState("")

  const contentUrl = React.useMemo(
    () => `/api/files/content?path=${encodeURIComponent(file.path)}`,
    [file.path],
  )

  const iframeUrl = React.useMemo(() => {
    const hashParts = [`zoom=${Math.round(zoom * 100)}`]

    if (search.trim()) {
      hashParts.push(`search=${encodeURIComponent(search.trim())}`)
    }

    return `${contentUrl}#${hashParts.join("&")}`
  }, [contentUrl, search, zoom])

  return (
    <div className="grid h-full grid-rows-[auto_1fr]">
      <div className="flex flex-wrap items-center gap-2 border-b px-3 py-2">
        <Button size="sm" variant="outline" onClick={() => setZoom((value) => Math.max(MIN_ZOOM, value - ZOOM_STEP))}>
          <ZoomOut data-icon="inline-start" className="size-4" />
          Zoom out
        </Button>
        <Button size="sm" variant="outline" onClick={() => setZoom((value) => Math.min(MAX_ZOOM, value + ZOOM_STEP))}>
          <ZoomIn data-icon="inline-start" className="size-4" />
          Zoom in
        </Button>
        <div className="min-w-24 text-xs text-muted-foreground">{Math.round(zoom * 100)}%</div>

        <div className="ml-auto flex w-full max-w-sm items-center gap-2">
          <Search className="size-4 text-muted-foreground" />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search in PDF"
            className="h-8"
          />
        </div>

        <Button size="sm" variant="outline" onClick={onOpenExternal}>
          <ExternalLink data-icon="inline-start" className="size-4" />
          Open in browser tab
        </Button>
      </div>

      <iframe title={file.name} src={iframeUrl} className="h-full w-full border-0" />
    </div>
  )
}
