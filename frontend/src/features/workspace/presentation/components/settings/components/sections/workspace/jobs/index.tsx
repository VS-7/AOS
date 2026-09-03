import * as React from "react";
import { toast } from "sonner";

import { aos } from "@/app/aos";
import { t } from "@/lib/i18n";
import { useAlert } from "@/components/ui/alert-provider";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  FormSection,
  FormSectionContent,
  FormSectionDescription,
  FormSectionHeader,
  FormSectionItem,
  FormSectionTitle,
} from "@/components/ui/form-section";
import { SettingsSectionShell } from "../../../section-shell";
import type { Job, QueueStats } from "@/features/job/interfaces/job.interfaces";

const STATUS_TONE: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
  pending: "secondary",
  claimed: "default",
  succeeded: "outline",
  failed: "destructive",
  dead: "destructive",
};

/**
 * The words a person reads for each of Go's job states.
 *
 * Keyed in English and translated where they are rendered, not where the
 * table is declared: `t()` in a module runs once, at import, before the
 * locale has been read — which would freeze whichever language happened to
 * be the default inside the constant. The same reason the onboarding
 * wizard's TONES and STYLES tables are written this way.
 */
const STATUS_LABEL: Record<string, string> = {
  pending: "Pending",
  claimed: "Running",
  succeeded: "Succeeded",
  failed: "Failed",
  dead: "Dead",
  retrying: "Retrying",
};

function statusLabel(status: string): string {
  const known = STATUS_LABEL[status];
  return known ? t(known) : status;
}

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
  const { confirm } = useAlert();

  // Through the facade, like every sibling section: the cache is keyed by
  // feature so a realtime event can invalidate it, the loading state comes
  // from the query rather than from a flag this file would have to keep, and
  // the `_reasoning` every call carries is written once, in one place.
  const statsQuery = aos.client.job.stats.useQuery<QueueStats>({
    query: { workspace: workspaceId },
    enabled: Boolean(workspaceId),
  });
  const jobsQuery = aos.client.job.list.useQuery<{ jobs?: Job[] }>({
    query: { workspace: workspaceId, limit: 50 },
    enabled: Boolean(workspaceId),
  });

  const refresh = React.useCallback(() => {
    void statsQuery.refetch();
    void jobsQuery.refetch();
  }, [statsQuery.refetch, jobsQuery.refetch]);

  const { mutate: recoverJobs, loading: isRecovering } = aos.client.job.recover.useMutation({
    onSuccess: (result: any) => {
      toast.success(
        t("{{count}} jobs handed back to the queue.", { count: result?.data?.recovered ?? 0 }),
      );
      refresh();
    },
    onError: (error: any) => {
      toast.error(error?.error?.message ?? error?.message ?? t("The jobs could not be recovered."));
    },
  });

  const { mutate: purgeJobs, loading: isPurging } = aos.client.job.purge.useMutation({
    onSuccess: (result: any) => {
      toast.success(t("{{count}} finished jobs removed.", { count: result?.data?.removed ?? 0 }));
      refresh();
    },
    onError: (error: any) => {
      toast.error(error?.error?.message ?? error?.message ?? t("The queue could not be purged."));
    },
  });

  const purge = async () => {
    // Purge removes finished jobs across the whole installation, not just
    // this workspace — the queue is one database for all of them. That is
    // worth asking about once, the way every other destructive action here
    // does, rather than being discovered from the count in the toast.
    const confirmed = await confirm({
      title: t("Remove finished jobs?"),
      description: t(
        "This removes succeeded and dead jobs older than the retention window, across every workspace of this installation. Work still queued or running is untouched.",
      ),
      confirmText: t("Remove"),
      variant: "destructive",
    });
    if (!confirmed) return;
    void purgeJobs({});
  };

  const stats = statsQuery.data ?? null;
  const jobs = jobsQuery.data?.jobs ?? [];
  const stale = stats?.stale ?? [];
  const dead = stats?.dead ?? [];
  const busy = isRecovering || isPurging;

  return (
    <SettingsSectionShell>
      <FormSection>
        <FormSectionHeader>
          <FormSectionTitle>{t("Jobs")}</FormSectionTitle>
          <FormSectionDescription>
            {t("Every turn, routine and background task runs through this queue.")}
          </FormSectionDescription>
        </FormSectionHeader>

        <FormSectionContent>
          <FormSectionItem>
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground">{t("Queue")}</p>
              <div className="mt-1 flex flex-wrap items-center gap-1.5">
                <Badge variant="secondary">
                  {t("{{count}} total", { count: stats?.total ?? 0 })}
                </Badge>
                {Object.entries(stats?.byStatus ?? {}).map(([status, count]) => (
                  <Badge key={status} variant={STATUS_TONE[status] ?? "outline"}>
                    {statusLabel(status)}: {count}
                  </Badge>
                ))}
              </div>
            </div>

            <div className="flex items-center gap-2">
              <Button
                type="button"
                size="sm"
                variant="secondary"
                disabled={busy || statsQuery.isFetching}
                onClick={refresh}
              >
                {t("Refresh")}
              </Button>
              <Button
                type="button"
                size="sm"
                variant="secondary"
                disabled={busy || stale.length === 0}
                onClick={() => void recoverJobs({})}
              >
                {t("Recover stalled")}
              </Button>
              <Button type="button" size="sm" variant="secondary" disabled={busy} onClick={purge}>
                {t("Purge finished")}
              </Button>
            </div>
          </FormSectionItem>

          {stale.length > 0 ? (
            <FormSectionItem>
              <div className="min-w-0">
                <p className="text-sm font-medium text-destructive">{t("Stalled work")}</p>
                <p className="text-sm text-muted-foreground">
                  {t(
                    "{{count}} jobs are held by a worker that stopped reporting. Recovering hands them back to the queue.",
                    { count: stale.length },
                  )}
                </p>
              </div>
            </FormSectionItem>
          ) : null}

          {dead.length > 0 ? (
            <FormSectionItem>
              <div className="min-w-0">
                <p className="text-sm font-medium text-foreground">{t("Failed work")}</p>
                <p className="text-sm text-muted-foreground">
                  {t("{{count}} jobs failed their last attempt.", { count: dead.length })}
                </p>
              </div>
            </FormSectionItem>
          ) : null}

          <div className="divide-y divide-border">
            {jobsQuery.isLoading ? (
              <p className="p-4 text-sm text-muted-foreground">{t("Loading jobs...")}</p>
            ) : jobs.length === 0 ? (
              <p className="p-4 text-sm text-muted-foreground">{t("The queue is empty.")}</p>
            ) : (
              jobs.map((job) => (
                <div key={job.id} className="flex flex-wrap items-center gap-3 p-4">
                  <Badge variant={STATUS_TONE[job.status] ?? "outline"}>
                    {statusLabel(job.status)}
                  </Badge>
                  <span className="text-sm font-medium text-foreground">{job.kind}</span>
                  <span className="text-sm text-muted-foreground">{job.queue}</span>
                  {job.attempts > 0 ? (
                    <span className="text-sm text-muted-foreground">
                      {t("attempt {{n}} of {{max}}", { n: job.attempts, max: job.maxTries })}
                    </span>
                  ) : null}
                  <span className="ml-auto font-mono text-xs text-muted-foreground">
                    {job.id.slice(0, 8)}
                  </span>
                </div>
              ))
            )}
          </div>
        </FormSectionContent>
      </FormSection>
    </SettingsSectionShell>
  );
}
