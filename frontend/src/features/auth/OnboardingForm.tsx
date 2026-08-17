import { useState } from "react";
import type { JSX, ReactNode } from "react";
import { onboarding } from "@/lib/auth";
import { client } from "@/lib/client";
import { DomainError } from "@/lib/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

interface OnboardingFormProps {
  /** Called once the account (and, best-effort, the workspace region) exist. */
  onDone: () => void;
}

type Tone = "friendly" | "efficient" | "professional" | "candid";
type Style = "concise" | "balanced" | "detailed";

interface WizardData {
  name: string;
  email: string;
  password: string;
  confirmPassword: string;
  language: string;
  timezone: string;
  city: string;
  country: string;
  orchestratorName: string;
  tone: Tone;
  style: Style;
  autonomy: number;
}

const STEPS = ["welcome", "user", "security", "region", "orchestrator", "init"] as const;
type Step = (typeof STEPS)[number];

// Only "welcome" through "orchestrator" count toward "n of 5" — init is the
// submission itself, not a fifth thing to fill in.
const FORM_STEPS: Step[] = ["welcome", "user", "security", "region", "orchestrator"];

function defaultData(): WizardData {
  let timezone = "UTC";
  try {
    timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  } catch {
    // Some environments (older webviews) don't resolve a timezone; UTC is
    // a safe, honest default rather than a guess.
  }
  return {
    name: "",
    email: "",
    password: "",
    confirmPassword: "",
    language: navigator.language || "en-US",
    timezone,
    city: "",
    country: "",
    orchestratorName: "Atlas",
    tone: "friendly",
    style: "balanced",
    autonomy: 0.5,
  };
}

/**
 * The first-run wizard, ported from the original's OnboardingForm: the same
 * five steps plus a submission step, welcome through orchestrator.
 *
 * Two steps the original has are not here. The waitlist gate is pure
 * commercial SaaS gating with no AOS backend behind it — there is nothing to
 * connect it to, and the app is not multi-tenant. The animated screenshot
 * carousel on the welcome step is decorative and was not ported for time;
 * the step itself, and everything that actually creates the account, was.
 */
export function OnboardingForm({ onDone }: OnboardingFormProps): JSX.Element {
  const [stepIndex, setStepIndex] = useState(0);
  const [data, setData] = useState<WizardData>(defaultData);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [phase, setPhase] = useState<string | null>(null);

  const step = FORM_STEPS[stepIndex] ?? "welcome";
  const isLastFormStep = stepIndex === FORM_STEPS.length - 1;

  function update<K extends keyof WizardData>(key: K, value: WizardData[K]) {
    setData((d) => ({ ...d, [key]: value }));
  }

  function validateStep(): string | null {
    if (step === "user") {
      if (!data.name.trim()) return "Enter your name.";
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(data.email)) return "Enter a valid email.";
    }
    if (step === "security") {
      if (data.password.length < 12) return "Use at least 12 characters — this account can run shell commands on this machine.";
      if (data.password !== data.confirmPassword) return "Passwords don't match.";
    }
    return null;
  }

  function next() {
    const problem = validateStep();
    if (problem) {
      setError(problem);
      return;
    }
    setError(null);
    if (isLastFormStep) {
      setStepIndex(FORM_STEPS.length); // move to "init"
      void submit();
      return;
    }
    setStepIndex((i) => i + 1);
  }

  function back() {
    setError(null);
    setStepIndex((i) => Math.max(0, i - 1));
  }

  async function submit() {
    setSubmitting(true);
    try {
      setPhase("Creating your account…");
      await onboarding(data.name.trim(), data.email.trim(), data.password);

      setPhase("Saving your preferences…");
      try {
        await client.invoke("config_update", {
          set: {
            "region.language": data.language,
            "region.timezone": data.timezone,
            "region.city": data.city,
            "region.country": data.country,
          },
          _reasoning: "the person just finished onboarding and chose these region settings",
        });
      } catch {
        // The account exists and the person is signed in either way; a
        // region preference that didn't save is not worth failing over.
      }

      setPhase("Done.");
      onDone();
    } catch (err) {
      setError(err instanceof DomainError ? err.message : "Something went wrong.");
      setStepIndex(FORM_STEPS.length - 1);
    } finally {
      setSubmitting(false);
    }
  }

  if (step === "init" || stepIndex >= FORM_STEPS.length) {
    return <InitStep phase={phase} error={error} onRetry={() => void submit()} submitting={submitting} />;
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle className="text-xl">Welcome to AOS</CardTitle>
          <CardDescription>
            Step {stepIndex + 1} of {FORM_STEPS.length} · {stepTitle(step)}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            className="flex flex-col gap-4"
            onSubmit={(e) => {
              e.preventDefault();
              next();
            }}
          >
            {step === "welcome" && <WelcomeStep />}
            {step === "user" && <UserStep data={data} update={update} />}
            {step === "security" && <SecurityStep data={data} update={update} />}
            {step === "region" && <RegionStep data={data} update={update} />}
            {step === "orchestrator" && <OrchestratorStep data={data} update={update} />}

            {error && (
              <p role="alert" className="text-sm text-destructive">
                {error}
              </p>
            )}

            <StepDots total={FORM_STEPS.length} current={stepIndex} />

            <div className="flex items-center justify-between gap-2">
              <Button type="button" variant="ghost" onClick={back} disabled={stepIndex === 0}>
                Previous
              </Button>
              <Button type="submit">{isLastFormStep ? "Create Workspace" : "Continue"}</Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}

