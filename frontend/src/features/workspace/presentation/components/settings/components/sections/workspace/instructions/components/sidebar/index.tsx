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
          placeholder="Search instructions..."
          value={searchQuery}
          onChange={(event) => setSearchQuery(event.target.value)}
        />
        <SplitPageLayout.SidebarHeaderActions>
          <Button
            size="icon"
            variant="ghost"
            className="rounded-md"
            onClick={startCreate}
          >
            <PlusSquareIcon />
          </Button>
        </SplitPageLayout.SidebarHeaderActions>
      </SplitPageLayout.SidebarHeader>

      <SplitPageLayout.SidebarContent>
        <SettingsContentContainer className="py-4">
          {types.length === 0 ? (
            <AnimatedEmptyState className="border-none shadow-none py-12">
              <AnimatedEmptyState.Content>
                <AnimatedEmptyState.Title>
                  No instructions found
                </AnimatedEmptyState.Title>
                <AnimatedEmptyState.Description>
                  {searchQuery
                    ? `No results for "${searchQuery}"`
                    : "Create a new instruction to start documenting workspace rules."}
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
                                <TooltipTrigger asChild>
                                  <button type="button" className="shrink-0">
                                    <InfoIcon className="size-3 opacity-60" />
                                  </button>
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
                                        +{instruction.paths.length - 3} more
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
          {instructions.length} instruction
          {instructions.length !== 1 ? "s" : ""}
        </span>
      </SplitPageLayout.SidebarFooter>
    </>
  );
}
