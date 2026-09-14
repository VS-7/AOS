import React from "react";
import { motion } from "framer-motion";
import { Link, useRouter } from "@tanstack/react-router";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { Project } from "@/features/project/interfaces/project.interfaces";
import { ProjectHelper } from "@/features/project/presentation/helpers/project.helper";
import { projectStatusConfig } from "@/features/project/presentation/consts/project";
import { useAlert } from "@/components/ui/alert-provider";
import { Icon } from "@/components/ui/icon";
import { aos } from "@/app/aos";
import { toast } from "sonner";
import { MoreHorizontal, Trash2, Copy } from "lucide-react";
import { Button } from "@/components/ui/button";
import { t } from "@/lib/i18n";
import { errorMessage } from "@/lib/aos-facade";

interface ProjectListRowProps {
  project: Project;
}

export function ProjectListRow({ project }: ProjectListRowProps) {
  const router = useRouter();
  const { confirm } = useAlert();
  const iconName = ProjectHelper.getIcon(project.icon);
  const status = projectStatusConfig(project.status);

  // The same question the project page asks: the row's menu removed the
  // project on the first click.
  const handleDelete = async () => {
    const confirmed = await confirm({
      title: t("Delete this project?"),
      description: t("This permanently removes {{name}}. Its tasks and goals are kept, without the project.", {
        name: project.name,
      }),
      confirmText: t("Delete project"),
      cancelText: t("Cancel"),
      variant: "destructive",
    });
    if (!confirmed) return;
    try {
      await aos.client.project.delete.mutateOrThrow({
        params: { project: project.id },
      });
      toast.success(t("Project {{name}} deleted", { name: project.name }));
      void aos.stores.projects.actions.refresh();
      router.invalidate();
    } catch (error) {
      toast.error(t("Failed to delete project"), { description: errorMessage(error) });
    }
  };

  const handleCopyIdentifier = () => {
    navigator.clipboard.writeText(project.id);
    toast.success(t("{{value}} copied", { value: project.id }));
  };

  return (
    <motion.div
      layout
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -4, transition: { duration: 0.15 } }}
      transition={{ duration: 0.2, ease: "easeOut" }}
      className="grid min-h-11 w-full grid-cols-[1fr_auto_auto_auto] items-center gap-3 px-3 py-2 transition-colors hover:border-input hover:bg-accent/40"
    >
      {/* Title and description */}
      <Link
        to="/projects/$id"
        params={{ id: project.id }}
        className="flex min-w-0 items-center gap-2"
      >
        <Icon
          value={iconName}
          fallback="Folder"
          className="size-4 shrink-0 text-muted-foreground"
        />
        <span className="truncate text-sm font-medium">{project.name}</span>
      </Link>

      {/* Status: the daemon keeps one for every project, and no screen showed it. */}
      <Badge variant="outline" className={`shrink-0 text-xs ${status.badgeClass}`}>
        {status.label}
      </Badge>

      {/* ID badge */}
      <Badge variant="outline" className="shrink-0 text-xs font-mono">
        {project.id}
      </Badge>

      {/* Actions */}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" className="size-7 rounded-md">
            <MoreHorizontal className="size-3.5 text-muted-foreground" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem
            onClick={handleCopyIdentifier}
            className="flex items-center gap-2"
          >
            <Copy className="size-3.5" />
            {t("Copy ID")}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={handleDelete}
            className="flex items-center gap-2 text-red-500 focus:text-red-500"
          >
            <Trash2 className="size-3.5" />
            {t("Delete")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </motion.div>
  );
}