function stepTitle(step: Step): string {
  switch (step) {
    case "welcome":
      return "Welcome";
    case "user":
      return "About you";
    case "security":
      return "Security";
    case "region":
      return "Region";
    case "orchestrator":
      return "Your orchestrator";
    default:
      return "";
  }
}

function StepDots({ total, current }: { total: number; current: number }): JSX.Element {
  return (
    <div className="flex justify-center gap-1.5 py-1" aria-hidden="true">
      {Array.from({ length: total }, (_, i) => (
        <span
          key={i}
          className={`size-1.5 rounded-full ${i === current ? "bg-primary" : "bg-muted"}`}
        />
      ))}
    </div>
  );
}

function Field({ label, htmlFor, children }: { label: string; htmlFor: string; children: ReactNode }): JSX.Element {
  return (
    <div className="flex flex-col gap-2">
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
    </div>
  );
}

function WelcomeStep(): JSX.Element {
  return (
    <p className="text-sm text-muted-foreground">
      AOS runs agents against your own workspace, with your own credentials, on your own machine.
      Let's set up your account.
    </p>
  );
}

function UserStep({ data, update }: { data: WizardData; update: <K extends keyof WizardData>(k: K, v: WizardData[K]) => void }): JSX.Element {
  return (
    <>
      <Field label="Full name" htmlFor="ob-name">
        <Input id="ob-name" autoFocus value={data.name} onChange={(e) => update("name", e.target.value)} required />
      </Field>
      <Field label="Email" htmlFor="ob-email">
        <Input
          id="ob-email"
          type="email"
          autoComplete="email"
          value={data.email}
          onChange={(e) => update("email", e.target.value)}
          required
        />
      </Field>
    </>
  );
}

function passwordStrength(password: string): { label: string; ratio: number } {
  let score = 0;
  if (password.length >= 12) score++;
  if (password.length >= 20) score++;
  if (/[a-z]/.test(password) && /[A-Z]/.test(password)) score++;
  if (/\d/.test(password)) score++;
  if (/[^a-zA-Z0-9]/.test(password) || password.includes(" ")) score++;
  const labels = ["Too short", "Weak", "Fair", "Good", "Strong", "Excellent"];
  return { label: labels[score] ?? "Weak", ratio: score / (labels.length - 1) };
}

function SecurityStep({ data, update }: { data: WizardData; update: <K extends keyof WizardData>(k: K, v: WizardData[K]) => void }): JSX.Element {
  const strength = passwordStrength(data.password);
  return (
    <>
      <Field label="Password" htmlFor="ob-password">
        <Input
          id="ob-password"
          type="password"
          autoComplete="new-password"
          autoFocus
          value={data.password}
          onChange={(e) => update("password", e.target.value)}
          required
        />
        <div className="h-1 w-full overflow-hidden rounded-full bg-muted">
          <div
            className="h-full bg-primary transition-all"
            style={{ width: `${Math.max(4, strength.ratio * 100)}%` }}
          />
        </div>
        <p className="text-xs text-muted-foreground">
          {strength.label} · at least 12 characters — this account can run shell commands on this machine.
        </p>
      </Field>
      <Field label="Confirm password" htmlFor="ob-confirm">
        <Input
          id="ob-confirm"
          type="password"
          autoComplete="new-password"
          value={data.confirmPassword}
          onChange={(e) => update("confirmPassword", e.target.value)}
          required
        />
      </Field>
      <p className="text-xs text-muted-foreground">
        Zero-knowledge: this password is hashed on this machine and never leaves it.
      </p>
    </>
  );
}

