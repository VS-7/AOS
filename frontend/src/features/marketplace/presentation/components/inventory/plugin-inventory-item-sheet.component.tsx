import * as React from "react";
import { useRouter } from "@tanstack/react-router";
import {
  Delete02Icon,
  Loading03Icon,
  PencilEdit01Icon,
} from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";
import { toast } from "sonner";

import type { FractalMarketplaceSkillComponentItem } from "@/features/marketplace/interfaces/marketplace.interfaces";
import type { FractalToolsetConfigRequirement } from "@/features/toolset/interfaces/toolset.interfaces";
import { PLUGIN_INVENTORY_SECTION_META } from "@/features/marketplace/presentation/components/plugin-detail-section.component";
import { openWorkspaceFileTab } from "@/features/file/presentation/helpers/open-file-tab.helper";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { aos } from "@/app/aos";
import { cn } from "@/lib/utils";

interface PluginInventoryItemSheetProps {
  item: FractalMarketplaceSkillComponentItem | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * Sheet for installed toolset inventory items only.
 * Other inventory kinds open their page or source file directly from the card.
 */
export function PluginInventoryItemSheet({
  item,
  open,
  onOpenChange,
}: PluginInventoryItemSheetProps) {
  const toolsetItem = item?.kind === "toolsets" ? item : null;
  const meta = toolsetItem
    ? PLUGIN_INVENTORY_SECTION_META.toolsets
    : null;

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        className="flex w-full flex-col gap-0 overflow-hidden p-0 sm:max-w-lg"
      >
        {toolsetItem && meta ? (
          <>
            <SheetHeader className="shrink-0 space-y-1 border-b border-border px-5 py-4 text-left">
              <SheetTitle className="flex items-center gap-2 text-base font-medium">
                <HugeiconsIcon
                  icon={meta.icon}
                  className="size-4 text-muted-foreground"
                />
                {toolsetItem.label}
              </SheetTitle>
              <SheetDescription className="text-[13px]">
                {toolsetItem.description || meta.label}
              </SheetDescription>
            </SheetHeader>

            <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
              <ToolsetInventoryBody
                item={toolsetItem}
                onClose={() => onOpenChange(false)}
              />
            </div>
          </>
        ) : null}
      </SheetContent>
    </Sheet>
  );
}

