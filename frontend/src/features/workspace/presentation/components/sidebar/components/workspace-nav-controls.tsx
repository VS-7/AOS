import * as React from "react";
import { ArrowLeft01Icon, ArrowRight01Icon } from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";
import { Button } from "@/components/ui/button";
import { Kbd, KbdGroup } from "@/components/ui/kbd";
import { SidebarTrigger } from "@/components/ui/sidebar";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useIsMobile } from "@/hooks/use-mobile";
import { aos } from "@/app/aos";
import { useNavigationState } from "@/features/workspace/presentation/hooks/use-navigation-state";
import { WorkspaceNavOrientationControls } from "./nav-orientation-controls";

export function WorkspaceNavControls() {
  const isMobile = useIsMobile();
  const state = useNavigationState();
  const tabs = aos.stores.viewport.useState((s) => s.tabs.items);
  const activeTabId = aos.stores.viewport.useState((s) => s.tabs.current);
  const activeTab = tabs.find((t) => t.id === activeTabId);
  const canGoBack =
    activeTab?.type === "browser" ? activeTab.canGoBack : state.canGoBack;
  const canGoForward =
    activeTab?.type === "browser" ? activeTab.canGoForward : state.canGoForward;

  const goBack = React.useCallback(() => {
    (aos.triggers as { dispatch: (id: string, input?: unknown) => Promise<unknown> }).dispatch("tabs.back");
  }, []);

  const goForward = React.useCallback(() => {
    (aos.triggers as { dispatch: (id: string, input?: unknown) => Promise<unknown> }).dispatch("tabs.forward");
  }, []);

  aos.triggers.use({
    trigger: "tabs.back",
    enabled: canGoBack,
    onPressKey: goBack,
  });

  aos.triggers.use({
    trigger: "tabs.forward",
    enabled: canGoForward,
    onPressKey: goForward,
  });

  return (
    <div className="flex shrink-0 items-center gap-0.5">
      <Tooltip>
        <TooltipTrigger asChild>
          <SidebarTrigger className="h-8 w-8 text-muted-foreground hover:text-foreground [&_svg]:size-4" />
        </TooltipTrigger>
        <TooltipContent
          side="bottom"
          align="start"
          className="flex items-center gap-2"
        >
          Toggle Sidebar
          <KbdGroup>
            <Kbd>⌘</Kbd>
            <Kbd>B</Kbd>
          </KbdGroup>
        </TooltipContent>
      </Tooltip>

      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8 text-muted-foreground hover:text-foreground disabled:opacity-20"
            onClick={goBack}
            disabled={!canGoBack}
          >
            <HugeiconsIcon icon={ArrowLeft01Icon} className="size-3" />
          </Button>
        </TooltipTrigger>
        <TooltipContent
          side="bottom"
          align="start"
          className="flex items-center gap-2"
        >
          Go Back
          <KbdGroup>
            <Kbd>⌘</Kbd>
            <Kbd>←</Kbd>
          </KbdGroup>
        </TooltipContent>
      </Tooltip>

      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8 text-muted-foreground hover:text-foreground disabled:opacity-20"
            onClick={goForward}
            disabled={!canGoForward}
          >
            <HugeiconsIcon icon={ArrowRight01Icon} className="size-3" />
          </Button>
        </TooltipTrigger>
        <TooltipContent
          side="bottom"
          align="start"
          className="flex items-center gap-2"
        >
          Go Forward
          <KbdGroup>
            <Kbd>⌘</Kbd>
            <Kbd>→</Kbd>
          </KbdGroup>
        </TooltipContent>
      </Tooltip>

      {!isMobile ? <WorkspaceNavOrientationControls /> : null}
    </div>
  );
}
