import { useCallback, useEffect, useRef, useState } from "react";
import type { JSX, ReactNode } from "react";
import { AUTHENTICATED_EVENT, SIGNED_OUT_EVENT, status } from "@/lib/auth";
import { UNAUTHENTICATED_EVENT } from "@/lib/client";
import { t } from "@/lib/i18n";
import { LoginPage } from "./LoginPage";
import { OnboardingForm } from "./OnboardingForm";

type Gate =
  | { checking: true }
  | { checking: false; waiting: true }
  | { checking: false; waiting?: false; onboarded: boolean; authenticated: boolean };

/**
 * How long to wait before asking the daemon again, while it is starting.
 *
 * It grows and it stops growing, the same shape the realtime channel's
 * backoff has: a fixed short interval hammers a daemon that is building its
 * first index, and an unbounded one leaves somebody looking at a splash long
 * after it came up.
 */
const RETRY_MS = [500, 1_000, 2_000, 3_000, 5_000] as const;

/**
 * How long an answer of "still signed in" settles the question.
 *
 * A screen whose calls keep being refused while the daemon says the session is
 * fine raises the suspicion again on every refusal. Each one used to cost a
 * status call and, before the application stayed mounted through a check, a
 * full unmount and remount — which is how a deleted conversation looped
 * forever. Inside this window further suspicions wait for one trailing check
 * instead, so a real revocation among them is still caught.
 */
const SETTLED_MS = 3_000;

function isSignedIn(gate: Gate): boolean {
  return !gate.checking && !gate.waiting && gate.authenticated;
}

/**
 * What renders before the router does: Onboarding for a fresh installation,
 * Login for one with an account this window isn't signed into, or the app
 * itself once a session exists.
 *
 * Ported from the original's route-level guard (OnboardingPage/LoginPage
 * sit ahead of every other route in @/router.tsx) as a gate around the
 * whole router instead of a per-route beforeLoad: every route here needs
 * the same answer to "is anyone signed in", so asking once is the faithful
 * behaviour, not a shortcut.
 */