function ToolsetInventoryBody({
  item,
  onClose,
}: {
  item: FractalMarketplaceSkillComponentItem;
  onClose: () => void;
}) {
  const router = useRouter();
  const toolsetId = item.id ?? item.name;
  const [activeTab, setActiveTab] = React.useState("tools");
  const [envValues, setEnvValues] = React.useState<Record<string, string>>({});

  const detailQuery = aos.client.toolset.getById.useQuery({
    params: { toolset: toolsetId },
    query: {},
  });

  const configQuery = aos.client.toolset.getConfig.useQuery({
    params: { toolset: toolsetId },
  });

  const { mutate: deleteToolset, loading: isDeleting } =
    aos.client.toolset.delete.useMutation({
      onSuccess: async () => {
        toast.success("Toolset deleted");
        onClose();
        await router.invalidate();
      },
      onError: (error: unknown) => {
        toast.error(
          error instanceof Error ? error.message : "Failed to delete toolset",
        );
      },
    });

  const { mutate: saveConfig, loading: isSaving } =
    aos.client.toolset.updateConfig.useMutation({
      onSuccess: async () => {
        toast.success("Environment variables saved");
        await configQuery.refetch();
      },
      onError: (error: unknown) => {
        toast.error(
          error instanceof Error ? error.message : "Failed to save config",
        );
      },
    });

  const toolset =
    detailQuery.data && "toolset" in detailQuery.data
      ? detailQuery.data.toolset
      : undefined;
  const tools: Array<{ name: string; description?: string }> =
    toolset?.tools ?? [];
  const requirements = configQuery.data?.requirements ?? [];
  const connectionType =
    item.connectionType ??
    configQuery.data?.connectionType ??
    toolset?.connection?.type;

  const hasConfig = requirements.length > 0;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        {connectionType ? (
          <Badge variant="outline" className="font-mono text-[10px]">
            {connectionType}
          </Badge>
        ) : null}
        {item.id ? (
          <span className="font-mono text-[11px] text-muted-foreground">
            {item.id}
          </span>
        ) : null}
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList className="h-8">
          <TabsTrigger value="tools" className="text-[12px]">
            Tools
          </TabsTrigger>
          {hasConfig ? (
            <TabsTrigger value="config" className="text-[12px]">
              Configuration
            </TabsTrigger>
          ) : null}
        </TabsList>

        <TabsContent value="tools" className="mt-3">
          {detailQuery.isLoading ? (
            <div className="flex items-center gap-2 text-[13px] text-muted-foreground">
              <HugeiconsIcon
                icon={Loading03Icon}
                className="size-3.5 animate-spin"
              />
              Loading tools…
            </div>
          ) : detailQuery.error ? (
            <p className="text-[13px] text-muted-foreground">
              Could not connect to list tools. Configure env vars if required,
              then reopen.
            </p>
          ) : tools.length === 0 ? (
            <p className="text-[13px] text-muted-foreground">
              No tools discovered for this toolset.
            </p>
          ) : (
            <div className="overflow-hidden rounded-lg border border-border">
              {tools.map((tool, index) => (
                <div
                  key={tool.name}
                  className={cn(
                    "flex items-start gap-3 px-3 py-2.5",
                    index > 0 && "border-t border-border",
                  )}
                >
                  <div className="min-w-0 flex-1">
                    <p className="font-mono text-[12px] font-medium text-foreground">
                      {tool.name}
                    </p>
                    {tool.description ? (
                      <p className="mt-0.5 line-clamp-1 text-[12px] leading-4 text-muted-foreground">
                        {tool.description}
                      </p>
                    ) : null}
                  </div>
                  {connectionType === "custom" && item.path ? (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-7 shrink-0"
                      title="Edit tool source"
                      onClick={() => openWorkspaceFileTab(item.path!)}
                    >
                      <HugeiconsIcon
                        icon={PencilEdit01Icon}
                        className="size-3.5"
                      />
                    </Button>
                  ) : null}
                </div>
              ))}
            </div>
          )}
        </TabsContent>

        {hasConfig ? (
          <TabsContent value="config" className="mt-3 space-y-3">
            {requirements.map((req: FractalToolsetConfigRequirement) => (
              <div key={req.lookupKey} className="space-y-1.5">
                <div className="flex items-center justify-between gap-2">
                  <Label
                    htmlFor={`env-${req.lookupKey}`}
                    className="font-mono text-[12px]"
                  >
                    {req.lookupKey}
                  </Label>
                  <Badge
                    variant={req.isSet ? "secondary" : "outline"}
                    className="text-[10px]"
                  >
                    {req.isSet ? "Set" : "Missing"}
                  </Badge>
                </div>
                <Input
                  id={`env-${req.lookupKey}`}
                  type="password"
                  autoComplete="off"
                  placeholder={
                    req.isSet ? "Leave blank to keep current" : "Enter value"
                  }
                  value={envValues[req.lookupKey] ?? ""}
                  onChange={(event) =>
                    setEnvValues((prev) => ({
                      ...prev,
                      [req.lookupKey]: event.target.value,
                    }))
                  }
                />
              </div>
            ))}
            <Button
              size="sm"
              disabled={isSaving}
              onClick={() => {
                const values: Record<string, string> = {};
                for (const [key, value] of Object.entries(envValues)) {
                  if (value.trim()) values[key] = value.trim();
                }
                saveConfig({
                  params: { toolset: toolsetId },
                  body: { values },
                });
              }}
            >
              {isSaving ? "Saving…" : "Save to .env"}
            </Button>
          </TabsContent>
        ) : null}
      </Tabs>

      <SheetFooter className="mt-auto flex-row items-center justify-between gap-2 border-t border-border px-0 pb-0 pt-4">
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button variant="destructive" size="sm" disabled={isDeleting}>
              <HugeiconsIcon icon={Delete02Icon} className="size-3.5" />
              Delete
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent size="sm">
            <AlertDialogHeader>
              <AlertDialogTitle>Delete this toolset?</AlertDialogTitle>
              <AlertDialogDescription>
                Removes <strong>{item.label}</strong> from disk. This cannot be
                undone.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction
                variant="destructive"
                onClick={() =>
                  deleteToolset({ params: { toolset: toolsetId } })
                }
              >
                Delete toolset
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>

        <div className="flex gap-2">
          {item.path ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => openWorkspaceFileTab(item.path!)}
            >
              <HugeiconsIcon icon={PencilEdit01Icon} className="size-3.5" />
              Edit
            </Button>
          ) : null}
          <Button size="sm" onClick={onClose}>
            Done
          </Button>
        </div>
      </SheetFooter>
    </div>
  );
}
