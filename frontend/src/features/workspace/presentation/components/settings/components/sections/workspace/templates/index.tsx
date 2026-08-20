import { aos } from "@/app/aos";
import { Form } from "@/components/ui/form";
import { TemplatesProvider, useTemplates } from "./contexts/templates.context";
import { TemplatesSidebar } from "./components/sidebar";
import { SelectedTemplateContent } from "./components/content";
import { SelectedTemplateDetail } from "./components/details";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import type { Template } from "@/features/template/interfaces/template.interfaces";

export function WorkspaceTemplatesSection() {
  const templatesQuery = aos.client.template.list.useQuery({ query: {} });
  const templates = (templatesQuery.data?.templates || []) as Template[];

  return (
    <TemplatesProvider
      templates={templates}
      refreshTemplates={async () => {
        await templatesQuery.refetch();
      }}
    >
      <WorkspaceTemplatesSectionLayout />
    </TemplatesProvider>
  );
}

function WorkspaceTemplatesSectionLayout() {
  const { selectedTemplateId, form } = useTemplates();

  return (
    <Form form={form} className="flex h-full flex-1 overflow-hidden">
      <SplitPageLayout
        variant="stacked"
        activeItemId={selectedTemplateId}
        className="bg-transparent"
      >
        <SplitPageLayout.Sidebar>
          <TemplatesSidebar />
        </SplitPageLayout.Sidebar>
        <SplitPageLayout.Content>
          <SelectedTemplateContent />
        </SplitPageLayout.Content>
        <SplitPageLayout.Detail>
          <SelectedTemplateDetail />
        </SplitPageLayout.Detail>
      </SplitPageLayout>
    </Form>
  );
}
