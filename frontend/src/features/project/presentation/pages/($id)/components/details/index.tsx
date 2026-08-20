import type { Project } from "@/features/project/interfaces/project.interfaces";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { Folder, CalendarDays } from "lucide-react";

interface ProjectDetailsSidebarProps {
  project: Project;
}

export function ProjectDetailsSidebar({ project }: ProjectDetailsSidebarProps) {
  return (
    <SplitPageLayout.DetailTabs defaultValue="overview">
      <SplitPageLayout.DetailTab value="overview" label="Overview">
        <SplitPageLayout.Widget>
          <SplitPageLayout.WidgetHeader>
            <SplitPageLayout.WidgetTitle>
              Properties
            </SplitPageLayout.WidgetTitle>
          </SplitPageLayout.WidgetHeader>
          <SplitPageLayout.WidgetContent>
            {/* ID */}
            <SplitPageLayout.WidgetItem>
              <Folder className="size-3.5 shrink-0 text-muted-foreground" />
              <span className="w-16 shrink-0 text-xs text-muted-foreground">
                ID
              </span>
              <span className="font-mono text-xs text-muted-foreground">
                {project.id}
              </span>
            </SplitPageLayout.WidgetItem>

            {/* Source */}
            {project.source && (
              <SplitPageLayout.WidgetItem>
                <Folder className="size-3.5 shrink-0 text-muted-foreground" />
                <span className="w-16 shrink-0 text-xs text-muted-foreground">
                  Source
                </span>
                <span
                  className="truncate text-xs text-muted-foreground"
                  title={project.source}
                >
                  {project.source}
                </span>
              </SplitPageLayout.WidgetItem>
            )}

            {/* Created */}
            <SplitPageLayout.WidgetItem>
              <CalendarDays className="size-3.5 shrink-0 text-muted-foreground" />
              <span className="w-16 shrink-0 text-xs text-muted-foreground">
                Created
              </span>
              <span className="text-xs text-muted-foreground">
                {new Date(project.createdAt).toLocaleDateString()}
              </span>
            </SplitPageLayout.WidgetItem>
          </SplitPageLayout.WidgetContent>
        </SplitPageLayout.Widget>

        {/* Description widget */}
        {project.description && (
          <SplitPageLayout.Widget>
            <SplitPageLayout.WidgetHeader>
              <SplitPageLayout.WidgetTitle>
                Description
              </SplitPageLayout.WidgetTitle>
            </SplitPageLayout.WidgetHeader>
            <SplitPageLayout.WidgetContent>
              <SplitPageLayout.WidgetItem>
                <p className="text-xs text-muted-foreground leading-relaxed">
                  {project.description}
                </p>
              </SplitPageLayout.WidgetItem>
            </SplitPageLayout.WidgetContent>
          </SplitPageLayout.Widget>
        )}
      </SplitPageLayout.DetailTab>
    </SplitPageLayout.DetailTabs>
  );
}
