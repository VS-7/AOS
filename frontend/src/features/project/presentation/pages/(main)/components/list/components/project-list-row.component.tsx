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
import { Icon } from "@/components/ui/icon";
import { aos } from "@/app/aos";
import { toast } from "sonner";
import { MoreHorizontal, Trash2, Copy, Folder } from "lucide-react";
import { Button } from "@/components/ui/button";

interface ProjectListRowProps {
  project: Project;
}

export function ProjectListRow({ project }: ProjectListRowProps) {
  const router = useRouter();
  const iconName = ProjectHelper.getIcon(project.icon);

  const handleDelete = async () => {
    try {
      await aos.client.project.delete.mutate({
        params: { project: project.id },
      });
      toast.success(`Project ${project.id} deleted`);
      router.invalidate();
    } catch {
      toast.error("Failed to delete project");
    }
  };

  const handleCopyIdentifier = () => {
    navigator.clipboard.writeText(project.id);
    toast.success(`${project.id} copied`);
  };

  return (
    <motion.div
      layout
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -4, transition: { duration: 0.15 } }}
      transition={{ duration: 0.2, ease: "easeOut" }}
      className="grid min-h-11 w-full grid-cols-[1fr_auto_auto] items-center gap-3 px-3 py-2 transition-colors hover:border-input hover:bg-accent/40"
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
            Copy ID
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={handleDelete}
            className="flex items-center gap-2 text-red-500 focus:text-red-500"
          >
            <Trash2 className="size-3.5" />
            Delete
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </motion.div>
  );
}
