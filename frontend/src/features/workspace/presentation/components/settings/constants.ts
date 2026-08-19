import type { IconSvgElement } from "@hugeicons/react";
import {
  AiBrain01Icon,
  BotIcon,
  Briefcase01Icon,
  CheckListIcon,
  CloudIcon,
  ComputerIcon,
  Folder01Icon,
  GitBranchIcon,
  Layers01Icon,
  PaintBoardIcon,
  GraduationScrollIcon,
  SourceCodeIcon,
  UserGroupIcon,
  UserIcon,
  UserMultiple02Icon,
} from "@hugeicons/core-free-icons";

export type SettingsSectionId =
  | "user.general"
  | "user.agents"
  | "user.appearance"
  | "user.profile"
  | "user.developers"
  | "user.users"
  | "user.tunnel"
  | "workspace.profile"
  | "workspace.members"
  | "workspace.agents"
  | "workspace.instructions"
  | "workspace.templates"
  | "workspace.tasks"
  | "workspace.git"
  | "workspace.worktrees";

export interface SettingsSectionDefinition {
  id: SettingsSectionId;
  title: string;
  description: string;
  group: "user" | "workspace";
  icon: IconSvgElement;
}

export const DEFAULT_SETTINGS_SECTION: SettingsSectionId = "user.general";

export const SETTINGS_SECTIONS: SettingsSectionDefinition[] = [
  {
    id: "user.general",
    title: "General",
    description: "Configure execution behavior and desktop notifications.",
    group: "user",
    icon: ComputerIcon,
  },
  {
    id: "user.agents",
    title: "AI Providers",
    description: "Configure your AI providers.",
    group: "user",
    icon: AiBrain01Icon,
  },
  {
    id: "user.appearance",
    title: "Appearance",
    description: "Adjust theme, typography, and visual customization.",
    group: "user",
    icon: PaintBoardIcon,
  },
  {
    id: "user.profile",
    title: "Profile",
    description: "Manage your profile, password, language, and regional defaults.",
    group: "user",
    icon: UserIcon,
  },
  {
    id: "user.developers",
    title: "Developers",
    description: "CLI, REST API, and MCP integration for agents.",
    group: "user",
    icon: SourceCodeIcon,
  },
  {
    id: "user.users",
    title: "Users",
    description: "Manage accounts that can sign in to this Fractal instance.",
    group: "user",
    icon: UserMultiple02Icon,
  },
  {
    id: "user.tunnel",
    title: "Tunnel",
    description: "Expose your instance to the internet via secure tunnel.",
    group: "user",
    icon: CloudIcon,
  },
  {
    id: "workspace.profile",
    title: "Profile",
    description: "Edit workspace branding and identity.",
    group: "workspace",
    icon: Briefcase01Icon,
  },
  {
    id: "workspace.members",
    title: "Members",
    description: "Invite and manage workspace members and roles.",
    group: "workspace",
    icon: UserGroupIcon,
  },
  {
    id: "workspace.agents",
    title: "Agents",
    description: "Manage workspace agents and their details.",
    group: "workspace",
    icon: BotIcon,
  },
  {
    id: "workspace.instructions",
    title: "Instructions",
    description: "Browse and edit workspace instructions.",
    group: "workspace",
    icon: GraduationScrollIcon,
  },
  {
    id: "workspace.templates",
    title: "Templates",
    description: "Manage reusable templates for the workspace.",
    group: "workspace",
    icon: Layers01Icon,
  },
  {
    id: "workspace.tasks",
    title: "Tasks",
    description: "Configure workspace task types and automation defaults.",
    group: "workspace",
    icon: CheckListIcon,
  },
  {
    id: "workspace.git",
    title: "Git",
    description: "Define branch, commit, and pull request preferences.",
    group: "workspace",
    icon: GitBranchIcon,
  },
  {
    id: "workspace.worktrees",
    title: "Worktrees",
    description: "Manage sandbox lifecycle and worktree initialization.",
    group: "workspace",
    icon: Folder01Icon,
  },
];

export const SETTINGS_CONTENT_MAX_WIDTH = "max-w-3xl";

export const SETTINGS_SECTION_MAP = Object.fromEntries(
  SETTINGS_SECTIONS.map((section) => [section.id, section]),
) as Record<SettingsSectionId, SettingsSectionDefinition>;
