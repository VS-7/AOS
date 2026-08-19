import * as React from "react";

import { cn } from "@/lib/utils";
import { SETTINGS_CONTENT_MAX_WIDTH } from "../constants";

interface SettingsSectionShellProps {
  children: React.ReactNode;
  className?: string;
  contentClassName?: string;
}

export function SettingsSectionShell({
  children,
  className,
  contentClassName,
}: SettingsSectionShellProps) {
  return (
    <div className={cn("flex h-full flex-1 flex-col overflow-hidden", className)}>
      <div
        className={cn(
          "mx-auto flex h-full w-full flex-1 flex-col gap-8 px-6 py-6",
          SETTINGS_CONTENT_MAX_WIDTH,
          contentClassName,
        )}
      >
        {children}
      </div>
    </div>
  );
}
