import { useEffect, useState, type JSX } from "react";
import { Events } from "@wailsio/runtime";
import { toast } from "sonner";

import { DAEMON_EVENT, system } from "@/lib/client";
import { t } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { isDesktopWindow } from "@/lib/wails";
import type { VersionSkew } from "@/features/update/interfaces/update.interfaces";

/**
 * The skew an `aos:daemon` event carries, or null when it carries none.
 *
 * Only a daemon that is answering can be a different release: the window
 * (`cmd/aos-desktop`'s watchdog) attaches the skew to healthy events alone,
 * and an event without one — the daemon matching again, or gone — is what
 * takes the banner down.
 */
export function skewOf(data: unknown): VersionSkew | null {
  const one = Array.isArray(data) ? data[0] : data;
  if (!one || typeof one !== "object") return null;
  const { healthy, skew } = one as { healthy?: unknown; skew?: unknown };
  if (healthy !== true || !skew || typeof skew !== "object") return null;
  const { window, daemon, windowOlder, compatible } = skew as Partial<VersionSkew>;
  if (typeof window !== "string" || typeof daemon !== "string") return null;
  return { window, daemon, windowOlder: windowOlder === true, compatible: compatible === true };
}

/**
 * A line across the top while this window and the daemon under it are
 * different releases.
 *
 * `aosd update apply` from a terminal replaces the daemon and the window's
 * binary on disk and restarts the daemon, and the window that is open goes on
 * running the previous release against it: its screens are the old ones, and
 * nothing said so. The other way round is a new window adopting a daemon an
 * earlier installation left running. Each has one fix, and the banner names
 * it — reopening the window, or restarting the daemon, which the window can
 * do itself.
 */
export function VersionSkewBanner(): JSX.Element | null {
  const [skew, setSkew] = useState<VersionSkew | null>(null);
  const [restarting, setRestarting] = useState(false);

  useEffect(() => {
    // Only the desktop window relays the daemon's health; a browser tab is
    // served by the daemon itself, which is always its own release.
    if (!isDesktopWindow) return;
    return Events.On(DAEMON_EVENT, (event: { data?: unknown }) => setSkew(skewOf(event?.data)));
  }, []);

  if (!skew) return null;

  const restart = async () => {
    setRestarting(true);
    try {
      await system.restartDaemon();
    } catch (error: unknown) {
      const message = error instanceof Error && error.message ? error.message : t("The daemon could not be restarted.");
      toast.error(message);
    } finally {
      setRestarting(false);
    }
  };

  const what = skew.windowOlder
    ? t("AOS was updated to {{daemon}} while this window was open ({{window}}). Quit AOS and open it again to use the new version.", {
        daemon: skew.daemon,
        window: skew.window,
      })
    : t("The daemon is still running {{daemon}}, older than this window ({{window}}). Restart the daemon to finish updating.", {
        daemon: skew.daemon,
        window: skew.window,
      });

  return (
    <div
      role="status"
      className={cn(
        "fixed inset-x-0 top-0 z-50 flex flex-wrap items-center justify-center gap-x-3 gap-y-1 px-4 py-1.5 text-center text-xs font-medium",
        skew.compatible ? "bg-amber-500 text-white" : "bg-destructive text-destructive-foreground",
      )}
    >
      <span>
        {what}
        {skew.compatible ? null : ` ${t("Until then, this window and the daemon cannot work together.")}`}
      </span>
      {skew.windowOlder ? null : (
        <button
          type="button"
          className="rounded border border-current px-2 py-0.5 disabled:opacity-60"
          disabled={restarting}
          onClick={() => void restart()}
        >
          {restarting ? t("Restarting…") : t("Restart daemon")}
        </button>
      )}
    </div>
  );
}
