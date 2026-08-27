import { useRef, useState } from "react";
import { ImagePlus, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { ButtonGroup } from "./button-group";
import { t } from "@/lib/i18n";

const DEFAULT_MAX_EDGE_PX = 512;
const DEFAULT_MAX_BYTES = 400_000;
const ACCEPT = "image/png,image/jpeg,image/jpg,image/webp,image/gif";

type ImageUploadProps = {
  /** Current image URL or data URI. */
  value?: string | null;
  /** Called with a data URI after a successful pick+resize. */
  onChange: (value: string) => void;
  /** Clears the image. */
  onRemove?: () => void;
  /** Disables pick/remove controls. */
  disabled?: boolean;
  /** Fallback initials or short label inside the empty avatar. */
  fallback?: string;
  /** Extra classes on the root row. */
  className?: string;
  /** Longest edge after resize (default 512). */
  maxEdgePx?: number;
  /** Max encoded payload size in bytes (default ~400KB). */
  maxBytes?: number;
};

/**
 * Compact avatar/logo picker that resizes the file client-side and emits a
 * data URI — no cloud upload required.
 */
export function ImageUpload({
  value,
  onChange,
  onRemove,
  disabled = false,
  fallback = "?",
  className,
  maxEdgePx = DEFAULT_MAX_EDGE_PX,
  maxBytes = DEFAULT_MAX_BYTES,
}: ImageUploadProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);

  const handlePick = () => {
    if (disabled || busy) return;
    inputRef.current?.click();
  };

  const handleFile = async (file: File | undefined) => {
    if (!file) return;

    if (!file.type.startsWith("image/")) {
      toast.error(t("Please choose an image file."));
      return;
    }

    setBusy(true);
    try {
      const dataUrl = await resizeImageToDataUrl(file, maxEdgePx, maxBytes);
      onChange(dataUrl);
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Failed to process image.";
      toast.error(message);
    } finally {
      setBusy(false);
      if (inputRef.current) inputRef.current.value = "";
    }
  };

  return (
    <div className={cn("flex items-center gap-3", className)}>
      <button
        type="button"
        disabled={disabled || busy}
        onClick={handlePick}
        className="rounded-md outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:opacity-50"
        aria-label={t("Change image")}
      >
        <Avatar className="size-7">
          {value ? <AvatarImage src={value} alt="" /> : null}
          <AvatarFallback>{fallback.slice(0, 2).toUpperCase()}</AvatarFallback>
        </Avatar>
      </button>

      <ButtonGroup>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={disabled || busy}
          onClick={handlePick}
          data-icon="inline-start"
        >
          <ImagePlus />
          {busy ? "Processing…" : value ? "Change" : "Upload"}
        </Button>

        {value && onRemove ? (
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            disabled={disabled || busy}
            onClick={onRemove}
            aria-label={t("Remove image")}
          >
            <Trash2 />
          </Button>
        ) : null}
      </ButtonGroup>

      <input
        ref={inputRef}
        type="file"
        accept={ACCEPT}
        className="hidden"
        disabled={disabled || busy}
        onChange={(event) => void handleFile(event.target.files?.[0])}
      />
    </div>
  );
}

/**
 * Resizes an image file to fit within `maxEdgePx` and encodes it as a data URI
 * under `maxBytes`. Prefers WebP, falls back to JPEG, then PNG.
 */
export async function resizeImageToDataUrl(
  file: File,
  maxEdgePx: number = DEFAULT_MAX_EDGE_PX,
  maxBytes: number = DEFAULT_MAX_BYTES,
): Promise<string> {
  const bitmap = await createImageBitmap(file);
  const scale = Math.min(1, maxEdgePx / Math.max(bitmap.width, bitmap.height));
  const width = Math.max(1, Math.round(bitmap.width * scale));
  const height = Math.max(1, Math.round(bitmap.height * scale));

  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;

  const context = canvas.getContext("2d");
  if (!context) {
    bitmap.close();
    throw new Error("Could not process image.");
  }

  context.drawImage(bitmap, 0, 0, width, height);
  bitmap.close();

  const keepAlpha = file.type === "image/png" || file.type === "image/webp";
  const candidates: Array<{ mime: string; quality?: number }> = keepAlpha
    ? [
        { mime: "image/webp", quality: 0.85 },
        { mime: "image/png" },
      ]
    : [
        { mime: "image/webp", quality: 0.85 },
        { mime: "image/jpeg", quality: 0.85 },
        { mime: "image/jpeg", quality: 0.7 },
      ];

  for (const candidate of candidates) {
    const dataUrl = canvas.toDataURL(candidate.mime, candidate.quality);
    if (estimateDataUrlBytes(dataUrl) <= maxBytes) {
      return dataUrl;
    }
  }

  throw new Error(
    "Image is too large after compression. Try a smaller file (under ~400KB).",
  );
}

function estimateDataUrlBytes(dataUrl: string): number {
  const base64 = dataUrl.split(",")[1] ?? "";
  return Math.ceil((base64.length * 3) / 4);
}
