import { aos } from "@/app/aos";
import { Form } from "@/components/ui/form";
import { AgentsProvider, useAgents } from "./contexts/agents.context";
import { AgentsSidebar } from "./components/sidebar";
import { SelectedAgentContent } from "./components/content";
import { SelectedAgentDetail } from "./components/details";
import { SplitPageLayout } from "@/components/ui/split-page-layout";

export function WorkspaceAgentsSection() {
  const agents = aos.stores.agent.useState((state) => state.items);

  return (
    <AgentsProvider agents={agents}>
      <WorkspaceAgentsSectionLayout />
    </AgentsProvider>
  );
}

function WorkspaceAgentsSectionLayout() {
  const { selectedAgentId, form } = useAgents();

  return (
    <Form form={form} className="flex h-full flex-1 overflow-hidden">
      <SplitPageLayout variant="stacked" activeItemId={selectedAgentId}>
        <SplitPageLayout.Sidebar>
          <AgentsSidebar />
        </SplitPageLayout.Sidebar>
        <SplitPageLayout.Content>
          <SelectedAgentContent />
        </SplitPageLayout.Content>
        <SplitPageLayout.Detail>
          <SelectedAgentDetail />
        </SplitPageLayout.Detail>
      </SplitPageLayout>
    </Form>
  );
}
