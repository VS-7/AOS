import type * as React from "react";
import type { SettingsSectionId } from "./constants";
import { UserGeneralSection } from "./components/sections/user/general";
import { UserAgentsSection } from "./components/sections/user/agents";
import { UserAppearanceSection } from "./components/sections/user/appearance";
import { UserProfileSection } from "./components/sections/user/profile";
import { UserDevelopersSection } from "./components/sections/user/developers";
import { UserUsersSection } from "./components/sections/user/users";
import { WorkspaceProfileSection } from "./components/sections/workspace/profile";
import { WorkspaceMembersSection } from "./components/sections/workspace/members";
import { WorkspaceAgentsSection } from "./components/sections/workspace/agents";
import { WorkspaceInstructionsSection } from "./components/sections/workspace/instructions";
import { WorkspaceTemplatesSection } from "./components/sections/workspace/templates";
import { WorkspaceTasksSection } from "./components/sections/workspace/tasks";
import { WorkspaceTunnelSection } from "./components/sections/workspace/tunnel";
import { WorkspaceGitSection } from "./components/sections/workspace/git";
import { WorkspaceWorktreesSection } from "./components/sections/workspace/worktrees";

/**
 * Map of settings section ids to their page components.
 * Kept as a single source of truth for the settings shell and routes.
 */
export const SETTINGS_SECTION_COMPONENTS: Record<
  SettingsSectionId,
  React.ComponentType
> = {
  "user.general": UserGeneralSection,
  "user.agents": UserAgentsSection,
  "user.appearance": UserAppearanceSection,
  "user.profile": UserProfileSection,
  "user.developers": UserDevelopersSection,
  "user.users": UserUsersSection,
  "user.tunnel": WorkspaceTunnelSection,
  "workspace.profile": WorkspaceProfileSection,
  "workspace.members": WorkspaceMembersSection,
  "workspace.agents": WorkspaceAgentsSection,
  "workspace.instructions": WorkspaceInstructionsSection,
  "workspace.templates": WorkspaceTemplatesSection,
  "workspace.tasks": WorkspaceTasksSection,
  "workspace.git": WorkspaceGitSection,
  "workspace.worktrees": WorkspaceWorktreesSection,
};
