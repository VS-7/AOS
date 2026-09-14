import { useCallback, useEffect, useRef } from "react";
import { toast } from "sonner";
import { useRouter } from "@tanstack/react-router";
import { t } from "@/lib/i18n";
import {
  activityTarget,
  activityTitle,
} from "@/features/activity/presentation/helpers/activity-presentation.helper";

export interface NotificationPayload {
  id: string;
  namespace: string;
  event: string;
  title: string;
  body: string;
  icon?: string;
  data?: Record<string, unknown>;
  actor?: string;
  actorType?: string;
  createdAt?: string;
  workspaceId?: string;
}

interface UseNotificationResult {
  notify: (payload: NotificationPayload) => Promise<void>;
}

/**
 * A short two-note chime, synthesised rather than loaded.
 *
 * The sound used to be `new Audio("./public/audio/notification.mp3")` — a
 * file that does not exist anywhere in the repository, at a path Vite would
 * not have served even if it did — so every notification requested it, got
 * the SPA's index.html or a 404, and played nothing. Synthesising it needs no
 * asset to ship and cannot 404.
 */
function playChime(context: AudioContext): void {
  const start = context.currentTime;
  for (const [offset, frequency] of [
    [0, 880],
    [0.12, 1320],
  ] as const) {
    const oscillator = context.createOscillator();
    const gain = context.createGain();
    oscillator.type = "sine";
    oscillator.frequency.value = frequency;
    gain.gain.setValueAtTime(0.0001, start + offset);
    gain.gain.exponentialRampToValueAtTime(0.08, start + offset + 0.02);
    gain.gain.exponentialRampToValueAtTime(0.0001, start + offset + 0.3);
    oscillator.connect(gain).connect(context.destination);
    oscillator.start(start + offset);
    oscillator.stop(start + offset + 0.32);
  }
}

export function useNotification(): UseNotificationResult {
  const router = useRouter();
  const isWindowFocusedRef = useRef<boolean>(typeof document === "undefined" ? false : document.hasFocus());
  const isDocumentVisibleRef = useRef<boolean>(typeof document === "undefined" ? false : document.visibilityState === "visible");
  const hasRequestedPermissionRef = useRef(false);
  const audioContextRef = useRef<AudioContext | null>(null);

  useEffect(() => {
    if (typeof window === "undefined" || typeof document === "undefined") {
      return;
    }

    const onFocus = () => {
      isWindowFocusedRef.current = true;
    };

    const onBlur = () => {
      isWindowFocusedRef.current = false;
    };

    const onVisibilityChange = () => {
      isDocumentVisibleRef.current = document.visibilityState === "visible";
    };

    window.addEventListener("focus", onFocus);
    window.addEventListener("blur", onBlur);
    document.addEventListener("visibilitychange", onVisibilityChange);

    return () => {
      window.removeEventListener("focus", onFocus);
      window.removeEventListener("blur", onBlur);
      document.removeEventListener("visibilitychange", onVisibilityChange);
    };
  }, []);

  useEffect(() => {
    return () => {
      void audioContextRef.current?.close().catch(() => {});
      audioContextRef.current = null;
    };
  }, []);

  // The record the activity is about — the same place its inbox line opens.
  // This used to send a task event to the task list and everything else to
  // Home, and a chat, skill or template to routes that do not exist.
  const openNotificationTarget = useCallback((payload: NotificationPayload) => {
    void router.navigate(activityTarget(payload) as never);
  }, [router]);

  const playNotificationSound = useCallback(() => {
    if (typeof window === "undefined" || typeof window.AudioContext !== "function") {
      return;
    }
    try {
      audioContextRef.current ??= new window.AudioContext();
      const context = audioContextRef.current;
      // A context created before the person interacted starts suspended;
      // resuming may be refused, and a notification is not worth an error.
      if (context.state === "suspended") {
        void context.resume().then(() => playChime(context)).catch(() => {});
        return;
      }
      playChime(context);
    } catch {
      // No audio device, or audio disallowed: the notification itself still shows.
    }
  }, []);

  const showPushNotification = useCallback(async (payload: NotificationPayload) => {
    if (typeof window === "undefined" || !("Notification" in window)) {
      return;
    }

    let permission = Notification.permission;

    if (permission === "default" && !hasRequestedPermissionRef.current) {
      hasRequestedPermissionRef.current = true;
      permission = await Notification.requestPermission();
    }

    if (permission !== "granted") {
      return;
    }

    // No `icon`: the activity's is an icon *name* the interface renders
    // ("CheckSquare"), which the Notification API would fetch as a URL.
    const notification = new Notification(activityTitle(payload), {
      body: payload.body,
      tag: payload.id,
    });

    notification.onclick = () => {
      window.focus();
      openNotificationTarget(payload);
      notification.close();
    };
  }, [openNotificationTarget]);

  const notify = useCallback(async (payload: NotificationPayload) => {
    playNotificationSound();

    const isInForeground = isWindowFocusedRef.current && isDocumentVisibleRef.current;

    if (isInForeground) {
      toast(activityTitle(payload), {
        id: payload.id,
        description: payload.body,
        action: {
          label: t("Open"),
          onClick: () => openNotificationTarget(payload),
        },
      });
      return;
    }

    await showPushNotification(payload);
  }, [openNotificationTarget, playNotificationSound, showPushNotification]);

  return {
    notify,
  };
}
