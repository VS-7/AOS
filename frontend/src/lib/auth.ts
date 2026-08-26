import { Call } from "@wailsio/runtime";
import { DomainError, unwrap } from "./client";
import { desktopRetryDelays, markDesktopConfirmed, sleep } from "./desktop-transport";
import { daemonURL } from "./daemon-origin";

/** Mirrors internal/transport/wailsvc.PublicUser / internal/domain/auth.Public. */
export interface PublicUser {
  id: string;
  name: string;
  username: string;
  email: string;
  role: string;
}

/** What a successful login or onboarding answers with. */
export interface AuthResult {
  user: PublicUser;
  expiresAt: string;
}

/** What the app checks before deciding what to show. */
export interface AuthStatus {
  onboarded: boolean;
  authenticated: boolean;
}

const WAILSVC_PKG = "github.com/OWNER/aos/internal/transport/wailsvc";
const AUTH_SERVICE = `${WAILSVC_PKG}.AuthService`;

/**
 * Whether a rejected Call.ByName is worth retrying rather than falling back
 * to HTTP. Mirrors client.ts's isRetryableDesktopError: a well-formed
 * DomainError means the call reached the daemon and back, so whatever it
 * says is the real answer, not a warm-up symptom.
 */
function isRetryable(err: unknown): boolean {
  return !(err instanceof DomainError);
}

// Shares its cold-start budget with client.ts's desktop transport — see
// desktop-transport.ts — rather than keeping its own separate clock. Two
// independent retry windows on the same page compound into a much longer
// wait than either alone: AuthGate's status() check and, moments later,
// RootLayout's own data queries would otherwise each pay the full price.
async function desktopCall<T>(method: string, ...args: unknown[]): Promise<T> {
  const delays = desktopRetryDelays();
  for (let attempt = 0; ; attempt++) {
    try {
      const raw = await Call.ByName(`${AUTH_SERVICE}.${method}`, ...args);
      markDesktopConfirmed();
      return unwrap<T>(typeof raw === "string" ? JSON.parse(raw) : raw);
    } catch (err) {
      const delay = delays[attempt];
      if (!isRetryable(err) || delay === undefined) throw err;
      await sleep(delay);
    }
  }
}

async function httpRequest<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    // Absolute inside the desktop window, where a relative path reaches the
    // application's own asset host rather than the daemon — see daemon-origin.
    response = await fetch(daemonURL(path), init);
  } catch (err) {
    throw new DomainError({
      code: "TRANSPORT_UNREACHABLE",
      message: err instanceof Error ? err.message : "the daemon could not be reached",
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
}

/**
 * Tries the desktop transport, falling back to HTTP — the same trade
 * client.ts's client.invoke makes, and for the same reason: there is no
 * reliable synchronous "am I in the desktop window" signal, but attempting
 * the call is itself the reliable signal.
 *
 * `desktopMethod` may be `null` for an endpoint AuthService does not bind
 * at all (see AuthService's own doc comment in
 * internal/transport/wailsvc/auth.go for which five it binds). Trying
 * `desktopCall` for a method that will never exist would still pay the
 * full retry budget in desktop-transport.ts before falling back — a
 * multi-second stall for no gain, since the answer is already known
 * without asking. `null` skips straight to HTTP instead.
 */
async function call<T>(desktopMethod: string | null, desktopArgs: unknown[], httpPath: string, httpInit?: RequestInit): Promise<T> {
  if (desktopMethod === null) return httpRequest<T>(httpPath, httpInit);
  try {
    return await desktopCall<T>(desktopMethod, ...desktopArgs);
  } catch {
    return httpRequest<T>(httpPath, httpInit);
  }
}

const jsonHeaders = { "content-type": "application/json" };

/** What the app should show right now, before anyone has a session. */
export function status(): Promise<AuthStatus> {
  return call<AuthStatus>("Status", [], "/api/auth/status");
}

/** Signs into an existing account. */
export function login(identifier: string, password: string): Promise<AuthResult> {
  return call<AuthResult>(
    "Login",
    [identifier, password],
    "/api/auth/login",
    { method: "POST", headers: jsonHeaders, body: JSON.stringify({ identifier, password }) },
  );
}

/** Creates the installation's first account. */
export function onboarding(name: string, email: string, password: string): Promise<AuthResult> {
  return call<AuthResult>(
    "Onboarding",
    [name, email, password],
    "/api/auth/onboarding",
    { method: "POST", headers: jsonHeaders, body: JSON.stringify({ name, email, password }) },
  );
}

/** Ends the current session. */
export async function logout(): Promise<void> {
  await call<Record<string, never>>("Logout", [], "/api/auth/logout", { method: "POST", headers: jsonHeaders, body: "{}" });
}

/**
 * The accounts on this installation.
 *
 * HTTP only: AuthService binds five methods over the Wails bridge and this is
 * not one of them, so asking the desktop first would pay the whole cold-start
 * retry budget before falling back — see `call`'s own doc on the `null`
 * argument.
 */
export function users(): Promise<{ users: PublicUser[] }> {
  return call<{ users: PublicUser[] }>(null, [], "/api/auth/users");
}

/** Reads the account the current session belongs to. */
export function session(): Promise<{ user: PublicUser }> {
  return call<{ user: PublicUser }>("Session", [], "/api/auth/session");
}

/**
 * Changes the current session's password.
 *
 * Desktop method is `null` on purpose: AuthService binds only Status,
 * Login, Onboarding, Logout and Session, not this. The HTTP route is real
 * and fully wired (internal/transport/authapi), so this goes straight
 * there instead of burning the desktop retry budget on a method call that
 * would always fail.
 */
export async function changePassword(current: string, next: string): Promise<void> {
  await call<Record<string, never>>(
    null,
    [],
    "/api/auth/password",
    { method: "POST", headers: jsonHeaders, body: JSON.stringify({ current, next }) },
  );
}
