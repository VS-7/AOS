import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { useEffect } from "react";

const status = vi.hoisted(() => vi.fn());
const login = vi.hoisted(() => vi.fn());

vi.mock("@/lib/auth", async () => {
  const actual = await vi.importActual<typeof import("@/lib/auth")>("@/lib/auth");
  return { ...actual, status, login };
});

import { AuthGate } from "./AuthGate";
import { AUTHENTICATED_EVENT, SIGNED_OUT_EVENT } from "@/lib/auth";
import { DomainError, UNAUTHENTICATED_EVENT } from "@/lib/client";
import { AppStateProvider } from "@/lib/app-state";

let mounts = 0;

function Application() {
  useEffect(() => {
    mounts += 1;
  }, []);
  return <div>the application</div>;
}

function mount() {
  return render(
    <AppStateProvider>
      <AuthGate>
        <Application />
      </AuthGate>
    </AppStateProvider>,
  );
}

async function flush() {
  await act(async () => {
    await Promise.resolve();
  });
}

beforeEach(() => {
  cleanup();
  mounts = 0;
  status.mockReset();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("AuthGate, once somebody is signed in", () => {
  // A 401 from a transport that never held the session — the desktop's old
  // HTTP fallback — arrived while the bridge still said "authenticated". The
  // gate unmounted the whole application to ask, remounted it on the answer,
  // the screen failed the same way again, and round it went about fifteen
  // times a second, forever.
  it("keeps the application mounted while it asks again", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    status.mockResolvedValue({ onboarded: true, authenticated: true });
    mount();
    expect(await screen.findByText("the application")).toBeTruthy();

    await act(async () => {
      window.dispatchEvent(new Event(UNAUTHENTICATED_EVENT));
      await vi.advanceTimersByTimeAsync(5_000);
    });
    await flush();

    expect(screen.getByText("the application")).toBeTruthy();
    expect(mounts).toBe(1);
    expect(status).toHaveBeenCalledTimes(2);
  });

  it("asks at most once for a burst, while the last answer is fresh", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    status.mockResolvedValue({ onboarded: true, authenticated: true });
    mount();
    expect(await screen.findByText("the application")).toBeTruthy();

    for (let i = 0; i < 20; i++) {
      await act(async () => {
        window.dispatchEvent(new Event(UNAUTHENTICATED_EVENT));
      });
      await flush();
    }

    // The first check at mount and one more for the whole burst; a trailing
    // one once things are quiet, so a real revocation inside the burst is not
    // lost either.
    expect(status.mock.calls.length).toBeLessThanOrEqual(2);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000);
    });
    expect(status.mock.calls.length).toBeLessThanOrEqual(3);
    expect(mounts).toBe(1);
  });

  it("goes to Login when the daemon says the session is gone", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    status.mockResolvedValueOnce({ onboarded: true, authenticated: true });
    mount();
    expect(await screen.findByText("the application")).toBeTruthy();

    status.mockResolvedValueOnce({ onboarded: true, authenticated: false });
    await act(async () => {
      window.dispatchEvent(new Event(UNAUTHENTICATED_EVENT));
      await vi.advanceTimersByTimeAsync(5_000);
    });

    expect(await screen.findByPlaceholderText("Password")).toBeTruthy();
    expect(screen.queryByText("the application")).toBeNull();
  });

  // Logging out used to navigate to /login while the auth store still said
  // "authenticated", so the router bounced back to / and the home screen's
  // loaders all fired without a credential before Login finally appeared.
  it("shows Login the moment somebody signs out, without a round trip", async () => {
    status.mockResolvedValue({ onboarded: true, authenticated: true });
    mount();
    expect(await screen.findByText("the application")).toBeTruthy();
    status.mockClear();

    act(() => {
      window.dispatchEvent(new Event(SIGNED_OUT_EVENT));
    });

    expect(screen.getByPlaceholderText("Password")).toBeTruthy();
    expect(screen.queryByText("the application")).toBeNull();
    expect(status).not.toHaveBeenCalled();
  });

  // The auth store the router reads was left saying "signed out" after a
  // sign-in through this gate, so the router sent the person to its own
  // login page a second time. The gate says so, and the store listens.
  it("says so when somebody signs in again", async () => {
    status.mockResolvedValue({ onboarded: true, authenticated: true });
    mount();
    expect(await screen.findByText("the application")).toBeTruthy();
    act(() => {
      window.dispatchEvent(new Event(SIGNED_OUT_EVENT));
    });

    const heard = vi.fn();
    window.addEventListener(AUTHENTICATED_EVENT, heard);
    login.mockResolvedValue({ user: { id: "u" }, expiresAt: "" });
    fireEvent.change(screen.getByPlaceholderText("Username or email"), { target: { value: "v@e.test" } });
    fireEvent.change(screen.getByPlaceholderText("Password"), { target: { value: "secret" } });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Sign In" }));
    });

    expect(await screen.findByText("the application")).toBeTruthy();
    expect(heard).toHaveBeenCalledTimes(1);
    window.removeEventListener(AUTHENTICATED_EVENT, heard);
  });
});

describe("AuthGate, before anybody has answered", () => {
  // The first status() used to walk the whole desktop retry budget before
  // failing, and the gate drew nothing at all meanwhile: about five seconds
  // of a blank window before "Starting AOS".
  it("says it is waiting as soon as the daemon is known to be down", async () => {
    status.mockRejectedValue(
      new DomainError({ code: "AOS_DAEMON_UNREACHABLE", message: "the daemon did not answer" }),
    );
    mount();

    expect(await screen.findByText("Starting AOS")).toBeTruthy();
  });

  it("draws the splash, not nothing, while the first answer is on its way", () => {
    status.mockReturnValue(new Promise(() => {}));
    const { container } = mount();

    expect(container.querySelector("[data-auth-gate='checking']")).not.toBeNull();
  });
});
