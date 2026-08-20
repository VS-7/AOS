import { AosTriggerGroup } from "@/app/builders/trigger";

export const projectGroup = AosTriggerGroup.create("Projects")
  .withOrder(7)
  .addTrigger({
    id: "projects.new",
    label: "Create New Project",
    keybind: "mod+shift+p",
    handler: ({ response }) => response.redirect("/projects/new"),
  })
  .addTrigger({
    id: "projects.list",
    label: "View All Projects",
    icon: "FolderKanban",
    handler: ({ response }) => response.redirect("/projects"),
  });
