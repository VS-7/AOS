import { beforeEach, describe, expect, it } from "vitest";
import { ViewportStore } from "@/features/workspace/presentation/stores/viewport.store";
import { tasksGroup } from "./tasks.trigger";

type NewTaskHandler = (args: { input?: unknown; stores: { viewport: typeof ViewportStore } }) => unknown;
const newTask = (input?: unknown) =>
  (tasksGroup.build().triggers["tasks.new"].handler as unknown as NewTaskHandler)({ input, stores: { viewport: ViewportStore } });
const dialog = () => ViewportStore.state.tasks.dialog as { visible: boolean; project?: string };

// "New Task" on a project's Tasks tab opened the create dialog with no
// project, and the dialog had no way to be given one: the task landed
// outside the project whose tab it was created from.
describe("tasks.new", () => {
  beforeEach(() => {
    ViewportStore.actions.toggle("tasks.dialog.visible", false);
    ViewportStore.actions.setTaskDialogProject(undefined);
  });

  it("opens the create dialog for the project it is given", () => {
    newTask({ project: "api-de-teste" });
    expect(dialog()).toEqual({ visible: true, project: "api-de-teste" });
  });

  it("opens it for no project when it is given none", () => {
    newTask();
    expect(dialog().visible).toBe(true);
    expect(dialog().project).toBeUndefined();
  });

  it("still toggles the dialog from its shortcut", () => {
    newTask();
    newTask();
    expect(dialog().visible).toBe(false);
  });
});
