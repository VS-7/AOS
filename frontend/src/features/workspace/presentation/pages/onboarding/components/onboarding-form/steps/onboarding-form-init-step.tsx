"use client";

import { useEffect, useReducer, useRef, useCallback } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { CheckCircle2, Loader2 } from "lucide-react";
import { useFormContext } from "react-hook-form";
import { aos } from "@/app/aos";
import { toast } from "sonner";
import type { FractalAuthOnboarding } from "@/features/auth/interfaces/auth.interfaces";

interface InitStage {
  id: string;
  label: string;
  message: string;
  duration: number; // ms
}

const STAGES: InitStage[] = [
  {
    id: "fractal",
    label: "Booting Fractal",
    message: "Warming up the runtime...",
    duration: 3500,
  },
  {
    id: "workspace",
    label: "Preparing workspace",
    message: "Setting up your personal environment...",
    duration: 4500,
  },
  {
    id: "copilot",
    label: "Configuring copilot",
    message: "Learning your style and preferences...",
    duration: 5000,
  },
  {
    id: "tools",
    label: "Connecting tools",
    message: "Syncing agents, skills, and capabilities...",
    duration: 4500,
  },
  {
    id: "ready",
    label: "Ready",
    message: "Your workspace is prepared.",
    duration: 3000,
  },
];

const TOTAL_DURATION = STAGES.reduce((acc, s) => acc + s.duration, 0); // ~20.5s

function isWaitlistError(err: any): boolean {
  const envelope = err?.error && typeof err.error === "object" ? err.error : err;
  const domainCode = envelope?.data?.code ?? envelope?.code ?? err?.data?.code;
  const errMsg = String(envelope?.message ?? err?.message ?? "");
  return (
    domainCode === "FRACTAL_AUTH_WAITLIST_NOT_APPROVED" ||
    errMsg.toLowerCase().includes("waitlist")
  );
}

type InitState = {
  currentStage: number;
  completed: boolean;
  redirected: boolean;
  error: boolean;
};

type InitAction =
  | { type: "ADVANCE" }
  | { type: "COMPLETE" }
  | { type: "REDIRECT" }
  | { type: "ERROR" };

function stepReducer(state: InitState, action: InitAction): InitState {
  switch (action.type) {
    case "ADVANCE":
      return { ...state, currentStage: state.currentStage + 1 };
    case "COMPLETE":
      return { ...state, completed: true };
    case "REDIRECT":
      return { ...state, redirected: true };
    case "ERROR":
      return { ...state, error: true };
    default:
      return state;
  }
}

// -----------------------------------------------------------------------
// Form values shape (mirrors OnboardingForm schema)
// -----------------------------------------------------------------------

export interface OnboardingFormInitStepProps {
  onError?: () => void;
  onWaitlistRequired?: (savedState: any) => void;
}

// -----------------------------------------------------------------------
// Component
// -----------------------------------------------------------------------

