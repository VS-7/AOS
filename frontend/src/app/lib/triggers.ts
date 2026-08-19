import { AosTrigger } from "../builders/trigger";
import { viewportGroup } from "@/features/workspace/presentation/triggers/viewport.trigger";
import { workspaceGroup } from "@/features/workspace/presentation/triggers/workspace.trigger";
import { settingsGroup } from "@/features/workspace/presentation/triggers/settings.trigger";
import { tasksGroup } from "@/features/task/presentation/triggers/tasks.trigger";
import { chatGroup } from "@/features/chat/presentation/triggers/chat.trigger";
import { tabsGroup } from "@/features/workspace/presentation/triggers/tabs.trigger";
import { collectionGroup } from "@/features/collection/presentation/triggers/collection.trigger";
import { viewGroup } from "@/features/view/presentation/triggers/view.trigger";
import { routineGroup } from "@/features/routine/presentation/triggers/routine.trigger";
import { projectGroup } from "@/features/project/presentation/triggers/project.trigger";
import { goalGroup } from "@/features/goal/presentation/triggers/goal.trigger";
import { artifactGroup } from "@/features/artifact/presentation/triggers/artifact.trigger";
import { filesGroup } from "@/features/file/presentation/triggers/files.trigger";

export const triggers = AosTrigger.create()
  .addGroup(viewportGroup)
  .addGroup(settingsGroup)
  .addGroup(workspaceGroup)
  .addGroup(tasksGroup)
  .addGroup(chatGroup)
  .addGroup(tabsGroup)
  .addGroup(filesGroup)
  .addGroup(collectionGroup)
  .addGroup(viewGroup)
  .addGroup(routineGroup)
  .addGroup(projectGroup)
  .addGroup(goalGroup)
  .addGroup(artifactGroup)
  .build();