export function AuthGate({ children }: { children: ReactNode }): JSX.Element {
  const [gate, setGate] = useState<Gate>({ checking: true });
  const current = useRef<Gate>(gate);
  const attempt = useRef(0);
  const retry = useRef<ReturnType<typeof setTimeout> | null>(null);
  const trailing = useRef<ReturnType<typeof setTimeout> | null>(null);
  const inFlight = useRef(false);
  const suspectedMeanwhile = useRef(false);
  const settledUntil = useRef(0);
  // Whether Login or Onboarding has been on screen since the application was
  // last shown — what makes a later "authenticated" a sign-in worth telling
  // the auth store about, rather than the ordinary answer at launch.
  const signedOut = useRef(false);

  const show = useCallback((next: Gate) => {
    current.current = next;
    setGate(next);
  }, []);

  const ask = useCallback(() => {
    inFlight.current = true;
    status()
      .then((s) => {
        inFlight.current = false;
        attempt.current = 0;
        if (s.authenticated) {
          settledUntil.current = Date.now() + SETTLED_MS;
          if (signedOut.current) {
            signedOut.current = false;
            window.dispatchEvent(new Event(AUTHENTICATED_EVENT));
          }
        } else {
          signedOut.current = true;
        }
        show({ checking: false, onboarded: s.onboarded, authenticated: s.authenticated });
        if (suspectedMeanwhile.current && s.authenticated) {
          suspectedMeanwhile.current = false;
          scheduleTrailing();
        }
      })
      .catch(() => {
        inFlight.current = false;
        // A daemon that has not answered *yet* is not an installation with
        // no account. Mapping the two together is what sent a fresh install
        // to a Login page it had nothing to log into — and nothing re-asked,
        // so the only way out was to quit and relaunch.
        //
        // Only a real answer decides between Onboarding and Login. Until one
        // arrives this keeps asking, with a backoff, and says it is waiting.
        // An application already on screen stays there: what it shows was
        // read from a daemon that was answering, and the banner says the rest.
        if (!isSignedIn(current.current)) show({ checking: false, waiting: true });
        const delay = RETRY_MS[Math.min(attempt.current, RETRY_MS.length - 1)];
        attempt.current += 1;
        retry.current = setTimeout(() => void ask(), delay);
      });
    // `scheduleTrailing` is declared below and stable; listing it would need
    // a forward reference.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [show]);

  const scheduleTrailing = useCallback(() => {
    if (trailing.current !== null) return;
    const wait = Math.max(0, settledUntil.current - Date.now());
    trailing.current = setTimeout(() => {
      trailing.current = null;
      if (isSignedIn(current.current) && !inFlight.current) ask();
    }, wait);
  }, [ask]);

  /** Asks from the top — at mount, and after Login or Onboarding succeeds. */
  const recheck = useCallback(() => {
    attempt.current = 0;
    if (retry.current !== null) clearTimeout(retry.current);
    show({ checking: true });
    ask();
  }, [ask, show]);

  useEffect(() => {
    recheck();

    // A call was refused for its credential — expired, or revoked from
    // another window. Asking again is what puts the person on the Login
    // screen instead of leaving them on an application that answers every
    // action with a toast and offers no way back.
    //
    // Asked in the background: the application stays mounted until the
    // answer says otherwise. And only while it is on screen — Login, the
    // wizard and the splash are already the answer, or already asking.
    const onSuspicion = () => {
      if (!isSignedIn(current.current)) return;
      if (inFlight.current) {
        suspectedMeanwhile.current = true;
        return;
      }
      if (Date.now() < settledUntil.current) {
        scheduleTrailing();
        return;
      }
      ask();
    };

    // The page signed out itself: no question to ask.
    const onSignedOut = () => {
      if (trailing.current !== null) clearTimeout(trailing.current);
      trailing.current = null;
      settledUntil.current = 0;
      signedOut.current = true;
      show({ checking: false, onboarded: true, authenticated: false });
    };

    window.addEventListener(UNAUTHENTICATED_EVENT, onSuspicion);
    window.addEventListener(SIGNED_OUT_EVENT, onSignedOut);
    return () => {
      window.removeEventListener(UNAUTHENTICATED_EVENT, onSuspicion);
      window.removeEventListener(SIGNED_OUT_EVENT, onSignedOut);
      if (retry.current !== null) clearTimeout(retry.current);
      if (trailing.current !== null) clearTimeout(trailing.current);
    };
  }, [recheck, ask, scheduleTrailing, show]);

  if (gate.checking) return <Splash />;
  if (gate.waiting) return <DaemonStarting />;
  if (!gate.onboarded) return <OnboardingForm onDone={recheck} />;
  if (!gate.authenticated) return <LoginPage onSignedIn={recheck} />;
  return <>{children}</>;
}

/**
 * What a person sees while the first answer is on its way.
 *
 * Deliberately empty of words: it is up for a fraction of a second when the
 * daemon answers, and "Starting AOS" there would be a claim about a state
 * nobody is in. It paints the application's background rather than leaving
 * the window blank.
 */
function Splash(): JSX.Element {
  return <div data-auth-gate="checking" className="h-screen w-screen bg-background" />;
}

/**
 * What a person sees while the daemon is coming up.
 *
 * Deliberately not a spinner over the Login page: the whole point is that
 * this state is *not* an answer about who is signed in, and drawing one of
 * the two answers under it is how the wrong one got shown for good.
 */
function DaemonStarting(): JSX.Element {
  return (
    <div className="flex h-screen w-screen flex-col items-center justify-center gap-2 bg-background">
      <p className="text-sm font-medium text-foreground">{t("Starting AOS")}</p>
      {/* text-foreground at reduced opacity rather than text-muted-foreground:
          this renders before the theme's muted tokens are resolved against a
          surface, and the subtitle came out nearly invisible. */}
      <p className="text-sm text-foreground/70">
        {t("Waiting for the daemon that holds your workspace.")}
      </p>
    </div>
  );
}
