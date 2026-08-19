import { AosStore } from "../builders/store";
import { ViewportStore } from "@/features/workspace/presentation/stores/viewport.store";
import { ThemeStore } from "@/features/theme/presentation/stores/theme.store";
import { BrowserStore } from "@/features/workspace/presentation/stores/browser.store";
import { ConfigStore } from "@/features/config/presentation/stores/config.store";
import { WorkspaceStore } from "@/features/workspace/presentation/stores/workspace.store";
import { AgentStore } from "@/features/agent/presentation/stores/agent.store";
import { CollectionStore } from "@/features/collection/presentation/stores/collection.store";
import { ViewStore } from "@/features/view/presentation/stores/view.store";
import { AuthStore } from "@/features/auth/presentation/stores/auth.store";
import { ActivityStore } from "@/features/activity/presentation/stores/activity.store";
import { ChatStore } from "@/features/chat/presentation/stores/chat.store";
import { RoutineStore } from "@/features/routine/presentation/stores/routine.store";
import { ProjectStore } from "@/features/project/presentation/stores/project.store";
import { GoalStore } from "@/features/goal/presentation/stores/goal.store";
import { ArtifactStore } from "@/features/artifact/presentation/stores/artifact.store";
import { RealtimeStore } from "@/features/agent/presentation/stores/realtime.store";
import { FilesStore } from "@/features/file/presentation/stores/files.store";

export const stores = AosStore.router({
  prefix: "fractal",
  stores: {
    viewport: ViewportStore,
    theme: ThemeStore,
    browser: BrowserStore,
    config: ConfigStore,
    workspace: WorkspaceStore,
    agent: AgentStore,
    collections: CollectionStore,
    views: ViewStore,
    auth: AuthStore,
    activity: ActivityStore,
    chat: ChatStore,
    routines: RoutineStore,
    projects: ProjectStore,
    goals: GoalStore,
    artifacts: ArtifactStore,
    realtime: RealtimeStore,
    files: FilesStore,
  },
});
