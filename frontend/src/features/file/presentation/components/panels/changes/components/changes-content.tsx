import * as React from "react";
import { LoaderCircle } from "lucide-react";
import { aos } from "@/app/aos";
import { useRealtime } from "@/hooks/use-realtime";
import { AnimatedEmptyState } from "@/components/ui/animated-empty-state";
import type {
  FileChangeEntry,
  FileExplorerContext,
} from "@/features/file/interfaces/file.interfaces";
import {
  explorerContextsEqual,
  serializeExplorerContext,
} from "@/features/file/presentation/helpers/files-explorer.helper";
import {
  DEFAULT_CHANGES_PREFERENCES,
  type ChangesPanelPreferences,
} from "@/features/file/presentation/helpers/changes.helper";
import { ChangesHeader } from "./changes-header";
import { ChangesFileList } from "./changes-file-list";
import { t } from "@/lib/i18n";

interface ChangesContentProps {
  tabId: string;
  explorerContext: FileExplorerContext;
}

export function ChangesContent({
  tabId,
  explorerContext,
}: ChangesContentProps) {
  const [preferences, setPreferences] = React.useState<ChangesPanelPreferences>(
    DEFAULT_CHANGES_PREFERENCES,
  );
  const [expandedPath, setExpandedPath] = React.useState<string | null>(null);
  const [findOpen, setFindOpen] = React.useState(false);
  const [findQuery, setFindQuery] = React.useState("");
  const refetchTimerRef = React.useRef<number | undefined>(undefined);

  const themeMode = aos.stores.theme.useState((state) => state.mode);
  const themeType = React.useMemo<"light" | "dark">(() => {
    if (themeMode === "light" || themeMode === "dark") return themeMode;
    if (typeof window === "undefined") return "light";
    return window.matchMedia("(prefers-color-scheme: dark)").matches
      ? "dark"
      : "light";
  }, [themeMode]);

  const changesQuery = aos.client.file.changes.useQuery({
    query: {
      context: serializeExplorerContext(explorerContext),
    },
  });

  const snapshot = changesQuery.data?.snapshot;
  const files: FileChangeEntry[] = snapshot?.files ?? [];
  const filteredFiles = React.useMemo(() => {
    const query = findQuery.trim().toLowerCase();
    if (!query) return files;
    return files.filter((file) => file.path.toLowerCase().includes(query));
  }, [files, findQuery]);

  useRealtime(
    "files:changed",
    (payload) => {
      if (
        payload.context &&
        !explorerContextsEqual(payload.context, explorerContext)
      ) {
        return;
      }

      window.clearTimeout(refetchTimerRef.current);
      refetchTimerRef.current = window.setTimeout(() => {
        void changesQuery.refetch();
      }, 350);
    },
    [explorerContext, changesQuery],
  );

  React.useEffect(() => {
    return () => window.clearTimeout(refetchTimerRef.current);
  }, []);

  React.useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      const isMod = event.metaKey || event.ctrlKey;
      if (!isMod) return;

      if (event.key.toLowerCase() === "f") {
        event.preventDefault();
        setFindOpen(true);
      }

      if (event.key.toLowerCase() === "r") {
        // Prefer Changes refresh when this tab owns focus.
        const active = document.activeElement;
        const panel = document.getElementById(`changes-panel-${tabId}`);
        if (panel && (panel === active || panel.contains(active))) {
          event.preventDefault();
          event.stopPropagation();
          void changesQuery.refetch();
        }
      }
    }

    window.addEventListener("keydown", handleKeyDown, true);
    return () => window.removeEventListener("keydown", handleKeyDown, true);
  }, [tabId, changesQuery]);

  function handleToggle(path: string) {
    setExpandedPath((current) => (current === path ? null : path));
  }

  const isInitialLoading = changesQuery.isLoading && !snapshot;

  return (
    <div
      id={`changes-panel-${tabId}`}
      tabIndex={-1}
      className="flex h-full min-h-0 flex-col outline-none"
    >
      <ChangesHeader
        explorerContext={explorerContext}
        fileCount={snapshot?.summary.fileCount ?? 0}
        additions={snapshot?.summary.additions ?? 0}
        deletions={snapshot?.summary.deletions ?? 0}
        readOnly={snapshot?.readOnly ?? false}
        isRefreshing={changesQuery.isFetching}
        preferences={preferences}
        findOpen={findOpen}
        findQuery={findQuery}
        onRefresh={() => void changesQuery.refetch()}
        onPreferencesChange={(next) =>
          setPreferences((current) => ({ ...current, ...next }))
        }
        onFindOpenChange={setFindOpen}
        onFindQueryChange={setFindQuery}
      />

      {isInitialLoading ? (
        <div className="flex flex-1 items-center justify-center gap-2 text-sm text-muted-foreground">
          <LoaderCircle className="size-4 animate-spin" />
          {t("Loading changes…")}
        </div>
      ) : changesQuery.isError ? (
        <AnimatedEmptyState className="border-none shadow-none">
          <AnimatedEmptyState.Content>
            <AnimatedEmptyState.Title>
              {t("Unable to load changes")}
            </AnimatedEmptyState.Title>
            <AnimatedEmptyState.Description>
              {(changesQuery.error as { message?: string })?.message ||
                "The Changes panel could not load git status for this context."}
            </AnimatedEmptyState.Description>
          </AnimatedEmptyState.Content>
        </AnimatedEmptyState>
      ) : filteredFiles.length === 0 ? (
        <AnimatedEmptyState className="border-none shadow-none">
          <AnimatedEmptyState.Content>
            <AnimatedEmptyState.Title>
              {findQuery.trim() ? "No matching changes" : "No changes"}
            </AnimatedEmptyState.Title>
            <AnimatedEmptyState.Description>
              {findQuery.trim()
                ? "Try a different filter."
                : "This context has a clean working tree."}
            </AnimatedEmptyState.Description>
          </AnimatedEmptyState.Content>
        </AnimatedEmptyState>
      ) : (
        <ChangesFileList
          files={filteredFiles}
          explorerContext={explorerContext}
          preferences={preferences}
          themeType={themeType}
          expandedPath={expandedPath}
          onToggle={handleToggle}
        />
      )}
    </div>
  );
}
