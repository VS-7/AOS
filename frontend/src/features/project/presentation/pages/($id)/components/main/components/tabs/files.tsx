import * as React from "react";
import {
  ArrowLeft,
  Folder,
  RefreshCw,
  Search,
} from "lucide-react";
import { motion } from "motion/react";
import { AnimatedEmptyState } from "@/components/ui/animated-empty-state";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { useDelayedLoading } from "@/hooks/use-delayed-loading.hook";
import { aos } from "@/app/aos";
import type { WorkspaceFile } from "@/features/file/interfaces/file.interfaces";
import type { Project } from "@/features/project/interfaces/project.interfaces";
import type { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";
import {
  filterFiles,
  formatFileSize,
  getFileIcon,
  getFileTypeLabel,
} from "@/features/file/presentation/components/panels/files/helpers/files.helper";

interface ProjectFilesTabProps {
  project: Project;
}

/**
 * Converts an absolute host `source` into a workspace-relative path for `file.list`.
 *
 * The file API rejects absolute paths (`AOS_FILE_INVALID_PATH`). Project
 * sources are stored absolute, so the Files tab must strip the workspace root.
 */
function toWorkspaceRelativePath(
  source: string,
  workspaceRoot: string | undefined,
): string | null {
  const normalizedSource = source.trim().replace(/\/+$/, "");
  if (!normalizedSource) return null;

  // Already workspace-relative (e.g. "@packages/site").
  if (!normalizedSource.startsWith("/")) {
    return normalizedSource;
  }

  if (!workspaceRoot) return null;

  const normalizedRoot = workspaceRoot.replace(/\/+$/, "");
  if (normalizedSource === normalizedRoot) return "";
  if (normalizedSource.startsWith(`${normalizedRoot}/`)) {
    return normalizedSource.slice(normalizedRoot.length + 1);
  }

  // Source is absolute but outside this workspace — file API will refuse it.
  return null;
}

export function ProjectFilesTab({ project }: ProjectFilesTabProps) {
  const viewportTabs = aos.stores.viewport.useState((state) => state.tabs);
  const workspacePath = aos.stores.workspace.useState(
    (state) => state.current?.path,
  );

  const projectFilesRoot = React.useMemo(() => {
    if (!project.source?.trim()) return null;
    return toWorkspaceRelativePath(project.source, workspacePath);
  }, [project.source, workspacePath]);

  const [currentPath, setCurrentPath] = React.useState(projectFilesRoot ?? "");
  const [searchQuery, setSearchQuery] = React.useState("");

  // [Reset]: Keep the browser rooted on the project source when it changes.
  React.useEffect(() => {
    setCurrentPath(projectFilesRoot ?? "");
    setSearchQuery("");
  }, [project.id, projectFilesRoot]);

  const canList = projectFilesRoot !== null;

  const fileQuery = aos.client.file.list.useQuery({
    query: {
      includeIgnored: false,
      path: currentPath || projectFilesRoot || ".",
      recursive: false,
    },
  });

  const entries = React.useMemo(() => {
    const all = [...((fileQuery.data?.files ?? []) as WorkspaceFile[])].sort(
      (left, right) => {
        if (left.type !== right.type) return left.type === "directory" ? -1 : 1;
        return left.name.localeCompare(right.name);
      },
    );
    return filterFiles(all, searchQuery);
  }, [fileQuery.data?.files, searchQuery]);

  const isLoading = useDelayedLoading(
    canList && fileQuery.isLoading && !fileQuery.data,
  );
  const hasError = fileQuery.isError;

  function getTabFilePath(tab: ViewportTabState) {
    return typeof tab.metadata?.filePath === "string"
      ? tab.metadata.filePath
      : undefined;
  }

  function openFileTab(file: WorkspaceFile) {
    const existingTab = viewportTabs.items.find(
      (tab) => tab.type === "file" && getTabFilePath(tab) === file.path,
    );

    if (existingTab) {
      aos.stores.viewport.actions.setActiveTab(existingTab.id);
      return;
    }

    const placeholderTab = viewportTabs.items.find(
      (tab) => tab.type === "file" && !getTabFilePath(tab),
    );

    const metadata = {
      fileAbsolutePath: file.absolutePath,
      fileBrowserUrl: file.browserUrl,
      fileExtension: file.extension,
      fileIsEditable: file.isEditable,
      fileName: file.name,
      filePath: file.path,
      fileViewer: file.viewer,
    };

    if (placeholderTab) {
      aos.stores.viewport.actions.updateTab(placeholderTab.id, {
        title: file.name,
        metadata: {
          ...placeholderTab.metadata,
          ...metadata,
        },
      });
      aos.stores.viewport.actions.setActiveTab(placeholderTab.id);
      return;
    }

    const createdTabId = aos.stores.viewport.actions.createTab({
      type: "file",
      title: file.name,
      closable: true,
      metadata,
    });

    if (createdTabId) {
      aos.stores.viewport.actions.setActiveTab(createdTabId);
    }
  }

  function getRelativeSegments(path: string) {
    if (projectFilesRoot === null) return [];
    const normalized = path.replace(/\/+$/, "");
    const root = projectFilesRoot.replace(/\/+$/, "");
    if (normalized === root) return [];
    if (!root) {
      return normalized.split("/").filter(Boolean);
    }
    if (!normalized.startsWith(`${root}/`)) return [];
    return normalized
      .slice(root.length + 1)
      .split("/")
      .filter(Boolean);
  }

  function navigateToFolder(entry: WorkspaceFile) {
    const base = currentPath.replace(/\/+$/, "");
    setCurrentPath(base ? `${base}/${entry.name}` : entry.name);
    setSearchQuery("");
  }

  function navigateUp() {
    if (projectFilesRoot === null) return;
    const relative = [...getRelativeSegments(currentPath)];
    if (relative.length === 0) return;
    relative.pop();
    const root = projectFilesRoot.replace(/\/+$/, "");
    const nextPath =
      relative.length > 0
        ? root
          ? `${root}/${relative.join("/")}`
          : relative.join("/")
        : root;
    setCurrentPath(nextPath);
    setSearchQuery("");
  }

  function navigateToSegment(index: number) {
    if (projectFilesRoot === null) return;
    // index 0 is the project name crumb → root; deeper crumbs map to relative segments.
    if (index <= 0) {
      setCurrentPath(projectFilesRoot);
      setSearchQuery("");
      return;
    }
    const relative = getRelativeSegments(currentPath).slice(0, index);
    const root = projectFilesRoot.replace(/\/+$/, "");
    const nextPath =
      relative.length > 0
        ? root
          ? `${root}/${relative.join("/")}`
          : relative.join("/")
        : root;
    setCurrentPath(nextPath);
    setSearchQuery("");
  }

  function handleRefresh() {
    void fileQuery.refetch();
  }

  const relativeSegments =
    projectFilesRoot === null ? [] : getRelativeSegments(currentPath);
  const breadcrumbSegments = [project.name, ...relativeSegments];
  const isAtRoot =
    projectFilesRoot !== null &&
    currentPath.replace(/\/+$/, "") === projectFilesRoot.replace(/\/+$/, "");

  if (!project.source?.trim()) {
    return (
      <div className="flex h-full items-center justify-center">
        <AnimatedEmptyState className="border-none shadow-none">
          <AnimatedEmptyState.Content>
            <AnimatedEmptyState.Title>
              No source directory
            </AnimatedEmptyState.Title>
            <AnimatedEmptyState.Description>
              Bind an absolute host directory in Overview to browse its files
              here.
            </AnimatedEmptyState.Description>
          </AnimatedEmptyState.Content>
        </AnimatedEmptyState>
      </div>
    );
  }

  if (projectFilesRoot === null) {
    return (
      <div className="flex h-full items-center justify-center">
        <AnimatedEmptyState className="border-none shadow-none">
          <AnimatedEmptyState.Content>
            <AnimatedEmptyState.Title>
              Source outside workspace
            </AnimatedEmptyState.Title>
            <AnimatedEmptyState.Description>
              This project source is not inside the current workspace, so files
              cannot be listed here.
            </AnimatedEmptyState.Description>
          </AnimatedEmptyState.Content>
        </AnimatedEmptyState>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col overflow-hidden">
      {/* Toolbar */}
      <div className="flex items-center gap-2 border-b px-5.5 py-3">
        <Button
          variant="outline"
          size="icon-sm"
          className="rounded-full"
          onClick={navigateUp}
          disabled={isAtRoot}
        >
          <ArrowLeft className="size-4" />
        </Button>
        <div className="flex flex-1 items-center gap-1 overflow-hidden">
          {breadcrumbSegments.map((segment, index) => (
            <React.Fragment key={index}>
              {index > 0 && <span className="text-muted-foreground">/</span>}
              <Button
                variant="ghost"
                size="sm"
                className="h-6 px-1 text-sm font-medium lowercase"
                onClick={() => navigateToSegment(index)}
                disabled={index === breadcrumbSegments.length - 1}
              >
                {segment}
              </Button>
            </React.Fragment>
          ))}
        </div>
        <div className="relative w-56">
          <Search className="absolute left-2 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search files..."
            className="h-8 pl-8 text-xs"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
        </div>
        <Button
          variant="ghost"
          size="icon"
          className="size-8"
          onClick={handleRefresh}
        >
          <RefreshCw className="size-4" />
        </Button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto">
        {isLoading ? (
          <FilesTableSkeleton />
        ) : hasError && entries.length === 0 ? (
          <div className="flex h-full items-center justify-center">
            <AnimatedEmptyState className="border-none shadow-none">
              <AnimatedEmptyState.Content>
                <AnimatedEmptyState.Title>
                  Unable to load files
                </AnimatedEmptyState.Title>
                <AnimatedEmptyState.Description>
                  Could not load files for this directory.
                </AnimatedEmptyState.Description>
              </AnimatedEmptyState.Content>
            </AnimatedEmptyState>
          </div>
        ) : entries.length === 0 ? (
          <div className="flex h-full items-center justify-center">
            <AnimatedEmptyState className="border-none shadow-none">
              <AnimatedEmptyState.Content>
                <AnimatedEmptyState.Title>
                  No files found
                </AnimatedEmptyState.Title>
                <AnimatedEmptyState.Description>
                  No files found in this directory.
                </AnimatedEmptyState.Description>
              </AnimatedEmptyState.Content>
            </AnimatedEmptyState>
          </div>
        ) : (
          <motion.div
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.24, ease: "easeOut" }}
          >
            <div className="grid grid-cols-[1fr_120px_100px_140px] px-4 py-2 text-xs font-medium text-muted-foreground border-b">
              <div>Name</div>
              <div>Type</div>
              <div>Size</div>
              <div>Modified</div>
            </div>
            {entries.map((entry) => {
              const Icon =
                entry.type === "directory" ? Folder : getFileIcon(entry);
              return (
                <div
                  key={entry.path}
                  className={cn(
                    "grid grid-cols-[1fr_120px_100px_140px] items-center px-4 py-2 text-sm transition-colors hover:bg-muted/50 cursor-pointer border-b border-border/50 last:border-b-0",
                  )}
                  onClick={() => {
                    if (entry.type === "directory") {
                      navigateToFolder(entry);
                    } else {
                      openFileTab(entry);
                    }
                  }}
                >
                  <div className="flex items-center gap-2 overflow-hidden">
                    <Icon className="size-4 shrink-0 text-primary" />
                    <span className="truncate">{entry.name}</span>
                  </div>
                  <div className="text-xs text-muted-foreground">
                    {getFileTypeLabel(entry)}
                  </div>
                  <div className="text-xs text-muted-foreground">
                    {entry.type === "directory"
                      ? "-"
                      : formatFileSize(entry.size)}
                  </div>
                  <div className="text-xs text-muted-foreground">
                    {new Date(entry.updatedAt).toLocaleDateString(undefined, {
                      year: "numeric",
                      month: "short",
                      day: "numeric",
                    })}
                  </div>
                </div>
              );
            })}
          </motion.div>
        )}
      </div>
    </div>
  );
}

function FilesTableSkeleton() {
  return (
    <div>
      <div className="grid grid-cols-[1fr_120px_100px_140px] px-4 py-2 text-xs font-medium text-muted-foreground border-b">
        <div>Name</div>
        <div>Type</div>
        <div>Size</div>
        <div>Modified</div>
      </div>
      {Array.from({ length: 8 }).map((_, i) => (
        <div
          key={i}
          className="grid grid-cols-[1fr_120px_100px_140px] items-center px-4 py-2 border-b border-border/50 last:border-b-0"
        >
          <div className="flex items-center gap-2 overflow-hidden">
            <Skeleton className="size-4 rounded shrink-0" />
            <Skeleton className="h-4 w-32 rounded" />
          </div>
          <Skeleton className="h-4 w-16 rounded" />
          <Skeleton className="h-4 w-12 rounded" />
          <Skeleton className="h-4 w-20 rounded" />
        </div>
      ))}
    </div>
  );
}
