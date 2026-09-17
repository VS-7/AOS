import * as React from "react";
import { LoaderCircle } from "lucide-react";
import { MultiFileDiff } from "@pierre/diffs/react";
import { getFiletypeFromFileName, getHighlighterOptions, preloadHighlighter } from "@pierre/diffs";
import type { FileContents } from "@pierre/diffs/react";
import { aos } from "@/app/aos";
import { serializeExplorerContext } from "@/features/file/presentation/helpers/files-explorer.helper";
import type { FileExplorerContext } from "@/features/file/interfaces/file.interfaces";
import type { ChangesPanelPreferences } from "@/features/file/presentation/helpers/changes.helper";
import { t } from "@/lib/i18n";
import { errorMessage } from "@/lib/aos-facade";

const DIFF_THEME = { dark: "pierre-dark", light: "pierre-light" } as const;

/**
 * Resolves once the shared highlighter has the themes and the grammar a path
 * needs.
 *
 * `MultiFileDiff` renders an empty `<pre>` when it mounts before the
 * highlighter has loaded, and in this window nothing redraws it afterwards:
 * the first diff anybody expanded stayed blank while every later one — the
 * highlighter loaded by then — rendered. Waiting for the load first puts
 * every diff on the path that renders synchronously.
 */
function useHighlighterReady(path: string): boolean {
  const [readyFor, setReadyFor] = React.useState<string | null>(null);
  React.useEffect(() => {
    let alive = true;
    const options = getHighlighterOptions(getFiletypeFromFileName(path), { theme: DIFF_THEME });
    // A grammar that fails to load still leaves plain text to show, so a
    // failure is readiness too.
    void preloadHighlighter(options)
      .catch(() => undefined)
      .finally(() => {
        if (alive) setReadyFor(path);
      });
    return () => {
      alive = false;
    };
  }, [path]);
  return readyFor === path;
}

interface ChangesFileDiffProps {
  path: string;
  explorerContext: FileExplorerContext;
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

  const highlighterReady = useHighlighterReady(path);
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

  if (diffQuery.isLoading || !highlighterReady) {
    return (
      <div className="flex items-center gap-2 px-4 py-6 text-sm text-muted-foreground">
        <LoaderCircle className="size-4 animate-spin" />
        {t("Loading diff…")}
      </div>
    );
  }

  if (diffQuery.isError || !snapshot) {
    return (
      <div className="px-4 py-6 text-sm text-muted-foreground">
        {errorMessage(diffQuery.error) ??
          t("Diff content not available. This file changed, but a text diff could not be rendered.")}
      </div>
    );
  }

  if (isBinary || !oldFile || !newFile) {
    return (
      <div className="px-4 py-6 text-sm text-muted-foreground">
        {t("Diff content not available. This file changed, but a text diff could not be rendered.")}
      </div>
    );
  }

  return (
    <div className="min-h-0 w-full overflow-auto border-t bg-background">
      <MultiFileDiff
        oldFile={oldFile}
        newFile={newFile}
        options={{
          theme: DIFF_THEME,
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
