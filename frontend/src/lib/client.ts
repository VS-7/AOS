import { Call } from "@wailsio/runtime";
import { daemonURL } from "./daemon-origin";
import type { CommandInput, CommandKey, CommandOutput } from "./schema";
import { desktopRetryDelays, isDesktopConfirmed, markDesktopConfirmed, sleep } from "./desktop-transport";

/**
 * The one door to the domain.
 *
 * The same component runs in a browser and inside the desktop window. In the
 * browser a command goes over HTTP to the daemon; in the desktop it goes
 * through the Wails binding, in process, with no network hop. Both arrive at
 * the same command registry with the same validation, so no component ever has
 * to know which one it is running in.
 */
export interface Client {
  invoke<K extends CommandKey>(key: K, input: CommandInput<K>): Promise<CommandOutput<K>>;
}

/**
 * Historically the shape Wails v2 put on `window.go`. Wails3 doesn't: it has
 * no such global at all, and calls a bound method through `@wailsio/runtime`'s
 * `Call.ByName("pkg.Struct.Method", ...args)` instead, which — inside the
 * desktop window — the native host intercepts before it ever reaches a real
 * network stack. `client.ts` targeted the v2 shape for a while, which is why
 * `window.go` was always undefined here and every desktop call silently fell
 * back to the browser transport. Some non-domain calls elsewhere in the
 * frontend (SystemService, ApprovalService) still expect this global and are
 * a known follow-up, not fixed by this change.
 */
/**
 * Whether this page is inside the desktop window rather than a browser tab.
 *
 * There is no synchronous ground truth for this in Wails3 — window.location
 * stays a normal http(s) origin either way, and the interception that makes
 * the desktop transport work happens at the network layer, nothing JS can
 * inspect ahead of a call. This reflects confirmedDesktop (declared further
 * down, alongside the desktop transport): false until the first domain call
 * actually succeeds through it, true from then on. client.invoke() itself
 * doesn't use this — it tries the desktop transport fresh on every call,
 * which is the only check that's actually reliable moment to moment. This
 * is for the handful of call sites (native chrome, the file picker) that
 * need a synchronous answer before they can act at all, and can tolerate
 * that answer starting out wrong for the first call or two after the
 * window opens.
 */
export function isDesktop(): boolean {
  return isDesktopConfirmed();
}

/**
 * An error the domain produced, with the code and the call to action it carries.
 *
 * The code is what makes a failure actionable in the interface: a screen can
 * recognise AOS_TASK_REVIEW_BLOCKED and point at the plan, rather than showing
 * a sentence and a shrug.
 */
export class DomainError extends Error {
  readonly code: string;
  readonly status: number;
  readonly issues: Record<string, unknown>;
  readonly actions: Array<{ label: string; command?: string; tool?: string }>;

  constructor(payload: {
    code?: string;
    message?: string;
    status?: number;
    issues?: Record<string, unknown>;
    actions?: Array<{ label: string; command?: string; tool?: string }>;
  }) {
    super(payload.message ?? payload.code ?? "the call failed");
    this.name = "DomainError";
    this.code = payload.code ?? "UNKNOWN";
    this.status = payload.status ?? 500;
    this.issues = payload.issues ?? {};
    this.actions = payload.actions ?? [];
  }
}

/** The envelope every surface wraps an answer in. */
interface Envelope<T> {
  data?: T;
  error?: ConstructorParameters<typeof DomainError>[0];
  notice?: { message: string };
}

/**
 * Unwraps the envelope every surface answers with. Exported for lib/file.ts,
 * which talks to /api/file directly rather than through Client — that
 * surface is outside the command registry (see File (Go)'s "não tem grupo de
 * comando") but still answers in the same envelope shape.
 */
export function unwrap<T>(raw: unknown): T {
  const envelope = raw as Envelope<T>;
  if (envelope && typeof envelope === "object" && "error" in envelope && envelope.error) {
    throw new DomainError(envelope.error);
  }
  if (envelope && typeof envelope === "object" && "data" in envelope) {
    return envelope.data as T;
  }
  // A surface that answered without an envelope answered with the value.
  return raw as T;
}

