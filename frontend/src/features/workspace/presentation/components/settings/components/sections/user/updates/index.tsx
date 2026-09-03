import * as React from "react";
import { toast } from "sonner";

import { aos } from "@/app/aos";
import { system } from "@/lib/client";
import { isDesktopWindow } from "@/lib/wails";
import { t } from "@/lib/i18n";
import { useAlert } from "@/components/ui/alert-provider";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  FormSection,
  FormSectionContent,
  FormSectionDescription,
  FormSectionFooter,
  FormSectionHeader,
  FormSectionItem,
  FormSectionTitle,
} from "@/components/ui/form-section";
import { SettingsSectionShell } from "../../../section-shell";
import type {
  CheckResult,
  GatewayState,
  Staged,
  UpdateStatus,
} from "@/features/update/interfaces/update.interfaces";

/**
 * Keeping this installation current.
 *
 * The daemon has had the whole flow since it was written — `update_check`
 * (which never downloads), `update_download` (which verifies the checksums
 * file's signature against the embedded public key *before* fetching a single
 * asset, and stages nothing on a mismatch), and `update_apply` (which waits
 * for in-flight turns, swaps the binaries, restarts, and rolls every one of
 * them back if the daemon does not come back healthy). None of the four was
 * reachable from the window: a desktop application that could not tell you a
 * new version existed, let alone install it.
 *
 * The three steps are deliberately three buttons rather than one "Update
 * now". Downloading is the step that touches the network and can refuse on a
 * bad signature; applying is the step that restarts the daemon underneath a
 * running window. Collapsing them would hide which one failed, and the
 * failures are exactly what somebody needs to see.
 */
function UpdatesPanel(): React.JSX.Element {
  const [check, setCheck] = React.useState<CheckResult | null>(null);
  const [staged, setStaged] = React.useState<Staged | null>(null);

  const statusQuery = aos.client.update.status.useQuery<UpdateStatus>();

  const { mutate: runCheck, loading: isChecking } = aos.client.update.check.useMutation({
    onSuccess: (result: any) => {
      const answer = result?.data as CheckResult | undefined;
      setCheck(answer ?? null);
      setStaged(null);
      if (answer?.upToDate) toast.success(t("You are on the newest release."));
      void statusQuery.refetch();
    },
    onError: (error: any) => {
      toast.error(
        error?.error?.message ?? error?.message ?? t("The release channel could not be reached."),
      );
    },
  });

  const { mutate: runDownload, loading: isDownloading } = aos.client.update.download.useMutation({
    onSuccess: (result: any) => {
      setStaged((result?.data?.staged as Staged | undefined) ?? null);
      toast.success(t("Downloaded and verified."));
    },
    onError: (error: any) => {
      // A signature or checksum failure lands here, and it is the one message
      // in this screen that must not be softened.
      toast.error(
        error?.error?.message ?? error?.message ?? t("The download could not be verified."),
      );
    },
  });

  const { mutate: runApply, loading: isApplying } = aos.client.update.apply.useMutation({
    onSuccess: () => {
      toast.success(t("Installed. The daemon restarted on the new version."));
      setCheck(null);
      setStaged(null);
      void statusQuery.refetch();
    },
    onError: (error: any) => {
      toast.error(error?.error?.message ?? error?.message ?? t("The update could not be applied."));
    },
  });

  const status = statusQuery.data ?? null;
  const busy = isChecking || isDownloading || isApplying;

  return (
    <>
      <FormSection>
        <FormSectionHeader>
          <FormSectionTitle>{t("Updates")}</FormSectionTitle>
          <FormSectionDescription>
            {t("Check the release channel, verify what it offers, and install it.")}
          </FormSectionDescription>
        </FormSectionHeader>

        <FormSectionContent>
          <FormSectionItem>
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground">{t("Running")}</p>
              <p className="text-sm text-muted-foreground">
                {status?.checkedAt
                  ? t("Last checked {{when}}.", { when: status.checkedAt })
                  : t("This installation has not been checked against the release channel yet.")}
              </p>
            </div>

            <div className="flex items-center gap-2">
              <Badge variant="secondary">{status?.current ?? "—"}</Badge>
              {status?.channel ? <Badge variant="outline">{status.channel}</Badge> : null}
              <Button type="button" size="sm" disabled={busy} onClick={() => void runCheck({})}>
                {isChecking ? t("Checking…") : t("Check for updates")}
              </Button>
            </div>
          </FormSectionItem>

          {check?.upToDate ? (
            <FormSectionItem>
              <p className="text-sm text-muted-foreground">{t("You are on the newest release.")}</p>
            </FormSectionItem>
          ) : null}
        </FormSectionContent>
      </FormSection>

      {check && !check.upToDate && check.release ? (
        <FormSection>
          <FormSectionHeader>
            <FormSectionTitle>
              {t("{{version}} is available", { version: check.release.version })}
            </FormSectionTitle>
            <FormSectionDescription>
              {staged
                ? t("Verified and staged. Installing restarts the daemon; in-flight work finishes first.")
                : t("Nothing is installed until you download it and the signature checks out.")}
            </FormSectionDescription>
          </FormSectionHeader>

          <FormSectionContent>
            <FormSectionItem>
              <div className="min-w-0">
                <p className="text-sm font-medium text-foreground">{t("Release notes")}</p>
                {check.release.notes ? (
                  <pre className="mt-1 max-h-40 overflow-auto whitespace-pre-wrap text-sm text-muted-foreground">
                    {check.release.notes}
                  </pre>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    {t("This release carries no notes.")}
                  </p>
                )}
              </div>
              <Badge variant="outline">{check.channel}</Badge>
            </FormSectionItem>
          </FormSectionContent>

          <FormSectionFooter className="flex justify-end gap-2">
            <Button
              type="button"
              size="sm"
              variant="secondary"
              disabled={busy || Boolean(staged)}
              onClick={() => void runDownload({ body: { release: check.release } })}
            >
              {isDownloading ? t("Downloading…") : t("Download and verify")}
            </Button>
            <Button
              type="button"
              size="sm"
              disabled={busy || !staged}
              onClick={() => void runApply({ body: { staged } })}
            >
              {isApplying ? t("Installing…") : t("Install and restart")}
            </Button>
          </FormSectionFooter>
        </FormSection>
      ) : null}
    </>
  );
}

