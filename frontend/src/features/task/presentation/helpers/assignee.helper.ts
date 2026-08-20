import type { TaskAssignee } from "@/features/task/interfaces/task.interfaces";
import type {
  WorkspaceDirectoryAgent,
  WorkspaceDirectoryUser,
} from "@/features/workspace/interfaces/directory.interfaces";
import type { AuthSelfProfile } from "@/features/auth/presentation/stores/auth.store";

/**
 * Display-ready identity for a task assignee.
 *
 * `type` mirrors the backend projection: `agent` (autonomous dispatch),
 * `user` (human-owned), or `unknown` (unresolvable id — safe, never dispatched).
 */
export interface AssigneeIdentity {
  id: string;
  type: "user" | "agent" | "unknown";
  name: string;
  image?: string;
  username?: string;
  role?: string;
}

/** Local directory snapshot used to resolve assignee ids in the UI. */
export interface AssigneeDirectoryInput {
  users: WorkspaceDirectoryUser[];
  agents: WorkspaceDirectoryAgent[];
  /** Current authenticated user — directory excludes the viewer, so self-assign needs it. */
  self?: AuthSelfProfile | null;
}

const SELF_REFERENCE = "__me__";

function normalizeUser(user: {
  id: string;
  name: string;
  username?: string;
  image?: string;
  role?: string;
}): AssigneeIdentity {
  return {
    id: user.id,
    type: "user",
    name: user.name,
    username: user.username,
    image: user.image,
    role: user.role,
  };
}

function findUser(
  directory: AssigneeDirectoryInput,
  ref: string,
): AssigneeIdentity | undefined {
  const member = directory.users.find(
    (user) =>
      user.id === ref || user.username === ref || user.email === ref,
  );
  if (member) {
    return normalizeUser(member);
  }

  const self = directory.self;
  if (
    self &&
    (self.id === ref ||
      self.username === ref ||
      self.email === ref)
  ) {
    return normalizeUser(self);
  }

  return undefined;
}

/**
 * Resolves a persisted assignee id into a display identity.
 *
 * Order: agent id → workspace member → current user → unknown. The legacy
 * `__me__` sentinel maps to the current user so old records still render a
 * name until re-saved.
 */
export function resolveAssignee(
  directory: AssigneeDirectoryInput,
  assigned: string | undefined,
): AssigneeIdentity | null {
  if (!assigned) {
    return null;
  }

  if (assigned === SELF_REFERENCE) {
    return directory.self
      ? normalizeUser(directory.self)
      : { id: SELF_REFERENCE, type: "unknown", name: "You" };
  }

  const agent = directory.agents.find((candidate) => candidate.id === assigned);
  if (agent) {
    return {
      id: agent.id,
      type: "agent",
      name: agent.name,
      image: agent.image,
      role: agent.role,
    };
  }

  return findUser(directory, assigned) ?? {
    id: assigned,
    type: "unknown",
    name: assigned,
  };
}

/**
 * Resolves a task into its assignee identity, preferring the backend rich
 * projection (`task.assignee`) when present and falling back to the local
 * directory for raw list records.
 */
export function resolveTaskAssignee(
  directory: AssigneeDirectoryInput,
  task: {
    assigned?: string;
    assignee?: TaskAssignee | null;
  },
): AssigneeIdentity | null {
  const projection = task.assignee;
  if (projection) {
    return {
      id: projection.id,
      type: projection.type,
      // Go's `ResolvedAssignee.Name` is `omitempty` (optional); `id` is
      // the same id-as-name fallback `resolveAssignee` below already uses
      // for an unresolved assignee.
      name: projection.name ?? projection.id,
      image: projection.image,
      username: projection.username,
      role: projection.role,
    };
  }

  return resolveAssignee(directory, task.assigned);
}

/** People candidates for the assignee dropdown (members + current user). */
export function assigneePeople(
  directory: AssigneeDirectoryInput,
): AssigneeIdentity[] {
  const people = directory.users.map(normalizeUser);

  const self = directory.self;
  if (self && !people.some((person) => person.id === self.id)) {
    people.push(normalizeUser(self));
  }

  return people;
}

/** Agent candidates for the assignee dropdown. */
export function assigneeAgents(
  directory: AssigneeDirectoryInput,
): AssigneeIdentity[] {
  return directory.agents.map((agent) => ({
    id: agent.id,
    type: "agent",
    name: agent.name,
    image: agent.image,
    role: agent.role,
  }));
}

/** Deterministic 1-2 letter initials for avatar fallbacks. */
export function assigneeInitials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) {
    return "?";
  }
  if (parts.length === 1) {
    return parts[0]!.slice(0, 2).toUpperCase();
  }
  return (parts[0]![0] + parts[parts.length - 1]![0]).toUpperCase();
}
