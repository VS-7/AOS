import { ExternalLink, FileCog, MoreHorizontal } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { TaskAttachment } from "@/features/task/interfaces/comment.interfaces";
import { Badge } from "@/components/ui/badge";
import { aos } from "@/app/aos";
import { cn, getDisplayName, getIconForExtension } from "@/lib/utils";

const TEXT_FILE_EXTENSIONS = new Set([
  "txt",
  "md",
  "mdx",
  "json",
  "yaml",
  "yml",
  "toml",
  "xml",
  "csv",
  "js",
  "jsx",
  "ts",
  "tsx",
  "css",
  "scss",
  "html",
  "sh",
  "zsh",
  "env",
  "log",
]);

const IMAGE_FILE_EXTENSIONS = new Set(["jpg", "jpeg", "png", "gif", "svg", "webp"]);
const BROWSER_FILE_EXTENSIONS = new Set(["pdf", "mp4", "webm", "mov", ...IMAGE_FILE_EXTENSIONS]);

function normalizeAttachmentUrl(uri: string) {
  if (uri.startsWith("uri://")) {
    return uri.replace(/^uri:\/\//, "");
  }

  if (uri.startsWith("file://")) {
    return uri;
  }

  if (uri.startsWith("/")) {
    return `file://${uri}`;
  }

  return uri;
}

function getAttachmentPath(uri: string) {
  if (uri.startsWith("/")) {
    return uri;
  }

  if (!uri.startsWith("file://")) {
    return null;
  }

  return decodeURIComponent(new URL(uri).pathname);
}

function getAttachmentExtension(uri: string) {
  const normalizedUrl = normalizeAttachmentUrl(uri);

  try {
    const pathname = normalizedUrl.startsWith("file://")
      ? new URL(normalizedUrl).pathname
      : new URL(normalizedUrl).pathname;
    const filename = pathname.split("/").pop() || "";

    return filename.split(".").pop()?.toLowerCase() || "";
  } catch {
    return normalizedUrl.split(".").pop()?.toLowerCase() || "";
  }
}

export function AttachmentItem({ attachment }: { attachment: TaskAttachment }) {
  const displayName = getDisplayName(attachment.uri, attachment.observation);
  const IconComponent = getIconForExtension(attachment.uri);
  const extension = getAttachmentExtension(attachment.uri);
  const normalizedUrl = normalizeAttachmentUrl(attachment.uri);
  const localPath = getAttachmentPath(attachment.uri);
  const isImage = IMAGE_FILE_EXTENSIONS.has(extension);
  const isTextFile = TEXT_FILE_EXTENSIONS.has(extension);
  const opensInBrowser = attachment.uri.startsWith("uri://") || BROWSER_FILE_EXTENSIONS.has(extension);

  async function handleOpen() {
    if (isTextFile && localPath && window.aos?.instructions?.openPath) {
      await window.aos.instructions.openPath(localPath);
      return;
    }

    aos.stores.viewport.actions.createTab({
      title: displayName,
      url: normalizedUrl,
      type: "browser",
    });
  }

  return (
    <div className="group flex items-center gap-3 h-12 px-4 transition-colors hover:bg-accent/30" onClick={() => void handleOpen()}>
      {isImage ? (
        <img
          src={normalizedUrl}
          alt={displayName}
          className="size-full object-cover"
        />
      ) : (
        <IconComponent className="size-3 text-muted-foreground transition-colors group-hover:text-primary" />
      )}

      <div className="flex min-w-0 flex-1 items-center gap-1 truncate text-left text-sm font-medium text-foreground hover:underline">
        {displayName}
      </div>

      <div className="flex items-center max-w-xs">
        <Button variant="ghost" size="icon" className={cn("size-8 rounded-md")} type="button" disabled>
          <MoreHorizontal className="size-4" />
          <span className="sr-only">More attachment actions</span>
        </Button>
      </div>
    </div>
  );
}
