import * as React from "react";
import { ArrowLeft } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";

interface SettingsStackedViewProps {
  open: boolean;
  title: string;
  description?: string;
  onBack: () => void;
  children: React.ReactNode;
  className?: string;
  contentClassName?: string;
}

export function SettingsStackedView({
  open,
  title,
  description,
  onBack,
  children,
  className,
  contentClassName,
}: SettingsStackedViewProps) {
  return (
    <div
      // Closed, the view is only transparent and moved aside, so without
      // `inert` its fields stayed reachable with Tab — a keyboard could land
      // on an invisible "Create task type" and press it.
      inert={!open}
      aria-hidden={!open}
      className={cn(
        "absolute inset-0 z-20 flex flex-col bg-popover transition-all duration-300 ease-out h-full",
        open
          ? "translate-x-0 opacity-100 pointer-events-auto animate-in slide-in-from-right-8 fade-in-0"
          : "translate-x-full opacity-0 pointer-events-none animate-out slide-out-to-right-8 fade-out-0",
        className,
      )}
    >
      <div className="flex items-start gap-3 border-b px-3 py-3">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="shrink-0"
          onClick={onBack}
          aria-label={t("Back")}
        >
          <ArrowLeft className="size-4" />
        </Button>
        {/* The title was accepted and never drawn, so creating a type and
            editing one looked the same. */}
        <div className="min-w-0 space-y-0.5 pt-1.5">
          <h2 className="text-sm font-semibold leading-none tracking-tight">{title}</h2>
          {description ? <p className="text-sm text-muted-foreground">{description}</p> : null}
        </div>
      </div>

      <div className={cn("min-h-0 flex-1 overflow-y-auto", contentClassName)}>
        {children}
      </div>
    </div>
  );
}
