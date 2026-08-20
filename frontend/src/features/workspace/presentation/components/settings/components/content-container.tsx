import * as React from "react";

import { cn } from "@/lib/utils";
import { SETTINGS_CONTENT_MAX_WIDTH } from "../constants";

interface SettingsContentContainerProps {
  children: React.ReactNode;
  className?: string;
}

export function SettingsContentContainer({
  children,
  className,
}: SettingsContentContainerProps) {
  return (
    <div
      className={cn(
        "mx-auto w-full px-6 py-6",
        SETTINGS_CONTENT_MAX_WIDTH,
        className,
      )}
    >
      {children}
    </div>
  );
}
