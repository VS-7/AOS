import { AosTriggerGroup } from "@/app/builders/trigger";

export const routineGroup = AosTriggerGroup.create("Routines")
  .withOrder(1)
  .addTrigger({
    id: "routines.new",
    label: "Create New Routine",
    keybind: "mod+shift+r",
    handler: ({ response }) => response.redirect("/routines/new"),
  })
  .addTrigger({
    id: "routines.list",
    label: "View Routines",
    icon: "Play",
    handler: ({ response }) => response.redirect("/routines"),
  });
