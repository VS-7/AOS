import React, { createContext, useContext, useMemo, useState } from "react";
import { useNavigate, useRouter } from "@tanstack/react-router";
import type { Project } from "@/features/project/interfaces/project.interfaces";

interface ProjectsPageSearchSchema {
  query?: string;
}

interface ProjectsContextValue {
  projects: Project[];
  filteredProjects: Project[];
  search: ProjectsPageSearchSchema;
  searchDraft: string;
  setSearchDraft: (value: string) => void;
  updateSearch: (next: Partial<ProjectsPageSearchSchema>) => void;
  handleSearchChange: (value: string) => void;
  clearFilters: () => void;
}

const ProjectsContext = createContext<ProjectsContextValue | null>(null);

export function useProjectsContext() {
  const ctx = useContext(ProjectsContext);
  if (!ctx) {
    throw new Error("useProjectsContext must be used within a ProjectsProvider");
  }
  return ctx;
}

interface ProjectsProviderProps {
  children: React.ReactNode;
  projects: Project[];
  search: ProjectsPageSearchSchema;
}

export function ProjectsProvider({ children, projects, search }: ProjectsProviderProps) {
  const navigate = useNavigate();
  const router = useRouter();
  const [searchDraft, setSearchDraft] = useState(search.query ?? "");

  const filteredProjects = useMemo(() => {
    if (!searchDraft.trim()) return projects;
    const q = searchDraft.toLowerCase();
    return projects.filter(
      (p) =>
        p.name.toLowerCase().includes(q) ||
        p.id.toLowerCase().includes(q) ||
        (p.description?.toLowerCase().includes(q) ?? false),
    );
  }, [projects, searchDraft]);

  function updateSearch(next: Partial<ProjectsPageSearchSchema>) {
    navigate({
      to: "/projects",
      search: (prev: Partial<ProjectsPageSearchSchema>) => ({ ...prev, ...next }),
    });
    router.invalidate();
  }

  function handleSearchChange(value: string) {
    setSearchDraft(value);
    updateSearch({ query: value.trim() ? value : undefined });
  }

  function clearFilters() {
    updateSearch({ query: undefined });
    setSearchDraft("");
  }

  const value: ProjectsContextValue = {
    projects,
    filteredProjects,
    search,
    searchDraft,
    setSearchDraft,
    updateSearch,
    handleSearchChange,
    clearFilters,
  };

  return <ProjectsContext.Provider value={value}>{children}</ProjectsContext.Provider>;
}
