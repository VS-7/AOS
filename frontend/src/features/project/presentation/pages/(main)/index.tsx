import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { Schema } from "@/core/helpers/schema.helper";
import { z } from "zod";

import { ProjectsProvider } from "./context";
import { ProjectsPageInner } from "./inner";

const ProjectsPageSearchSchema = Schema.object({
  query: z.string().optional(),
});

export const ProjectsPage = aos.page("/projects")
  .withMetadata({
    title: "Projects",
    description: "Project listing workspace",
  })
  .withQuery(ProjectsPageSearchSchema)
  .use(WorkspacePageMiddleware())
  .withLoader(async ({ client, request }) => {
    const query = request.query || {};

    const response = await client.project.list.query({
      query: {
        query: query.query?.trim() || undefined,
      },
    });

    const projects = response.data?.projects || [];
    return { projects };
  })
  .withComponent(({ route, client }) => {
    const { projects } = route.useLoaderData();
    const search = route.useSearch();

    return (
      <ProjectsProvider projects={projects} search={search}>
        <ProjectsPageInner />
      </ProjectsProvider>
    );
  })
  .build();
