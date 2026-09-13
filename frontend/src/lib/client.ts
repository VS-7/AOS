import { Call } from "@wailsio/runtime";
import { daemonURL } from "./daemon-origin";
import { isDesktopWindow } from "./wails";
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
 * actually succeeds through it, true from then on. client.invoke() does not
 * use this: it reads `isDesktopWindow` (lib/wails.ts), which the window
 * states in its own URL and is right from the first line of the bundle.
 * Prefer that one; this is kept for the call sites that already read it.
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
    // The names the daemon actually sends (internal/core/apperr's `issue`
    // and `cta` tags). Only the plural spellings were read, so every error
    // that reached the page arrived with no issue and no call to action.
    issue?: Record<string, unknown>;
    cta?: Array<{ label: string; command?: string; tool?: string }>;
  }) {
    super(payload.message ?? payload.code ?? "the call failed");
    this.name = "DomainError";
    this.code = payload.code ?? "UNKNOWN";
    this.status = payload.status ?? 500;
    this.issues = payload.issues ?? payload.issue ?? {};
    this.actions = payload.actions ?? payload.cta ?? [];
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
/**
 * The event the page fires when the daemon says the credential is no good.
 *
 * A session lasts thirty days, and it can also be revoked — a logout from
 * another window, a daemon whose accounts were reset. There was no path back
 * from either: `AuthGate` asks once, at mount, so the application stayed on
 * its screens and answered every action with "this request carries no valid
 * credential" as a toast. The only ways out were a reload (browser) or a
 * restart (desktop).
 *
 * The transports fire this; AuthGate listens and re-asks, which sends the
 * person to the Login screen exactly once and by the front door.
 */
export const UNAUTHENTICATED_EVENT = "aos:unauthenticated";

/**
 * The code every transport uses for "the daemon did not answer at all".
 *
 * Distinct from a refusal: the daemon answering 403 is a working system
 * saying no, and a connection that never completed is the system being
 * absent. The layout draws them differently, which it could not do while one
 * of them arrived as a bare TypeError.
 */
export const DAEMON_UNREACHABLE_CODE = "AOS_DAEMON_UNREACHABLE";

/** The event the page fires when the daemon stops answering, and when it comes back. */
export const DAEMON_EVENT = "aos:daemon";

/** Whether an error is the daemon being absent rather than refusing. */
export function isDaemonUnreachable(error: unknown): boolean {
  if (!error || typeof error !== "object") return false;
  const code = (error as { code?: string }).code;
  return code === DAEMON_UNREACHABLE_CODE || code === "TRANSPORT_UNREACHABLE";
}

/**
 * Whether an error says the credential is no good.
 *
 * By any code that ends that way: the daemon answers AOS_HTTP_UNAUTHENTICATED
 * when a request carries no credential at all, and passes auth.Service's
 * AOS_AUTH_UNAUTHENTICATED through for one that is expired or revoked
 * (httpapi's authenticate middleware). Only the first was recognised, so a
 * revoked session never went back to Login — its screens rendered empty and
 * said nothing. The envelope carries no HTTP status (apperr's HTTPStatus is
 * not serialised), so `status` only helps an error the page built itself.
 */
function isUnauthenticated(error: { code?: string; status?: number }): boolean {
  return error.status === 401 || /(^|_)UNAUTHENTICATED$/.test(error.code ?? "");
}

function announceIfUnauthenticated(error: { code?: string; status?: number }): void {
  if (!isUnauthenticated(error) || typeof window === "undefined") return;
  window.dispatchEvent(new Event(UNAUTHENTICATED_EVENT));
}

/** A domain error, announced first when it is about the credential. */
function raise(payload: ConstructorParameters<typeof DomainError>[0]): DomainError {
  announceIfUnauthenticated(payload);
  return new DomainError(payload);
}

export function unwrap<T>(raw: unknown): T {
  const envelope = raw as Envelope<T>;
  if (envelope && typeof envelope === "object" && "error" in envelope && envelope.error) {
    throw raise(envelope.error);
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
 * What a rejected bridge call means, when the bridge answered at all.
 *
 * A bound Go method that returns an error does not resolve with an envelope
 * the way DomainService.Invoke does: Wails rejects the call with a
 * RuntimeError whose `cause` is that error marshalled — for an apperr, the
 * same `{code, message, issue, cta}` a daemon envelope carries. That is a
 * real answer from a warm bridge (AuthService refusing a password, the Go
 * client saying the daemon is down, Fetch refusing a path), and it was read
 * as the bridge being absent: retried for four seconds and then repeated over
 * HTTP, which is how one wrong password was checked six times.
 *
 * Null for a rejection that carries no code — a fetch that never reached a
 * host, a bridge still warming up. Those are the only failures worth waiting
 * out.
 */
export function bridgeAnswer(err: unknown): DomainError | null {
  if (err instanceof DomainError) return err;
  if (!err || typeof err !== "object") return null;

  let cause: unknown = (err as { cause?: unknown }).cause;
  if (typeof cause === "string") {
    try {
      cause = JSON.parse(cause);
    } catch {
      // Not JSON; the message below is the other place a code can be.
    }
  }
  const payload = cause as ConstructorParameters<typeof DomainError>[0] | null;
  if (payload && typeof payload === "object" && typeof payload.code === "string" && payload.code) {
    return raise(payload);
  }

  // An error Wails could not marshal still reaches the page as its Error()
  // string, and an apperr's reads "CODE: text".
  const message = (err as { message?: unknown }).message;
  const coded = typeof message === "string" ? /^(AOS_[A-Z0-9_]+): ([\s\S]*)$/.exec(message) : null;
  if (coded) return raise({ code: coded[1], message: coded[2] });
  return null;
}

/** What a desktop call reports when the bridge itself never answered. */
function bridgeUnavailable(method: string, cause: unknown): DomainError {
  return new DomainError({
    code: "TRANSPORT_UNREACHABLE",
    message: "the desktop bridge did not answer",
    status: 503,
    issues: { method, cause: cause instanceof Error ? cause.message : String(cause) },
  });
}

/**
 * Calls a bound Go method, waiting out a bridge that is still warming up and
 * nothing else.
 *
 * `retryAnswer` names the answers worth asking again for; by default none is.
 * What this never does is fall back to HTTP. Inside the desktop window the
 * credential lives in the Go process, so a plain request from the page is
 * cross-origin and anonymous, and its failure — a 401, a refused connection —
 * is not the answer to anything the person did. Every screen used to show
 * that failure instead of the bridge's real one.
 */
export async function callBridge(
  method: string,
  args: unknown[],
  retryAnswer: (answer: DomainError) => boolean = () => false,
): Promise<unknown> {
  const delays = desktopRetryDelays();
  for (let attempt = 0; ; attempt++) {
    try {
      const raw = await Call.ByName(method, ...args);
      markDesktopConfirmed();
      return raw;
    } catch (err) {
      const answer = bridgeAnswer(err);
      // An answer of any kind proves the bridge is there.
      if (answer) markDesktopConfirmed();
      if (answer && !retryAnswer(answer)) throw answer;
      const delay = delays[attempt];
      if (delay === undefined) throw answer ?? bridgeUnavailable(method, err);
      await sleep(delay);
    }
  }
}

/**
 * The answers from DomainService.Invoke worth asking again for: a daemon the
 * window started moments ago that has not finished starting, and an argument
 * mixup in the bridge under concurrency. Not found, validation, a refusal —
 * none of those is fixed by repeating it.
 */
function retryInvokeAnswer(answer: DomainError): boolean {
  return answer.code === DAEMON_UNREACHABLE_CODE || answer.code === "AOS_DESKTOP_NO_COMMAND_NAMED";
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
 * Waits out a bridge that is still warming up, and asks again only for the
 * answers retryInvokeAnswer names — see callBridge. The daemon's own refusal
 * arrives inside the envelope and is thrown as it is.
 */
const desktop: Client = {
  async invoke(key, input) {
    const raw = await callBridge(DOMAIN_SERVICE_INVOKE, [key, input ?? {}], retryInvokeAnswer);
    return unwrap(typeof raw === "string" ? JSON.parse(raw) : raw);
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
   * Stops the daemon and starts it again.
   *
   * Not a command, and deliberately: inside the daemon `gateway_restart`
   * refuses, because Stop would signal its own pid — which is exactly what
   * used to happen, leaving the window with a terminated daemon, an
   * unclassified 500 from the connection dropping, and no way back short of
   * relaunching the application. Supervision belongs to whatever launched
   * the daemon, and in the desktop that is the window's own process.
   *
   * A browser tab has no supervisor and the bridge is absent; the caller
   * shows the terminal instruction instead.
   */
  async restartDaemon(): Promise<void> {
    await Call.ByName(`${WAILSVC_PKG}.SystemService.RestartDaemon`);
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
 * Desktop window only. A browser tab has no bridge and never asks — it uses
 * `fetch`, where the session cookie is sent automatically — and inside the
 * window a failure here is the answer: see callBridge for why it is never
 * repeated over HTTP.
 */
export async function bridgeFetch(
  method: string,
  path: string,
  contentType = "",
  body = "",
): Promise<BridgeResponse> {
  return (await callBridge(`${WAILSVC_PKG}.DomainService.Fetch`, [
    method,
    path,
    contentType,
    body,
  ])) as BridgeResponse;
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
    let response: Response;
    try {
      response = await fetch(daemonURL(`/api/${key.replaceAll("_", "/")}`), {
        method: "POST",
        headers: {
          "content-type": "application/json",
          ...workspaceHeader(),
        },
        body: JSON.stringify(input ?? {}),
      });
    } catch {
      // A daemon that is not answering is not a programming error, and it
      // used to reach the screen as one: `fetch` rejects a refused connection
      // with a bare TypeError, which is what every action showed — "Load
      // failed" — after the daemon crashed or was stopped from a terminal.
      // Classified, it reads as what it is, and the layout can say the
      // window is waiting for the daemon rather than that something broke.
      throw new DomainError({
        code: DAEMON_UNREACHABLE_CODE,
        message: "the daemon is not answering",
        status: 503,
      });
    }

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
 *
 * Resolves once the bridge has applied it, and a caller about to make a
 * workspace-scoped call awaits that. It was fire-and-forget, so the store's
 * first `workspace_get` raced it and sometimes answered for whichever
 * workspace the Go side had adopted on its own. The bridge is told every
 * time, even for the id this module already holds: the Go side forgets the
 * page's choice when the daemon restarts or somebody signs in again, and a
 * repeated id was exactly what never reached it.
 */
export async function setWorkspace(id: string): Promise<void> {
  activeWorkspace = id;
  try {
    if (id) localStorage.setItem(WORKSPACE_STORAGE_KEY, id);
  } catch {
    // The choice still applies to this session.
  }
  // A browser tab has no bridge to tell; the header above is the whole
  // answer there.
  if (id && isDesktopWindow) {
    await callBridge(`${WAILSVC_PKG}.DomainService.SetWorkspace`, [id]);
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
 * The client this page runs on: the bridge inside the desktop window, HTTP in
 * a browser tab, decided once by `isDesktopWindow` — a synchronous fact, since
 * the window states it in the URL it opens (`?daemon=`).
 *
 * There is no fallback from one to the other, in either direction. A browser
 * tab that probed the bridge paid a rejected `POST /wails/runtime` per call
 * and roughly seven seconds of cold-start retries before its first screen.
 * And a window that fell back to HTTP after *any* bridge failure — a chat that
 * does not exist, a precondition, a daemon that is down — sent that request
 * from `wails://localhost` with no credential, so the screen showed the
 * retry's 401 or refused connection instead of the real answer, and a 401
 * announced from a transport that never held the session sent AuthGate round
 * a loop that remounted the whole application.
 */
export const client: Client = {
  async invoke(key, input) {
    return isDesktopWindow ? desktop.invoke(key, input) : http.invoke(key, input);
  },
};
