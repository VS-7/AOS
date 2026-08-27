"use client";

import { useState, useCallback, useMemo, useEffect } from "react";
import { ArrowLeft } from "lucide-react";
import { AnimatePresence, motion } from "framer-motion";
import { FormProvider } from "react-hook-form";
import { useRouter } from "@tanstack/react-router";

import { aos } from "@/app/aos";
import { Logo } from "@/components/ui/logo";
import { Button } from "@/components/ui/button";

import { AuthOnboardingSchema } from "@/features/auth/schemas/auth.schema";

import { PLACEHOLDER_IMAGE } from "@/assets/placeholder";
import { OnboardingFormWelcomeStep } from "./steps/onboarding-form-welcome-step";
import { OnboardingFormUserStep } from "./steps/onboarding-form-user-step";
import { OnboardingFormSecurityStep } from "./steps/onboarding-form-security-step";
import { OnboardingFormRegionStep } from "./steps/onboarding-form-region-step";
import {
  OnboardingFormOrchestratorStep,
  type OnboardingFormOrchestratorStepProps,
} from "./steps/onboarding-form-orchestrator-step";
import { OnboardingFormInitStep } from "./steps/onboarding-form-init-step";
import { t } from "@/lib/i18n";

// ---------------------------------------------------------------------------
// Step definitions — fields to validate before advancing
// ---------------------------------------------------------------------------

const STEPS = [
  { id: "welcome", title: "Welcome", fields: [] as string[] },
  { id: "user", title: "About You", fields: ["user.name", "user.email"] },
  { id: "security", title: "Security", fields: ["security.password"] },
  { id: "region", title: "Preferences", fields: ["region.language"] },
  { id: "orchestrator", title: "Copilot", fields: ["orchestrator.name"] },
];

const TOTAL_STEPS = STEPS.length + 1; // +1 for init step

// ---------------------------------------------------------------------------
// Browser defaults
// ---------------------------------------------------------------------------

function detectRegionDefaults() {
  try {
    const browserLang = navigator.language || "en";
    const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || "";
    return { language: browserLang, timezone };
  } catch {
    return { language: "en", timezone: "" };
  }
}

