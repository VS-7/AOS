import React from "react"
import { Maximize, RotateCw, Scan, ZoomIn, ZoomOut } from "lucide-react"
import { Button } from "@/components/ui/button"
import type { WorkspaceFile } from "@/features/file/interfaces/file.interfaces"

interface FilesImageViewerProps {
  file: WorkspaceFile
}

const SCALE_STEP = 0.15
const MIN_SCALE = 0.2
const MAX_SCALE = 5

export function FilesImageViewer({ file }: FilesImageViewerProps) {
  const [scale, setScale] = React.useState(1)
  const [rotation, setRotation] = React.useState(0)
  const [position, setPosition] = React.useState({ x: 0, y: 0 })
  const isPanningRef = React.useRef(false)
  const panOriginRef = React.useRef({ x: 0, y: 0 })
  const contentUrl = React.useMemo(
    () => `/api/files/content?path=${encodeURIComponent(file.path)}`,
    [file.path],
  )

  function clampScale(value: number) {
    return Math.max(MIN_SCALE, Math.min(MAX_SCALE, value))
  }

  function resetView() {
    setScale(1)
    setRotation(0)
    setPosition({ x: 0, y: 0 })
  }

  return (
    <div className="grid h-full grid-rows-[auto_1fr]">
      <div className="flex items-center gap-2 border-b px-3 py-2">
        <Button size="sm" variant="outline" onClick={() => setScale((value) => clampScale(value - SCALE_STEP))}>
          <ZoomOut data-icon="inline-start" className="size-4" />
          Zoom out
        </Button>
        <Button size="sm" variant="outline" onClick={() => setScale((value) => clampScale(value + SCALE_STEP))}>
          <ZoomIn data-icon="inline-start" className="size-4" />
          Zoom in
        </Button>
        <Button size="sm" variant="outline" onClick={() => setRotation((value) => (value + 90) % 360)}>
          <RotateCw data-icon="inline-start" className="size-4" />
          Rotate
        </Button>
        <Button size="sm" variant="outline" onClick={resetView}>
          <Scan data-icon="inline-start" className="size-4" />
          Reset
        </Button>
        <Button size="sm" variant="outline" onClick={() => document.documentElement.requestFullscreen?.()}>
          <Maximize data-icon="inline-start" className="size-4" />
          Fullscreen
        </Button>
        <div className="ml-auto text-xs text-muted-foreground">
          {(scale * 100).toFixed(0)}%
        </div>
      </div>

      <div
        className="relative overflow-hidden bg-muted/20"
        onWheel={(event) => {
          event.preventDefault()
          const delta = event.deltaY > 0 ? -SCALE_STEP : SCALE_STEP
          setScale((value) => clampScale(value + delta))
        }}
        onMouseDown={(event) => {
          isPanningRef.current = true
          panOriginRef.current = { x: event.clientX - position.x, y: event.clientY - position.y }
        }}
        onMouseMove={(event) => {
          if (!isPanningRef.current) return

          setPosition({
            x: event.clientX - panOriginRef.current.x,
            y: event.clientY - panOriginRef.current.y,
          })
        }}
        onMouseUp={() => {
          isPanningRef.current = false
        }}
        onMouseLeave={() => {
          isPanningRef.current = false
        }}
      >
        <div className="absolute inset-0 flex items-center justify-center">
          <img
            alt={file.name}
            draggable={false}
            src={contentUrl}
            className="max-h-full max-w-full select-none object-contain"
            style={{
              transform: `translate(${position.x}px, ${position.y}px) scale(${scale}) rotate(${rotation}deg)`,
              transformOrigin: "center center",
            }}
          />
        </div>
      </div>
    </div>
  )
}
