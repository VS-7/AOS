import { getLocale, t } from "@/lib/i18n";
import type {
  CheckResult,
  Release,
  Staged,
  UpdateInstall,
  UpdateStatus,
} from "@/features/update/interfaces/update.interfaces";

/**
 * The refusal a mutation settled with, if it settled with one.
 *
 * `useMutation` from `lib/aos-facade` resolves every call to `{ data, error }`
 * and never rejects, so a refused `update_download` ran this screen's
 * `onSuccess` and toasted "Downloaded and verified." — while its `onError`,
 * the handler written for exactly that refusal, could not run. Reading the
 * envelope here keeps the screen honest whichever way the facade reports a
 * refusal: in the result today, as a rejection once it throws.
 */
export function refusalOf(result: unknown): unknown {
  if (result && typeof result === "object" && "error" in result) {
    return (result as { error?: unknown }).error ?? null;
  }
  return null;
}

/** The daemon's own words for a refusal, or the screen's when it has none. */
export function messageOf(error: unknown, fallback: string): string {
  if (error && typeof error === "object") {
    const nested = (error as { error?: { message?: unknown } }).error?.message;
    if (typeof nested === "string" && nested.trim()) return nested;
    const message = (error as { message?: unknown }).message;
    if (typeof message === "string" && message.trim()) return message;
  }
  return fallback;
}

/** A check's time, in the reader's locale. */
export function formatCheckedAt(iso: string): string {
  const at = new Date(iso);
  return Number.isNaN(at.getTime()) ? iso : at.toLocaleString(getLocale());
}

/**
 * Whether checking can tell this installation anything. A build with no
 * release feed can only ever answer "not enabled", which the status line
 * already says — a button that can only produce that is noise.
 */
export function canCheck(status: UpdateStatus | null): boolean {
  return status?.configured !== false;
}

/** The line under "Running": what is known about this installation. */
export function statusLine(status: UpdateStatus | null): string {
  if (status?.configured === false) {
    return t("Automatic updates are not enabled in this build. To update, run the installer again.");
  }
  if (status?.checkedAt) {
    return t("Last checked {{when}}.", { when: formatCheckedAt(status.checkedAt) });
  }
  return t("This installation has not been checked against the release channel yet.");
}

/**
 * The answer a check gives in its own row, when the answer is a sentence
 * rather than a release to act on.
 */
export function checkLine(check: CheckResult | null, status: UpdateStatus | null): string | null {
  switch (check?.state) {
    case "up-to-date":
      return t("You are on the newest release.");
    case "not-configured":
      // Already the status line once the status reflects it.
      return status?.configured === false
        ? null
        : t("Automatic updates are not enabled in this build. To update, run the installer again.");
    case "developer-build":
      return check.release
        ? t("This is a development build ({{current}}), updated by rebuilding it rather than from the release channel. The newest release is {{version}}.", {
            current: check.current,
            version: check.release.version,
          })
        : t("This is a development build ({{current}}), updated by rebuilding it rather than from the release channel.", {
            current: check.current,
          });
    default:
      return null;
  }
}

/** A newer release, and what this installation can do with it. */
export interface Offer {
  release: Release;
  install: UpdateInstall;
  /** The verified copy of this release waiting to be installed, if any. */
  staged: Staged | null;
}

/**
 * What the screen offers after a check: the release, how it installs here,
 * and whether it is already staged — by a download in this visit, or by one
 * the daemon remembers from before.
 */
export function offerOf(
  check: CheckResult | null,
  downloaded: Staged | null,
  status: UpdateStatus | null,
): Offer | null {
  if (check?.state !== "available" || !check.release) return null;
  const version = check.release.version;
  const staged =
    (downloaded?.version === version ? downloaded : null) ??
    (status?.staged?.version === version ? status.staged : null);
  return {
    release: check.release,
    install: check.install ?? { method: "here" },
    staged: staged ?? null,
  };
}

/** The sentence under an offered release's title. */
export function offerLine(offer: Offer): string {
  switch (offer.install.method) {
    case "reinstall":
      return t("This installation is an application bundle, which is updated by installing the new version over it — with the installer, or from the release page.");
    case "terminal":
      return offer.staged
        ? t("Verified and staged. The daemon cannot restart itself onto a new version, so install it from a terminal:")
        : t("Nothing is installed until you download it and the signature checks out.");
    default:
      return offer.staged
        ? t("Verified and staged. Installing restarts the daemon; in-flight work finishes first.")
        : t("Nothing is installed until you download it and the signature checks out.");
  }
}

/**
 * The release's page, when it is a web page: https, or http on this machine
 * (a feed served locally). Anything else is null.
 *
 * The address comes from the release manifest, which is not signed, and it is
 * handed to the operating system's browser. The daemon already drops a page
 * that is not a web page; this screen checks again rather than trust whatever
 * answered it.
 */
export function releasePage(release: Release): string | null {
  let url: URL;
  try {
    url = new URL(release.pageUrl ?? "");
  } catch {
    return null;
  }
  if (url.protocol === "https:") return url.href;
  const loopback = url.hostname === "localhost" || url.hostname === "[::1]" || /^127\./.test(url.hostname);
  return url.protocol === "http:" && loopback ? url.href : null;
}
