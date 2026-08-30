import * as React from "react";
import type { SettingsSectionId } from "./constants";
import { DormantGate } from "@/components/DormantDomain";
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
 * Task 10: four settings sections surface a domain `lib/command-map.ts`
 * marks dormant (`DORMANT_DOMAINS`) — `/settings/$group/$section` is one
 * route shared by every section (`SettingsSectionPage`, `../($group)/
 * ($section)/index.tsx`), so there is no single route to wrap for these;
 * gating the leaf component here, before it ever mounts (and so before it
 * runs its own dormant `client.*` calls), has the same effect. `<Suspense
 * fallback>`-free: `DormantGate` renders synchronously, no data fetch of
 * its own.
 */
function dormant(feature: string, Section: React.ComponentType): React.ComponentType {
  // `React.createElement`, not JSX: this file is `.ts` (matching the
  // pristine original, which has no reason for JSX — AOS's own
  // backend has no dormant domains), and `.ts` files cannot contain JSX
  // syntax regardless of the `jsx` compiler option.
  return function DormantSection() {
    return React.createElement(DormantGate, { feature, children: React.createElement(Section) });
  };
}

/**
 * C6 of the final review: `WorkspaceMembersSection` isn't in a dormant
 * *domain* the way the four `dormant(...)`-wrapped sections below are —
 * `workspace.create`/`.update`/`.list` are real — but every command this
 * one section actually calls is individually `null`
 * (`workspace.addMember`/`.listMembers`/`.removeMember`/`.updateMember`,
 * `user.list`). Ungated, it rendered a functional-looking member form that
 * silently no-op'd on every action. `DormantGate`'s `commands` prop (added
 * for exactly this) gates on those five paths instead of the domain.
 */
function dormantCommands(feature: string, commands: string[], Section: React.ComponentType): React.ComponentType {
  return function DormantSection() {
    return React.createElement(DormantGate, { feature, commands, children: React.createElement(Section) });
  };
}

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
  "user.users": dormant("user", UserUsersSection),
  "user.updates": UserUpdatesSection,
  "user.tunnel": dormant("tunnel", WorkspaceTunnelSection),
  "workspace.profile": WorkspaceProfileSection,
  // The four membership calls, and not `user.list`.
  //
  // `user.list` was in this list, and the gate only fires when *every* listed
  // path is dormant — so the day `user.list` was lit up against the identity
  // HTTP surface, this gate silently stopped firing and the section began
  // rendering a member form over four commands that do nothing. That is the
  // "Members shows an empty list forever" defect.
  //
  // `user.list` is not what this section is for: it fills the picker in the
  // add-member dialog. The four that decide whether the section can work at
  // all are the ones gating it.
  "workspace.members": dormantCommands(
    "workspace",
    ["workspace.addMember", "workspace.listMembers", "workspace.removeMember", "workspace.updateMember"],
    WorkspaceMembersSection,
  ),
  "workspace.agents": WorkspaceAgentsSection,
  "workspace.instructions": dormant("instruction", WorkspaceInstructionsSection),
  "workspace.templates": dormant("template", WorkspaceTemplatesSection),
  "workspace.tasks": WorkspaceTasksSection,
  "workspace.git": WorkspaceGitSection,
  "workspace.worktrees": WorkspaceWorktreesSection,
  "workspace.jobs": WorkspaceJobsSection,
};
