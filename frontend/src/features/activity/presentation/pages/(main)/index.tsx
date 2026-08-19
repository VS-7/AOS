import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { InboxPanel } from "@/features/workspace/presentation/components/panels/inbox";

export const ActivitiesPage = aos.page("/activities")
  .withMetadata({
    title: "Activities",
    description: "Activities",
  })
  .use(WorkspacePageMiddleware())
  .withComponent(({ route, client }) => {
    return (
      <div className="overflow-y-auto h-screen">
        <div className="container max-w-5xl mx-auto">
          <InboxPanel className="w-full border-l-0" />
        </div>
      </div>
    );
  })
  .build();


