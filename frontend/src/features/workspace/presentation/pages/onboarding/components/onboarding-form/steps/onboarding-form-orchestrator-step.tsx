"use client";

import {
  Zap,
  Smile,
  Briefcase,
  MessageSquare,
  Scissors,
  Scale,
  FileText,
  Bot,
  ShieldAlert,
  CheckCircle2,
} from "lucide-react";
import { UseFormReturn } from "react-hook-form";
import {
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Slider } from "@/components/ui/slider";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";

export interface OnboardingFormOrchestratorStepProps {
  form: UseFormReturn<any>;
}

const TONES = [
  { value: "friendly", icon: Smile, label: "Friendly", desc: "Warm & open" },
  { value: "efficient", icon: Zap, label: "Efficient", desc: "Direct & fast" },
  { value: "professional", icon: Briefcase, label: "Professional", desc: "Structured" },
  { value: "candid", icon: MessageSquare, label: "Candid", desc: "Clear & frank" },
] as const;

const STYLES = [
  { value: "concise", icon: Scissors, label: "Concise", desc: "Short answers" },
  { value: "balanced", icon: Scale, label: "Balanced", desc: "Optimal depth" },
  { value: "detailed", icon: FileText, label: "Detailed", desc: "Full context" },
] as const;

const NAME_SUGGESTIONS = ["Atlas", "Nova", "Sage", "Orion", "Echo"];

function getAutonomyDescription(level: number) {
  if (level <= 0.3) {
    return {
      title: "Guided Mode",
      icon: ShieldAlert,
      desc: "Copilot asks for your explicit approval before performing edits or running actions.",
      color: "text-amber-500",
    };
  }
  if (level <= 0.7) {
    return {
      title: "Balanced Mode (Recommended)",
      icon: CheckCircle2,
      desc: "Executes routine workflows and asks for confirmation only on high-impact steps.",
      color: "text-primary",
    };
  }
  return {
    title: "Autonomous Mode",
    icon: Bot,
    desc: "Operates independently to carry out complex multi-step tasks end-to-end.",
    color: "text-emerald-500",
  };
}
export function OnboardingFormOrchestratorStep({
  form,
}: OnboardingFormOrchestratorStepProps) {
  return (
    <div className="flex flex-col items-center justify-center h-full px-8 py-10">
      <div className="w-full max-w-sm space-y-6">
        <div>
          <h2 className="text-xl font-semibold tracking-tight">{t("Meet your copilot")}</h2>
          <p className="text-sm text-muted-foreground mt-1">{t("Give it a name and a style — it'll adapt from there.")}</p>
        </div>

        <div className="space-y-5">
          {/* Name */}
          <FormField
            control={form.control}
            name="orchestrator.name"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t("Name")}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t("e.g. Atlas, Nova, Sage...")}
                    autoFocus
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          {/* Tone */}
          <FormField
            control={form.control}
            name="orchestrator.tone"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t("Tone")}</FormLabel>
                <div className="flex flex-wrap divide-x rounded-md border w-fit bg-background/30 overflow-hidden">
                  {TONES.map((tone) => {
                    const Icon = tone.icon;
                    return (
                      <button
                        key={tone.value}
                        type="button"
                        onClick={() => field.onChange(tone.value)}
                        className={cn(
                          "flex flex-col items-start justify-between p-3.5 transition-all duration-200 cursor-pointer w-[76px] h-[78px] text-left",
                          "hover:bg-accent/50",
                          field.value === tone.value
                            ? "bg-secondary shadow-sm"
                            : "bg-transparent",
                        )}
                      >
                        <Icon className="w-4 h-4 text-foreground" />
                        <span className="text-[10px] font-medium leading-tight truncate w-full text-left">
                          {tone.label}
                        </span>
                      </button>
                    );
                  })}
                </div>
                <FormMessage />
              </FormItem>
            )}
          />

          {/* Style */}
          <FormField
            control={form.control}
            name="orchestrator.style"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t("Response style")}</FormLabel>
                <div className="flex flex-wrap divide-x rounded-md border w-fit bg-background/30 overflow-hidden">
                  {STYLES.map((style) => {
                    const Icon = style.icon;
                    return (
                      <button
                        key={style.value}
                        type="button"
                        onClick={() => field.onChange(style.value)}
                        className={cn(
                          "flex flex-col items-start justify-between p-3.5 transition-all duration-200 cursor-pointer w-[76px] h-[78px] text-left",
                          "hover:bg-accent/50",
                          field.value === style.value
                            ? "bg-secondary shadow-sm"
                            : "bg-transparent",
                        )}
                      >
                        <Icon className="w-4 h-4 text-foreground" />
                        <span className="text-[10px] font-medium leading-tight truncate w-full text-left">
                          {style.label}
                        </span>
                      </button>
                    );
                  })}
                </div>
                <FormMessage />
              </FormItem>
            )}
          />

          {/* Autonomy */}
          <FormField
            control={form.control}
            name="orchestrator.autonomy"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t("Autonomy level")}</FormLabel>
                <FormControl>
                  <Slider
                    value={field.value ?? 0.5}
                    onChange={(v) => field.onChange(v)}
                    min={0}
                    max={1}
                    step={0.1}
                    showValue={false}
                  />
                </FormControl>
                <div className="flex justify-between text-[11px] text-muted-foreground pt-1">
                  <span>{t("Asks permission")}</span>
                  <span>{t("Fully autonomous")}</span>
                </div>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>
      </div>
    </div>
  );
}
