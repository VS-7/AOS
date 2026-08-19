import * as React from "react";
import { useNavigate, useRouter } from "@tanstack/react-router";
import {
  BookOpen01Icon,
  Calendar01Icon,
  DatabaseIcon,
  Delete02Icon,
  SquareArrowUpRightIcon,
  File01Icon,
  DashboardSquare01Icon,
  Package01Icon,
  PencilEdit01Icon,
  ToolsIcon,
  WebhookIcon,
} from "@hugeicons/core-free-icons";
import { HugeiconsIcon, type IconSvgElement } from "@hugeicons/react";
import { toast } from "sonner";

import type {
  FractalMarketplaceSkillComponentItem,
  FractalMarketplaceSkillComponentKind,
} from "@/features/marketplace/interfaces/marketplace.interfaces";
import { ArtifactHelper } from "@/features/artifact/presentation/helpers/artifact.helper";
import { openWorkspaceFileTab } from "@/features/file/presentation/helpers/open-file-tab.helper";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { aos } from "@/app/aos";
import { cn } from "@/lib/utils";

export const PLUGIN_INVENTORY_SECTION_META: Record<
  FractalMarketplaceSkillComponentKind,
  { label: string; icon: IconSvgElement }
> = {
  toolsets: { label: "Toolsets", icon: ToolsIcon },
  collections: { label: "Collections", icon: DatabaseIcon },
  views: { label: "Views", icon: DashboardSquare01Icon },
  hooks: { label: "Hooks", icon: WebhookIcon },
  instructions: { label: "Instructions", icon: BookOpen01Icon },
  templates: { label: "Templates", icon: File01Icon },
  artifacts: { label: "Artifacts", icon: Package01Icon },
  routines: { label: "Routines", icon: Calendar01Icon },
};

/** Kinds that open a dedicated app page (or artifact browser tab). */
const KINDS_WITH_PAGE = new Set<FractalMarketplaceSkillComponentKind>([
  "views",
  "collections",
  "artifacts",
  "routines",
]);

const KINDS_WITH_DELETE = new Set<FractalMarketplaceSkillComponentKind>([
  "toolsets",
  "collections",
  "views",
  "instructions",
  "templates",
  "artifacts",
  "routines",
]);

interface PluginDetailSectionProps {
  kind: FractalMarketplaceSkillComponentKind;
  items: FractalMarketplaceSkillComponentItem[];
  folderUrl?: string;
  interactive?: boolean;
  /** Only called for toolsets — opens the inventory sheet. */
  onToolsetClick?: (item: FractalMarketplaceSkillComponentItem) => void;
}

