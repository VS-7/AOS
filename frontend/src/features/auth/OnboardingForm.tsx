import { useCallback, useEffect, useReducer, useRef, useState } from "react";
import type { JSX } from "react";
import { AnimatePresence, motion } from "framer-motion";
import {
  ArrowLeft,
  Briefcase,
  CheckCircle2,
  Clock,
  Eye,
  EyeOff,
  FileText,
  Loader2,
  Lock,
  Mail,
  MapPin,
  MessageSquare,
  Scale,
  Scissors,
  Smile,
  User,
  Zap,
} from "lucide-react";
import { onboarding } from "@/lib/auth";
import { client } from "@/lib/client";
import { DomainError } from "@/lib/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Logo } from "@/components/ui/logo";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Slider } from "@/components/ui/slider";
import { FolderInput } from "@/components/ui/folder-input";
import { cn } from "@/lib/utils";
import { PLACEHOLDER_IMAGE } from "@/assets/placeholder";
import { t } from "@/lib/i18n";

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
  workspaceName: string;
  workspacePath: string;
  orchestratorName: string;
  tone: Tone;
  style: Style;
  autonomy: number;
}

const STEPS = ["welcome", "user", "security", "region", "workspace", "orchestrator", "init"] as const;
type Step = (typeof STEPS)[number];

// Only "welcome" through "orchestrator" count toward the progress bar — init
// is the submission itself, not another thing to fill in.
const FORM_STEPS: Step[] = ["welcome", "user", "security", "region", "workspace", "orchestrator"];

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
    workspaceName: "",
    workspacePath: "",
    orchestratorName: "",
    tone: "friendly",
    style: "balanced",
    autonomy: 0.5,
  };
}

// ---------------------------------------------------------------------------
// Slide transition, ported from the router-bound copy at features/workspace/
// presentation/pages/onboarding/components/onboarding-form/index.tsx
// ---------------------------------------------------------------------------

const slideVariants = {
  enter: (direction: number) => ({
    x: direction > 0 ? 40 : -40,
    opacity: 0,
    scale: 0.98,
  }),
  center: {
    x: 0,
    opacity: 1,
    scale: 1,
  },
  exit: (direction: number) => ({
    x: direction > 0 ? -40 : 40,
    opacity: 0,
    scale: 0.98,
  }),
};

const slideTransition = {
  x: { type: "spring" as const, stiffness: 450, damping: 35 },
  opacity: { duration: 0.2 },
  scale: { duration: 0.2 },
};

/**
 * The first-run wizard. Logic is unchanged from before this pass — same
 * `WizardData`, same `lib/auth.ts` `onboarding()` call, same best-effort
 * `config_update` for region, same validation — only the rendering changed,
 * brought over from the pixel-identical but router-bound copy at
 * `features/workspace/presentation/pages/onboarding/components/
 * onboarding-form/` (reachable only through the router, which AuthGate
 * never mounts before a session exists, so it was dead weight sitting next
 * to this file's own plain `Card` version).
 *
 * Two steps the original has are still not here, for the same reasons as
 * before: the waitlist gate is pure commercial SaaS gating with no AOS
 * backend behind it, and the welcome step's screenshot carousel has no real
 * screenshots to show yet (it renders the same placeholder image the router
 * copy's own carousel does).
 */
