import { useCallback, useEffect, useRef } from "react";
import { toast } from "sonner";
import { useRouter } from "@tanstack/react-router";

export interface NotificationPayload {
  id: string;
  namespace: string;
  event: string;
  title: string;
  body: string;
  icon?: string;
  actions?: Array<{ label: string; to: string }>;
  data?: Record<string, unknown>;
  createdAt?: string;
  workspaceId?: string;
}

interface UseNotificationResult {
  notify: (payload: NotificationPayload) => Promise<void>;
}

export function useNotification(): UseNotificationResult {
  const router = useRouter();
  const isWindowFocusedRef = useRef<boolean>(typeof document === "undefined" ? false : document.hasFocus());
  const isDocumentVisibleRef = useRef<boolean>(typeof document === "undefined" ? false : document.visibilityState === "visible");
  const hasRequestedPermissionRef = useRef(false);
  const audioRef = useRef<HTMLAudioElement | null>(null);

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
    if (typeof window === "undefined") {
      return;
    }

    audioRef.current = new Audio("./public/audio/notification.mp3");
    audioRef.current.preload = "auto";
  }, []);

  const resolveNotificationRoute = useCallback((payload: NotificationPayload) => {
    if (payload.actions?.[0]?.to) {
      return payload.actions[0].to;
    }

    switch (payload.namespace) {
      case "task":
      case "todo":
      case "comment":
        return "/tasks";
      case "chat":
        return "/chats";
      case "skill":
        return "/skills";
      case "instruction":
        return "/instructions";
      case "template":
        return "/templates";
      case "collection":
        return "/collections";
      default:
        return "/";
    }
  }, []);

  const openNotificationTarget = useCallback((payload: NotificationPayload) => {
    const to = resolveNotificationRoute(payload);
    router.navigate({ to });
  }, [resolveNotificationRoute, router]);

  const playNotificationSound = useCallback(async () => {
    const audio = audioRef.current;

    if (!audio) {
      return;
    }

    try {
      audio.currentTime = 0;
      await audio.play();
    } catch {
      // Autoplay can be blocked by the browser; ignore and keep notification flow.
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

    const notification = new Notification(payload.title, {
      body: payload.body,
      icon: payload.icon,
      tag: payload.id,
    });

    notification.onclick = () => {
      window.focus();
      openNotificationTarget(payload);
      notification.close();
    };
  }, [openNotificationTarget]);

  const notify = useCallback(async (payload: NotificationPayload) => {
    await playNotificationSound();

    const isInForeground = isWindowFocusedRef.current && isDocumentVisibleRef.current;

    if (isInForeground) {
      toast(payload.title, {
        id: payload.id,
        description: payload.body,
        action: {
          label: "Open",
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
