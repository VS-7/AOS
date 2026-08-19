"use client";

import * as React from "react";
import { icons } from "lucide-react";
import { ImagePlus, Pencil, X } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Command,
  CommandEmpty,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import { Icon } from "@/components/ui/icon";
import { resizeImageToDataUrl } from "@/components/ui/image-upload";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import { ProjectHelper } from "@/features/project/presentation/helpers/project.helper";

/** Curated icons shown before the user searches. */
const FEATURED_ICONS = [
  "Folder",
  "FolderKanban",
  "Rocket",
  "Code2",
  "Boxes",
  "Globe",
  "Smartphone",
  "Server",
  "Database",
  "Sparkles",
  "Hexagon",
  "Layers",
  "Package",
  "Terminal",
  "Cpu",
  "Cloud",
  "GitBranch",
  "DashboardSquare01Icon",
  "Briefcase",
  "Target",
  "Zap",
  "Puzzle",
  "Box",
  "AppWindow",
] as const;

const MAX_SEARCH_RESULTS = 60;

/** Cheap string catalog — Lucide components resolve on demand via {@link Icon}. */
const ALL_ICON_NAMES = Object.keys(icons)
  .filter((name) => !name.endsWith("Icon"))
  .sort((a, b) => a.localeCompare(b));

type ProjectIconPickerProps = {
  /** Current Lucide name or image data URI / URL. */
  value?: string | null;
  /** Persist the next icon or image value (empty string clears). */
  onChange: (value: string) => void;
  disabled?: boolean;
  className?: string;
};

/**
 * Compact project icon control for the page header.
 *
 * Shows the current preview; hover hints editability; click opens a popover
 * with photo upload + searchable Lucide grid (featured first, lazy on search).
 */
export function ProjectIconPicker({
  value,
  onChange,
  disabled = false,
  className,
}: ProjectIconPickerProps) {
  const [open, setOpen] = React.useState(false);
  const [query, setQuery] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const fileRef = React.useRef<HTMLInputElement>(null);

  const hasValue = Boolean(value?.trim());
  const isImage = ProjectHelper.isImageIcon(value);
  const displayValue = ProjectHelper.getIcon(value ?? undefined);

  const visibleIcons = React.useMemo(() => {
    const term = query.trim().toLowerCase();
    if (!term) return [...FEATURED_ICONS];

    return ALL_ICON_NAMES.filter((name) =>
      name.toLowerCase().includes(term),
    ).slice(0, MAX_SEARCH_RESULTS);
  }, [query]);

  const handlePickPhoto = () => {
    if (disabled || busy) return;
    fileRef.current?.click();
  };

  const handleFile = async (file: File | undefined) => {
    if (!file) return;
    if (!file.type.startsWith("image/")) {
      toast.error("Please choose an image file.");
      return;
    }

    setBusy(true);
    try {
      const dataUrl = await resizeImageToDataUrl(file);
      onChange(dataUrl);
      setOpen(false);
      setQuery("");
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Failed to process image.";
      toast.error(message);
    } finally {
      setBusy(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  };

  const handleSelectIcon = (iconName: string) => {
    onChange(iconName);
    setOpen(false);
    setQuery("");
  };

  const handleClear = () => {
    onChange("");
  };

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) setQuery("");
      }}
    >
      <PopoverTrigger asChild>
        <button
          type="button"
          disabled={disabled}
          aria-label="Change project icon"
          title="Change icon"
          className={cn(
            "group relative flex size-7 shrink-0 items-center justify-center overflow-hidden rounded-full border bg-card/10 outline-none transition-colors",
            "hover:border-foreground/25 hover:bg-accent/40",
            "focus-visible:ring-1 focus-visible:ring-ring",
            "disabled:pointer-events-none disabled:opacity-50",
            className,
          )}
        >
          {isImage ? (
            <img
              src={value!.trim()}
              alt=""
              className="size-full object-cover"
            />
          ) : (
            <Icon
              value={displayValue}
              fallback="Folder"
              className="size-3.5 text-muted-foreground transition-opacity group-hover:opacity-40"
            />
          )}
          <span
            className={cn(
              "pointer-events-none absolute inset-0 flex items-center justify-center rounded-full bg-foreground/45 transition-opacity",
              isImage
                ? "opacity-0 group-hover:opacity-100"
                : "opacity-0 group-hover:opacity-100",
            )}
            aria-hidden
          >
            <Pencil className="size-3 text-background" strokeWidth={2.25} />
          </span>
        </button>
      </PopoverTrigger>

      <PopoverContent
        align="start"
        sideOffset={8}
        className="w-[280px] gap-0 overflow-hidden p-0"
      >
        <div className="border-b px-1 h-9 flex items-center">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-8 w-full justify-start gap-2 px-2"
            disabled={disabled || busy}
            onClick={handlePickPhoto}
          >
            <ImagePlus className="size-3.5 text-muted-foreground" />
            {busy ? "Processing…" : "Choose photo"}
          </Button>
          <input
            ref={fileRef}
            type="file"
            accept="image/png,image/jpeg,image/jpg,image/webp,image/gif"
            className="hidden"
            disabled={disabled || busy}
            onChange={(event) => void handleFile(event.target.files?.[0])}
          />
        </div>

        <Command shouldFilter={false} className="rounded-none border-0">
          <CommandInput
            placeholder="Search icons…"
            value={query}
            onValueChange={setQuery}
            className="!border-b-0"
          />
          <CommandList className="max-h-[220px] p-2 border-t-0">
            <CommandEmpty className="py-6 text-center text-xs">
              No icons found.
            </CommandEmpty>
            <div className="grid grid-cols-6 gap-1">
              {visibleIcons.map((iconName) => {
                const selected = !isImage && displayValue === iconName;

                return (
                  <CommandItem
                    key={iconName}
                    value={iconName}
                    onSelect={() => handleSelectIcon(iconName)}
                    title={iconName}
                    className={cn(
                      "flex size-9 items-center justify-center rounded-md p-0 data-[selected=true]:bg-accent",
                      selected && "bg-accent ring-1 ring-border",
                    )}
                  >
                    <Icon
                      value={iconName}
                      fallback="Folder"
                      className="size-4 text-muted-foreground"
                    />
                    <span className="sr-only">{iconName}</span>
                  </CommandItem>
                );
              })}
            </div>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
