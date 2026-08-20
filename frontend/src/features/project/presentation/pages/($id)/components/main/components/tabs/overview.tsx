import * as React from "react";
import { Icon } from "@/components/ui/icon";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { MarkdownEditor } from "@/components/ui/markdown-editor";
import { Skeleton } from "@/components/ui/skeleton";
import { SlidingNumber } from "@/components/ui/sliding-number";
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { FolderInput } from "@/components/ui/folder-input";
import { useDelayedLoading } from "@/hooks/use-delayed-loading.hook";
import { aos } from "@/app/aos";
import type { Project } from "@/features/project/interfaces/project.interfaces";
import type { Task } from "@/features/task/interfaces/task.interfaces";

interface ProjectOverviewTabProps {
  form: any;
  project?: Project | null;
}

// ─── Apple-accent palette (macOS 26) ─────────────────────────────────────
const CARD_ACCENTS = [
  {
    icon: "ClipboardList",
    label: "Total Tasks",
    color: "oklch(0.62 0.16 258)",
  },
  { icon: "CircleDot", label: "Open", color: "oklch(0.72 0.17 72)" },
  { icon: "Timer", label: "In Progress", color: "oklch(0.66 0.16 202)" },
  { icon: "CheckCircle2", label: "Done", color: "oklch(0.70 0.18 146)" },
] as const;

function tint(color: string, amount: number): string {
  return `color-mix(in srgb, ${color} ${Math.round(amount * 100)}%, transparent)`;
}

// ─── KPI Components ──────────────────────────────────────────────────────
interface KpiCardProps {
  icon: string;
  label: string;
  value: number;
  progress: number;
  accentColor: string;
}

function KpiCard({ icon, label, value, progress, accentColor }: KpiCardProps) {
  return (
    <div className="group relative overflow-hidden p-5">
      <div
        className="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"
        style={{
          background: `radial-gradient(120% 80% at 50% 0%, ${tint(accentColor, 0.07)} 0%, transparent 70%)`,
        }}
      />
      <Icon value={icon} className="size-5 mb-8 opacity-60" />
      <div className="relative flex items-baseline gap-0.5">
        <div className="text-3xl font-light tracking-tight tabular-nums text-foreground">
          <SlidingNumber value={value} />
        </div>
      </div>
      <div className="relative mt-1.5 text-[11px] font-semibold text-muted-foreground/70 tracking-[0.08em] uppercase">
        {label}
      </div>
      <div className="relative mt-4 h-0.75 w-full overflow-hidden rounded-full bg-muted/60">
        <div
          className="h-full rounded-full transition-all duration-700 ease-out"
          style={{
            width: `${Math.min(progress, 100)}%`,
            backgroundColor: accentColor,
          }}
        />
      </div>
    </div>
  );
}

function KpiSkeleton() {
  return (
    <div className="grid grid-cols-4 bg-card/5 rounded-2xl border divide-x">
      {Array.from({ length: 4 }).map((_, i) => (
        <div key={i} className="relative overflow-hidden p-5">
          <Skeleton className="size-5 rounded mb-8" />
          <div className="relative space-y-1.5">
            <Skeleton className="h-9 w-16 rounded" />
            <Skeleton className="h-3 w-20 rounded" />
          </div>
          <Skeleton className="relative mt-4 h-0.75 w-full rounded-full" />
        </div>
      ))}
    </div>
  );
}

interface ProjectKpiCardsProps {
  projectId: string;
}

function ProjectKpiCards({ projectId }: ProjectKpiCardsProps) {
  const taskQuery = aos.client.task.list.useQuery({
    query: { project: [projectId], limit: "200" },
    staleTime: 5 * 60 * 1000,
  });

  const tasks: Task[] = taskQuery.data?.tasks ?? [];
  const isLoading = useDelayedLoading(taskQuery.isLoading);

  const metrics = React.useMemo(() => {
    const total = tasks.length;
    const done = tasks.filter((t) => t.status === "finished").length;
    const inProgress = tasks.filter((t) => t.status === "in_progress").length;
    const open = total - done;
    return {
      total,
      open,
      inProgress,
      done,
      completion: total > 0 ? Math.round((done / total) * 100) : 0,
    };
  }, [tasks]);

  const cardValues = React.useMemo(
    () => [metrics.total, metrics.open, metrics.inProgress, metrics.done],
    [metrics],
  );
  const cardProgress = React.useMemo(
    () => [
      100,
      metrics.total > 0 ? (metrics.open / metrics.total) * 100 : 0,
      metrics.total > 0 ? (metrics.inProgress / metrics.total) * 100 : 0,
      metrics.total > 0 ? (metrics.done / metrics.total) * 100 : 0,
    ],
    [metrics],
  );

  return taskQuery.isLoading && isLoading ? (
    <KpiSkeleton />
  ) : (
    <div className="grid grid-cols-4 bg-card/5 rounded-2xl border divide-x">
      {CARD_ACCENTS.map((card, i) => (
        <KpiCard
          key={card.label}
          icon={card.icon}
          label={card.label}
          value={cardValues[i]}
          progress={cardProgress[i]}
          accentColor={card.color}
        />
      ))}
    </div>
  );
}

// ─── Main Overview Tab ───────────────────────────────────────────────────
export function ProjectOverviewTab({ form, project }: ProjectOverviewTabProps) {
  return (
    <div className="container mx-auto max-w-3xl py-6 pb-10 space-y-6">
      {/* ── KPI Cards (edit mode only) ──────────────────────── */}
      {project && <ProjectKpiCards projectId={project.id} />}

      <FormField
        control={form.control}
        name="name"
        render={({ field }) => (
          <FormItem className="space-y-2">
            <FormLabel className="opacity-60">Name</FormLabel>
            <FormControl>
              <Input
                placeholder="AOS OS"
                className="h-auto rounded-none border-0 bg-transparent px-0 py-0 text-2xl font-semibold shadow-none focus-visible:ring-0"
                {...field}
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name="description"
        render={({ field }) => (
          <FormItem className="space-y-2">
            <FormLabel className="opacity-60">Description</FormLabel>
            <FormControl>
              <Textarea
                placeholder="What is this project about?"
                className="min-h-10 max-h-48 resize-none rounded-none border-0 bg-transparent px-0 py-0 text-sm shadow-none focus-visible:ring-0 overflow-y-auto"
                {...field}
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      <div className="grid gap-6 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <FormField
          control={form.control}
          name="source"
          render={({ field }) => (
            <FormItem className="space-y-2 md:col-span-2">
              <FormLabel className="opacity-60">Source</FormLabel>
              <FormControl>
                <FolderInput
                  placeholder="/absolute/path/to/project"
                  inputClassName="border-0 rounded-none p-0 outline-0"
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>

      <FormField
        control={form.control}
        name="content"
        render={({ field }) => (
          <FormItem>
            <FormLabel className="opacity-60">Content</FormLabel>
            <MarkdownEditor
              value={field.value ?? ""}
              onValueChange={field.onChange}
              title="Content"
              placeholder="Add context, notes, or documentation for this project..."
            />
            <FormMessage />
          </FormItem>
        )}
      />
    </div>
  );
}
