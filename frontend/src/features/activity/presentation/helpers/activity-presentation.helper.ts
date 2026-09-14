import { getLocale, t } from "@/lib/i18n";
import { TASK_STATUS_CONFIG } from "@/features/task/presentation/consts/task";

/**
 * How an activity reads and where it leads, shared by the inbox and the
 * notification toast so the two can never disagree.
 *
 * The daemon's own `title` is a line for any reader — the CLI, an agent — and
 * it is English, with raw enums in it ("moved to in_progress"). The interface
 * rebuilds the line from the structured `data` for the events it knows, in the
 * language on screen, and keeps the daemon's line for everything else: an
 * event added in Go still shows up, just untranslated.
 */

/** The fields of an activity this module reads; the inbox and the realtime payload both have them. */
export interface ActivityLike {
  id: string;
  namespace: string;
  event: string;
  title: string;
  data?: Record<string, unknown>;
}

/** A route the router can navigate to. */
export interface ActivityTarget {
  to: string;
  params?: Record<string, string>;
}

function text(data: Record<string, unknown> | undefined, key: string): string {
  const value = data?.[key];
  return typeof value === "string" ? value.trim() : "";
}

function taskStatusLabel(status: string): string {
  return (TASK_STATUS_CONFIG as Record<string, { label: string } | undefined>)[status]?.label ?? status;
}

/** The kinds of record whose created/updated/deleted lines are built the same way. */
const RECORD_LINES: Record<string, { created: string; deleted: string; nameKey: string }> = {
  project: { created: "New project: {{name}}", deleted: "Deleted project: {{name}}", nameKey: "name" },
  goal: { created: "New goal: {{name}}", deleted: "Deleted goal: {{name}}", nameKey: "title" },
  agent: { created: "New agent: {{name}}", deleted: "Deleted agent: {{name}}", nameKey: "name" },
};

/** The line a person reads for this activity, in the interface's language. */
export function activityTitle(activity: ActivityLike): string {
  const { namespace, event, data } = activity;

  if (namespace === "task") {
    const name = text(data, "name");
    if (!name) return activity.title;
    switch (event) {
      case "status_changed": {
        const status = text(data, "to") || text(data, "status");
        return status ? t("{{name}} moved to {{status}}", { name, status: taskStatusLabel(status) }) : activity.title;
      }
      case "created":
        return t("New task: {{name}}", { name });
      case "updated":
        return t("{{name}} updated", { name });
      case "deleted":
        return t("Deleted task: {{name}}", { name });
      case "branched":
        return t("{{name}} was branched", { name });
      default:
        return activity.title;
    }
  }

  const record = RECORD_LINES[namespace];
  if (record) {
    const name = text(data, record.nameKey) || text(data, "name");
    if (!name) return activity.title;
    switch (event) {
      case "created":
        return t(record.created, { name });
      case "updated":
        return t("{{name}} updated", { name });
      case "deleted":
        return t(record.deleted, { name });
      default:
        return activity.title;
    }
  }

  return activity.title;
}

/** Namespaces whose payload names the record, and where that record's page is. */
const RECORD_ROUTES: Record<string, { key: string; detail: string; list: string }> = {
  task: { key: "task", detail: "/tasks/$id", list: "/tasks" },
  goal: { key: "goal", detail: "/goals/$id", list: "/goals" },
  project: { key: "project", detail: "/projects/$id", list: "/projects" },
  routine: { key: "routine", detail: "/routines/$id", list: "/routines" },
  chat: { key: "chat", detail: "/chats/$id", list: "/activities" },
};

/**
 * Where opening this activity takes the person: the record it is about, its
 * list when the record was deleted or is not named, and the inbox itself when
 * there is no page for it at all.
 */
export function activityTarget(activity: ActivityLike): ActivityTarget {
  const route = RECORD_ROUTES[activity.namespace];
  if (route) {
    const id = text(activity.data, route.key);
    if (!id || activity.event === "deleted") return { to: route.list };
    return { to: route.detail, params: { id } };
  }
  if (activity.namespace === "agent") {
    return { to: "/settings/$group/$section", params: { group: "workspace", section: "agents" } };
  }
  return { to: "/activities" };
}

/**
 * Whether the signed-in person caused this activity themselves.
 *
 * Their own create or move already got its own confirmation on the screen
 * they did it from; announcing it again as a notification put two toasts (and
 * a sound) on every click. An agent acting on their behalf is an agent, not
 * them, and is still announced.
 */
export function isOwnActivity(
  activity: { actor?: string; actorType?: string } | undefined,
  selfId: string | undefined,
): boolean {
  return !!activity && !!selfId && activity.actorType === "user" && activity.actor === selfId;
}

function startOfDay(date: Date): number {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
}

/**
 * The day heading an activity is grouped under: Today, Yesterday, or the
 * date. By calendar day — the same rule for the heading and anything else that
 * says which day an entry belongs to.
 */
export function activityDayLabel(createdAt: string, now: Date = new Date()): string {
  const date = new Date(createdAt);
  const days = Math.round((startOfDay(now) - startOfDay(date)) / 86_400_000);
  if (days === 0) return t("Today");
  if (days === 1) return t("Yesterday");
  return new Intl.DateTimeFormat(getLocale(), { weekday: "long", month: "short", day: "numeric" }).format(date);
}

/**
 * The time of day on each row. The heading already says which day; the row
 * repeating it ("Today" fifty times in a row) told the reader nothing.
 */
export function activityTimeLabel(createdAt: string): string {
  return new Intl.DateTimeFormat(getLocale(), { hour: "numeric", minute: "2-digit" }).format(new Date(createdAt));
}
