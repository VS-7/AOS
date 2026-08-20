import { aos } from "@/app/aos";
import { Form } from "@/components/ui/form";
import {
  InstructionsProvider,
  useInstructions,
} from "./contexts/instructions.context";
import { InstructionsSidebar } from "./components/sidebar";
import { SelectedInstructionContent } from "./components/content";
import { SelectedInstructionDetail } from "./components/details";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import type { Instruction } from "@/features/instruction/interfaces/instruction.interfaces";

export function WorkspaceInstructionsSection() {
  const instructionsQuery = aos.client.instruction.list.useQuery();
  const instructions = (instructionsQuery.data?.instructions ||
    []) as Instruction[];

  return (
    <InstructionsProvider
      instructions={instructions}
      refreshInstructions={async () => {
        await instructionsQuery.refetch();
      }}
    >
      <WorkspaceInstructionsSectionLayout />
    </InstructionsProvider>
  );
}

function WorkspaceInstructionsSectionLayout() {
  const { selectedInstructionId, form } = useInstructions();

  return (
    <Form form={form} className="flex h-full flex-1 overflow-hidden">
      <SplitPageLayout variant="stacked" activeItemId={selectedInstructionId}>
        <SplitPageLayout.Sidebar>
          <InstructionsSidebar />
        </SplitPageLayout.Sidebar>
        <SplitPageLayout.Content>
          <SelectedInstructionContent />
        </SplitPageLayout.Content>
        <SplitPageLayout.Detail>
          <SelectedInstructionDetail />
        </SplitPageLayout.Detail>
      </SplitPageLayout>
    </Form>
  );
}