export function PluginDetailSection({
  kind,
  items,
  folderUrl,
  interactive = false,
  onToolsetClick,
}: PluginDetailSectionProps) {
  const router = useRouter();
  const navigate = useNavigate();
  const showEmpty = interactive;

  const onDeleted = React.useCallback(async () => {
    toast.success("Item deleted");
    await router.invalidate();
  }, [router]);

  const onDeleteError = React.useCallback((error: unknown) => {
    toast.error(error instanceof Error ? error.message : "Failed to delete item");
  }, []);

  const { mutate: deleteToolset, loading: deletingToolset } =
    aos.client.toolset.delete.useMutation({
      onSuccess: onDeleted,
      onError: onDeleteError,
    });
  const { mutate: deleteView, loading: deletingView } =
    aos.client.view.delete.useMutation({
      onSuccess: onDeleted,
      onError: onDeleteError,
    });
  const { mutate: deleteArtifact, loading: deletingArtifact } =
    aos.client.artifact.delete.useMutation({
      onSuccess: onDeleted,
      onError: onDeleteError,
    });
  const { mutate: deleteRoutine, loading: deletingRoutine } =
    aos.client.routine.delete.useMutation({
      onSuccess: onDeleted,
      onError: onDeleteError,
    });
  const { mutate: deleteCollection, loading: deletingCollection } =
    aos.client.collection.delete.useMutation({
      onSuccess: onDeleted,
      onError: onDeleteError,
    });
  const { mutate: deleteInstruction, loading: deletingInstruction } =
    aos.client.instruction.delete.useMutation({
      onSuccess: onDeleted,
      onError: onDeleteError,
    });
  const { mutate: deleteTemplate, loading: deletingTemplate } =
    aos.client.template.delete.useMutation({
      onSuccess: onDeleted,
      onError: onDeleteError,
    });

  const isDeleting =
    deletingToolset ||
    deletingView ||
    deletingArtifact ||
    deletingRoutine ||
    deletingCollection ||
    deletingInstruction ||
    deletingTemplate;

  if (items.length === 0 && !showEmpty) return null;

  const meta = PLUGIN_INVENTORY_SECTION_META[kind];
  const hasPage = KINDS_WITH_PAGE.has(kind);
  const canDelete = KINDS_WITH_DELETE.has(kind);

  function handleDelete(item: FractalMarketplaceSkillComponentItem) {
    const entityId = item.id ?? item.name;
    switch (kind) {
      case "toolsets":
        deleteToolset({ params: { toolset: entityId } });
        break;
      case "views":
        deleteView({ params: { view: entityId } });
        break;
      case "artifacts":
        deleteArtifact({ params: { artifact: entityId } });
        break;
      case "routines":
        deleteRoutine({ params: { routine: entityId } });
        break;
      case "collections":
        deleteCollection({ params: { collection: entityId } });
        break;
      case "instructions":
        deleteInstruction({ params: { instruction: entityId } });
        break;
      case "templates":
        deleteTemplate({ params: { template: entityId } });
        break;
      default:
        break;
    }
  }

  async function handleOpenPage(item: FractalMarketplaceSkillComponentItem) {
    const entityId = item.id ?? item.name;

    if (kind === "views") {
      await navigate({ to: "/views/$id", params: { id: entityId } });
      return;
    }

    if (kind === "collections") {
      await navigate({ to: "/collections/$id", params: { id: entityId } });
      return;
    }

    if (kind === "routines") {
      await navigate({ to: "/routines/$id", params: { id: entityId } });
      return;
    }

    if (kind === "artifacts") {
      try {
        const result = await aos.client.artifact.getById.query({
          params: { artifact: entityId },
        });
        const artifact = result.data?.artifact;
        const urls = result.data?.urls;
        if (!artifact || !urls?.local) {
          toast.error("Artifact URL not available");
          return;
        }
        ArtifactHelper.openInBrowserTab({ ...artifact, urls });
      } catch (error) {
        toast.error(
          error instanceof Error ? error.message : "Failed to open artifact",
        );
      }
    }
  }

  function handleRowClick(item: FractalMarketplaceSkillComponentItem) {
    if (kind === "toolsets") {
      onToolsetClick?.(item);
      return;
    }

    if (hasPage) {
      void handleOpenPage(item);
      return;
    }

    if (item.path) {
      openWorkspaceFileTab(item.path);
    }
  }

  return (
    <section className="flex flex-col gap-3">
      {folderUrl ? (
        <a
          href={folderUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-2 text-[15px] font-medium text-foreground transition-colors hover:text-foreground/80"
        >
          {meta.label}
          <span className="text-[13px] font-normal text-muted-foreground">
            {items.length}
          </span>
        </a>
      ) : (
        <div className="inline-flex items-center gap-2 text-[15px] font-medium text-foreground">
          {meta.label}
          <span className="text-[13px] font-normal text-muted-foreground">
            {items.length}
          </span>
        </div>
      )}

      {items.length === 0 ? (
        <div className="flex h-12 w-full items-center justify-center rounded-md border-2 border-dotted">
          <span className="text-muted-foreground/60">
            No {meta.label.toLowerCase()} in this plugin yet!
          </span>
        </div>
      ) : (
        <TooltipProvider delayDuration={200}>
          <div className="overflow-hidden rounded-md border border-border bg-card">
            {items.map((item, index) => {
              const rowClassName = cn(
                "group flex w-full items-center gap-3 px-4 py-3.5 text-left transition-colors hover:bg-muted/50",
                index > 0 && "border-t border-border",
              );

              if (interactive) {
                return (
                  <div
                    key={`${item.kind}-${item.id ?? item.name}-${item.instructionType ?? ""}`}
                    className={rowClassName}
                  >
                    <button
                      type="button"
                      className="flex min-w-0 flex-1 items-center gap-3 text-left"
                      onClick={() => handleRowClick(item)}
                    >
                      <HugeiconsIcon
                        icon={meta.icon}
                        className="size-4 shrink-0 text-muted-foreground"
                      />
                      <span className="min-w-0 flex-1 truncate text-[13px] font-medium text-foreground">
                        {item.label}
                      </span>
                      {item.description ? (
                        <span className="hidden max-w-[40%] truncate text-[12px] text-muted-foreground sm:inline">
                          {item.description}
                        </span>
                      ) : null}
                    </button>

                    <div className="flex shrink-0 items-center gap-0.5">
                      {hasPage ? (
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              className="size-7 text-muted-foreground hover:text-foreground"
                              onClick={(event) => {
                                event.stopPropagation();
                                void handleOpenPage(item);
                              }}
                            >
                              <HugeiconsIcon
                                icon={SquareArrowUpRightIcon}
                                className="size-3.5"
                              />
                              <span className="sr-only">Open</span>
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent side="top">Open</TooltipContent>
                        </Tooltip>
                      ) : null}

                      {item.path ? (
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              className="size-7 text-muted-foreground hover:text-foreground"
                              onClick={(event) => {
                                event.stopPropagation();
                                openWorkspaceFileTab(item.path!);
                              }}
                            >
                              <HugeiconsIcon
                                icon={PencilEdit01Icon}
                                className="size-3.5"
                              />
                              <span className="sr-only">Edit</span>
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent side="top">Edit</TooltipContent>
                        </Tooltip>
                      ) : null}

                      {canDelete ? (
                        <AlertDialog>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <AlertDialogTrigger asChild>
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="icon"
                                  className="size-7 text-muted-foreground hover:text-destructive"
                                  disabled={isDeleting}
                                  onClick={(event) => event.stopPropagation()}
                                >
                                  <HugeiconsIcon
                                    icon={Delete02Icon}
                                    className="size-3.5"
                                  />
                                  <span className="sr-only">Delete</span>
                                </Button>
                              </AlertDialogTrigger>
                            </TooltipTrigger>
                            <TooltipContent side="top">Delete</TooltipContent>
                          </Tooltip>
                          <AlertDialogContent size="sm">
                            <AlertDialogHeader>
                              <AlertDialogTitle>
                                Delete this item?
                              </AlertDialogTitle>
                              <AlertDialogDescription>
                                Removes <strong>{item.label}</strong>{" "}
                                permanently.
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel>Cancel</AlertDialogCancel>
                              <AlertDialogAction
                                variant="destructive"
                                onClick={() => handleDelete(item)}
                              >
                                Delete
                              </AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                        </AlertDialog>
                      ) : null}
                    </div>
                  </div>
                );
              }

              if (item.githubUrl) {
                return (
                  <a
                    key={`${item.kind}-${item.label}-${item.instructionType ?? ""}`}
                    href={item.githubUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className={rowClassName}
                  >
                    <HugeiconsIcon
                      icon={meta.icon}
                      className="size-4 shrink-0 text-muted-foreground"
                    />
                    <span className="min-w-0 flex-1 truncate text-[13px] font-medium text-foreground">
                      {item.label}
                    </span>
                    <HugeiconsIcon
                      icon={SquareArrowUpRightIcon}
                      aria-hidden
                      className="size-3.5 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100"
                    />
                  </a>
                );
              }

              return (
                <div
                  key={`${item.kind}-${item.label}-${item.instructionType ?? ""}`}
                  className={rowClassName}
                >
                  <HugeiconsIcon
                    icon={meta.icon}
                    className="size-4 shrink-0 text-muted-foreground"
                  />
                  <span className="min-w-0 flex-1 truncate text-[13px] font-medium text-foreground">
                    {item.label}
                  </span>
                </div>
              );
            })}
          </div>
        </TooltipProvider>
      )}
    </section>
  );
}
