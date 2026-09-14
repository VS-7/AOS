import type { Project, ProjectStatus } from "@/features/project/interfaces/project.interfaces";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { FormField, FormItem } from "@/components/ui/form";
import {
  PROJECT_STATUS_CONFIG,
  PROJECT_STATUS_ORDER,
  projectStatusConfig,
} from "@/features/project/presentation/consts/project";
import { Folder, CalendarDays, ChevronDown } from "lucide-react";
import { getLocale, useTranslation } from "@/lib/i18n";

interface ProjectDetailsSidebarProps {
  project: Project;
  /** The project page's form, which the status is saved with. */
  form?: any;
}

export function ProjectDetailsSidebar({ project, form }: ProjectDetailsSidebarProps) {
  const { t } = useTranslation();

  return (
    <SplitPageLayout.DetailTabs defaultValue="overview">
      <SplitPageLayout.DetailTab value="overview" label={t("Overview")}>
        <SplitPageLayout.Widget>
          <SplitPageLayout.WidgetHeader>
            <SplitPageLayout.WidgetTitle>
              {t("Properties")}
            </SplitPageLayout.WidgetTitle>
          </SplitPageLayout.WidgetHeader>
          <SplitPageLayout.WidgetContent>
            {/* Status: every project has one — active, paused, done or
                archived — and no screen showed or changed it. */}
            {form ? (
              <FormField
                control={form.control}
                name="status"
                render={({ field }: { field: { value?: ProjectStatus; onChange: (value: ProjectStatus) => void } }) => {
                  const current = projectStatusConfig(field.value);
                  const CurrentIcon = current.icon;
                  return (
                    <FormItem className="border-0 p-0">
                      <SplitPageLayout.WidgetItem>
                        <CurrentIcon className={`size-3.5 shrink-0 ${current.color}`} />
                        <span className="w-16 shrink-0 text-xs text-muted-foreground">
                          {t("Status")}
                        </span>
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <button
                              type="button"
                              className="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs hover:bg-accent"
                            >
                              <span>{current.label}</span>
                              <ChevronDown className="size-3" />
                            </button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="start">
                            <DropdownMenuRadioGroup
                              value={field.value ?? "active"}
                              onValueChange={(value) => field.onChange(value as ProjectStatus)}
                            >
                              {PROJECT_STATUS_ORDER.map((status) => {
                                const config = PROJECT_STATUS_CONFIG[status];
                                const Icon = config.icon;
                                return (
                                  <DropdownMenuRadioItem
                                    key={status}
                                    value={status}
                                    className="flex items-center gap-2"
                                  >
                                    <Icon className={`size-4 ${config.color}`} />
                                    <span className="whitespace-nowrap pr-2">{config.label}</span>
                                  </DropdownMenuRadioItem>
                                );
                              })}
                            </DropdownMenuRadioGroup>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </SplitPageLayout.WidgetItem>
                    </FormItem>
                  );
                }}
              />
            ) : null}

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
                  {t("Source")}
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
                {t("Created")}
              </span>
              <span className="text-xs text-muted-foreground">
                {/* The interface's language, not the system's. */}
                {new Date(project.createdAt).toLocaleDateString(getLocale(), {
                  dateStyle: "medium",
                })}
              </span>
            </SplitPageLayout.WidgetItem>
          </SplitPageLayout.WidgetContent>
        </SplitPageLayout.Widget>

        {/* Description widget */}
        {project.description && (
          <SplitPageLayout.Widget>
            <SplitPageLayout.WidgetHeader>
              <SplitPageLayout.WidgetTitle>
                {t("Description")}
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
