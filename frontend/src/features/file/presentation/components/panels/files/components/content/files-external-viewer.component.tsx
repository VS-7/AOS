import React from "react";
import {
  ArrowUpRight,
  ExternalLink,
  FileType2,
  ImageIcon,
  PlaySquare,
  Table2,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

import type { WorkspaceFile } from "@/features/file/interfaces/file.interfaces";

interface FilesExternalViewerProps {
  file: WorkspaceFile;
  onOpenExternal: () => void;
}

export function FilesExternalViewer({ file, onOpenExternal }: FilesExternalViewerProps) {
  const icon = getViewerIcon(file.viewer);
  const Icon = icon;

  return (
    <div className="flex h-full items-center justify-center p-6">
      <Card className="w-full max-w-2xl gap-4 border-border/70 bg-card/70">
        <CardHeader>
          <div className="flex items-center gap-3">
            <div className="flex size-12 items-center justify-center rounded-2xl bg-primary/10 text-primary">
              <Icon className="size-5" />
            </div>
            <div className="min-w-0">
              <CardTitle className="truncate">{file.name}</CardTitle>
              <CardDescription>
                This renderer is intentionally external for now while we shape the Files UX.
              </CardDescription>
            </div>
          </div>
        </CardHeader>

        <CardContent className="flex flex-col gap-4">
          <div className="flex flex-wrap gap-2">
            <Badge variant="outline">{file.viewer}</Badge>
            <Badge variant="secondary">{file.extension || "unknown"}</Badge>
            <Badge variant="secondary">Open in browser tab</Badge>
          </div>

          <p className="text-sm leading-relaxed text-muted-foreground">
            The center panel supports in-app text, Markdown, JSON, image, PDF, and video
            viewers. DOCX, Excel, and CSV currently hand off to an external browser tab
            while we keep the app bundle lean.
          </p>

          <div className="rounded-xl border bg-muted/20 p-4">
            <div className="flex items-center gap-2 text-sm font-medium">
              <ExternalLink className="size-4 text-muted-foreground" />
              External handoff
            </div>
            <p className="mt-2 break-all font-mono text-xs leading-5 text-muted-foreground">
              {file.absolutePath}
            </p>
          </div>

          <div className="flex items-center gap-2">
            <Button onClick={onOpenExternal}>
              <ArrowUpRight data-icon="inline-start" className="size-4" />
              Open in browser tab
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function getViewerIcon(viewer: WorkspaceFile["viewer"]) {
  switch (viewer) {
    case "image":
      return ImageIcon;
    case "xlsx":
    case "csv":
      return Table2;
    case "video":
    case "audio":
      return PlaySquare;
    default:
      return FileType2;
  }
}
