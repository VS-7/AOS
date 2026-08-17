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
import {
  THEME_PREVIEW_SWATCH_SIZE,
  ThemePreviewSwatch,
  type ThemePreviewColors,
} from "@/components/ui/theme-preview-swatch";

export const DEFAULT_THEME_ID = "aos";
export const DEFAULT_THEME_COMMAND_VALUE = "default-theme-option";

export type ThemeSelectorOption = {
  id: string;
  name: string;
  preview?: ThemePreviewColors | null;
};

export type ThemeSelectorProps = {
  value?: string;
  onValueChange?: (value: string) => void;
  themes: ThemeSelectorOption[];
  defaultThemeId?: string;
  placeholder?: string;
  searchPlaceholder?: string;
  className?: string;
  disabled?: boolean;
};

export function ThemeSelector({
  value,
  onValueChange,
  themes,
  defaultThemeId = DEFAULT_THEME_ID,
  placeholder = "Select theme",
  searchPlaceholder = "Search themes",
  className,
  disabled = false,
}: ThemeSelectorProps) {
  const [open, setOpen] = React.useState(false);

  const themeById = React.useMemo(
    () => new Map(themes.map((theme) => [theme.id, theme])),
    [themes],
  );

  const defaultTheme = themeById.get(defaultThemeId);
  const otherThemes = React.useMemo(
    () =>
      themes
        .filter((theme) => theme.id !== defaultThemeId)
        .sort((left, right) => left.name.localeCompare(right.name)),
    [defaultThemeId, themes],
  );

  const selectedTheme = value ? themeById.get(value) : undefined;
  const displayLabel = selectedTheme?.name ?? placeholder;

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
          <span className="flex min-w-0 items-center gap-2 truncate text-left">
            {selectedTheme?.preview ? (
              <ThemePreviewSwatch
                colors={selectedTheme.preview}
                size={THEME_PREVIEW_SWATCH_SIZE}
              />
            ) : null}
            <span className="truncate">{displayLabel}</span>
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
            <CommandEmpty>No themes found.</CommandEmpty>
            {defaultTheme ? (
              <CommandGroup>
                <CommandItem
                  value={DEFAULT_THEME_COMMAND_VALUE}
                  keywords={[
                    defaultTheme.name,
                    "default",
                    "aos",
                    "system",
                  ]}
                  onSelect={() => {
                    onValueChange?.(defaultTheme.id);
                    setOpen(false);
                  }}
                >
                  <span className="flex min-w-0 flex-1 items-center gap-2 truncate">
                    {defaultTheme.preview ? (
                      <ThemePreviewSwatch
                        colors={defaultTheme.preview}
                        size={THEME_PREVIEW_SWATCH_SIZE}
                      />
                    ) : null}
                    {defaultTheme.name}
                  </span>
                  <CheckIcon
                    className={cn(
                      "size-4 text-primary",
                      value === defaultTheme.id ? "opacity-100" : "opacity-0",
                    )}
                  />
                </CommandItem>
              </CommandGroup>
            ) : null}
            {otherThemes.length > 0 ? (
              <>
                <CommandSeparator />
                <CommandGroup heading="All themes">
                  {otherThemes.map((theme) => {
                    const isSelected = theme.id === value;

                    return (
                      <CommandItem
                        key={theme.id}
                        value={theme.id}
                        keywords={[theme.name]}
                        onSelect={() => {
                          onValueChange?.(theme.id);
                          setOpen(false);
                        }}
                      >
                        <span className="flex min-w-0 flex-1 items-center gap-2 truncate">
                          {theme.preview ? (
                            <ThemePreviewSwatch
                              colors={theme.preview}
                              size={THEME_PREVIEW_SWATCH_SIZE}
                            />
                          ) : null}
                          {theme.name}
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
              </>
            ) : null}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
