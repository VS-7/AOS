import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { InboxPanel } from "@/features/workspace/presentation/components/panels/inbox";

export const ActivitiesPage = aos.page("/activities")
  .withMetadata({
    title: "Activities",
    description: "Activities",
  })
  .use(WorkspacePageMiddleware())
  .withComponent(() => {
    // The height the layout gives the page, not the window's: `h-screen`
    // under the top bar ran the list's end — and "Load more" — past the
    // bottom edge. The panel scrolls its own list, so its header and the
    // Mark all as read button stay where they are.
    return (
      <div className="h-full min-h-0 overflow-hidden">
        <div className="container mx-auto h-full min-h-0 max-w-5xl">
          <InboxPanel className="h-full w-full border-l-0" />
        </div>
      </div>
    );
  })
  .build();