const WAILSVC_PKG = "github.com/OWNER/aos/internal/transport/wailsvc";

/** One place a coding agent reads skills from — pkg/skill.Target. */
export interface SkillTarget {
  id: string;
  label: string;
  dir: string;
  present: boolean;
  installed: boolean;
}

/** What an install did — pkg/skill.InstallResult. */
export interface SkillInstallResult {
  installed: string[] | null;
  skipped: Record<string, string>;
}
const DOMAIN_SERVICE_INVOKE = `${WAILSVC_PKG}.DomainService.Invoke`;

/**
 * Whether a failed desktop call is worth retrying rather than surfacing (or
 * falling back to HTTP) immediately.
 *
 * A well-formed DomainError means the call reached the daemon and back —
 * we're genuinely inside the desktop window, and a business error (not
 * found, validation, ...) isn't fixed by trying again. Anything else —
 * Call.ByName rejecting outright with a ReferenceError, TypeError, or a
 * plain network error — is exactly what happens both when there's truly no
 * Wails host to intercept the call *and* when there is one but it isn't
 * warmed up yet on the very first calls a window makes. Retrying costs
 * milliseconds in the first case and saves the second.
 */
function isRetryableDesktopError(err: unknown): boolean {
  if (!(err instanceof DomainError)) return true;
  return (
    err.code === "AOS_DAEMON_UNREACHABLE" || // the daemon hasn't finished starting yet
    err.code === "AOS_DESKTOP_NO_COMMAND_NAMED" // an argument mixup in the bridge under concurrency
  );
}

/**
 * The desktop transport: internal/transport/wailsvc.DomainService.Invoke,
 * called by its fully qualified Go name ("package.Struct.Method", per
 * @wailsio/runtime's Call.ByName) rather than through a generated per-command
 * binding — the same one-generic-method design domainservice.js itself
 * documents when `wails3 generate bindings` is run over this package.
 *
 * The second argument is passed as a plain object, not pre-stringified: the
 * Go parameter is json.RawMessage, and Call.ByName's own request envelope
 * already JSON-encodes every argument once on the way across. Stringifying
 * it here first meant the Go side received a JSON string *containing* JSON
 * — valid bytes, wrong shape — and every command handler's own
 * json.Unmarshal into its typed input then failed with "cannot unmarshal
 * string into Go value of type ...". Handing over the object lets that one
 * encoding pass do the job once, correctly.
 *
 * Retries — Call.ByName included, not just the response it resolves with —
 * before giving up; see isRetryableDesktopError for why a rejected call
 * needs this exactly as much as a successful one carrying a transient
 * DomainError, and desktopRetryDelays for why how long to retry depends on
 * whether the desktop has already proven itself. client.invoke() below only
 * sees the final, exhausted failure, which is what lets a page that's never
 * inside the desktop still fall back to HTTP after a bounded number of
 * attempts rather than none.
 */
const desktop: Client = {
  async invoke(key, input) {
    const delays = desktopRetryDelays();
    for (let attempt = 0; ; attempt++) {
      try {
        const raw = await Call.ByName(DOMAIN_SERVICE_INVOKE, key, input ?? {});
        markDesktopConfirmed();
        return unwrap(typeof raw === "string" ? JSON.parse(raw) : raw);
      } catch (err) {
        const delay = delays[attempt];
        if (!isRetryableDesktopError(err) || delay === undefined) throw err;
        await sleep(delay);
      }
    }
  },
};

/**
 * internal/transport/wailsvc.SystemService — the platform calls: opening a
 * URL, picking a folder, syncing the native window material with a theme
 * change. Unlike DomainService's single generic Invoke, each of these is
 * its own bound Go method with its own typed arguments, called the same
 * way (Call.ByName with the fully qualified name) but without Invoke's
 * envelope: a Go error here rejects the call directly as a RuntimeError,
 * so callers just try/catch around the call itself.
 *
 * No retry here, deliberately: every caller already gates these behind
 * isDesktop() (see its own comment on why that can be wrong immediately
 * after the window opens) or otherwise treats a rejection as "not
 * available right now" rather than a failure worth surfacing — retrying a
 * platform call that isn't there yet buys nothing a browser tab (which
 * will never have one) doesn't also pay for.
 */
