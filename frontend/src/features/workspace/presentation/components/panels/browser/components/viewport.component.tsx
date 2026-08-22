import * as React from "react";
import { PageBody } from "@/components/ui/page";
import { ShieldAlert } from "lucide-react";
import { BrowserRenderer } from "./renderer.component";
import { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";

interface BrowserViewportProps {
  tabs: ViewportTabState[];
  activeId: string | null;
  activeTab: ViewportTabState | null;
  onUpdateTab: (tabId: string, patch: Partial<ViewportTabState>) => void;
}

export function BrowserViewport({
  tabs,
  activeId,
  activeTab,
  onUpdateTab,
}: BrowserViewportProps) {
  return (
    <PageBody className="min-h-0 overflow-hidden bg-transparent p-0 rounded-b-md">
      <div className="relative flex-1 overflow-hidden">
        {tabs.map((tab) => (
          <BrowserRenderer
            key={tab.id}
            tab={tab}
            active={tab.id === activeId}
            onStateChange={onUpdateTab}
          />
        ))}

        {activeTab?.error && (
          <div className="pointer-events-none absolute inset-x-0 top-4 flex justify-center px-4">
            <div className="flex items-center gap-2 rounded-md border border-destructive/30 bg-background/90 px-4 py-2 text-sm text-destructive shadow-lg backdrop-blur">
              <ShieldAlert className="size-4" />
              <span className="truncate">{activeTab.error}</span>
            </div>
          </div>
        )}
      </div>
    </PageBody>
  );
}
