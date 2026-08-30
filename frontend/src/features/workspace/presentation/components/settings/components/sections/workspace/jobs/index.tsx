import * as React from "react";
import { toast } from "sonner";

import { aos } from "@/app/aos";
import { client } from "@/lib/client";
import { t } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  FormSection,
  FormSectionContent,
  FormSectionDescription,
  FormSectionHeader,
  FormSectionTitle,
} from "@/components/ui/form-section";

interface QueueStats {
  total: number;
  byStatus?: Record<string, number>;
  byQueue?: Record<string, number>;
  dead?: string[];
  stale?: string[];
  at?: string;
}

interface JobRow {
  id: string;
  queue: string;
  kind: string;
  status: string;
  attempts: number;
  maxTries: number;
  runAt?: string;
}

const STATUS_TONE: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
  pending: "secondary",
  claimed: "default",
  succeeded: "outline",
  failed: "destructive",
  dead: "destructive",
};

/**
 * The execution queue, which was invisible.
 *
 * Every turn an agent takes, every routine that fires and every task the
 * system runs on its own goes through this queue, and the window showed
 * nothing between "asked" and "answered" — a job that was retrying, or dead,
 * or held by a worker that stopped reporting, looked exactly like a job that
 * was simply taking a while.
 *
 * `stale` is the one that matters most and the reason `recover` is a button
 * here: a job still marked claimed whose lease has lapsed is not busy, its
 * worker is gone, and recovering is what hands it back to the queue. Nothing
 * in the interface could say that, let alone do it.
 */
export function WorkspaceJobsSection(): React.JSX.Element {
  const workspaceId = aos.stores.workspace.useState((state) => state.current?.id);
  const [stats, setStats] = React.useState<QueueStats | null>(null);
  const [jobs, setJobs] = React.useState<JobRow[]>([]);
  const [busy, setBusy] = React.useState(false);

  const refresh = React.useCallback(async () => {
    if (!workspaceId) return;
    try {
      const [statsAnswer, listAnswer] = await Promise.all([
        client.invoke("jobs_stats", {
          workspace: workspaceId,
          _reasoning: "the settings screen is showing the shape of the execution queue",
        }) as Promise<QueueStats | undefined>,
        client.invoke("jobs_list", {
          workspace: workspaceId,
          limit: 50,
          _reasoning: "the settings screen is listing what the queue holds",
        }) as Promise<{ jobs?: JobRow[] } | undefined>,
      ]);
      setStats(statsAnswer ?? null);
      setJobs(listAnswer?.jobs ?? []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("The queue could not be read."));
    }
  }, [workspaceId]);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  const recover = async () => {
    setBusy(true);
    try {
      const answer = (await client.invoke("jobs_recover", {
        _reasoning: "a person is handing back the jobs whose worker stopped reporting",
      })) as { recovered?: number } | undefined;
      toast.success(
        t("{{count}} jobs handed back to the queue.").replace("{{count}}", String(answer?.recovered ?? 0)),
      );
      await refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("The jobs could not be recovered."));
    } finally {
      setBusy(false);
    }
  };

  const purge = async () => {
    setBusy(true);
    try {
      const answer = (await client.invoke("jobs_purge", {
        _reasoning: "a person is clearing finished jobs older than the retention window",
      })) as { removed?: number } | undefined;
      toast.success(
        t("{{count}} finished jobs removed.").replace("{{count}}", String(answer?.removed ?? 0)),
      );
      await refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("The queue could not be purged."));
    } finally {
      setBusy(false);
    }
  };

  const stale = stats?.stale ?? [];
  const dead = stats?.dead ?? [];

  return (
    <FormSection>
      <FormSectionHeader>
        <FormSectionTitle>{t("Jobs")}</FormSectionTitle>
        <FormSectionDescription>
          {t("Every turn, routine and background task runs through this queue.")}
        </FormSectionDescription>
      </FormSectionHeader>
      <FormSectionContent>
        <div className="space-y-4">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="secondary">
              {t("{{count}} total").replace("{{count}}", String(stats?.total ?? 0))}
            </Badge>
            {Object.entries(stats?.byStatus ?? {}).map(([status, count]) => (
              <Badge key={status} variant={STATUS_TONE[status] ?? "outline"}>
                {status}: {count}
              </Badge>
            ))}
            <div className="ml-auto flex gap-2">
              <Button size="sm" variant="secondary" disabled={busy} onClick={refresh}>
                {t("Refresh")}
              </Button>
              <Button size="sm" variant="secondary" disabled={busy || stale.length === 0} onClick={recover}>
                {t("Recover stalled")}
              </Button>
              <Button size="sm" variant="secondary" disabled={busy} onClick={purge}>
                {t("Purge finished")}
              </Button>
            </div>
          </div>

          {stale.length > 0 ? (
            <p className="rounded-lg border border-destructive/40 bg-destructive/5 p-3 text-sm">
              {t(
                "{{count}} jobs are held by a worker that stopped reporting. Recovering hands them back to the queue.",
              ).replace("{{count}}", String(stale.length))}
            </p>
          ) : null}
          {dead.length > 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("{{count}} jobs failed their last attempt.").replace("{{count}}", String(dead.length))}
            </p>
          ) : null}

          <div className="overflow-auto rounded-lg border border-border/60">
            {jobs.length === 0 ? (
              <p className="p-4 text-sm text-muted-foreground">{t("The queue is empty.")}</p>
            ) : (
              <ul className="divide-y divide-border/60">
                {jobs.map((job) => (
                  <li key={job.id} className="flex items-center gap-3 p-3 text-sm">
                    <Badge variant={STATUS_TONE[job.status] ?? "outline"}>{job.status}</Badge>
                    <span className="font-medium">{job.kind}</span>
                    <span className="text-xs text-muted-foreground">{job.queue}</span>
                    {job.attempts > 0 ? (
                      <span className="text-xs text-muted-foreground">
                        {t("attempt {{n}} of {{max}}")
                          .replace("{{n}}", String(job.attempts))
                          .replace("{{max}}", String(job.maxTries))}
                      </span>
                    ) : null}
                    <span className="ml-auto font-mono text-[11px] text-muted-foreground">
                      {job.id.slice(0, 8)}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      </FormSectionContent>
    </FormSection>
  );
}
