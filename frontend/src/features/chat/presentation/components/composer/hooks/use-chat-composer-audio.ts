import * as React from "react"
import { toast } from "sonner"
import type { PendingAudioClip } from "../composer.types"

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
      toast.error("Audio recording is not available in this environment.")
      return
    }

    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      const mimeType = MediaRecorder.isTypeSupported("audio/webm;codecs=opus") ? "audio/webm;codecs=opus" : "audio/webm"
      const recorder = new MediaRecorder(stream, { mimeType })

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
          toast.error("The recording was empty. Please try again.")
          return
        }

        const filename = `voice-note-${new Date().toISOString().replace(/[:.]/g, "-")}.webm`
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
        toast.error("Unable to record audio right now.")
      }

      recorder.start()
      setIsRecording(true)
    } catch (error) {
      setIsRecording(false)
      toast.error(error instanceof Error ? error.message : "Unable to access the microphone.")
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
