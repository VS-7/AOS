import React, { useMemo } from "react";
import { Bot, PlusSquareIcon } from "lucide-react";
import { AnimatedEmptyState } from "@/components/ui/animated-empty-state";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { Button } from "@/components/ui/button";
import { AvatarAgentFallback } from "@/components/ui/avatar";
import { SettingsContentContainer } from "../../../../../content-container";
import { useAgents } from "../../contexts/agents.context";
import { t } from "@/lib/i18n";

export function AgentsSidebar() {
  const {
    filteredAgents,
    agents,
    selectedAgentId,
    setSelectedAgentId,
    searchQuery,
    setSearchQuery,
    startCreate,
  } = useAgents();

  const groupedBySkill = useMemo(() => {
    const groups: Record<string, typeof filteredAgents> = {};

    for (const agent of filteredAgents) {
      const skill = agent.skill || "general";
      if (!groups[skill]) groups[skill] = [];
      groups[skill].push(agent);
    }

    return groups;
  }, [filteredAgents]);

  const skills = Object.keys(groupedBySkill).sort((left, right) => {
    if (left === "general") return -1;
    if (right === "general") return 1;
    return left.localeCompare(right);
  });

  return (
    <>
      <SplitPageLayout.SidebarHeader>
        <SplitPageLayout.SearchInput
          placeholder={t("Search agents...")}
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
          {skills.length === 0 ? (
            <AnimatedEmptyState className="border-none shadow-none py-12">
              <AnimatedEmptyState.Content>
                <AnimatedEmptyState.Title>
                  {t("No agents found")}
                </AnimatedEmptyState.Title>
                <AnimatedEmptyState.Description>
                  {searchQuery
                    ? `No results for "${searchQuery}"`
                    : "Create a new agent to add a specialized workspace companion."}
                </AnimatedEmptyState.Description>
              </AnimatedEmptyState.Content>
            </AnimatedEmptyState>
          ) : (
            skills.map((skill) => {
              const items = groupedBySkill[skill];

              return (
                <SplitPageLayout.SidebarGroup key={skill} id={`skill-${skill}`}>
                  <SplitPageLayout.SidebarGroupHeader
                    label={skill === "general" ? "General" : skill}
                    count={items.length}
                  />
                  <SplitPageLayout.SidebarGroupContent variant="grouped">
                    {items.map((agent) => (
                      <SplitPageLayout.SidebarItemCard
                        key={agent.id}
                        isActive={selectedAgentId === agent.id}
                        onClick={() => setSelectedAgentId(agent.id)}
                      >
                        <div className="flex w-full items-center gap-2 text-sm">
                          <AvatarAgentFallback
                            name={agent.id}
                            image={agent.image}
                          />
                          <div className="min-w-0">
                            <p className="truncate font-medium leading-none">
                              {agent.name}
                            </p>
                            <p className="truncate pt-1 text-xs text-muted-foreground">
                              {agent.role ||
                                agent.description ||
                                agent.provider ||
                                "General agent"}
                            </p>
                          </div>
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
          <Bot className="size-3.5 text-muted-foreground" />
          {agents.length} agent{agents.length !== 1 ? "s" : ""}
        </span>
      </SplitPageLayout.SidebarFooter>
    </>
  );
}