export const system = {
  /**
   * Where the daemon is, as an http(s) origin — empty outside the desktop.
   *
   * The realtime channel is the one connection the webview opens itself
   * rather than routing through the bridge, so it needs a real address:
   * inside the desktop window the page is served by the application, and a
   * URL built from window.location reaches the asset host, never the
   * daemon.
   */
  async daemonAddress(): Promise<string> {
    return (await Call.ByName(`${WAILSVC_PKG}.SystemService.DaemonAddress`)) as string;
  },

  /**
   * The directory the window was launched in, or "" when it was launched
   * nowhere in particular (from the dock, from Spotlight) and in a browser.
   *
   * The onboarding wizard offers it as the default folder for the first
   * workspace. Until this existed the desktop registered that directory
   * itself, before the wizard had asked for a name — which is what made the
   * first workspace always take the folder's name and the copilot always
   * take the default one.
   */
  async launchDirectory(): Promise<string> {
    try {
      return ((await Call.ByName(`${WAILSVC_PKG}.SystemService.LaunchDirectory`)) as string) ?? "";
    } catch {
      // A browser tab has no window to have been launched anywhere, and a
      // desktop whose bridge is still warming up answers later. Neither is
      // worth an error: the field simply stays empty, which is a valid
      // choice the wizard already supports.
      return "";
    }
  },

  async setAppearance(appearance: string, windows: string): Promise<void> {
    await Call.ByName(`${WAILSVC_PKG}.SystemService.SetAppearance`, appearance, windows);
  },
  async openExternal(url: string): Promise<void> {
    await Call.ByName(`${WAILSVC_PKG}.SystemService.OpenExternal`, url);
  },
  /**
   * The coding agents the skill can be installed into on this machine —
   * internal/transport/wailsvc.SystemService.SkillTargets. Desktop only:
   * a browser tab has no view of the user's home directory.
   */
  async skillTargets(): Promise<SkillTarget[]> {
    return (await Call.ByName(`${WAILSVC_PKG}.SystemService.SkillTargets`)) as SkillTarget[];
  },
  /**
   * Writes the skill compiled into the application into one agent's skills
   * directory ("claude-code", "codex", …) or every detected one ("all") —
   * the desktop's own `aos self skill install`.
   */
  async installSkill(target: string): Promise<SkillInstallResult> {
    return (await Call.ByName(`${WAILSVC_PKG}.SystemService.InstallSkill`, target)) as SkillInstallResult;
  },
  async pickFiles(opts: {
    title?: string;
    directory?: string;
    multiple?: boolean;
    directories?: boolean;
    extensions?: string[];
  }): Promise<string[]> {
    return (await Call.ByName(`${WAILSVC_PKG}.SystemService.PickFiles`, opts)) as string[];
  },
};

/** One answer from a non-registry surface, forwarded by the bridge. */
export interface BridgeResponse {
  status: number;
  body: string;
}

/**
 * Calls one of the daemon's non-registry HTTP surfaces (`/api/file/*`,
 * `/api/auth/*`) through the Wails bridge, which attaches the window's own
 * credential.
 *
 * These two surfaces are not commands, so they have no `Invoke` path — and
 * called with a plain `fetch` from inside the desktop window they were
 * cross-origin, carried neither cookie nor bearer, and came back 401 (when
 * they were not blocked by CORS before leaving at all). That is why the file
 * tree, the editor, the diffs and the account roster were empty in the
 * application while working perfectly in a browser tab.
 *
 * Rejects when there is no bridge — a browser tab — which is the signal for
 * the caller to use `fetch`, where the session cookie is sent automatically.
 */
