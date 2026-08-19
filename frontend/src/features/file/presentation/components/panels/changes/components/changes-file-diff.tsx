import * as React from "react";
import { LoaderCircle } from "lucide-react";
import { MultiFileDiff } from "@pierre/diffs/react";
import type { FileContents } from "@pierre/diffs/react";
import { aos } from "@/app/aos";
import { serializeExplorerContext } from "@/features/file/presentation/helpers/files-explorer.helper";
import type { FractalFileExplorerContext } from "@/features/file/interfaces/file.interfaces";
import type { ChangesPanelPreferences } from "@/features/file/presentation/helpers/changes.helper";

interface ChangesFileDiffProps {
  path: string;
  explorerContext: FractalFileExplorerContext;
  preferences: ChangesPanelPreferences;
  themeType: "light" | "dark";
}

export function ChangesFileDiff({
  path,
  explorerContext,
  preferences,
  themeType,
}: ChangesFileDiffProps) {
  const diffQuery = aos.client.file.diff.useQuery({
    query: {
      path,
      context: serializeExplorerContext(explorerContext),
    },
  });

  const snapshot = diffQuery.data?.snapshot;
  const isBinary = Boolean(snapshot?.isBinary);

  const oldFile = React.useMemo<FileContents | null>(() => {
    if (!snapshot || isBinary) return null;
    if (snapshot.oldFile) {
      return {
        name: snapshot.oldFile.name,
        contents: snapshot.oldFile.contents,
        cacheKey: `old:${path}:${snapshot.oldFile.contents.length}`,
      };
    }
    return {
      name: path.split("/").pop() || path,
      contents: "",
      cacheKey: `old-empty:${path}`,
    };
  }, [snapshot, isBinary, path]);

  const newFile = React.useMemo<FileContents | null>(() => {
    if (!snapshot || isBinary) return null;
    if (snapshot.newFile) {
      return {
        name: snapshot.newFile.name,
        contents: snapshot.newFile.contents,
        cacheKey: `new:${path}:${snapshot.newFile.contents.length}`,
      };
    }
    return {
      name: path.split("/").pop() || path,
      contents: "",
      cacheKey: `new-empty:${path}`,
    };
  }, [snapshot, isBinary, path]);

  if (diffQuery.isLoading) {
    return (
      <div className="flex items-center gap-2 px-4 py-6 text-sm text-muted-foreground">
        <LoaderCircle className="size-4 animate-spin" />
        Loading diff…
      </div>
    );
  }

  if (diffQuery.isError || !snapshot) {
    return (
      <div className="px-4 py-6 text-sm text-muted-foreground">
        Diff content not available. This file changed, but a text diff could not
        be rendered.
      </div>
    );
  }

  if (isBinary || !oldFile || !newFile) {
    return (
      <div className="px-4 py-6 text-sm text-muted-foreground">
        Diff content not available. This file changed, but a text diff could not
        be rendered.
      </div>
    );
  }

  return (
    <div className="min-h-0 w-full overflow-auto border-t bg-background">
      <MultiFileDiff
        oldFile={oldFile}
        newFile={newFile}
        options={{
          theme: { dark: "pierre-dark", light: "pierre-light" },
          themeType,
          diffStyle: preferences.diffStyle,
          overflow: preferences.wordWrap ? "wrap" : "scroll",
          disableFileHeader: true,
          stickyHeader: false,
          parseDiffOptions: preferences.ignoreWhitespace
            ? { ignoreWhitespace: true }
            : undefined,
        }}
      />
    </div>
  );
}
