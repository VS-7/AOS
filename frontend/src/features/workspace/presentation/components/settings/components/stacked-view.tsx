import * as React from "react";
import { ArrowLeft } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

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
      className={cn(
        "absolute inset-0 z-20 flex flex-col bg-popover transition-all duration-300 ease-out h-full",
        open
          ? "translate-x-0 opacity-100 pointer-events-auto animate-in slide-in-from-right-8 fade-in-0"
          : "translate-x-full opacity-0 pointer-events-none animate-out slide-out-to-right-8 fade-out-0",
        className,
      )}
    >
      <div className="flex items-start gap-4 border-b px-3">
        <Button
          type="button"
          variant="ghost"
          className="mt-0.5 shrink-0"
          onClick={onBack}
        >
          <ArrowLeft className="size-4" />
          Back
        </Button>
      </div>

      <div className={cn("min-h-0 flex-1 overflow-y-auto", contentClassName)}>
        {children}
      </div>
    </div>
  );
}