export async function bridgeFetch(
  method: string,
  path: string,
  contentType = "",
  body = "",
): Promise<BridgeResponse> {
  return (await Call.ByName(
    `${WAILSVC_PKG}.DomainService.Fetch`,
    method,
    path,
    contentType,
    body,
  )) as BridgeResponse;
}

/**
 * The browser transport.
 *
 * The workspace goes in a header rather than a cookie. That is defect #5 of the
 * original: a cookie is sent by the browser on a WebSocket upgrade whether or
 * not the page meant to, which is what made its realtime channel reachable from
 * another origin.
 */
const http: Client = {
  async invoke(key, input) {
    const response = await fetch(daemonURL(`/api/${key.replaceAll("_", "/")}`), {
      method: "POST",
      headers: {
        "content-type": "application/json",
        ...workspaceHeader(),
      },
      body: JSON.stringify(input ?? {}),
    });

    let payload: unknown;
    try {
      payload = await response.json();
    } catch {
      throw new DomainError({
        code: "TRANSPORT_UNREADABLE",
        message: `the daemon answered ${response.status} with something that is not JSON`,
        status: response.status,
      });
    }
    return unwrap(payload);
  },
};

let activeWorkspace = "";

/** Where the chosen workspace is remembered between reloads. */
const WORKSPACE_STORAGE_KEY = "aos.workspace";

/**
 * The workspace this page addressed last time, if it said.
 *
 * Read at boot by the workspace store, which used to have no memory at all:
 * it asked for the list and took the first entry, so switching workspace
 * lasted exactly until the next reload and then silently reverted.
 */
export function rememberedWorkspace(): string {
  try {
    return localStorage.getItem(WORKSPACE_STORAGE_KEY) ?? "";
  } catch {
    // Private mode, or a webview with site data off. Not a reason to fail;
    // the caller falls back to the first workspace as it always did.
    return "";
  }
}

/**
 * Sets the workspace every subsequent call addresses.
 *
 * Three places have to agree, and before this only the first did:
 *
 * - this module's own HTTP header, for a browser tab;
 * - the Go daemon client behind the Wails bridge, which is what actually
 *   sends the header for every command in the desktop window — it was pinned
 *   to whatever workspace the window opened with, and its header beats the
 *   cookie the page sets, so switching workspace in the application changed
 *   nothing at all;
 * - localStorage, so the choice survives a reload.
 */
export function setWorkspace(id: string): void {
  if (activeWorkspace === id) return;
  activeWorkspace = id;
  try {
    if (id) localStorage.setItem(WORKSPACE_STORAGE_KEY, id);
  } catch {
    // The choice still applies to this session.
  }
  // Fire and forget: in a browser tab there is no bridge to tell, and the
  // header above is already the whole answer there.
  if (id) {
    void Call.ByName(`${WAILSVC_PKG}.DomainService.SetWorkspace`, id).catch(() => {});
  }
}

/** The workspace every subsequent call addresses. */
export function getWorkspace(): string {
  return activeWorkspace;
}

function workspaceHeader(): Record<string, string> {
  return activeWorkspace ? { "x-workspace-id": activeWorkspace } : {};
}

/**
 * The client this page runs on.
 *
 * Every call tries the desktop transport first and falls back to HTTP on
 * failure, rather than deciding once which one applies. There is no reliable
 * synchronous "am I in the desktop window" signal in Wails3 (see isDesktop's
 * comment) — but attempting the call is itself a reliable signal: inside the
 * desktop window the native host answers, and in a browser tab the request
 * to /wails/runtime just 404s against whatever the daemon or dev server
 * serves there, which Call.ByName surfaces as a rejected promise. Once a
 * page has confirmed which one it is (see confirmedDesktop above), this
 * costs one fast rejected call per request for the loser transport; only
 * the first few seconds of a page that turns out not to be the desktop pay
 * the full retry budget, while genuinely waiting for the window's own
 * bridge to warm up.
 */
export const client: Client = {
  async invoke(key, input) {
    try {
      return await desktop.invoke(key, input);
    } catch {
      return http.invoke(key, input);
    }
  },
};
