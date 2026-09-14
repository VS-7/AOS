import { Archive, CheckCircle2, CircleDashed, CirclePause } from "lucide-react";
import { t } from "@/lib/i18n";
import type { ProjectStatus } from "@/features/project/interfaces/project.interfaces";

/**
 * How each project status reads. The labels are getters so they are looked up
 * in the language of the moment they are rendered, not the one the module
 * happened to load in.
 *
 * "Ongoing" and "On hold" rather than the goal list's "Active" and "Paused":
 * those translate to the feminine forms a goal (meta) takes, and a project
 * (projeto) would read "Ativa" and "Pausada".
 */
export const PROJECT_STATUS_CONFIG: Record<
  ProjectStatus,
  { label: string; icon: typeof CircleDashed; color: string; badgeClass: string }
> = {
  active: {
    get label() { return t("Ongoing"); },
    icon: CircleDashed,
    color: "text-primary",
    badgeClass: "bg-primary/10 text-primary border-primary/20",
  },
  paused: {
    get label() { return t("On hold"); },
    icon: CirclePause,
    color: "text-muted-foreground",
    badgeClass: "bg-muted text-muted-foreground border-muted",
  },
  done: {
    get label() { return t("Done"); },
    icon: CheckCircle2,
    color: "text-success",
    badgeClass: "bg-success/10 text-success border-success/20",
  },
  archived: {
    get label() { return t("Archived"); },
    icon: Archive,
    color: "text-muted-foreground",
    badgeClass: "bg-muted text-muted-foreground border-muted",
  },
};

export const PROJECT_STATUS_ORDER: ProjectStatus[] = ["active", "paused", "done", "archived"];

/** A total lookup: a status this build does not know reads as active. */
export function projectStatusConfig(status: ProjectStatus | string | undefined) {
  return PROJECT_STATUS_CONFIG[status as ProjectStatus] ?? PROJECT_STATUS_CONFIG.active;
}
