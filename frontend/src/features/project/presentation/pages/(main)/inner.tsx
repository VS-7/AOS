import { AnimatePresence } from "framer-motion";
import { useNavigate } from "@tanstack/react-router";
import { Page, PageBody, PageSecondaryHeader } from "@/components/ui/page";
import { Button } from "@/components/ui/button";
import { Plus } from "lucide-react";
import { ProjectListRow } from "./components/list/components/project-list-row.component";
import { cn } from "@/lib/utils";
import { useProjectsContext } from "./context";
import { t } from "@/lib/i18n";

export function ProjectsPageInner() {
  const navigate = useNavigate();
  const { filteredProjects } = useProjectsContext();

  return (
    <>
      <Page className="h-full overflow-hidden">
        <PageSecondaryHeader className="justify-between border-b px-4 py-2">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium">{t("Projects")}</span>
            <span className="text-xs text-muted-foreground">
              {filteredProjects.length}
            </span>
          </div>

          <div className="flex items-center gap-2">
            <Button
              className="h-9"
              onClick={() =>
                void navigate({ to: "/projects/$id", params: { id: "new" } })
              }
            >
              <Plus data-icon="inline-start" />
              {t("Add project")}
            </Button>
          </div>
        </PageSecondaryHeader>

        <PageBody className="overflow-y-auto">
          <div className={cn("mx-auto flex w-full flex-col h-full")}>
            <div className="gap-4 p-4">
              <AnimatePresence mode="popLayout">
                {filteredProjects.length === 0 ? (
                  <div className="flex h-32 items-center justify-center text-sm text-muted-foreground">
                    {t("No projects yet")}
                  </div>
                ) : (
                  <div className="flex flex-col rounded-md border shadow-inner bg-muted divide-y overflow-hidden">
                    {filteredProjects.map((project) => (
                      <ProjectListRow key={project.id} project={project} />
                    ))}
                  </div>
                )}
              </AnimatePresence>
            </div>
          </div>
        </PageBody>
      </Page>
    </>
  );
}
