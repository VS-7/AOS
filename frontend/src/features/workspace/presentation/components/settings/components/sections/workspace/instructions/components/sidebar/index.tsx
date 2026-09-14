import React, { useMemo } from "react";
import { FileText, InfoIcon, PlusSquareIcon } from "lucide-react";
import { AnimatedEmptyState } from "@/components/ui/animated-empty-state";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { SettingsContentContainer } from "../../../../../content-container";
import { useInstructions } from "../../contexts/instructions.context";
import { t } from "@/lib/i18n";

export function InstructionsSidebar() {
  const {
    filteredInstructions,
    instructions,
    selectedInstructionId,
    setSelectedInstructionId,
    searchQuery,
    setSearchQuery,
    startCreate,
  } = useInstructions();

  const groupedByType = useMemo(() => {
    const groups: Record<string, typeof filteredInstructions> = {};

    for (const instruction of filteredInstructions) {
      const type = instruction.type || "other";
      if (!groups[type]) groups[type] = [];
      groups[type].push(instruction);
    }

    return groups;
  }, [filteredInstructions]);

  const types = Object.keys(groupedByType).sort();

  return (
    <>
      <SplitPageLayout.SidebarHeader>
        <SplitPageLayout.SearchInput
          placeholder={t("Search instructions...")}
          value={searchQuery}
          onChange={(event) => setSearchQuery(event.target.value)}
        />
        <SplitPageLayout.SidebarHeaderActions>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                size="icon"
                variant="ghost"
                className="rounded-md"
                onClick={startCreate}
                aria-label={t("New instruction")}
              >
                <PlusSquareIcon />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{t("New instruction")}</TooltipContent>
          </Tooltip>
        </SplitPageLayout.SidebarHeaderActions>
      </SplitPageLayout.SidebarHeader>

      <SplitPageLayout.SidebarContent>
        <SettingsContentContainer className="py-4">
          {types.length === 0 ? (
            <AnimatedEmptyState className="border-none shadow-none py-12">
              <AnimatedEmptyState.Content>
                <AnimatedEmptyState.Title>
                  {t("No instructions found")}
                </AnimatedEmptyState.Title>
                <AnimatedEmptyState.Description>
                  {searchQuery
                    ? t("No results for \"{{query}}\"", { query: searchQuery })
                    : t("Create a new instruction to start documenting workspace rules.")}
                </AnimatedEmptyState.Description>
              </AnimatedEmptyState.Content>
            </AnimatedEmptyState>
          ) : (
            types.map((type) => {
              const items = groupedByType[type];

              return (
                <SplitPageLayout.SidebarGroup key={type} id={`type-${type}`}>
                  <SplitPageLayout.SidebarGroupHeader
                    label={type.charAt(0).toUpperCase() + type.slice(1)}
                    count={items.length}
                  />
                  <SplitPageLayout.SidebarGroupContent variant="grouped">
                    {items.map((instruction) => (
                      <SplitPageLayout.SidebarItemCard
                        key={instruction.id}
                        isActive={selectedInstructionId === instruction.id}
                        onClick={() => setSelectedInstructionId(instruction.id)}
                      >
                        <div className="flex w-full items-center justify-between gap-2 text-sm">
                          <span className="truncate font-medium leading-none">
                            {instruction.name}
                          </span>
                          {instruction.paths?.length ? (
                            <TooltipProvider>
                              <Tooltip>
                                {/* A span, not a button: the card is already
                                    the button, and a button inside a button is
                                    invalid HTML that React logged on every
                                    load. The hint needs no action of its own. */}
                                <TooltipTrigger asChild>
                                  <span className="inline-flex shrink-0">
                                    <InfoIcon className="size-3 opacity-60" />
                                  </span>
                                </TooltipTrigger>
                                <TooltipContent>
                                  <div className="flex flex-col gap-1">
                                    {instruction.paths
                                      .slice(0, 3)
                                      .map((path) => (
                                        <code
                                          key={path}
                                          className="text-sm font-mono"
                                        >
                                          {path}
                                        </code>
                                      ))}
                                    {instruction.paths.length > 3 ? (
                                      <span className="text-sm text-muted-foreground">
                                        {t("+{{count}} more", { count: instruction.paths.length - 3 })}
                                      </span>
                                    ) : null}
                                  </div>
                                </TooltipContent>
                              </Tooltip>
                            </TooltipProvider>
                          ) : null}
                        </div>
                      </SplitPageLayout.SidebarItemCard>
                    ))}
                  </SplitPageLayout.SidebarGroupContent>
                </SplitPageLayout.SidebarGroup>
              );
            })
          )}
        </SettingsContentContainer>
      </SplitPageLayout.SidebarContent>

      <SplitPageLayout.SidebarFooter>
        <span className="inline-flex items-center gap-2">
          <FileText className="size-3.5 text-muted-foreground" />
          {instructions.length === 1
            ? t("1 instruction")
            : t("{{count}} instructions", { count: instructions.length })}
        </span>
      </SplitPageLayout.SidebarFooter>
    </>
  );
}