export function OnboardingFormInitStep({
  onError,
  onWaitlistRequired,
}: OnboardingFormInitStepProps) {
  const form = useFormContext<FractalAuthOnboarding>();
  const [state, dispatch] = useReducer(stepReducer, {
    currentStage: 0,
    completed: false,
    redirected: false,
    error: false,
  });

  const startedRef = useRef(false);
  const startedAtRef = useRef(0);
  const initStartedRef = useRef(false);

  const isDone = state.currentStage >= STAGES.length - 1 && state.completed;
  const isRedirected = state.redirected;

  // Progress: elapsed / total
  const progress = (() => {
    if (state.completed) return 100;
    if (!startedRef.current) return 0;
    const elapsed = performance.now() - startedAtRef.current;
    return Math.min(100, (elapsed / TOTAL_DURATION) * 100);
  })();

  const performInit = useCallback(async () => {
    if (initStartedRef.current) return;
    startedRef.current = true;
    startedAtRef.current = performance.now();
    initStartedRef.current = true;

    try {
      const data = form.getValues();

      const result = await aos.client.auth.onboarding.mutate({
        body: {
          user: {
            name: data.user.name,
            email: data.user.email,
          },
          security: {
            password: data.security.password,
          },
          region: {
            language: data.region.language,
            city: data.region.city,
            country: data.region.country,
            timezone: data.region.timezone,
          },
          orchestrator: {
            name: data.orchestrator.name,
            tone: data.orchestrator.tone,
            style: data.orchestrator.style,
            autonomy: data.orchestrator.autonomy,
          },
        },
      });

      if (result.error || !result.data?.workspaceId) {
        if (isWaitlistError(result.error)) {
          const waitlistPayload = {
            email: data.user.email,
            formData: data,
            status: "waiting" as const,
            savedAt: new Date().toISOString(),
          };
          localStorage.setItem(
            "FRACTAL_ONBOARDING_WAITLIST_STATE",
            JSON.stringify(waitlistPayload),
          );
          onWaitlistRequired?.(waitlistPayload);
          return;
        }

        dispatch({ type: "ERROR" });
        toast.error("An error occurred while creating your workspace", {
          description: "Please check the data and try again.",
        });
        onError?.();
        return;
      }

      // [Auto-Login]: Authenticate user directly so they stay logged in on dashboard
      await aos.stores.auth.actions.login({
        email: data.user.email,
        password: data.security.password,
      });

      // [Cleanup]: Remove waitlist cache once completed
      localStorage.removeItem("FRACTAL_ONBOARDING_WAITLIST_STATE");

      toast.success("Your workspace has been created successfully!");
      await aos.stores.workspace.actions.switch(result.data.workspaceId);
    } catch (err: any) {
      if (isWaitlistError(err)) {
        const data = form.getValues();
        const waitlistPayload = {
          email: data.user.email,
          formData: data,
          status: "waiting" as const,
          savedAt: new Date().toISOString(),
        };
        localStorage.setItem(
          "FRACTAL_ONBOARDING_WAITLIST_STATE",
          JSON.stringify(waitlistPayload),
        );
        onWaitlistRequired?.(waitlistPayload);
        return;
      }

      dispatch({ type: "ERROR" });
      toast.error("An error occurred while creating your workspace", {
        description: "Please check the data and try again.",
      });
      onError?.();
      return;
    }
  }, [form, onError, onWaitlistRequired]);

  // Start init on mount (parent sets isSubmitted to trigger this)
  useEffect(() => {
    performInit();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Advance stages based on elapsed time from mount
  useEffect(() => {
    if (!startedRef.current || state.completed) return;

    const tick = () => {
      const elapsed = performance.now() - startedAtRef.current;
      let stageIndex = 0;
      let cumulative = 0;

      for (let i = 0; i < STAGES.length; i++) {
        cumulative += STAGES[i].duration;
        if (elapsed < cumulative) {
          stageIndex = i;
          break;
        }
        stageIndex = i;
      }

      if (stageIndex !== state.currentStage) {
        if (stageIndex >= STAGES.length - 1) {
          dispatch({ type: "COMPLETE" });
        } else {
          dispatch({ type: "ADVANCE" });
        }
      }
    };

    // Poll every 100ms
    const interval = setInterval(tick, 100);
    tick(); // run immediately

    return () => clearInterval(interval);
  }, [state.currentStage, state.completed]);

  // Redirect after completion
  useEffect(() => {
    if (!state.completed || isRedirected) return;

    const redirectTimer = setTimeout(() => {
      dispatch({ type: "REDIRECT" });
      window.location.replace("/?welcome=true");
    }, 1200);

    return () => clearTimeout(redirectTimer);
  }, [state.completed, isRedirected]);

  return (
    <div className="flex flex-col items-center justify-center h-full px-8 py-12">
      <div className="w-full max-w-sm space-y-10">
        {/* Logo + title */}
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
              : "Just a moment while Fractal prepares your workspace..."}
          </p>
        </div>

        {/* Stage progress */}
        <div className="space-y-4">
          {STAGES.map((stage, index) => {
            const isStageDone = index < state.currentStage || state.completed;
            const isStageActive =
              index === state.currentStage && !state.completed && !state.error;
            const isReadyStage = stage.id === "ready";

            return (
              <div key={stage.id} className="flex items-start gap-3">
                {/* Indicator dot */}
                <div className="relative mt-1.5 flex-shrink-0">
                  <motion.div
                    className="w-2 h-2 rounded-full"
                    animate={{
                      backgroundColor: isStageDone
                        ? "var(--primary)"
                        : isStageActive
                          ? "var(--primary)"
                          : "var(--muted)",
                      scale: isStageActive ? [1, 1.3, 1] : 1,
                    }}
                    transition={{
                      scale: {
                        duration: 1.2,
                        repeat: Infinity,
                        ease: "easeInOut",
                      },
                    }}
                  />
                  {isStageActive && (
                    <motion.div
                      className="absolute inset-0 w-2 h-2 rounded-full bg-primary/40"
                      animate={{ scale: [1, 2.5], opacity: [0.6, 0] }}
                      transition={{
                        duration: 1.2,
                        repeat: Infinity,
                        ease: "easeOut",
                      }}
                    />
                  )}
                </div>

                {/* Label + message */}
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
                        <p
                          className={`text-sm font-medium ${
                            isStageDone
                              ? "text-muted-foreground"
                              : "text-foreground"
                          }`}
                        >
                          {isStageDone ? (
                            <span className="line-through opacity-60">
                              {stage.label}
                            </span>
                          ) : (
                            stage.label
                          )}
                        </p>
                        {isStageActive && (
                          <p className="text-xs text-muted-foreground mt-0.5">
                            {stage.message}
                          </p>
                        )}
                      </motion.div>
                    )}
                  </AnimatePresence>
                </div>
              </div>
            );
          })}
        </div>

        {/* Overall progress bar */}
        <div className="space-y-2">
          <div className="h-1 rounded-full bg-muted overflow-hidden">
            <motion.div
              className="h-full rounded-full bg-primary"
              initial={{ width: "0%" }}
              animate={{
                width: state.completed ? "100%" : `${progress}%`,
              }}
              transition={{ duration: 0.8, ease: "easeInOut" }}
            />
          </div>
          <p className="text-xs text-muted-foreground text-right">
            {Math.round(progress)}%
          </p>
        </div>
      </div>
    </div>
  );
}
