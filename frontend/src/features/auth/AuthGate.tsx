import { useEffect, useState } from "react";
import type { JSX, ReactNode } from "react";
import { status } from "@/lib/auth";
import { LoginPage } from "./LoginPage";
import { OnboardingForm } from "./OnboardingForm";

type Gate = { checking: true } | { checking: false; onboarded: boolean; authenticated: boolean };

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

  const recheck = () => {
    setGate({ checking: true });
    status()
      .then((s) => setGate({ checking: false, onboarded: s.onboarded, authenticated: s.authenticated }))
      // A daemon that hasn't answered yet reads the same as "not signed in
      // yet" — the desktop cold-start window resolves this on its own once
      // the bridge warms up and the person interacts with the page again.
      .catch(() => setGate({ checking: false, onboarded: true, authenticated: false }));
  };

  useEffect(recheck, []);

  if (gate.checking) return <></>;
  if (!gate.onboarded) return <OnboardingForm onDone={recheck} />;
  if (!gate.authenticated) return <LoginPage onSignedIn={recheck} />;
  return <>{children}</>;
}
