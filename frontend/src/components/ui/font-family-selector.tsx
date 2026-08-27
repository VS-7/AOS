"use client";

import * as React from "react";
import { CheckIcon, ChevronsUpDownIcon } from "lucide-react";

import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";

import { SYSTEM_FONT_VALUE } from "@/features/theme/presentation/helpers/font-settings.helper";
import { t } from "@/lib/i18n";

export { SYSTEM_FONT_VALUE };
export const SYSTEM_FONT_COMMAND_VALUE = "system-font-option";

const FALLBACK_FONTS = [
  "Academy Engraved LET",
  "American Typewriter",
  "Andale Mono",
  "Apple Chancery",
  "Arial",
  "Avenir",
  "Avenir Next",
  "Courier New",
  "Futura",
  "Georgia",
  "Helvetica Neue",
  "Inter",
  "IoskeleyMono",
  "JetBrains Mono",
  "Menlo",
  "Monaco",
  "New York",
  "Roboto",
  "SF Mono",
  "SF Pro",
  "Times New Roman",
  "Trebuchet MS",
  "Verdana",
];

async function loadAvailableFonts(): Promise<string[]> {
  if (typeof window !== "undefined" && "queryLocalFonts" in window) {
    try {
      const fonts = await (
        window as Window & {
          queryLocalFonts: () => Promise<Array<{ family: string }>>;
        }
      ).queryLocalFonts();

      const families = [...new Set(fonts.map((font) => font.family))].sort(
        (left, right) => left.localeCompare(right),
      );

      if (families.length > 0) {
        return families;
      }
    } catch {
      // Fall back to the curated list when permission is denied.
    }
  }

  return [...FALLBACK_FONTS];
}

export type FontFamilySelectorProps = {
  value?: string;
  onValueChange?: (value: string) => void;
  placeholder?: string;
  systemLabel?: string;
  searchPlaceholder?: string;
  className?: string;
  disabled?: boolean;
  mono?: boolean;
};

export function FontFamilySelector({
  value,
  onValueChange,
  placeholder = "Select a font",
  systemLabel = "System font",
  searchPlaceholder = "Search fonts",
  className,
  disabled = false,
  mono = false,
}: FontFamilySelectorProps) {
  const [open, setOpen] = React.useState(false);
  const [fonts, setFonts] = React.useState<string[]>(FALLBACK_FONTS);
  const selectedValue = value ?? SYSTEM_FONT_VALUE;
  const isSystemFont =
    selectedValue === SYSTEM_FONT_VALUE || selectedValue.length === 0;
  const displayLabel = isSystemFont ? systemLabel : selectedValue || placeholder;

  React.useEffect(() => {
    let active = true;

    void loadAvailableFonts().then((nextFonts) => {
      if (active) {
        setFonts(nextFonts);
      }
    });

    return () => {
      active = false;
    };
  }, []);

  const previewFamily = isSystemFont
    ? mono
      ? "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
      : "system-ui, sans-serif"
    : `"${selectedValue}", ${mono ? "ui-monospace, monospace" : "system-ui, sans-serif"}`;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          disabled={disabled}
          className={cn(
            "flex h-8 w-48 items-center justify-between gap-2 rounded-md border border-input bg-transparent px-2 text-sm shadow-none outline-none transition-[color,box-shadow] focus-visible:border-ring focus-visible:ring-1 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50",
            className,
          )}
        >
          <span
            className="truncate text-left"
            style={{ fontFamily: previewFamily }}
          >
            {displayLabel}
          </span>
          <ChevronsUpDownIcon className="size-3.5 shrink-0 text-muted-foreground" />
        </button>
      </PopoverTrigger>
      <PopoverContent
        align="end"
        className="w-[var(--radix-popover-trigger-width)] min-w-56 p-0"
      >
        <Command>
          <CommandInput placeholder={searchPlaceholder} />
          <CommandList className="max-h-72">
            <CommandEmpty>{t("No fonts found.")}</CommandEmpty>
            <CommandGroup>
              <CommandItem
                value={SYSTEM_FONT_COMMAND_VALUE}
                keywords={[systemLabel, "system", "default", "monospace"]}
                onSelect={() => {
                  onValueChange?.(SYSTEM_FONT_VALUE);
                  setOpen(false);
                }}
              >
                <span
                  className="flex-1 truncate"
                  style={{
                    fontFamily: mono
                      ? "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
                      : "system-ui, sans-serif",
                  }}
                >
                  {systemLabel}
                </span>
                <CheckIcon
                  className={cn(
                    "size-4 text-primary",
                    isSystemFont ? "opacity-100" : "opacity-0",
                  )}
                />
              </CommandItem>
            </CommandGroup>
            <CommandSeparator />
            <CommandGroup heading={t("All fonts")}>
              {fonts.map((font) => {
                const isSelected = font === selectedValue;

                return (
                  <CommandItem
                    key={font}
                    value={font}
                    onSelect={() => {
                      onValueChange?.(font);
                      setOpen(false);
                    }}
                  >
                    <span
                      className="flex-1 truncate"
                      style={{
                        fontFamily: `"${font}", ${mono ? "ui-monospace, monospace" : "system-ui, sans-serif"}`,
                      }}
                    >
                      {font}
                    </span>
                    <CheckIcon
                      className={cn(
                        "size-4 text-primary",
                        isSelected ? "opacity-100" : "opacity-0",
                      )}
                    />
                  </CommandItem>
                );
              })}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
