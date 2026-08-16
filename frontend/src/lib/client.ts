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

/** The shape Wails puts on the window when the page is inside the desktop. */
declare global {
  interface Window {
    go?: {
      DomainService?: {
        Invoke(key: string, input: string): Promise<string>;
      };
      SystemService?: {
        SetAppearance(appearance: string, windows: string): Promise<void>;
        OpenExternal(url: string): Promise<void>;
        Ping(): Promise<boolean>;
      };
      ApprovalService?: {
        Resolve(id: string, decision: unknown): Promise<boolean>;
        Pending(): Promise<unknown[]>;
      };
    };
  }
}

/** Whether this page is inside the desktop window rather than a browser tab. */
export function isDesktop(): boolean {
  return typeof window !== "undefined" && !!window.go?.DomainService;
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

/**
 * The desktop transport. Wails hands strings across the boundary, so the
 * payload is serialised here and the answer parsed back.
 */
const desktop: Client = {
  async invoke(key, input) {
    const service = window.go?.DomainService;
    if (!service) throw new DomainError({ code: "DESKTOP_UNAVAILABLE", message: "the desktop binding is gone" });
    const raw = await service.Invoke(key, JSON.stringify(input ?? {}));
    return unwrap(JSON.parse(raw));
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

/** The client this page runs on. */
export const client: Client = isDesktop() ? desktop : http;