// ---------------------------------------------------------------------------
// Transition config
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

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function OnboardingForm() {
  const router = useRouter();

  // ── Safety guard: redirect if onboarding is already complete ──
  const onboarding = aos.stores.auth.state.onboarding;

  useEffect(() => {
    if (onboarding === "done") {
      router.navigate({ to: "/" });
    }
  }, [onboarding, router]);

  const [step, setStep] = useState(0);
  const [direction, setDirection] = useState(1);

  const regionDefaults = useMemo(detectRegionDefaults, []);
  const initialValues = useMemo(
    () => ({
      user: { name: "", email: "" },
      security: { password: "" },
      region: {
        language: regionDefaults.language,
        city: "",
        country: "",
        timezone: regionDefaults.timezone,
      },
      orchestrator: {
        name: "",
        tone: "friendly" as const,
        style: "balanced" as const,
        autonomy: 0.5,
      },
    }),
    [regionDefaults],
  );

  const form = aos.useForm({
    schema: AuthOnboardingSchema,
    values: initialValues,
    onSubmit: async (_values) => {
      // Workspace creation is handled by OnboardingFormInitStep.
    },
  });

  const goNext = useCallback(async () => {
    // Welcome step — just advance, no validation
    if (step === 0) {
      setDirection(1);
      setStep(1);
      return;
    }

    // Validate current step fields
    const currentStepConfig = STEPS[step];
    if (currentStepConfig.fields.length > 0) {
      const valid = await form.trigger(currentStepConfig.fields as any);
      if (!valid) return;
    }

    // Last form step — transition to init step
    if (step === TOTAL_STEPS - 2) {
      setDirection(1);
      setStep(TOTAL_STEPS - 1);
      return;
    }

    setDirection(1);
    setStep((s) => s + 1);
  }, [step, form]);

  const goBack = useCallback(() => {
    if (step === 0) return;
    setDirection(-1);
    setStep((s) => s - 1);
  }, [step]);

  // Global Keyboard Navigation
  useEffect(() => {
    if (step === TOTAL_STEPS - 1) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      const isInput =
        target?.tagName === "INPUT" || target?.tagName === "TEXTAREA";

      if (e.key === "Enter" && isInput) {
        e.preventDefault();
        goNext();
      } else if (e.key === "Escape" && step > 0) {
        e.preventDefault();
        goBack();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [goNext, goBack, step]);

  const handleInitError = useCallback(() => {
    // Go back to the orchestrator step so the user can try again
    setDirection(-1);
    setStep(TOTAL_STEPS - 2);
  }, []);

  const isFirstStep = step === 0;
  const isLastFormStep = step === TOTAL_STEPS - 2;
  const isInitStep = step === TOTAL_STEPS - 1;

  const buttonLabel = isFirstStep
    ? "Get Started"
    : isLastFormStep
      ? "Create Workspace"
      : "Continue";

  const orchestratorFormProps: OnboardingFormOrchestratorStepProps = {
    form,
  };

  const progressPercentage = ((step + 1) / STEPS.length) * 100;
  return (
    <div className="grid relative overflow-hidden h-screen grid-rows-[auto_1fr] gap-2 p-4 sm:gap-4 sm:p-6">
      {/* Feature Background */}
      <div className="absolute inset-0 overflow-hidden pointer-events-none">
        <img
          src={PLACEHOLDER_IMAGE}
          alt=""
          aria-hidden
          className="size-full object-cover"
        />
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
        <FormProvider {...form}>
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
                  key={step}
                  custom={direction}
                  variants={slideVariants}
                  initial="enter"
                  animate="center"
                  exit="exit"
                  transition={slideTransition}
                  className="h-full"
                >
                  {step === 0 && <OnboardingFormWelcomeStep />}
                  {step === 1 && <OnboardingFormUserStep form={form} />}
                  {step === 2 && <OnboardingFormSecurityStep form={form} />}
                  {step === 3 && <OnboardingFormRegionStep form={form} />}
                  {step === 4 && (
                    <OnboardingFormOrchestratorStep
                      {...orchestratorFormProps}
                    />
                  )}
                  {step === 5 && (
                    <OnboardingFormInitStep onError={handleInitError} />
                  )}
                </motion.div>
              </AnimatePresence>
            </main>

            {/* Footer — hidden on init step */}
            {!isInitStep && (
              <footer className="flex items-center justify-between px-6 py-4 border-t border-border/40 bg-muted/10">
                {/* Step dots & title */}
                <div className="flex items-center gap-3">
                  <div className="flex items-center gap-1.5">
                    {Array.from({ length: STEPS.length }).map((_, i) => (
                      <button
                        key={i}
                        type="button"
                        onClick={() => {
                          if (i < step) {
                            setDirection(-1);
                            setStep(i);
                          }
                        }}
                        disabled={i > step}
                        className={`h-1.5 rounded-full transition-all duration-300 ${
                          i === step
                            ? "w-6 bg-primary"
                            : i < step
                              ? "w-2 bg-primary/40 hover:bg-primary/60 cursor-pointer"
                              : "w-1.5 bg-foreground/15 cursor-not-allowed"
                        }`}
                        aria-label={`Step ${i + 1}`}
                      />
                    ))}
                  </div>
                  <span className="text-xs text-muted-foreground font-medium">
                    {step + 1} of {STEPS.length} · {STEPS[step]?.title}
                  </span>
                </div>

                <div className="flex items-center gap-2">
                  {!isFirstStep && (
                    <Button
                      type="button"
                      variant="secondary"
                      className="rounded-full"
                      onClick={goBack}
                    >
                      {t("Previous")}
                    </Button>
                  )}
                  <Button
                    type="button"
                    className="rounded-full"
                    onClick={goNext}
                    disabled={form.formState.isLoading}
                  >
                    {form.formState.isLoading ? "Starting up..." : buttonLabel}
                  </Button>
                </div>
              </footer>
            )}
          </div>
        </FormProvider>
      </div>
    </div>
  );
}