export function OnboardingForm({ onDone }: OnboardingFormProps): JSX.Element {
  const [stepIndex, setStepIndex] = useState(0);
  const [direction, setDirection] = useState(1);
  const [data, setData] = useState<WizardData>(defaultData);
  const [error, setError] = useState<string | null>(null);

  const step = FORM_STEPS[stepIndex] ?? "welcome";
  const isFirstStep = stepIndex === 0;
  const isLastFormStep = stepIndex === FORM_STEPS.length - 1;
  const isInitStep = stepIndex >= FORM_STEPS.length;

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
    if (step === "workspace") {
      if (!data.workspaceName.trim()) return "Name your workspace.";
    }
    return null;
  }

  const goNext = useCallback(() => {
    if (isInitStep) return;
    if (step === "welcome") {
      setError(null);
      setDirection(1);
      setStepIndex(1);
      return;
    }
    const problem = validateStep();
    if (problem) {
      setError(problem);
      return;
    }
    setError(null);
    if (isLastFormStep) {
      setDirection(1);
      setStepIndex(FORM_STEPS.length); // move to "init"
      return;
    }
    setDirection(1);
    setStepIndex((i) => i + 1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [step, isLastFormStep, isInitStep, data]);

  const goBack = useCallback(() => {
    if (isFirstStep || isInitStep) return;
    setError(null);
    setDirection(-1);
    setStepIndex((i) => Math.max(0, i - 1));
  }, [isFirstStep, isInitStep]);

  // Global keyboard navigation, same as the router-bound copy.
  useEffect(() => {
    if (isInitStep) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      const isInput = target?.tagName === "INPUT" || target?.tagName === "TEXTAREA";
      if (e.key === "Enter" && isInput) {
        e.preventDefault();
        goNext();
      } else if (e.key === "Escape" && !isFirstStep) {
        e.preventDefault();
        goBack();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [goNext, goBack, isFirstStep, isInitStep]);

  const handleInitError = useCallback((message: string) => {
    setError(message);
    setDirection(-1);
    setStepIndex(FORM_STEPS.length - 1);
  }, []);

  const buttonLabel = isFirstStep ? "Get Started" : isLastFormStep ? "Create Workspace" : "Continue";
  const progressPercentage = ((stepIndex + 1) / FORM_STEPS.length) * 100;

  return (
    <div className="grid relative overflow-hidden h-screen grid-rows-[auto_1fr] gap-2 p-4 sm:gap-4 sm:p-6">
      {/* Feature background */}
      <div className="absolute inset-0 overflow-hidden pointer-events-none">
        <img src={PLACEHOLDER_IMAGE} alt="" aria-hidden className="size-full object-cover" />
        <div className="absolute inset-0 bg-background/20 backdrop-blur-xs" />
      </div>

      {/* Top bar */}
      <div className="grid grid-cols-[1fr_auto] items-center">
        <div className="flex items-center gap-2">
          <Logo className="h-5" />
        </div>
      </div>

      {/* Main card */}
      <div className="h-full min-h-0 flex items-center justify-center max-w-5xl mx-auto container">
        <div className="bg-card/95 relative flex flex-col rounded-2xl border border-border/60 shadow-2xl shadow-black/10 dark:shadow-black/50 w-full h-full max-h-[720px] z-10 backdrop-blur-xl overflow-hidden">
          {/* Top progress line */}
          {!isInitStep && (
            <div className="absolute top-0 inset-x-0 h-0.5 bg-muted/40 z-20 overflow-hidden">
              <motion.div
                className="h-full bg-primary"
                initial={false}
                animate={{ width: `${progressPercentage}%` }}
                transition={{ duration: 0.4, ease: "easeInOut" }}
              />
            </div>
          )}

          {/* Back button — hidden on welcome and init step */}
          <AnimatePresence>
            {!isFirstStep && !isInitStep && (
              <motion.header
                className="p-4 absolute left-0 top-0 z-10"
                initial={{ opacity: 0, x: -8 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -8 }}
                transition={{ duration: 0.2 }}
              >
                <Button
                  variant="secondary"
                  size="icon-lg"
                  className="rounded-full bg-secondary/60 backdrop-blur-lg border border-border"
                  onClick={goBack}
                >
                  <ArrowLeft />
                </Button>
              </motion.header>
            )}
          </AnimatePresence>

          {/* Step content */}
          <main className="overflow-hidden flex-1 min-h-0">
            <AnimatePresence mode="wait" custom={direction}>
              <motion.div
                key={stepIndex}
                custom={direction}
                variants={slideVariants}
                initial="enter"
                animate="center"
                exit="exit"
                transition={slideTransition}
                className="h-full"
              >
                {step === "welcome" && <WelcomeStep />}
                {step === "user" && <UserStep data={data} update={update} />}
                {step === "security" && <SecurityStep data={data} update={update} />}
                {step === "region" && <RegionStep data={data} update={update} />}
                {step === "workspace" && <WorkspaceStep data={data} update={update} />}
                {step === "orchestrator" && <OrchestratorStep data={data} update={update} />}
                {isInitStep && <InitStep data={data} onError={handleInitError} onDone={onDone} />}
              </motion.div>
            </AnimatePresence>
          </main>

          {/* Footer — hidden on init step */}
          {!isInitStep && (
            <footer className="flex items-center justify-between px-6 py-4 border-t border-border/40 bg-muted/10">
              <div className="flex items-center gap-3">
                <div className="flex items-center gap-1.5">
                  {FORM_STEPS.map((_, i) => (
                    <button
                      key={i}
                      type="button"
                      onClick={() => {
                        if (i < stepIndex) {
                          setDirection(-1);
                          setStepIndex(i);
                        }
                      }}
                      disabled={i > stepIndex}
                      className={`h-1.5 rounded-full transition-all duration-300 ${
                        i === stepIndex
                          ? "w-6 bg-primary"
                          : i < stepIndex
                            ? "w-2 bg-primary/40 hover:bg-primary/60 cursor-pointer"
                            : "w-1.5 bg-foreground/15 cursor-not-allowed"
                      }`}
                      aria-label={`Step ${i + 1}`}
                    />
                  ))}
                </div>
                <span className="text-xs text-muted-foreground font-medium">
                  {stepIndex + 1} of {FORM_STEPS.length} · {stepTitle(step)}
                </span>
              </div>

              <div className="flex items-center gap-2">
                {!isFirstStep && (
                  <Button type="button" variant="secondary" className="rounded-full" onClick={goBack}>
                    {t("Previous")}
                  </Button>
                )}
                <Button type="button" className="rounded-full" onClick={goNext}>
                  {buttonLabel}
                </Button>
              </div>
            </footer>
          )}

          {error && !isInitStep && (
            <p role="alert" className="absolute bottom-20 left-0 right-0 px-6 text-center text-sm text-destructive">
              {error}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}

function stepTitle(step: Step): string {
  switch (step) {
    case "welcome":
      return "Welcome";
    case "user":
      return "About You";
    case "security":
      return "Security";
    case "region":
      return "Preferences";
    case "orchestrator":
      return "Copilot";
    default:
      return "";
  }
}

// ---------------------------------------------------------------------------
// Welcome
// ---------------------------------------------------------------------------

function WelcomeStep(): JSX.Element {
  return (
    <div className="h-full flex flex-col justify-between overflow-hidden">
      <main className="relative flex-1 w-full overflow-hidden min-h-0 bg-background">
        <img
          src={PLACEHOLDER_IMAGE}
          alt=""
          aria-hidden
          className="absolute inset-0 h-full w-full object-cover object-top"
        />
      </main>
      <footer className="p-4">
        <div className="flex max-w-[36rem] mx-auto flex-col items-center gap-2 text-center">
          <h2 className="text-base font-semibold tracking-tight">{t("Welcome to AOS")}</h2>
          <p className="text-base text-muted-foreground">
            {t("AOS runs agents against your own workspace, with your own credentials, on your own machine. Let's set up your account.")}
          </p>
        </div>
      </footer>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Shared field helpers
// ---------------------------------------------------------------------------

type Update = <K extends keyof WizardData>(key: K, value: WizardData[K]) => void;

// ---------------------------------------------------------------------------
// User
// ---------------------------------------------------------------------------

function UserStep({ data, update }: { data: WizardData; update: Update }): JSX.Element {
  return (
    <div className="flex flex-col items-center justify-center h-full px-8 py-8">
      <div className="w-full max-w-md space-y-6">
        <div>
          <h2 className="text-xl font-semibold tracking-tight text-foreground">{t("A bit about you")}</h2>
          <p className="text-sm text-muted-foreground leading-relaxed mt-1">
            {t("This helps your copilot identify you and personalize your local workspace.")}
          </p>
        </div>

        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="ob-name" className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
              {t("Full Name")}
            </Label>
            <div className="relative">
              <User className="w-4 h-4 text-muted-foreground/60 absolute left-3.5 top-1/2 -translate-y-1/2" />
              <Input
                id="ob-name"
                placeholder={t("e.g. Ada Lovelace")}
                className="pl-10 h-11 bg-muted/20 border-border/60 rounded-xl focus-visible:ring-primary/20 transition-all"
                autoFocus
                value={data.name}
                onChange={(e) => update("name", e.target.value)}
                required
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="ob-email" className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
              {t("Email Address")}
            </Label>
            <div className="relative">
              <Mail className="w-4 h-4 text-muted-foreground/60 absolute left-3.5 top-1/2 -translate-y-1/2" />
              <Input
                id="ob-email"
                type="email"
                autoComplete="email"
                placeholder={t("you@example.com")}
                className="pl-10 h-11 bg-muted/20 border-border/60 rounded-xl focus-visible:ring-primary/20 transition-all"
                value={data.email}
                onChange={(e) => update("email", e.target.value)}
                required
              />
            </div>
            <p className="text-[11px] text-muted-foreground/80 leading-normal pl-1">
              {t("Stored locally for workspace session identity and local commits.")}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Security
// ---------------------------------------------------------------------------

// The one real requirement (validateStep(), above) is 12+ characters — the
// chips below describe that requirement plus non-blocking strength signals,
// they don't gate advancing on their own.
const REQUIREMENTS: { id: string; label: string; met: (p: string) => boolean }[] = [
  { id: "minLen", label: "12+ chars", met: (p) => p.length >= 12 },
  { id: "upper", label: "Uppercase", met: (p) => /[A-Z]/.test(p) },
  { id: "number", label: "Number", met: (p) => /[0-9]/.test(p) },
  { id: "symbol", label: "Symbol", met: (p) => /[^A-Za-z0-9]/.test(p) },
];

function passwordStrength(password: string): { label: string; score: number; color: string } {
  if (!password) return { label: "Enter a password", score: 0, color: "bg-muted" };
  let score = 0;
  if (password.length >= 12) score++;
  if (password.length >= 20) score++;
  if (/[A-Z]/.test(password) && /[a-z]/.test(password)) score++;
  if (/[0-9]/.test(password)) score++;
  if (/[^A-Za-z0-9]/.test(password)) score++;
  const labels = ["Too short", "Weak", "Fair", "Good", "Strong", "Excellent"];
  const colors = ["bg-muted", "bg-red-500", "bg-amber-500", "bg-yellow-500", "bg-green-500", "bg-emerald-400"];
  return { label: labels[score] ?? "Weak", score, color: colors[score] ?? "bg-muted" };
}

function SecurityStep({ data, update }: { data: WizardData; update: Update }): JSX.Element {
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const strength = passwordStrength(data.password);

  return (
    <div className="flex flex-col items-center justify-center h-full px-8 py-8">
      <div className="w-full max-w-md space-y-6">
        <div>
          <h2 className="text-xl font-semibold tracking-tight text-foreground">{t("Your space, your security")}</h2>
          <p className="text-sm text-muted-foreground leading-relaxed mt-1">
            {t("Set a password for this account — it can run shell commands on this machine, so make it a strong one.")}
          </p>
        </div>

        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="ob-password" className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
              {t("Password")}
            </Label>
            <div className="relative">
              <Lock className="w-4 h-4 text-muted-foreground/60 absolute left-3.5 top-1/2 -translate-y-1/2" />
              <Input
                id="ob-password"
                type={showPassword ? "text" : "password"}
                autoComplete="new-password"
                placeholder={t("Enter a strong password")}
                className="pl-10 pr-10 h-11 bg-muted/20 border-border/60 rounded-xl focus-visible:ring-primary/20 transition-all"
                autoFocus
                value={data.password}
                onChange={(e) => update("password", e.target.value)}
                required
              />
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="absolute right-1 top-1/2 -translate-y-1/2 h-8 w-8 text-muted-foreground hover:text-foreground hover:bg-transparent"
                onClick={() => setShowPassword((s) => !s)}
                tabIndex={-1}
              >
                {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </Button>
            </div>
          </div>

          <div className="space-y-2.5 pt-1">
            <div className="flex items-center justify-between text-xs">
              <span className="text-muted-foreground font-medium">{t("Password Strength")}</span>
              <span className={cn("font-medium transition-colors", data.password ? "text-foreground" : "text-muted-foreground")}>
                {strength.label}
              </span>
            </div>
            <div className="flex gap-1.5">
              {Array.from({ length: 5 }).map((_, i) => (
                <div
                  key={i}
                  className={cn(
                    "h-1.5 flex-1 rounded-full transition-all duration-300",
                    i < strength.score ? strength.color : "bg-muted/50",
                  )}
                />
              ))}
            </div>
          </div>

          <div className="flex flex-wrap gap-2 pt-1">
            {REQUIREMENTS.map((req) => {
              const isMet = req.met(data.password);
              return (
                <span
                  key={req.id}
                  className={cn(
                    "inline-flex items-center gap-1.5 text-xs px-2.5 py-1 rounded-lg border transition-all duration-200",
                    isMet
                      ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20 font-medium"
                      : "bg-background/40 text-muted-foreground/70 border-border/40",
                  )}
                >
                  <CheckCircle2 className={cn("w-3.5 h-3.5 transition-colors", isMet ? "text-emerald-500" : "text-muted-foreground/30")} />
                  {req.label}
                </span>
              );
            })}
          </div>

          <div className="space-y-2">
            <Label htmlFor="ob-confirm" className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
              {t("Confirm Password")}
            </Label>
            <div className="relative">
              <Lock className="w-4 h-4 text-muted-foreground/60 absolute left-3.5 top-1/2 -translate-y-1/2" />
              <Input
                id="ob-confirm"
                type={showConfirm ? "text" : "password"}
                autoComplete="new-password"
                placeholder={t("Re-enter your password")}
                className="pl-10 pr-10 h-11 bg-muted/20 border-border/60 rounded-xl focus-visible:ring-primary/20 transition-all"
                value={data.confirmPassword}
                onChange={(e) => update("confirmPassword", e.target.value)}
                required
              />
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="absolute right-1 top-1/2 -translate-y-1/2 h-8 w-8 text-muted-foreground hover:text-foreground hover:bg-transparent"
                onClick={() => setShowConfirm((s) => !s)}
                tabIndex={-1}
              >
                {showConfirm ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </Button>
            </div>
          </div>

          <div className="pt-2 border-t border-border/30 flex items-center gap-2 text-[11px] text-muted-foreground/80">
            <span className="shrink-0">🔒</span>
            <span>{t("Zero-knowledge — this password is hashed on this machine and never leaves it.")}</span>
          </div>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Region
// ---------------------------------------------------------------------------

const LANGUAGES = [
  { value: "en-US", label: "English (US)" },
  { value: "en-GB", label: "English (UK)" },
  { value: "pt-BR", label: "Português (Brasil)" },
  { value: "pt-PT", label: "Português (Portugal)" },
  { value: "es-ES", label: "Español" },
  { value: "fr-FR", label: "Français" },
  { value: "de-DE", label: "Deutsch" },
  { value: "it-IT", label: "Italiano" },
  { value: "ja-JP", label: "日本語" },
  { value: "ko-KR", label: "한국어" },
  { value: "zh-CN", label: "中文" },
  { value: "ru-RU", label: "Русский" },
];

function RegionStep({ data, update }: { data: WizardData; update: Update }): JSX.Element {
  return (
    <div className="flex flex-col items-center justify-center h-full px-6 py-10">
      <div className="w-full max-w-md space-y-8">
        <div>
          <h2 className="text-xl font-semibold tracking-tight text-foreground">{t("Language & Region")}</h2>
          <p className="text-sm text-muted-foreground leading-relaxed mt-1">
            {t("Pick your language and local timezone so your copilot speaks to you naturally.")}
          </p>
        </div>

        <div className="space-y-5 bg-muted/20 border border-border/40 p-6 rounded-2xl backdrop-blur-sm">
          <div className="space-y-2">
            <Label htmlFor="ob-language" className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
              {t("Primary Language")}
            </Label>
            <Select value={data.language} onValueChange={(v) => update("language", v)}>
              <SelectTrigger id="ob-language" className="h-11 bg-background/60 border-border/60 rounded-xl focus:ring-primary/20 transition-all w-full">
                <SelectValue placeholder={t("Select primary language")} />
              </SelectTrigger>
              <SelectContent className="rounded-xl border-border/60 max-h-56">
                {LANGUAGES.map((lang) => (
                  <SelectItem key={lang.value} value={lang.value} className="rounded-lg text-sm">
                    {lang.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="ob-timezone" className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
              {t("Timezone")}
            </Label>
            <div className="relative">
              <Clock className="w-4 h-4 text-muted-foreground/60 absolute left-3.5 top-1/2 -translate-y-1/2" />
              <Input
                id="ob-timezone"
                placeholder={t("e.g. America/Sao_Paulo")}
                className="pl-10 h-11 bg-background/60 border-border/60 rounded-xl focus-visible:ring-primary/20 transition-all"
                value={data.timezone}
                onChange={(e) => update("timezone", e.target.value)}
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3 pt-1">
            <div className="space-y-2">
              <Label htmlFor="ob-city" className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                {t("City")} <span className="normal-case text-muted-foreground/60 font-normal">{t("(optional)")}</span>
              </Label>
              <div className="relative">
                <MapPin className="w-3.5 h-3.5 text-muted-foreground/60 absolute left-3 top-1/2 -translate-y-1/2" />
                <Input
                  id="ob-city"
                  placeholder={t("São Paulo")}
                  className="pl-8 h-10 bg-background/60 border-border/60 rounded-xl text-sm focus-visible:ring-primary/20 transition-all"
                  value={data.city}
                  onChange={(e) => update("city", e.target.value)}
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="ob-country" className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                {t("Country")} <span className="normal-case text-muted-foreground/60 font-normal">{t("(optional)")}</span>
              </Label>
              <Input
                id="ob-country"
                placeholder={t("Brazil")}
                className="h-10 bg-background/60 border-border/60 rounded-xl text-sm focus-visible:ring-primary/20 transition-all"
                value={data.country}
                onChange={(e) => update("country", e.target.value)}
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Workspace
// ---------------------------------------------------------------------------

/**
 * Where the work will live.
 *
 * This step did not exist, and its absence was the single most expensive
 * defect in the application: onboarding created the account, said "your
 * workspace is live", and registered no workspace at all. The daemon then had
 * nothing to serve — the sidebar showed "No Workspace", the switcher was
 * empty, and there was no way to make one from the interface, because the only
 * button that creates a workspace lives inside that switcher.
 *
 * The directory is optional on purpose. Leaving it empty is the right default
 * for somebody who opened an application and does not have a repository in
 * mind: the daemon creates one under its own state directory
 * (`workspace_create` with no `path`). Pointing it at an existing repository
 * is the other real case, and the picker is there for it.
 */
function WorkspaceStep({ data, update }: { data: WizardData; update: Update }): JSX.Element {
  return (
    <div className="flex flex-col items-center justify-center h-full px-8 py-10">
      <div className="w-full max-w-sm space-y-6">
        <div>
          <h2 className="text-xl font-semibold tracking-tight">{t("Your first workspace")}</h2>
          <p className="text-sm text-muted-foreground mt-1">
            {t("A workspace is a folder your agents work in. Everything they write — agents, tasks, notes — is a file inside it.")}
          </p>
        </div>

        <div className="space-y-5">
          <div className="space-y-2">
            <Label htmlFor="ob-ws-name">{t("Workspace name")}</Label>
            <Input
              id="ob-ws-name"
              placeholder={t("e.g. Acme Corp")}
              className="h-10 bg-background/60 border-border/60 rounded-xl text-sm focus-visible:ring-primary/20 transition-all"
              value={data.workspaceName}
              onChange={(e) => update("workspaceName", e.target.value)}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="ob-ws-path">{t("Folder (optional)")}</Label>
            <FolderInput
              value={data.workspacePath}
              onChange={(value) => update("workspacePath", value)}
              placeholder={t("Leave empty and AOS picks a folder for you")}
              inputClassName="h-10 bg-background/60 border-border/60 rounded-xl text-sm"
            />
            <p className="text-xs text-muted-foreground">
              {t("Point this at a Git repository to have your agents work directly on it.")}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Orchestrator
// ---------------------------------------------------------------------------

const TONES: { value: Tone; icon: typeof Smile; label: string }[] = [
  { value: "friendly", icon: Smile, label: "Friendly" },
  { value: "efficient", icon: Zap, label: "Efficient" },
  { value: "professional", icon: Briefcase, label: "Professional" },
  { value: "candid", icon: MessageSquare, label: "Candid" },
];

const STYLES: { value: Style; icon: typeof Scissors; label: string }[] = [
  { value: "concise", icon: Scissors, label: "Concise" },
  { value: "balanced", icon: Scale, label: "Balanced" },
  { value: "detailed", icon: FileText, label: "Detailed" },
];

function OrchestratorStep({ data, update }: { data: WizardData; update: Update }): JSX.Element {
  return (
    <div className="flex flex-col items-center justify-center h-full px-8 py-10">
      <div className="w-full max-w-sm space-y-6">
        <div>
          <h2 className="text-xl font-semibold tracking-tight">{t("Meet your copilot")}</h2>
          <p className="text-sm text-muted-foreground mt-1">{t("Give it a name and a style — it'll adapt from there.")}</p>
        </div>

        <div className="space-y-5">
          <div className="space-y-2">
            <Label htmlFor="ob-orch-name">{t("Name")}</Label>
            <Input
              id="ob-orch-name"
              placeholder={t("e.g. Atlas, Nova, Sage...")}
              value={data.orchestratorName}
              onChange={(e) => update("orchestratorName", e.target.value)}
            />
          </div>

          <div className="space-y-2">
            <Label>{t("Tone")}</Label>
            <div className="flex flex-wrap divide-x rounded-md border w-fit bg-background/30 overflow-hidden">
              {TONES.map((tone) => {
                const Icon = tone.icon;
                return (
                  <button
                    key={tone.value}
                    type="button"
                    onClick={() => update("tone", tone.value)}
                    className={cn(
                      "flex flex-col items-start justify-between p-3.5 transition-all duration-200 cursor-pointer w-[76px] h-[78px] text-left",
                      "hover:bg-accent/50",
                      data.tone === tone.value ? "bg-secondary shadow-sm" : "bg-transparent",
                    )}
                  >
                    <Icon className="w-4 h-4 text-foreground" />
                    <span className="text-[10px] font-medium leading-tight truncate w-full text-left">{tone.label}</span>
                  </button>
                );
              })}
            </div>
          </div>

          <div className="space-y-2">
            <Label>{t("Response style")}</Label>
            <div className="flex flex-wrap divide-x rounded-md border w-fit bg-background/30 overflow-hidden">
              {STYLES.map((style) => {
                const Icon = style.icon;
                return (
                  <button
                    key={style.value}
                    type="button"
                    onClick={() => update("style", style.value)}
                    className={cn(
                      "flex flex-col items-start justify-between p-3.5 transition-all duration-200 cursor-pointer w-[76px] h-[78px] text-left",
                      "hover:bg-accent/50",
                      data.style === style.value ? "bg-secondary shadow-sm" : "bg-transparent",
                    )}
                  >
                    <Icon className="w-4 h-4 text-foreground" />
                    <span className="text-[10px] font-medium leading-tight truncate w-full text-left">{style.label}</span>
                  </button>
                );
              })}
            </div>
          </div>

          <div className="space-y-2">
            <Label>{t("Autonomy level")}</Label>
            <Slider
              value={data.autonomy}
              onChange={(v) => update("autonomy", v as number)}
              min={0}
              max={1}
              step={0.1}
              showValue={false}
            />
            <div className="flex justify-between text-[11px] text-muted-foreground pt-1">
              <span>{t("Asks permission")}</span>
              <span>{t("Fully autonomous")}</span>
            </div>
          </div>

          <p className="text-xs text-muted-foreground">
            {t("These preferences aren't wired to a running orchestrator yet — the one this workspace gets is seeded automatically when it's created.")}
          </p>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Init — the account-creation step. Same STAGES-timer choreography as the
// router-bound copy; the network calls underneath are the plain
// `lib/auth.ts` ones this file always used.
// ---------------------------------------------------------------------------

interface InitStage {
  id: string;
  label: string;
  message: string;
  duration: number; // ms
}

const STAGES: InitStage[] = [
  { id: "account", label: "Creating your account", message: "Hashing your password locally...", duration: 2500 },
  { id: "workspace", label: "Preparing workspace", message: "Setting up your personal environment...", duration: 2500 },
  { id: "preferences", label: "Saving your preferences", message: "Applying your region and language...", duration: 1800 },
  { id: "ready", label: "Ready", message: "Your workspace is prepared.", duration: 1200 },
];

const TOTAL_DURATION = STAGES.reduce((acc, s) => acc + s.duration, 0);

type InitState = { currentStage: number; completed: boolean; error: boolean };
type InitAction = { type: "ADVANCE" } | { type: "COMPLETE" } | { type: "ERROR" };

function stepReducer(state: InitState, action: InitAction): InitState {
  switch (action.type) {
    case "ADVANCE":
      return { ...state, currentStage: state.currentStage + 1 };
    case "COMPLETE":
      return { ...state, completed: true };
    case "ERROR":
      return { ...state, error: true };
    default:
      return state;
  }
}

/**
 * Registers the first workspace, unless the installation already has one.
 *
 * The check matters on a second run of onboarding against a daemon that was
 * already set up (a reinstalled application over a live state directory):
 * creating a second workspace there would split the person's work in two
 * without saying so.
 */
async function ensureWorkspace(data: WizardData): Promise<void> {
  const listed = (await client.invoke("workspace_list", {
    _reasoning: "onboarding is finishing and needs to know whether a workspace already exists",
  })) as { workspaces?: unknown[] } | undefined;
  if ((listed?.workspaces?.length ?? 0) > 0) return;

  const path = data.workspacePath.trim();
  await client.invoke("workspace_create", {
    name: data.workspaceName.trim() || `${data.name.trim() || "My"} workspace`,
    ...(path ? { path } : {}),
    orchestrator: {
      ...(data.orchestratorName.trim() ? { name: data.orchestratorName.trim() } : {}),
      tone: data.tone,
      style: data.style,
      autonomy: data.autonomy,
    },
    _reasoning: "onboarding is creating the installation's first workspace",
  });
}

function InitStep({
  data,
  onError,
  onDone,
}: {
  data: WizardData;
  onError: (message: string) => void;
  onDone: () => void;
}): JSX.Element {
  const [state, dispatch] = useReducer(stepReducer, { currentStage: 0, completed: false, error: false });
  const startedAtRef = useRef(0);
  const initStartedRef = useRef(false);

  const progress = state.completed ? 100 : Math.min(100, ((performance.now() - startedAtRef.current) / TOTAL_DURATION) * 100);

  const performInit = useCallback(async () => {
    if (initStartedRef.current) return;
    initStartedRef.current = true;
    startedAtRef.current = performance.now();

    try {
      await onboarding(data.name.trim(), data.email.trim(), data.password);

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

      // The workspace. This is the step that was missing, and it is not
      // optional: without a registered workspace the daemon has nothing to
      // serve, so the sidebar reads "No Workspace", the switcher is empty and
      // every workspace-scoped screen shows an empty state that looks like a
      // bug — which is exactly what a fresh installation did.
      //
      // The orchestrator's name, tone, style and autonomy travel with it.
      // `workspace_create` has taken them since it was written; the wizard
      // collected all four and sent none, so the copilot somebody named on
      // the previous screen was always called Atlas.
      await ensureWorkspace(data);

      dispatch({ type: "COMPLETE" });
      // A short beat so the "Ready" stage is actually seen before AuthGate
      // re-checks status and swaps this whole screen out.
      setTimeout(onDone, 900);
    } catch (err) {
      dispatch({ type: "ERROR" });
      onError(err instanceof DomainError ? err.message : "Something went wrong.");
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    void performInit();
  }, [performInit]);

  // Advance the visual stage list on the same fixed schedule the router-bound
  // copy uses — decorative pacing, not tied to when the real call resolves.
  useEffect(() => {
    if (state.completed || state.error) return;
    const tick = () => {
      const elapsed = performance.now() - startedAtRef.current;
      let stageIndex = 0;
      let cumulative = 0;
      for (let i = 0; i < STAGES.length; i++) {
        cumulative += STAGES[i].duration;
        stageIndex = i;
        if (elapsed < cumulative) break;
      }
      if (stageIndex !== state.currentStage) {
        dispatch({ type: "ADVANCE" });
      }
    };
    const interval = setInterval(tick, 100);
    return () => clearInterval(interval);
  }, [state.currentStage, state.completed, state.error]);

  return (
    <div className="flex flex-col items-center justify-center h-full px-8 py-12">
      <div className="w-full max-w-sm space-y-10">
        <div className="text-center space-y-3">
          <div className="mx-auto w-12 h-12 rounded-2xl bg-primary/10 flex items-center justify-center">
            {state.completed ? (
              <CheckCircle2 className="w-6 h-6 text-primary animate-in" />
            ) : (
              <Loader2 className="w-6 h-6 text-primary animate-spin" />
            )}
          </div>
          <h2 className="text-2xl font-semibold tracking-tight">
            {state.completed ? "You're all set" : "Getting everything ready"}
          </h2>
          <p className="text-muted-foreground text-sm">
            {state.completed
              ? "Your workspace is live. Let's find out what it can do."
              : "Just a moment while AOS prepares your workspace..."}
          </p>
        </div>

        <div className="space-y-4">
          {STAGES.map((stage, index) => {
            const isStageDone = index < state.currentStage || state.completed;
            const isStageActive = index === state.currentStage && !state.completed && !state.error;

            return (
              <div key={stage.id} className="flex items-start gap-3">
                <div className="relative mt-1.5 flex-shrink-0">
                  <motion.div
                    className="w-2 h-2 rounded-full"
                    animate={{
                      backgroundColor: isStageDone || isStageActive ? "var(--primary)" : "var(--muted)",
                      scale: isStageActive ? [1, 1.3, 1] : 1,
                    }}
                    transition={{ scale: { duration: 1.2, repeat: Infinity, ease: "easeInOut" } }}
                  />
                  {isStageActive && (
                    <motion.div
                      className="absolute inset-0 w-2 h-2 rounded-full bg-primary/40"
                      animate={{ scale: [1, 2.5], opacity: [0.6, 0] }}
                      transition={{ duration: 1.2, repeat: Infinity, ease: "easeOut" }}
                    />
                  )}
                </div>

                <div className="flex-1 min-w-0">
                  <AnimatePresence mode="wait">
                    {(isStageActive || isStageDone) && (
                      <motion.div
                        key={`${stage.id}-${isStageDone ? "done" : "active"}`}
                        initial={{ opacity: 0, y: 4 }}
                        animate={{ opacity: 1, y: 0 }}
                        exit={{ opacity: 0, y: -4 }}
                        transition={{ duration: 0.3 }}
                      >
                        <p className={`text-sm font-medium ${isStageDone ? "text-muted-foreground" : "text-foreground"}`}>
                          {isStageDone ? <span className="line-through opacity-60">{stage.label}</span> : stage.label}
                        </p>
                        {isStageActive && <p className="text-xs text-muted-foreground mt-0.5">{stage.message}</p>}
                      </motion.div>
                    )}
                  </AnimatePresence>
                </div>
              </div>
            );
          })}
        </div>

        <div className="space-y-2">
          <div className="h-1 rounded-full bg-muted overflow-hidden">
            <motion.div
              className="h-full rounded-full bg-primary"
              initial={{ width: "0%" }}
              animate={{ width: state.completed ? "100%" : `${progress}%` }}
              transition={{ duration: 0.8, ease: "easeInOut" }}
            />
          </div>
          <p className="text-xs text-muted-foreground text-right">{Math.round(progress)}%</p>
        </div>
      </div>
    </div>
  );
}
