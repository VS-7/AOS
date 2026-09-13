import { DomainError, bridgeFetch, callBridge, unwrap } from "./client";
import { isDesktopWindow } from "./wails";
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

/**
 * The page's own "somebody just signed out", fired by the logout action.
 *
 * Distinct from lib/client.ts's UNAUTHENTICATED_EVENT, which is a *suspicion*
 * a failed call raises and AuthGate checks before acting on. This one is a
 * fact the page established itself, so the gate shows Login at once — before
 * the router can react to the navigation the account menu makes next.
 */
export const SIGNED_OUT_EVENT = "aos:signed-out";

/**
 * AuthGate's "the daemon says somebody is signed in again", after it had
 * shown Login or Onboarding.
 *
 * The auth store the router reads is not the gate's, and nothing else tells it:
 * after signing in through the gate it went on saying "signed out", and the
 * router sent the person to its own login page a second time.
 */
export const AUTHENTICATED_EVENT = "aos:authenticated";

const WAILSVC_PKG = "github.com/OWNER/aos/internal/transport/wailsvc";
const AUTH_SERVICE = `${WAILSVC_PKG}.AuthService`;

/**
 * One of AuthService's bound methods.
 *
 * A refusal — a wrong password, no session, a daemon that is down — arrives
 * as a Go error, which callBridge reads as the domain error it is and hands
 * straight back. It used to be taken for a bridge still warming up: retried
 * four times over nearly four seconds and then sent again over HTTP, so one
 * wrong password was checked six times before the form said anything, and a
 * daemon that was down kept the window blank for five seconds before AuthGate
 * could say it was waiting. AuthGate has its own backoff for that; this does
 * not need a second one.
 */
async function desktopCall<T>(method: string, ...args: unknown[]): Promise<T> {
  const raw = await callBridge(`${AUTH_SERVICE}.${method}`, args);
  return unwrap<T>(typeof raw === "string" ? JSON.parse(raw) : raw);
}

/** Reads a surface's answer, which is the daemon's own JSON envelope. */
function readEnvelope<T>(status: number, body: string): T {
  let payload: unknown;
  try {
    payload = JSON.parse(body);
  } catch {
    throw new DomainError({
      code: "TRANSPORT_UNREADABLE",
      message: `the daemon answered ${status} with something that is not JSON`,
      status,
    });
  }
  return unwrap<T>(payload);
}

/**
 * One request to /api/auth, over the transport this page has.
 *
 * Inside the desktop window that is the bridge, which attaches the window's
 * credential: a plain fetch from there is cross-origin and anonymous, which is
 * why the account roster, the profile form and the password change were all
 * refused in the application. In a browser tab it is fetch, same-origin, with
 * the session cookie. Never both — see lib/client.ts's `client` for what a
 * fallback from one to the other cost.
 */
async function httpRequest<T>(path: string, init?: RequestInit): Promise<T> {
  if (isDesktopWindow) {
    const headers = (init?.headers ?? {}) as Record<string, string>;
    const answer = await bridgeFetch(
      init?.method ?? "GET",
      path,
      headers["content-type"] ?? "",
      typeof init?.body === "string" ? init.body : "",
    );
    return readEnvelope<T>(answer.status, answer.body);
  }

  let response: Response;
  try {
    response = await fetch(daemonURL(path), init);
  } catch (err) {
    throw new DomainError({
      code: "TRANSPORT_UNREACHABLE",
      message: err instanceof Error ? err.message : "the daemon could not be reached",
      status: 503,
    });
  }
  return readEnvelope<T>(response.status, await response.text());
}

/**
 * AuthService's method inside the desktop window, the HTTP route in a browser.
 *
 * `desktopMethod` is `null` for an endpoint AuthService does not bind (see
 * AuthService's own doc comment in internal/transport/wailsvc/auth.go for
 * which five it binds); the window reaches those through the bridge's Fetch.
 */
async function call<T>(desktopMethod: string | null, desktopArgs: unknown[], httpPath: string, httpInit?: RequestInit): Promise<T> {
  if (isDesktopWindow && desktopMethod !== null) return desktopCall<T>(desktopMethod, ...desktopArgs);
  return httpRequest<T>(httpPath, httpInit);
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
 * No AuthService method: AuthService binds five over the Wails bridge and this
 * is not one of them, so the window reaches the route through the bridge's
 * Fetch — see `call`'s own doc on the `null` argument.
 */
export function users(): Promise<{ users: PublicUser[] }> {
  return call<{ users: PublicUser[] }>(null, [], "/api/auth/users");
}

/** Reads the account the current session belongs to. */
export function session(): Promise<{ user: PublicUser }> {
  return call<{ user: PublicUser }>("Session", [], "/api/auth/session");
}

/**
 * Changes the signed-in account's name and email.
 *
 * No AuthService method, same as `changePassword` below and for the same
 * reason: AuthService binds five methods over the bridge and this is not one
 * of them.
 */
export function updateProfile(name: string, email: string): Promise<{ user: PublicUser }> {
  return call<{ user: PublicUser }>(
    null,
    [],
    "/api/auth/profile",
    { method: "POST", headers: jsonHeaders, body: JSON.stringify({ name, email }) },
  );
}

/**
 * Changes the current session's password.
 *
 * Desktop method is `null` on purpose: AuthService binds only Status,
 * Login, Onboarding, Logout and Session, not this. The route is real and
 * fully wired (internal/transport/authapi), so it is reached directly — over
 * the bridge's Fetch inside the window — rather than through a bound method
 * that does not exist.
 */
export async function changePassword(current: string, next: string): Promise<void> {
  await call<Record<string, never>>(
    null,
    [],
    "/api/auth/password",
    // `currentPassword`/`newPassword`, not `current`/`next`: those are the
    // names the handler decodes (internal/transport/authapi's
    // changePassword). Sending the short ones meant Go decoded two empty
    // strings, so every attempt failed the current-password check and the
    // screen reported a wrong password the person had typed correctly.
    {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify({ currentPassword: current, newPassword: next }),
    },
  );
}
