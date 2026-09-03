import { useCallback, useEffect, useRef, useState } from "react";
import type { JSX, ReactNode } from "react";
import { status } from "@/lib/auth";
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
  const attempt = useRef(0);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const recheck = useCallback(() => {
    attempt.current = 0;
    setGate({ checking: true });
    void ask();
    // `ask` is stable for the life of the component; listing it would need a
    // forward reference it cannot have.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const ask = useCallback(() => {
    status()
      .then((s) => {
        attempt.current = 0;
        setGate({ checking: false, onboarded: s.onboarded, authenticated: s.authenticated });
      })
      .catch(() => {
        // A daemon that has not answered *yet* is not an installation with
        // no account. Mapping the two together is what sent a fresh install
        // to a Login page it had nothing to log into — and nothing re-asked,
        // so the only way out was to quit and relaunch.
        //
        // Only a real answer decides between Onboarding and Login. Until one
        // arrives this keeps asking, with a backoff, and says it is waiting.
        setGate({ checking: false, waiting: true });
        const delay = RETRY_MS[Math.min(attempt.current, RETRY_MS.length - 1)];
        attempt.current += 1;
        timer.current = setTimeout(() => void ask(), delay);
      });
  }, []);

  useEffect(() => {
    recheck();
    // The daemon says the credential is no good — expired, or revoked from
    // another window. Asking again is what puts the person on the Login
    // screen instead of leaving them on an application that answers every
    // action with a toast and offers no way back.
    window.addEventListener(UNAUTHENTICATED_EVENT, recheck);
    return () => {
      window.removeEventListener(UNAUTHENTICATED_EVENT, recheck);
      if (timer.current !== null) clearTimeout(timer.current);
    };
  }, [recheck]);

  if (gate.checking) return <></>;
  if (gate.waiting) return <DaemonStarting />;
  if (!gate.onboarded) return <OnboardingForm onDone={recheck} />;
  if (!gate.authenticated) return <LoginPage onSignedIn={recheck} />;
  return <>{children}</>;
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
    <div className="flex h-screen w-screen flex-col items-center justify-center gap-2">
      <p className="text-sm font-medium text-foreground">{t("Starting AOS")}</p>
      <p className="text-sm text-muted-foreground">
        {t("Waiting for the daemon that holds your workspace.")}
      </p>
    </div>
  );
}
