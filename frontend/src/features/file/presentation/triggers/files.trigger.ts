import { AosTriggerGroup } from "@/app/builders/trigger";
import { openChangesTab } from "@/features/file/presentation/helpers/open-changes-tab.helper";

export const filesGroup = AosTriggerGroup.create("Files")
  .withOrder(4)
  .addTrigger({
    id: "files.changes.open",
    label: "Open Changes",
    keybind: "mod+shift+c",
    icon: "GitCompareArrows",
    handler: () => {
      openChangesTab();
    },
  })
  .build();
