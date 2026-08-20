import { AosTriggerGroup } from "@/app/builders/trigger";

export const goalGroup = AosTriggerGroup.create("Goals")
  .withOrder(8)
  .addTrigger({
    id: "goals.new",
    label: "Create New Goal",
    keybind: "mod+shift+g",
    handler: ({ response }) => response.redirect("/goals/new"),
  })
  .addTrigger({
    id: "goals.list",
    label: "View All Goals",
    icon: "Target",
    handler: ({ response }) => response.redirect("/goals"),
  });
