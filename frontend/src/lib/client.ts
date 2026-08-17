import { Call } from "@wailsio/runtime";
import type { CommandInput, CommandKey, CommandOutput } from "./schema";

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
declare global {
  interface Window {
    go?: {
      SystemService?: {
        SetAppearance(appearance: string, windows: string): Promise<void>;
        OpenExternal(url: string): Promise<void>;
        Ping(): Promise<boolean>;
        PickFiles(opts: {
          title?: string;
          directory?: string;
          multiple?: boolean;
          directories?: boolean;
          extensions?: string[];
        }): Promise<string[]>;
      };
      ApprovalService?: {
        Resolve(id: string, decision: unknown): Promise<boolean>;
        Pending(): Promise<unknown[]>;
      };
    };
  }
}

/**
 * Whether this page is inside the desktop window rather than a browser tab.
 *
 * There is no synchronous signal for this in Wails3 — window.location stays
 * a normal http(s) origin either way, and the interception that makes the
 * desktop transport work happens at the network layer, not in anything JS
 * can inspect ahead of a call. client.invoke() below doesn't use this: it
 * tries the desktop transport and falls back on failure, which is the only
 * check that's actually reliable. This export is a best-effort synchronous
 * guess for the handful of call sites (native chrome, the file picker) that
 * need an answer before they can make any call at all.
 */
export function isDesktop(): boolean {
  return typeof window !== "undefined" && !!window.go?.SystemService;
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

function unwrap<T>(raw: unknown): T {
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

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

const DOMAIN_SERVICE_INVOKE = "github.com/OWNER/aos/internal/transport/wailsvc.DomainService.Invoke";

// Backoff (ms) before each retried desktop call, longest first attempt last.
// ensureDaemon in cmd/aos-desktop/main.go starts the daemon in the background
// so a slow one never blocks the window from opening — which means a window
// that just opened can genuinely be rendering, and firing its first queries,
// before the daemon it's supervising has finished booting (first boot also
// runs an argon2id hash for ensureLocalAccount, deliberately slow). Total
// budget here (~3.7s) is chosen to comfortably cover that, not to paper over
// a daemon that's actually not coming up — errNeverBecameHealthy in
// gateway.Service still fires on its own, independent timeout.
const DESKTOP_RETRY_DELAYS_MS = [200, 500, 1000, 2000];

// A rejected Call.ByName looks identical whether there's truly no Wails host
// (a plain browser tab) or there is one that just hasn't warmed up yet (the
// window's first few calls). Patient retries are right for the second case
// and expensive for the first — a browser tab would eat ~3.7s on every call,
// forever, if it always retried. confirmedDesktop breaks the tie: any
// successful desktop call proves the host is real, after which failures are
// assumed transient and always retried patiently. Before that first success,
// patience is bounded to the window right after the page loads, when a real
// host warming up is the likely explanation; past it, one quick attempt is
// enough to conclude this is not the desktop and hand off to HTTP.
let confirmedDesktop = false;
const desktopColdStartUntil = typeof window === "undefined" ? 0 : Date.now() + 5_000;

function desktopRetryDelays(): readonly number[] {
  if (confirmedDesktop || Date.now() < desktopColdStartUntil) return DESKTOP_RETRY_DELAYS_MS;
  return [];
}

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
        confirmedDesktop = true;
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
 * The browser transport.
 *
 * The workspace goes in a header rather than a cookie. That is defect #5 of the
 * original: a cookie is sent by the browser on a WebSocket upgrade whether or
 * not the page meant to, which is what made its realtime channel reachable from
 * another origin.
 */
const http: Client = {
  async invoke(key, input) {
    const response = await fetch(`/api/${key.replaceAll("_", "/")}`, {
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

/** Sets the workspace every subsequent call addresses. */
export function setWorkspace(id: string): void {
  activeWorkspace = id;
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
