"use client";

import { CheckIcon, FolderIcon } from "lucide-react";

import { aos } from "@/app/aos";
import { ProjectHelper } from "@/features/project/presentation/helpers/project.helper";
import { Icon } from "./icon";
import {
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from "./dropdown-menu";

interface ProjectSelectorDropdownProps {
  currentProject?: string;
  onProjectChange: (project: string | undefined) => void;
}

export function ProjectSelectorDropdown({
  currentProject,
  onProjectChange,
}: ProjectSelectorDropdownProps) {
  const projects = aos.stores.projects.useState((state) => state.items);

  if (projects.length === 0) {
    return (
      <DropdownMenuItem className="text-muted-foreground">
        No projects available
      </DropdownMenuItem>
    );
  }

  return (
    <div className="flex flex-col gap-1 p-2">
      <DropdownMenuItem
        onClick={() => onProjectChange(undefined)}
        className="flex items-center gap-2"
      >
        <FolderIcon className="size-4 text-muted-foreground shrink-0" />
        <span>No project</span>
        {!currentProject ? (
          <CheckIcon className="ml-auto size-4 text-muted-foreground shrink-0" />
        ) : null}
      </DropdownMenuItem>

      <DropdownMenuSeparator />

      <DropdownMenuLabel className="text-xs font-medium text-muted-foreground">
        Projects
      </DropdownMenuLabel>

      {projects.map((project) => (
        <DropdownMenuItem
          key={project.id}
          onClick={() => onProjectChange(project.id)}
          className="flex items-center gap-2"
        >
          <Icon
            value={ProjectHelper.getIcon(project.icon)}
            fallback="Folder"
            className="size-4 text-muted-foreground shrink-0"
          />
          <span className="text-sm line-clamp-1">{project.name}</span>
          {currentProject === project.id ? (
            <CheckIcon className="ml-auto size-4 text-muted-foreground shrink-0" />
          ) : null}
        </DropdownMenuItem>
      ))}
    </div>
  );
}
