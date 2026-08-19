import { AosTriggerGroup } from "@/app/builders/trigger";

export const tasksGroup = AosTriggerGroup.create("Tasks")
  .withOrder(1)
  .addTrigger({
    id: "tasks.new",
    label: "Create New Task",
    keybind: "mod+n",
    handler: ({ stores }) => stores.viewport.actions.toggle('tasks.dialog.visible'),
  })
  .addTrigger({
    id: "tasks.list",
    label: "View All Tasks",
    icon: "Layers",
    handler: ({ response }) => response.redirect("/tasks"),
  });
