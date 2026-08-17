import { useState } from "react";
import type { JSX } from "react";
import { FileTree } from "./FileTree";
import { MonacoViewer } from "./MonacoViewer";
import { isNotYetAvailableInDesktop } from "@/lib/file";

/**
 * The files panel: a workspace tree on the left, Monaco on the right.
 *
 * HTTP only for now — see lib/file.ts's package doc. Inside the desktop
 * window this shows a plain notice rather than a confusing fetch failure,
 * the same gap SystemService closed for the platform calls; a wailsvc
 * FileService that proxies to the daemon is the equivalent fix still ahead
 * for this one.
 */
export function FilesPage(): JSX.Element {
  const [selected, setSelected] = useState<string | null>(null);

  if (isNotYetAvailableInDesktop()) {
    return (
      <div className="flex h-full items-center justify-center p-8 text-center text-sm text-muted-foreground">
        The file explorer isn't wired up for the desktop window yet — open this workspace in a
        browser to browse and edit files for now.
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 overflow-hidden rounded-md border">
      <aside className="w-64 shrink-0 overflow-y-auto border-r">
        <FileTree selected={selected} onSelect={setSelected} />
      </aside>
      <div className="min-w-0 flex-1">
        {selected ? (
          <MonacoViewer path={selected} />
        ) : (
          <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
            Select a file to open it.
          </div>
        )}
      </div>
    </div>
  );
}
