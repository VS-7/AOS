import { FolderOpen } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { isDesktop, system } from "@/lib/client";
import { isDesktopWindow } from "@/lib/wails";
import { t } from "@/lib/i18n";

interface FolderInputProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  inputClassName?: string;
}

export function FolderInput({
  value,
  onChange,
  placeholder,
  disabled,
  inputClassName,
}: FolderInputProps) {
  // `isDesktopWindow` first: `isDesktop()` only turns true once a domain
  // call has succeeded over the bridge, and the first screen that needs to
  // pick a folder — choosing the very first workspace — runs before any
  // has. The window's own query string says which it is, synchronously,
  // from the first paint.
  const native = isDesktopWindow || isDesktop();

  const handleSelectFolder = async () => {
    const picked = await system.pickFiles({ directories: true });
    if (picked.length > 0 && picked[0]) {
      onChange(picked[0]);
    }
  };

  return (
    <div className="flex w-full items-center gap-2">
      <Input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={
          placeholder ?? "e.g. /Users/name/Projects/aos-workspace"
        }
        className={cn("flex-1", inputClassName)}
        disabled={disabled}
      />
      {native && (
        <Button
          type="button"
          variant="secondary"
          size="icon"
          onClick={handleSelectFolder}
          disabled={disabled}
          title={t("Select Folder")}
        >
          <FolderOpen className="size-4" />
        </Button>
      )}
    </div>
  );
}
