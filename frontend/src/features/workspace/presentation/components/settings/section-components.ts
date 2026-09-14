import type * as React from "react";
import type { SettingsSectionId } from "./constants";
import { UserGeneralSection } from "./components/sections/user/general";
import { UserAgentsSection } from "./components/sections/user/agents";
import { UserAppearanceSection } from "./components/sections/user/appearance";
import { UserProfileSection } from "./components/sections/user/profile";
import { UserDevelopersSection } from "./components/sections/user/developers";
import { UserUsersSection } from "./components/sections/user/users";
import { UserUpdatesSection } from "./components/sections/user/updates";
import { WorkspaceJobsSection } from "./components/sections/workspace/jobs";
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
 * No section here is gated on a dormant domain any more. Every domain these
 * screens read is published; where a section depends on calls that are not
 * (Members' membership calls, Users' account writes), the section says so in
 * its own words instead of the generic "Domain not available yet" panel,
 * which spoke about the Go backend to whoever was using the app.
 */
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
  // Ungated: the roster is readable, and the section itself says what cannot
  // be done from it (see its own doc comment). `tunnel` was never dormant.
  "user.users": UserUsersSection,
  "user.updates": UserUpdatesSection,
  "user.tunnel": WorkspaceTunnelSection,
  "workspace.profile": WorkspaceProfileSection,
  // Ungated. The section renders its own "membership is not available in this
  // build yet" state from the four dormant membership calls, in words meant
  // for whoever uses it; the gate pre-empted that with a panel about the Go
  // backend, assembled from fragments that read "O workspace a interface já
  // existe" in Portuguese.
  "workspace.members": WorkspaceMembersSection,
  "workspace.agents": WorkspaceAgentsSection,
  "workspace.instructions": WorkspaceInstructionsSection,
  "workspace.templates": WorkspaceTemplatesSection,
  "workspace.tasks": WorkspaceTasksSection,
  "workspace.git": WorkspaceGitSection,
  "workspace.worktrees": WorkspaceWorktreesSection,
  "workspace.jobs": WorkspaceJobsSection,
};
