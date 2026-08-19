import { AosTriggerGroup } from "@/app/builders/trigger";
import type { FractalViewSummary } from "../../interfaces/view.interfaces";
import { String } from "@/core/helpers/string";

export const viewGroup = AosTriggerGroup.create("Views")
  .withOrder(5)
  .withLoader(({ query, stores }) => {
    const views: FractalViewSummary[] = stores.views.state.items;

    const items = !query
      ? views
      : views.filter((view) => {
        const title = (view.title || view.name).toLowerCase();
        const name = view.name.toLowerCase();
        const q = query.toLowerCase();
        return title.includes(q) || name.includes(q);
      });

    const triggerItems = items.map((view) => ({
      id: `view.navigate.${view.name}`,
      label: String.capitalizeFirst(view.title || view.name),
      icon: "Layout" as "Layout",
      group: "Views",
      handler: ({ response }: { response: import("@/app/builders/response").AosResponse }) => {
        response.redirect(`/views/${view.name}`);
      },
    }));

    return triggerItems as any;
  })
  .build();