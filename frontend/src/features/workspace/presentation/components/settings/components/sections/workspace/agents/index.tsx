import { FormProvider } from "react-hook-form";
import { aos } from "@/app/aos";
import { AgentsProvider, useAgents } from "./contexts/agents.context";
import { AgentsSidebar } from "./components/sidebar";
import { SelectedAgentContent } from "./components/content";
import { SelectedAgentDetail } from "./components/details";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { t } from "@/lib/i18n";

export function WorkspaceAgentsSection() {
  const agents = aos.stores.agent.useState((state) => state.items);

  return (
    <AgentsProvider agents={agents}>
      <WorkspaceAgentsSectionLayout />
    </AgentsProvider>
  );
}

function WorkspaceAgentsSectionLayout() {
  const {
    selectedAgentId,
    form,
    pendingSelection,
    confirmPendingSelection,
    cancelPendingSelection,
  } = useAgents();

  // A provider, not a <Form>: the Channels tab renders a <Form> of its own
  // inside this layout, and a <form> nested in another makes Blink and
  // WebKit stop its submit event at the outer one — "Save Telegram" reloaded
  // the window with the bot token in the URL. The agent itself saves from
  // the header button, so this layout never needed to be a <form>.
  return (
    <FormProvider {...form}>
      <div className="flex h-full flex-1 overflow-hidden">
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
      </div>

      {/* Switching agents replaces the form; unsaved edits used to go
          with it, silently. */}
      <AlertDialog
        open={pendingSelection !== null}
        onOpenChange={(open) => {
          if (!open) cancelPendingSelection();
        }}
      >
        <AlertDialogContent size="sm">
          <AlertDialogHeader>
            <AlertDialogTitle>{t("Discard unsaved changes?")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("This agent has edits that were not saved. Leaving it discards them.")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("Keep editing")}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={confirmPendingSelection}>
              {t("Discard")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </FormProvider>
  );
}
