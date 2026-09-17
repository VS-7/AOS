import { t } from "@/lib/i18n";
import * as React from "react"
import { toast } from "sonner"
import type { PendingAudioClip } from "../composer.types"

/**
 * What a failed microphone request means, in words the interface owns.
 *
 * The browser's own message went straight to the toast — "Requested device
 * not found", in English, in a Portuguese window — and it names the API that
 * failed rather than what the person can do about it.
 */
export function microphoneErrorMessage(error: unknown): string {
  const name = error && typeof error === "object" && "name" in error ? String((error as { name?: unknown }).name) : "";
  switch (name) {
    case "NotAllowedError":
    case "SecurityError":
      return t("Microphone access was denied. Allow it for this app in the system's privacy settings.")
    case "NotFoundError":
    case "OverconstrainedError":
      return t("No microphone was found.")
    case "NotReadableError":
    case "AbortError":
      return t("The microphone is in use by another application.")
    case "NotSupportedError":
      return t("Recording audio is not supported here.")
    default:
      return t("Unable to access the microphone.")
  }
}

/** Recording formats, best first. WebKit records MP4/AAC and not WebM. */
const RECORDING_TYPES = ["audio/webm;codecs=opus", "audio/webm", "audio/mp4", "audio/aac"]

/**
 * The first format this engine can record, or `undefined` to let it choose.
 *
 * Asking for WebM unconditionally threw NotSupportedError in the desktop
 * window, whose engine is WebKit.
 */
export function recordingMimeType(isSupported: (type: string) => boolean): string | undefined {
  return RECORDING_TYPES.find((type) => isSupported(type))
}

export function useChatComposerAudio() {
  const mediaRecorderRef = React.useRef<MediaRecorder | null>(null)
  const mediaStreamRef = React.useRef<MediaStream | null>(null)
  const recordingChunksRef = React.useRef<Blob[]>([])
  const recordingStartedAtRef = React.useRef<number | null>(null)

  const [isRecording, setIsRecording] = React.useState(false)
  const [pendingAudioClip, setPendingAudioClip] = React.useState<PendingAudioClip | null>(null)

  React.useEffect(() => {
    return () => {
      if (pendingAudioClip?.url) {
        URL.revokeObjectURL(pendingAudioClip.url)
      }

      mediaRecorderRef.current?.stream.getTracks().forEach((track) => track.stop())
      mediaStreamRef.current?.getTracks().forEach((track) => track.stop())
    }
  }, [pendingAudioClip?.url])

  const handleRecordToggle = React.useCallback(async () => {
    if (isRecording) {
      mediaRecorderRef.current?.stop()
      return
    }

    if (typeof window === "undefined" || !navigator.mediaDevices?.getUserMedia || typeof MediaRecorder === "undefined") {
      toast.error(t("Audio recording is not available in this environment."))
      return
    }

    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      const mimeType = recordingMimeType((type) => MediaRecorder.isTypeSupported(type))
      const recorder = new MediaRecorder(stream, mimeType ? { mimeType } : undefined)

      mediaRecorderRef.current = recorder
      mediaStreamRef.current = stream
      recordingChunksRef.current = []
      recordingStartedAtRef.current = Date.now()

      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) {
          recordingChunksRef.current.push(event.data)
        }
      }

      recorder.onstop = () => {
        const startedAt = recordingStartedAtRef.current
        const durationMs = startedAt ? Date.now() - startedAt : 0
        const blob = new Blob(recordingChunksRef.current, {
          type: recorder.mimeType || "audio/webm",
        })

        stream.getTracks().forEach((track) => track.stop())
        mediaRecorderRef.current = null
        mediaStreamRef.current = null
        recordingChunksRef.current = []
        recordingStartedAtRef.current = null
        setIsRecording(false)

        if (blob.size === 0) {
          toast.error(t("The recording was empty. Please try again."))
          return
        }

        const extension = blob.type.includes("mp4") || blob.type.includes("aac") ? "m4a" : "webm"
        const filename = `voice-note-${new Date().toISOString().replace(/[:.]/g, "-")}.${extension}`
        const url = URL.createObjectURL(blob)

        setPendingAudioClip((current) => {
          if (current?.url) {
            URL.revokeObjectURL(current.url)
          }

          return {
            blob,
            durationMs,
            filename,
            url,
          }
        })
      }

      recorder.onerror = () => {
        stream.getTracks().forEach((track) => track.stop())
        mediaRecorderRef.current = null
        mediaStreamRef.current = null
        recordingChunksRef.current = []
        recordingStartedAtRef.current = null
        setIsRecording(false)
        toast.error(t("Unable to record audio right now."))
      }

      recorder.start()
      setIsRecording(true)
    } catch (error) {
      setIsRecording(false)
      toast.error(microphoneErrorMessage(error))
    }
  }, [isRecording])

  const closePendingAudio = React.useCallback((open: boolean) => {
    if (open) {
      return
    }

    setPendingAudioClip((current) => {
      if (current?.url) {
        URL.revokeObjectURL(current.url)
      }

      return null
    })
  }, [])

  const buildPendingAudioFile = React.useCallback(() => {
    if (!pendingAudioClip) {
      return null
    }

    return new File([pendingAudioClip.blob], pendingAudioClip.filename, {
      type: pendingAudioClip.blob.type || "audio/webm",
    })
  }, [pendingAudioClip])

  return {
    buildPendingAudioFile,
    closePendingAudio,
    handleRecordToggle,
    isRecording,
    pendingAudioClip,
  }
}
