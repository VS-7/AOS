import type { IconSvgElement } from "@hugeicons/react";
import { t } from "@/lib/i18n";
import {
  AiBrain01Icon,
  BotIcon,
  Briefcase01Icon,
  CheckListIcon,
  CloudIcon,
  DownloadCircle01Icon,
  Queue01Icon,
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
  | "user.updates"
  | "user.tunnel"
  | "workspace.profile"
  | "workspace.members"
  | "workspace.agents"
  | "workspace.instructions"
  | "workspace.templates"
  | "workspace.tasks"
  | "workspace.git"
  | "workspace.worktrees"
  | "workspace.jobs";

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
    get title() { return t("General"); },
    get description() { return t("Configure execution behavior and desktop notifications."); },
    group: "user",
    icon: ComputerIcon,
  },
  {
    id: "user.agents",
    get title() { return t("AI Providers"); },
    get description() { return t("Configure your AI providers."); },
    group: "user",
    icon: AiBrain01Icon,
  },
  {
    id: "user.appearance",
    get title() { return t("Appearance"); },
    get description() { return t("Adjust theme, typography, and visual customization."); },
    group: "user",
    icon: PaintBoardIcon,
  },
  {
    id: "user.profile",
    get title() { return t("Profile"); },
    get description() { return t("Manage your profile, password, language, and regional defaults."); },
    group: "user",
    icon: UserIcon,
  },
  {
    id: "user.developers",
    get title() { return t("Developers"); },
    get description() { return t("CLI, REST API, and MCP integration for agents."); },
    group: "user",
    icon: SourceCodeIcon,
  },
  {
    id: "user.users",
    get title() { return t("Users"); },
    get description() { return t("Manage accounts that can sign in to this AOS instance."); },
    group: "user",
    icon: UserMultiple02Icon,
  },
  {
    id: "user.tunnel",
    get title() { return t("Tunnel"); },
    get description() { return t("Expose your instance to the internet via secure tunnel."); },
    group: "user",
    icon: CloudIcon,
  },
  {
    id: "user.updates",
    get title() { return t("Updates"); },
    get description() { return t("Check the release channel and install a new version."); },
    group: "user",
    icon: DownloadCircle01Icon,
  },
  {
    id: "workspace.profile",
    get title() { return t("Profile"); },
    get description() { return t("Edit workspace branding and identity."); },
    group: "workspace",
    icon: Briefcase01Icon,
  },
  {
    id: "workspace.members",
    get title() { return t("Members"); },
    get description() { return t("Invite and manage workspace members and roles."); },
    group: "workspace",
    icon: UserGroupIcon,
  },
  {
    id: "workspace.agents",
    get title() { return t("Agents"); },
    get description() { return t("Manage workspace agents and their details."); },
    group: "workspace",
    icon: BotIcon,
  },
  {
    id: "workspace.instructions",
    get title() { return t("Instructions"); },
    get description() { return t("Browse and edit workspace instructions."); },
    group: "workspace",
    icon: GraduationScrollIcon,
  },
  {
    id: "workspace.templates",
    get title() { return t("Templates"); },
    get description() { return t("Manage reusable templates for the workspace."); },
    group: "workspace",
    icon: Layers01Icon,
  },
  {
    id: "workspace.tasks",
    get title() { return t("Tasks"); },
    get description() { return t("Configure workspace task types and automation defaults."); },
    group: "workspace",
    icon: CheckListIcon,
  },
  {
    id: "workspace.git",
    get title() { return t("Git"); },
    get description() { return t("Define branch, commit, and pull request preferences."); },
    group: "workspace",
    icon: GitBranchIcon,
  },
  {
    id: "workspace.worktrees",
    get title() { return t("Worktrees"); },
    get description() { return t("Manage sandbox lifecycle and worktree initialization."); },
    group: "workspace",
    icon: Folder01Icon,
  },
  {
    id: "workspace.jobs",
    get title() { return t("Jobs"); },
    get description() { return t("The execution queue behind every turn, routine and background task."); },
    group: "workspace",
    icon: Queue01Icon,
  },
];

export const SETTINGS_CONTENT_MAX_WIDTH = "max-w-3xl";

export const SETTINGS_SECTION_MAP = Object.fromEntries(
  SETTINGS_SECTIONS.map((section) => [section.id, section]),
) as Record<SettingsSectionId, SettingsSectionDefinition>;
