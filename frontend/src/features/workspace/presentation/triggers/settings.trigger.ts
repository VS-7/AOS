import { AosTriggerGroup } from "@/app/builders/trigger";
import type { SettingsSectionId } from "@/features/workspace/presentation/components/settings/constants";
import type { icons } from "lucide-react";

type SettingsTriggerDef = {
  id: string;
  label: string;
  icon: keyof typeof icons;
  section: SettingsSectionId;
  group: "User Settings" | "Workspace Settings";
};

const SETTINGS_TRIGGERS: SettingsTriggerDef[] = [
  {
    id: "settings.user.general",
    label: "Open General Settings",
    icon: "Monitor",
    section: "user.general",
    group: "User Settings",
  },
  {
    id: "settings.user.agents",
    label: "Open AI Providers Settings",
    icon: "Key",
    section: "user.agents",
    group: "User Settings",
  },
  {
    id: "settings.user.appearance",
    label: "Open Appearance Settings",
    icon: "Palette",
    section: "user.appearance",
    group: "User Settings",
  },
  {
    id: "settings.user.profile",
    label: "Open User Profile Settings",
    icon: "User",
    section: "user.profile",
    group: "User Settings",
  },
  {
    id: "settings.user.developers",
    label: "Open Developers Settings",
    icon: "Code",
    section: "user.developers",
    group: "User Settings",
  },
  {
    id: "settings.user.users",
    label: "Open Users Settings",
    icon: "Users",
    section: "user.users",
    group: "User Settings",
  },
  {
    id: "settings.user.tunnel",
    label: "Open Tunnel Settings",
    icon: "Cloud",
    section: "user.tunnel",
    group: "User Settings",
  },
  {
    id: "settings.workspace.profile",
    label: "Open Workspace Profile Settings",
    icon: "Briefcase",
    section: "workspace.profile",
    group: "Workspace Settings",
  },
  {
    id: "settings.workspace.members",
    label: "Open Workspace Members Settings",
    icon: "Users",
    section: "workspace.members",
    group: "Workspace Settings",
  },
  {
    id: "settings.workspace.agents",
    label: "Open Agents Settings",
    icon: "Users",
    section: "workspace.agents",
    group: "Workspace Settings",
  },
  {
    id: "settings.workspace.instructions",
    label: "Open Instructions Settings",
    icon: "ScrollText",
    section: "workspace.instructions",
    group: "Workspace Settings",
  },
  {
    id: "settings.workspace.templates",
    label: "Open Templates Settings",
    icon: "Layers",
    section: "workspace.templates",
    group: "Workspace Settings",
  },
  {
    id: "settings.workspace.tasks",
    label: "Open Task Types Settings",
    icon: "ListTodo",
    section: "workspace.tasks",
    group: "Workspace Settings",
  },
  {
    id: "settings.workspace.git",
    label: "Open Git Settings",
    icon: "GitBranch",
    section: "workspace.git",
    group: "Workspace Settings",
  },
  {
    id: "settings.workspace.worktrees",
    label: "Open Worktrees Settings",
    icon: "FolderTree",
    section: "workspace.worktrees",
    group: "Workspace Settings",
  },
  // The two sections the last audit added. The file's own comment below says
  // this list is 1:1 with SETTINGS_SECTIONS, and it had stopped being: neither
  // Updates nor Jobs could be opened from the command palette at all.
  {
    id: "settings.user.updates",
    label: "Open Updates Settings",
    icon: "CloudDownload",
    section: "user.updates",
    group: "User Settings",
  },
  {
    id: "settings.workspace.jobs",
    label: "Open Jobs Settings",
    icon: "ListOrdered",
    section: "workspace.jobs",
    group: "Workspace Settings",
  },
];

/**
 * Command palette triggers for every settings section (1:1 with SETTINGS_SECTIONS).
 */
let settingsTriggerBuilder = AosTriggerGroup.create("Settings").withOrder(5);

for (const trigger of SETTINGS_TRIGGERS) {
  settingsTriggerBuilder = settingsTriggerBuilder.addTrigger({
    id: trigger.id,
    label: trigger.label,
    icon: trigger.icon,
    group: trigger.group,
    handler: ({ stores }) => stores.viewport.actions.openSettings(trigger.section),
  });
}

export const settingsGroup = settingsTriggerBuilder.build();
