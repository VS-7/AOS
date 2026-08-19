import * as React from "react";
import { AnimatedEmptyState } from "@/components/ui/animated-empty-state";
import { PageBody } from "@/components/ui/page";
import { ShieldAlert } from "lucide-react";
import { BrowserRenderer, type BrowserWebViewElement } from "./renderer.component";
import { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";

interface BrowserViewportProps {
  isNative: boolean;
  tabs: ViewportTabState[];
  activeId: string | null;
  activeTab: ViewportTabState | null;
  onUpdateTab: (tabId: string, patch: Partial<ViewportTabState>) => void;
  onSetWebviewRef: (tabId: string, node: BrowserWebViewElement | null) => void;
}

export function BrowserViewport({
  isNative,
  tabs,
  activeId,
  activeTab,
  onUpdateTab,
  onSetWebviewRef,
}: BrowserViewportProps) {
  return (
    <PageBody className="min-h-0 overflow-hidden bg-transparent p-0 rounded-b-md">
      {!isNative && (
        <div className="flex h-full items-center justify-center p-6">
          <AnimatedEmptyState className="w-full max-w-xl shadow-none">
            <AnimatedEmptyState.Pill variant="secondary">Desktop only</AnimatedEmptyState.Pill>
            <AnimatedEmptyState.Content>
              <AnimatedEmptyState.Title>Browser is available in the Electron app</AnimatedEmptyState.Title>
              <AnimatedEmptyState.Description>
                The Browser workspace needs the native Fractal bridge to render secure web content.
              </AnimatedEmptyState.Description>
            </AnimatedEmptyState.Content>
          </AnimatedEmptyState>
        </div>
      )}

      {isNative && (
        <div className="relative flex-1 overflow-hidden">
          {tabs.map((tab) => (
            <BrowserRenderer
              key={tab.id}
              tab={tab}
              active={tab.id === activeId}
              onStateChange={onUpdateTab}
              onRefChange={onSetWebviewRef}
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
      )}
    </PageBody>
  );
}
