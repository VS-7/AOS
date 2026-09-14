import { AosTriggerGroup } from "@/app/builders/trigger";

export const tasksGroup = AosTriggerGroup.create("Tasks")
  .withOrder(1)
  .addTrigger({
    id: "tasks.new",
    label: "Create New Task",
    keybind: "mod+n",
    // Given { project }, it opens the dialog for that project — how a
    // project's Tasks tab creates a task inside the project. Without one it
    // toggles, as its shortcut always did.
    handler: ({ input, stores }) => {
      const project = (input as { project?: unknown } | undefined)?.project;
      if (typeof project === "string" && project) {
        stores.viewport.actions.setTaskDialogProject(project);
        return stores.viewport.actions.toggle('tasks.dialog.visible', true);
      }
      return stores.viewport.actions.toggle('tasks.dialog.visible');
    },
  })
  .addTrigger({
    id: "tasks.list",
    label: "View All Tasks",
    icon: "Layers",
    handler: ({ response }) => response.redirect("/tasks"),
  });