function RegionStep({ data, update }: { data: WizardData; update: <K extends keyof WizardData>(k: K, v: WizardData[K]) => void }): JSX.Element {
  return (
    <>
      <Field label="Language" htmlFor="ob-language">
        <Select value={data.language} onValueChange={(v) => update("language", v)}>
          <SelectTrigger id="ob-language" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="en-US">English (US)</SelectItem>
            <SelectItem value="pt-BR">Português (Brasil)</SelectItem>
            <SelectItem value="es-ES">Español</SelectItem>
            <SelectItem value="fr-FR">Français</SelectItem>
            <SelectItem value="de-DE">Deutsch</SelectItem>
            <SelectItem value="ja-JP">日本語</SelectItem>
          </SelectContent>
        </Select>
      </Field>
      <div className="grid grid-cols-2 gap-4">
        <Field label="City" htmlFor="ob-city">
          <Input id="ob-city" value={data.city} onChange={(e) => update("city", e.target.value)} />
        </Field>
        <Field label="Country" htmlFor="ob-country">
          <Input id="ob-country" value={data.country} onChange={(e) => update("country", e.target.value)} />
        </Field>
      </div>
      <Field label="Timezone" htmlFor="ob-timezone">
        <Input id="ob-timezone" value={data.timezone} onChange={(e) => update("timezone", e.target.value)} />
      </Field>
    </>
  );
}

function OrchestratorStep({ data, update }: { data: WizardData; update: <K extends keyof WizardData>(k: K, v: WizardData[K]) => void }): JSX.Element {
  return (
    <>
      <Field label="Name your orchestrator" htmlFor="ob-orch-name">
        <Input id="ob-orch-name" value={data.orchestratorName} onChange={(e) => update("orchestratorName", e.target.value)} />
      </Field>
      <Field label="Tone" htmlFor="ob-tone">
        <Select value={data.tone} onValueChange={(v) => update("tone", v as Tone)}>
          <SelectTrigger id="ob-tone" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="friendly">Friendly</SelectItem>
            <SelectItem value="efficient">Efficient</SelectItem>
            <SelectItem value="professional">Professional</SelectItem>
            <SelectItem value="candid">Candid</SelectItem>
          </SelectContent>
        </Select>
      </Field>
      <Field label="Style" htmlFor="ob-style">
        <Select value={data.style} onValueChange={(v) => update("style", v as Style)}>
          <SelectTrigger id="ob-style" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="concise">Concise</SelectItem>
            <SelectItem value="balanced">Balanced</SelectItem>
            <SelectItem value="detailed">Detailed</SelectItem>
          </SelectContent>
        </Select>
      </Field>
      <Field label={`Autonomy — ${Math.round(data.autonomy * 100)}%`} htmlFor="ob-autonomy">
        <input
          id="ob-autonomy"
          type="range"
          min={0}
          max={1}
          step={0.05}
          value={data.autonomy}
          onChange={(e) => update("autonomy", Number(e.target.value))}
          className="accent-primary"
        />
      </Field>
      <p className="text-xs text-muted-foreground">
        These preferences aren't wired to a running orchestrator yet — the one this workspace gets
        is seeded automatically when it's created.
      </p>
    </>
  );
}

function InitStep({
  phase,
  error,
  submitting,
  onRetry,
}: {
  phase: string | null;
  error: string | null;
  submitting: boolean;
  onRetry: () => void;
}): JSX.Element {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-sm">
        <CardContent className="flex flex-col items-center gap-4 py-8 text-center">
          {error ? (
            <>
              <p role="alert" className="text-sm text-destructive">
                {error}
              </p>
              <Button onClick={onRetry} disabled={submitting}>
                {submitting ? "Retrying…" : "Try again"}
              </Button>
            </>
          ) : (
            <>
              <div className="size-8 animate-spin rounded-full border-2 border-muted border-t-primary" />
              <p className="text-sm text-muted-foreground">{phase ?? "Setting things up…"}</p>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
