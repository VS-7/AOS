import { AosTriggerGroup } from "@/app/builders/trigger";
import type { ViewSummary } from "../../interfaces/view.interfaces";
import { String } from "@/core/helpers/string";

export const viewGroup = AosTriggerGroup.create("Views")
  .withOrder(5)
  .withLoader(({ query, stores }) => {
    // `view`, the key app/stores.ts registers. `views` was undefined, and the
    // throw it caused emptied the whole command palette, not just this group.
    const views: ViewSummary[] = stores.view?.state.items ?? [];

    const items = !query
      ? views
      : views.filter((view) => {
        const title = (view.title || view.name).toLowerCase();
        const name = view.name.toLowerCase();
        const q = query.toLowerCase();
        return title.includes(q) || name.includes(q);
      });

    const triggerItems = items.map((view) => ({
      // By id: `/views/$id` looks the view up by id, and a name ("Contacts
      // table") landed on "Page not found".
      // The skill is part of the id: two scopes may ship the same view id.
      // The address carries only the id; the view page resolves a skill's
      // view from the same list this group reads.
      id: `view.navigate.${view.skill ? `${view.skill}:` : ""}${view.id}`,
      label: String.capitalizeFirst(view.title || view.name),
      icon: "Layout" as "Layout",
      group: "Views",
      handler: ({ response }: { response: import("@/app/builders/response").AosResponse }) => {
        response.redirect(`/views/${encodeURIComponent(view.id)}`);
      },
    }));

    return triggerItems as any;
  })
  .build();