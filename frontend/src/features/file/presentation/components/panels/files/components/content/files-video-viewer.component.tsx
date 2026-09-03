import { t } from "@/lib/i18n";
import React from "react"
import { Maximize, Pause, Play } from "lucide-react"
import { Button } from "@/components/ui/button"
import { contentURL } from "@/lib/file"
import type { WorkspaceFile } from "@/features/file/interfaces/file.interfaces"

interface FilesVideoViewerProps {
  file: WorkspaceFile
}

export function FilesVideoViewer({ file }: FilesVideoViewerProps) {
  const [playbackRate, setPlaybackRate] = React.useState(1)
  const videoRef = React.useRef<HTMLVideoElement>(null)
  const contentUrl = React.useMemo(() => contentURL(file.path), [file.path])

  return (
    <div className="grid h-full grid-rows-[auto_1fr]">
      <div className="flex flex-wrap items-center gap-2 border-b px-3 py-2">
        <Button
          size="sm"
          variant="outline"
          onClick={() => {
            const video = videoRef.current
            if (!video) return
            if (video.paused) {
              void video.play()
            } else {
              video.pause()
            }
          }}
        >
          <Play data-icon="inline-start" className="size-4" />
          <Pause data-icon="inline-start" className="size-4" />
          {t("Play/Pause")}
        </Button>

        <label className="flex items-center gap-2 text-xs text-muted-foreground">
          {t("Speed")}
          <select
            value={playbackRate}
            onChange={(event) => {
              const nextRate = Number(event.target.value)
              setPlaybackRate(nextRate)
              if (videoRef.current) {
                videoRef.current.playbackRate = nextRate
              }
            }}
            className="h-8 rounded-md border bg-background px-2 text-xs"
          >
            <option value={0.5}>0.5x</option>
            <option value={0.75}>0.75x</option>
            <option value={1}>1x</option>
            <option value={1.25}>1.25x</option>
            <option value={1.5}>1.5x</option>
            <option value={2}>2x</option>
          </select>
        </label>

        <Button size="sm" variant="outline" onClick={() => videoRef.current?.requestFullscreen()}>
          <Maximize data-icon="inline-start" className="size-4" />
          {t("Fullscreen")}
        </Button>
      </div>

      <div className="flex items-center justify-center bg-black/80">
        <video ref={videoRef} src={contentUrl} controls className="h-full w-full" />
      </div>
    </div>
  )
}
