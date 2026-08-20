import { z } from "zod";
import type { ResponseWithCTA } from "@/core/interfaces/response.interfaces";
import { Schema } from "@/core/helpers/schema.helper";

/**
 * Zod schema for a AOS Project.
 *
 * Projects represent top-level workspaces that can be linked to external directories
 * via symlinks or materialized as internal directories under `.aos/projects/{id}/`.
 *
 * @description Defines the full data model for a project record persisted in the collection.
 * The `content` field (Markdown body) is handled natively by Igniter Collections and omitted from the schema.
 */
export const ProjectSchema = z.object({
  /** Unique slug identifier for the project, auto-generated from name via Slug.generate. */
  id: z.string(),
  /** Human-readable name of the project. */
  name: z.string(),
  /** Lucide icon name string (e.g., "Folder", "Rocket"). Validated only on the frontend via the Icon component. */
  icon: z.string().optional(),
  /** Short description stored as YAML frontmatter field. */
  description: z.string().optional(),
  /** Markdown body content of the project, stored as the file body in YAML frontmatter collections. */
  content: z.string().optional(),
  /** Absolute path to the project's directory on the host machine. Creates a symlink when provided. */
  path: z.string().optional(),
  /**
   * Absent from this file's source — reconstructed from `presentation/
   * pages/($id)/index.tsx`'s upsert form and `.../tabs/files.tsx`'s file
   * browser, both of which read/write `project.source` as the linked
   * source directory (workspace-relative, unlike `path` above).
   */
  source: z.string().optional(),
  /** ISO timestamp of when the project was created. Auto-generated. */
  createdAt: z.string(),
  /** ISO timestamp of when the project was last updated. Auto-generated. */
  updatedAt: z.string(),
});

/**
 * Zod schema for creating a new Project.
 *
 * @description Omits `id` (auto-generated from name via slug), `createdAt`, and `updatedAt` (auto-generated).
 * The `name` field is required.
 */
export const ProjectCreateSchema = z.object({
  /** Human-readable name of the project. Required. */
  name: z.string(),
  /** Lucide icon name string. Optional. */
  icon: z.string().optional(),
  /** Short description stored as YAML frontmatter field. Optional. */
  description: z.string().optional(),
  /** Markdown body content of the project. Optional. */
  content: z.string().optional(),
  /** Absolute path on the host machine. If provided, creates a symlink. Optional. */
  path: z.string().optional(),
});

/**
 * Zod schema for updating an existing Project.
 *
 * @description All fields are partial except `id`. Use `.partial()` to allow updating only changed fields.
 */
export const ProjectUpdateSchema = z.object({
  /** Human-readable name of the project. Optional. */
  name: z.string().optional(),
  /** Lucide icon name string. Optional. */
  icon: z.string().optional(),
  /** Short description stored as YAML frontmatter field. Optional. */
  description: z.string().optional(),
  /** Markdown body content of the project. Optional. */
  content: z.string().optional(),
  /** Absolute path on the host machine. If changed, symlinks are updated accordingly. Optional. */
  path: z.string().optional().nullable(),
});

/**
 * Zod schema for querying/listing Projects.
 *
 * @description Supports optional full-text search, limit, and offset parameters.
 */
export const ProjectListQuerySchema = Schema.object({
  /** Full-text search query across project fields. */
  query: z.string().optional(),
  /** Maximum number of results to return. */
  limit: z.string().optional(),
  /** Number of results to skip for pagination. */
  offset: z.string().optional(),
});

/** Inferred TypeScript type for a full Project record. */
export type Project = z.infer<typeof ProjectSchema>;

/** Inferred TypeScript type for creating a new Project. */
export type ProjectCreateInput = z.infer<
  typeof ProjectCreateSchema
>;

/** Inferred TypeScript type for updating an existing Project. */
export type ProjectUpdateInput = z.infer<
  typeof ProjectUpdateSchema
>;

/** Inferred TypeScript type for querying/listing Projects. */
export type ProjectListQueryInput = z.infer<
  typeof ProjectListQuerySchema
>;

/** Parameters for retrieving a single Project by its ID. */
export interface ProjectGetParams {
  /** The unique slug identifier of the project. */
  id: string;
}

/** Parameters for updating an existing Project. */
export interface ProjectUpdateParams {
  /** The unique slug identifier of the project. */
  id: string;
  /** Partial data to update the project with. */
  data: ProjectUpdateInput;
}

/** Parameters for deleting a Project. */
export interface ProjectDeleteParams {
  /** The unique slug identifier of the project. */
  id: string;
}

/**
 * Contract for the ProjectService managing the full CRUD lifecycle for Project records.
 *
 * @description Defines the public API surface for project operations including
 * collection persistence, symlink/directory management, and CTA-enriched responses.
 */
export interface IProjectService {
  /**
   * Lists all projects, optionally filtered by a search query and pagination parameters.
   *
   * @param params - Optional query, limit, and offset parameters.
   * @returns A CTA-enriched response containing the list of projects.
   *
   * @example
   * ```typescript
   * const result = await projectService.list({ query: "my-app" });
   * console.log(result.projects);
   * ```
   */
  list(
    params?: ProjectListQueryInput,
  ): Promise<ResponseWithCTA<{ projects: Project[] }>>;

  /**
   * Retrieves a single project by its unique slug identifier.
   *
   * @param params - Contains the `id` of the project to retrieve.
   * @returns A CTA-enriched response containing the project record.
   * @throws {ProjectError} with code `AOS_PROJECT_NOT_FOUND` if the project does not exist.
   *
   * @example
   * ```typescript
   * const result = await projectService.getById({ id: "my-app" });
   * console.log(result.project.name);
   * ```
   */
  getById(
    params: ProjectGetParams,
  ): Promise<ResponseWithCTA<{ project: Project }>>;

  /**
   * Creates a new project, generating its ID from the name via slugification.
   * Handles symlink creation or directory materialization based on the `path` field.
   *
   * @param params - The creation payload (name is required, path/icon/description are optional).
   * @returns A CTA-enriched response containing the created project.
   * @throws {ProjectError} with code `AOS_PROJECT_PERSISTENCE_ERROR` on collection failure.
   * @throws {ProjectError} with code `AOS_PROJECT_SYMLINK_FAILED` on symlink failure.
   *
   * @example
   * ```typescript
   * const result = await projectService.create({ name: "My App", path: "/home/user/projects/my-app" });
   * console.log(result.project.id); // "my-app"
   * ```
   */
  create(
    params: ProjectCreateInput,
  ): Promise<ResponseWithCTA<{ project: Project }>>;

  /**
   * Updates an existing project, handling symlink/directory transitions if `path` changes.
   *
   * @param params - Contains the `id` and partial `data` to update.
   * @returns A CTA-enriched response containing the updated project.
   * @throws {ProjectError} with code `AOS_PROJECT_NOT_FOUND` if the project does not exist.
   *
   * @example
   * ```typescript
   * const result = await projectService.update({ id: "my-app", data: { name: "My Updated App" } });
   * ```
   */
  update(
    params: ProjectUpdateParams,
  ): Promise<ResponseWithCTA<{ project: Project }>>;

  /**
   * Permanently deletes a project, removing its symlink or directory and collection record.
   *
   * @param params - Contains the `id` of the project to delete.
   * @returns A CTA-enriched confirmation response.
   * @throws {ProjectError} with code `AOS_PROJECT_NOT_FOUND` if the project does not exist.
   *
   * @example
   * ```typescript
   * await projectService.delete({ id: "my-app" });
   * ```
   */
  delete(params: ProjectDeleteParams): Promise<ResponseWithCTA>;
}
