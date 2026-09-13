/**
 * Keeping this installation current, and the process that serves it.
 *
 * These mirror `internal/domain/update`'s status/check/download shapes and
 * `internal/domain/gateway`'s `State`. They live in a feature's
 * `interfaces/` folder like every other ported domain's types rather than
 * inside the screen that reads them — a type declared in a component is one
 * the next reader of the same data declares again, slightly differently, and
 * the two drift without anything failing.
 */

/**
 * What a check found, as one word — `internal/domain/update`'s CheckState.
 *
 * `upToDate` alone could not say it: "this build has no release feed" and
 * "a development build cannot be compared with a release" both used to come
 * back as `upToDate: true`, and this screen answered them with a green "You
 * are on the newest release." while a newer release was published.
 */
export type CheckState = "up-to-date" | "available" | "not-configured" | "developer-build";

/**
 * How a newer release reaches this installation.
 *
 * `here`: Download and Install work from this screen. `terminal`: the
 * binaries can be replaced, but the daemon answering this screen cannot
 * restart itself onto them — `command` installs the staged release from a
 * terminal. `reinstall`: the new version is installed whole, for `reason`.
 */
export interface UpdateInstall {
  method: "here" | "terminal" | "reinstall";
  /**
   * Why an installation is reinstalled: a signed macOS `bundle`; a folder
   * this account cannot change (`read-only` — an AppImage, Program Files);
   * or a `server` daemon, which carries the web interface the release's own
   * daemon does not.
   */
  reason?: "bundle" | "read-only" | "server";
  command?: string;
  /** The window is among the binaries an install replaces here. */
  reopen?: boolean;
  /**
   * The daemon answering was started by hand or by a service manager, not by
   * the application or `aos gateway`: `command` cannot restart it, so it is
   * stopped where it was started first.
   */
  unsupervised?: boolean;
}

/** Which version this installation runs, and what the last check found. */
export interface UpdateStatus {
  current: string;
  channel: string;
  /** False when this build has no release feed at all. */
  configured?: boolean;
  install?: UpdateInstall;
  latestKnown?: string;
  checkedAt?: string;
  lastState?: CheckState;
  /** A verified release waiting to be installed. */
  staged?: Staged;
  /** A download or install is running on this installation right now. */
  busy?: boolean;
}

/**
 * A release, shaped exactly as `update_check` returns it and
 * `update_download` takes it back.
 *
 * The interface reads two of these fields and passes the rest through
 * untouched — it never constructs one. The full shape is spelled out anyway
 * because the generated command types check it, and a looser type here would
 * only mean a cast that stops checking anything.
 */
export interface Release {
  version: string;
  channel: string;
  checksumsUrl: string;
  signatureUrl: string;
  publishedAt: string;
  assets: unknown;
  notes?: string;
  /** Where a person downloads this release by hand. */
  pageUrl?: string;
}

/** What `update_download` staged, handed straight back to `update_apply`. */
export interface Staged {
  version: string;
  dir: string;
  binaries: Record<string, string>;
}

/** What the release channel had to say. */
export interface CheckResult {
  state: CheckState;
  upToDate: boolean;
  current: string;
  channel: string;
  release?: Release;
  /** Set when `state` is `available`. */
  install?: UpdateInstall;
  checkedAt?: string;
}

/** Where the daemon is listening, and as which process. */
export interface GatewayMeta {
  pid: number;
  port: number;
  host: string;
  version?: string;
  startedAt?: string;
}

/**
 * What the daemon says about itself.
 *
 * `status` used to describe the *supervisor's* record, which only whatever
 * spawned the daemon writes — so a daemon started by a systemd unit, by
 * `task dev`, or by hand reported "stopped" while answering the call. It
 * answers for itself now.
 */
export interface GatewayState {
  status: string;
  healthy: boolean;
  meta?: GatewayMeta;
}
