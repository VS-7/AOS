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

/** Which version this installation runs, and against which channel. */
export interface UpdateStatus {
  current: string;
  channel: string;
  latestKnown?: string;
  checkedAt?: string;
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
}

/** What `update_download` staged, handed straight back to `update_apply`. */
export interface Staged {
  version: string;
  dir: string;
  binaries: Record<string, string>;
}

/** What the release channel had to say. */
export interface CheckResult {
  upToDate: boolean;
  current: string;
  channel: string;
  release?: Release;
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