/**
 * What the daemon says about itself.
 *
 * Only status and restart. Starting a daemon that is already answering this
 * very call is meaningless, and stopping it would have the window cut the
 * connection it is speaking over — supervision belongs to whatever launched
 * the daemon, not to a panel inside the thing being supervised.
 *
 * Restart earns its place, and it goes through the window's own supervisor
 * rather than through `gateway_restart`: asked over HTTP, the daemon would
 * signal its own pid, terminate mid-request, answer nothing, and never come
 * back. It refuses that now (AOS_GATEWAY_SELF_RESTART), and the process that
 * launched it does the work.
 */
export function DaemonStatusPanel(): React.JSX.Element {
  const { confirm } = useAlert();
  const [isRestarting, setRestarting] = React.useState(false);
  const stateQuery = aos.client.gateway.status.useQuery<GatewayState>();
  const state = stateQuery.data ?? null;

  const restart = async () => {
    const confirmed = await confirm({
      title: t("Restart the daemon?"),
      description: t(
        "Work in flight finishes first. The window reconnects on its own once the daemon is back.",
      ),
      confirmText: t("Restart"),
    });
    if (!confirmed) return;

    setRestarting(true);
    try {
      await system.restartDaemon();
      toast.success(t("The daemon is restarting."));
      // The daemon drops this connection while it comes back, so the first
      // status read after a restart is expected to fail. Giving it a beat is
      // the difference between showing "stopped" for a second and showing
      // the truth.
      setTimeout(() => void stateQuery.refetch(), 2500);
    } catch (error: any) {
      toast.error(error?.message ?? t("The daemon could not be restarted."));
    } finally {
      setRestarting(false);
    }
  };

  return (
    <FormSection>
      <FormSectionHeader>
        <FormSectionTitle>{t("Daemon")}</FormSectionTitle>
        <FormSectionDescription>
          {t("The process that owns the workspace. The window is a client of it.")}
        </FormSectionDescription>
      </FormSectionHeader>

      <FormSectionContent>
        <FormSectionItem>
          <div className="min-w-0">
            <p className="text-sm font-medium text-foreground">{t("Status")}</p>
            <p className="text-sm text-muted-foreground">
              {state?.meta
                ? t("{{host}}:{{port}} · pid {{pid}}", {
                    host: state.meta.host,
                    port: state.meta.port,
                    pid: state.meta.pid,
                  })
                : t("The daemon has not reported where it is listening.")}
            </p>
          </div>

          <div className="flex items-center gap-2">
            <Badge variant={state?.healthy ? "secondary" : "outline"}>
              {state?.healthy ? t("Healthy") : (state?.status ?? t("Unknown"))}
            </Badge>
            {state?.meta?.version ? <Badge variant="outline">{state.meta.version}</Badge> : null}
            <Button
              type="button"
              size="sm"
              variant="secondary"
              disabled={stateQuery.isFetching || isRestarting}
              onClick={() => void stateQuery.refetch()}
            >
              {t("Refresh")}
            </Button>
            {/* A browser tab did not launch the daemon and has no bridge to
                one; saying so beats a button that always fails. */}
            {isDesktopWindow ? (
              <Button
                type="button"
                size="sm"
                variant="secondary"
                disabled={isRestarting}
                onClick={restart}
              >
                {isRestarting ? t("Restarting…") : t("Restart daemon")}
              </Button>
            ) : null}
          </div>
        </FormSectionItem>

        {isDesktopWindow ? null : (
          <FormSectionItem>
            <p className="text-sm text-muted-foreground">
              {t("Restarting belongs to whatever launched this daemon — try `aos gateway restart`.")}
            </p>
          </FormSectionItem>
        )}
      </FormSectionContent>
    </FormSection>
  );
}

/**
 * The two things that are about this installation rather than this workspace:
 * which version it runs, and whether the process that owns the workspace is
 * alive. They share a screen because they share a question — "is this
 * machine's AOS in good shape" — and because both were unreachable from the
 * window until now.
 */
export function UserUpdatesSection(): React.JSX.Element {
  return (
    <SettingsSectionShell>
      <UpdatesPanel />
      <DaemonStatusPanel />
    </SettingsSectionShell>
  );
}
