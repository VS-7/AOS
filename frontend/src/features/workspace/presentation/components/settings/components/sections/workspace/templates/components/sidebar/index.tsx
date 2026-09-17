import React, { useMemo } from "react";
import { InfoIcon, LayoutTemplate, PlusSquareIcon } from "lucide-react";
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
import { useTemplates } from "../../contexts/templates.context";
import { t } from "@/lib/i18n";

export function TemplatesSidebar() {
  const {
    filteredTemplates,
    templates,
    selectedTemplateId,
    setSelectedTemplateId,
    searchQuery,
    setSearchQuery,
    startCreate,
  } = useTemplates();

  const groupedBySkill = useMemo(() => {
    const groups: Record<string, typeof filteredTemplates> = {};

    for (const template of filteredTemplates) {
      const skill = template.skill || "global";
      if (!groups[skill]) groups[skill] = [];
      groups[skill].push(template);
    }

    return groups;
  }, [filteredTemplates]);

  const skills = Object.keys(groupedBySkill).sort((left, right) => {
    if (left === "global") return -1;
    if (right === "global") return 1;
    return left.localeCompare(right);
  });

  return (
    <>
      <SplitPageLayout.SidebarHeader>
        <SplitPageLayout.SearchInput
          placeholder={t("Search templates...")}
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
                aria-label={t("New template")}
              >
                <PlusSquareIcon />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{t("New template")}</TooltipContent>
          </Tooltip>
        </SplitPageLayout.SidebarHeaderActions>
      </SplitPageLayout.SidebarHeader>

      <SplitPageLayout.SidebarContent>
        <SettingsContentContainer className="py-4">
          {skills.length === 0 ? (
            <AnimatedEmptyState className="border-none shadow-none py-12">
              <AnimatedEmptyState.Content>
                <AnimatedEmptyState.Title>
                  {t("No templates found")}
                </AnimatedEmptyState.Title>
                <AnimatedEmptyState.Description>
                  {searchQuery
                    ? t("No results for \"{{query}}\"", { query: searchQuery })
                    : t("Create a new template to make common outputs reusable.")}
                </AnimatedEmptyState.Description>
              </AnimatedEmptyState.Content>
            </AnimatedEmptyState>
          ) : (
            skills.map((skill) => {
              const items = groupedBySkill[skill];

              return (
                <SplitPageLayout.SidebarGroup key={skill} id={`skill-${skill}`}>
                  <SplitPageLayout.SidebarGroupHeader
                    label={skill === "global" ? t("Global") : skill}
                    count={items.length}
                  />
                  <SplitPageLayout.SidebarGroupContent variant="grouped">
                    {items.map((template) => (
                      <SplitPageLayout.SidebarItemCard
                        key={template.id}
                        isActive={selectedTemplateId === template.id}
                        onClick={() => setSelectedTemplateId(template.id)}
                      >
                        <div className="flex w-full items-center justify-between gap-2 text-sm">
                          <span className="truncate font-medium leading-none">
                            {template.name}
                          </span>
                          {template.output ? (
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
                                  <code className="text-sm font-mono">
                                    {template.output}
                                  </code>
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
          <LayoutTemplate className="size-3.5 text-muted-foreground" />
          {templates.length === 1
            ? t("1 template")
            : t("{{count}} templates", { count: templates.length })}
        </span>
      </SplitPageLayout.SidebarFooter>
    </>
  );
}
