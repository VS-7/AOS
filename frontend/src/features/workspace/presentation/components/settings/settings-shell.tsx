"use client";

import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/lib/utils";
import {
  SETTINGS_SECTION_MAP,
  type SettingsSectionId,
} from "./constants";
import { SETTINGS_SECTION_COMPONENTS } from "./section-components";
import { SettingsRouteHelper } from "@/features/workspace/presentation/helpers/settings-route.helper";

export interface SettingsShellProps {
  sectionId: SettingsSectionId;
}

/**
 * Renders the active settings section with shared header chrome.
 */
export function SettingsShell({ sectionId }: SettingsShellProps) {
  const section = SETTINGS_SECTION_MAP[sectionId];
  const ActiveSectionComponent = SETTINGS_SECTION_COMPONENTS[sectionId];
  const isFullBleed = SettingsRouteHelper.isFullBleed(sectionId);

  return (
    <div className="flex min-h-0 h-full flex-1 flex-col overflow-hidden">
      <header className="flex shrink-0 items-center justify-between gap-4 px-6 py-4">
        <div className="min-w-0 flex items-center gap-2">
          <h2 className="truncate font-semibold text-sm">{section.title}</h2>
          <p className="truncate text-muted-foreground text-s">{section.description}</p>
        </div>
      </header>
      <div
        className={cn(
          "flex min-h-0 flex-1 overflow-hidden",
        )}
      >
        {isFullBleed ? (
          <ActiveSectionComponent />
        ) : (
          <ScrollArea className="min-h-0 flex-1">
            <ActiveSectionComponent />
          </ScrollArea>
        )}
      </div>
    </div>
  );
}
