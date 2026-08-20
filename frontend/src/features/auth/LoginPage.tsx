import { useState } from "react";
import type { FormEvent, JSX } from "react";
import { login } from "@/lib/auth";
import { DomainError } from "@/lib/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Logo } from "@/components/ui/logo";

interface LoginPageProps {
  /** Called once a session exists — the caller re-checks status and moves on. */
  onSignedIn: () => void;
}

/**
 * The single-panel sign-in form, ported from the original's LoginPage —
 * visually, at least. AuthGate renders this before the router (and its own,
 * pixel-identical but router-bound copy at `features/auth/presentation/
 * pages/login`) ever mounts, so the two can't share one component; this is
 * the styling brought over by hand, kept on the working `login()` transport
 * (`lib/auth.ts`) rather than switching to the router copy's `aos.client`
 * path.
 */
export function LoginPage({ onSignedIn }: LoginPageProps): JSX.Element {
  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setPending(true);
    setError(null);
    try {
      await login(identifier, password);
      onSignedIn();
    } catch (err) {
      setError(err instanceof DomainError ? err.message : "Something went wrong.");
    } finally {
      setPending(false);
    }
  };

  return (
    <div className="flex h-screen w-full flex-col items-center justify-center bg-background p-8">
      <div className="flex w-full max-w-[400px] flex-col items-center gap-8 text-center">
        <div className="flex h-16 w-16 items-center justify-center rounded-full text-primary-foreground shadow-sm">
          <Logo className="h-10 w-10" />
        </div>

        <h1 className="text-3xl font-bold tracking-tight">Log in to AOS</h1>

        <form onSubmit={submit} className="flex w-full flex-col gap-4">
          <Input
            placeholder="Username or email"
            value={identifier}
            onChange={(e) => setIdentifier(e.target.value)}
            disabled={pending}
            className="h-11 rounded-full text-center text-base"
            autoFocus
            autoComplete="username"
            required
          />
          <Input
            type="password"
            placeholder="Password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            disabled={pending}
            className="h-11 rounded-full text-center text-base"
            autoComplete="current-password"
            required
          />
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
          <Button
            type="submit"
            disabled={pending || !identifier.trim() || !password.trim()}
            className="h-11 rounded-full"
          >
            {pending ? "Signing in…" : "Sign In"}
          </Button>
        </form>
      </div>
    </div>
  );
}
