"use client";

import { useState, useEffect, useCallback } from "react";
import { Clock, CheckCircle2, RefreshCw, ExternalLink } from "lucide-react";
import { Button } from "@/components/ui/button";
import { aos } from "@/app/aos";
import { toast } from "sonner";

const WAITLIST_POLL_INTERVAL_MS = 60_000;

export interface SavedOnboardingWaitlistState {
  email: string;
  formData: any;
  status: "waiting" | "approved";
  savedAt: string;
}

interface OnboardingFormWaitlistStepProps {
  savedState: SavedOnboardingWaitlistState;
  onApprovedSubmitted: () => Promise<void>;
  onReset: () => void;
}

function XIcon(props: React.SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" {...props}>
      <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
    </svg>
  );
}

function LinkedInIcon(props: React.SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" {...props}>
      <path d="M19 3a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h14m-.5 15.5v-5.3a3.26 3.26 0 0 0-3.26-3.26c-.85 0-1.84.52-2.28 1.3v-1.11h-2.79v8.37h2.79v-4.93c0-.77.62-1.4 1.39-1.4a1.4 1.4 0 0 1 1.4 1.4v4.93h2.75M6.88 8.56a1.68 1.68 0 0 0 1.68-1.68c0-.93-.75-1.69-1.68-1.69a1.69 1.69 0 0 0-1.69 1.69c0 .93.76 1.68 1.69 1.68m1.39 9.94v-8.37H5.5v8.37h2.77z" />
    </svg>
  );
}

export function OnboardingFormWaitlistStep({
  savedState,
  onApprovedSubmitted,
  onReset,
}: OnboardingFormWaitlistStepProps) {
  const [status, setStatus] = useState<"waiting" | "approved">(
    savedState.status || "waiting",
  );
  const [isChecking, setIsChecking] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const checkWaitlistStatus = useCallback(
    async (silent = false) => {
      if (!savedState.email) return false;
      setIsChecking(true);

      try {
        const res = await aos.client.auth.verifyWaitlist.mutate({
          body: {
            email: savedState.email,
            name: savedState.formData?.user?.name,
          },
        });

        if (res?.data?.approved === true) {
          setStatus("approved");
          const updated: SavedOnboardingWaitlistState = {
            ...savedState,
            status: "approved",
          };
          localStorage.setItem(
            "FRACTAL_ONBOARDING_WAITLIST_STATE",
            JSON.stringify(updated),
          );

          if (!silent) {
            toast.success("Your access has been approved.");
          }
          return true;
        }
      } catch (err) {
        if (!silent) {
          console.warn("Could not check waitlist status:", err);
        }
      } finally {
        setIsChecking(false);
      }
      return false;
    },
    [savedState],
  );

  // Poll every 60s while waiting
  useEffect(() => {
    if (status === "approved") return;

    checkWaitlistStatus(true);

    const interval = setInterval(() => {
      checkWaitlistStatus(true);
    }, WAITLIST_POLL_INTERVAL_MS);

    return () => clearInterval(interval);
  }, [status, checkWaitlistStatus]);

  const handleStart = async () => {
    setIsSubmitting(true);
    try {
      await onApprovedSubmitted();
    } catch (err) {
      toast.error("Something went wrong while creating your workspace.", {
        description: err instanceof Error ? err.message : "Please try again.",
      });
      setIsSubmitting(false);
    }
  };

  const xShareUrl = `https://twitter.com/intent/tweet?text=${encodeURIComponent(
    "Testing @tryfractal, the workspace where AI agents keep real context and ship work end to end. https://tryfractal.co",
  )}`;

  const linkedinShareUrl = `https://www.linkedin.com/sharing/share-offsite/?url=${encodeURIComponent(
    "https://tryfractal.co",
  )}`;

  if (status === "approved") {
    return (
      <div className="flex flex-col items-center justify-center h-full px-8 py-8">
        <div className="w-full max-w-md space-y-6 text-center">
          <div className="mx-auto w-11 h-11 rounded-xl bg-primary/10 flex items-center justify-center">
            <CheckCircle2 className="w-5 h-5 text-primary" />
          </div>

          <div className="space-y-2">
            <h2 className="text-xl font-semibold tracking-tight text-foreground">
              You&apos;re approved
            </h2>
            <p className="text-sm text-muted-foreground leading-relaxed">
              Your waitlist spot is confirmed. Create your workspace to get
              started with Fractal.
            </p>
          </div>

          <Button
            type="button"
            size="lg"
            className="w-auto rounded-xl mx-auto"
            onClick={handleStart}
            disabled={isSubmitting}
          >
            {isSubmitting ? (
              <>
                <RefreshCw className="w-4 h-4 animate-spin" />
                <span>Creating your workspace</span>
              </>
            ) : (
              "Create your workspace"
            )}
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center justify-center h-full px-8 py-8 overflow-y-auto">
      <div className="w-full max-w-md space-y-6 text-center">
        <div className="mx-auto w-11 h-11 rounded-xl bg-muted flex items-center justify-center">
          <Clock className="w-5 h-5 text-muted-foreground" />
        </div>

        <div className="space-y-2">
          <h2 className="text-xl font-semibold tracking-tight text-foreground">
            You&apos;re on the waitlist
          </h2>
          <p className="text-sm text-muted-foreground leading-relaxed">
            Your details for{" "}
            <span className="font-medium text-foreground">{savedState.email}</span>{" "}
            were saved. Your access unlocks as soon as it&apos;s approved, and
            you can pick up right where you left off.
          </p>
        </div>

        <div className="border-t border-border/60 pt-5 space-y-4">
          <p className="text-xs text-muted-foreground leading-relaxed">
            Want to skip the line? Share Fractal on X or LinkedIn and mention
            us — most approvals land within a few hours.
          </p>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
            <Button variant="outline" className="w-full rounded-xl" asChild>
              <a href={xShareUrl} target="_blank" rel="noopener noreferrer">
                <XIcon className="w-3.5 h-3.5" />
                <span>Share on X</span>
                <ExternalLink className="w-3 h-3 opacity-60 ml-auto" />
              </a>
            </Button>

            <Button variant="outline" className="w-full rounded-xl" asChild>
              <a href={linkedinShareUrl} target="_blank" rel="noopener noreferrer">
                <LinkedInIcon className="w-3.5 h-3.5" />
                <span>Share on LinkedIn</span>
                <ExternalLink className="w-3 h-3 opacity-60 ml-auto" />
              </a>
            </Button>
          </div>
        </div>

        <div className="flex items-center justify-center gap-3 pt-1">
          <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
            <RefreshCw
              className={`w-3 h-3 ${isChecking ? "animate-spin text-primary" : ""}`}
            />
            {isChecking ? "Checking status" : "Checks every 60 seconds"}
          </span>
          <button
            type="button"
            onClick={() => checkWaitlistStatus(false)}
            disabled={isChecking}
            className="text-xs text-muted-foreground underline-offset-4 hover:text-foreground hover:underline transition-colors disabled:opacity-50"
          >
            Check now
          </button>
        </div>

        <button
          type="button"
          onClick={onReset}
          className="text-xs text-muted-foreground underline-offset-4 hover:text-foreground hover:underline transition-colors"
        >
          Use a different email
        </button>
      </div>
    </div>
  );
}
