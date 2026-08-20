import * as React from "react";
import { Button } from "@/components/ui/button";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
  InputGroupText,
} from "@/components/ui/input-group";
import { cn } from "@/lib/utils";
import {
  Compass,
  Globe,
  RefreshCw,
  Search,
} from "lucide-react";
import { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";

interface BrowserToolbarProps {
  activeTab: ViewportTabState | null;
  addressBarValue: string;
  addressBarRef: React.RefObject<HTMLInputElement | null>;
  onNavigate: () => void;
  onReload: () => void;
  onAddressBarChange: (value: string) => void;
  onAddressBarFocus: (focused: boolean) => void;
  onGoHome: () => void;
}

export function BrowserToolbar({
  activeTab,
  addressBarValue,
  addressBarRef,
  onNavigate,
  onReload,
  onAddressBarChange,
  onAddressBarFocus,
  onGoHome,
}: BrowserToolbarProps) {
  return (
    <div className="flex items-center w-full gap-2">
      <Globe className="size-3" />

      <InputGroup className="h-10 flex-1 rounded-md px-0 border-transparent bg-transparent shadow-none has-[[data-slot=input-group-control]:focus-visible]:bg-transparent">
        <InputGroupInput
          ref={addressBarRef}
          value={addressBarValue}
          onChange={(event) => onAddressBarChange(event.target.value)}
          onFocus={() => onAddressBarFocus(true)}
          onBlur={() => onAddressBarFocus(false)}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              onNavigate();
            }
          }}
          placeholder="Search the web or enter a URL"
        />
      </InputGroup>

      <Button
        size="icon-sm"
        variant="ghost"
        disabled={!activeTab}
        onClick={onReload}
      >
        <RefreshCw className={cn("size-3", activeTab?.status === 'loading' && "animate-spin")} />
      </Button>
    </div>
  );
}
