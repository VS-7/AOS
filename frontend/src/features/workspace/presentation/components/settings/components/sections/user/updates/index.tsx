import * as React from "react";
import { toast } from "sonner";

import { aos } from "@/app/aos";
import { system } from "@/lib/client";
import { isDesktopWindow, openExternal } from "@/lib/wails";
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
import {
  canCheck,
  checkLine,
  messageOf,
  offerLine,
  offerOf,
  refusalOf,
  releasePage,
  reopenLine,
  statusLine,
} from "./updates.helper";

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
 *
 * What a check found is shown as found, never rounded up to good news: a
 * build with no release feed, a development build and an installation that
 * has to be reinstalled each say so, instead of the green "You are on the
 * newest release." all three used to get. And a refusal is a refusal
 * wherever the facade reports it — see `refusalOf`.
 */
function UpdatesPanel(): React.JSX.Element {
  const [check, setCheck] = React.useState<CheckResult | null>(null);
  const [downloaded, setDownloaded] = React.useState<Staged | null>(null);

  const statusQuery = aos.client.update.status.useQuery<UpdateStatus>();
  const status = statusQuery.data ?? null;

  const { mutate: runCheck, loading: isChecking } = aos.client.update.check.useMutation({
    onSuccess: (result: any) => {
      const refused = refusalOf(result);
      if (refused) {
        setCheck(null);
        toast.error(messageOf(refused, t("The release channel could not be reached.")));
        return;
      }
      const answer = (result?.data as CheckResult | undefined) ?? null;
      setCheck(answer);
      if (answer?.state === "up-to-date") toast.success(t("You are on the newest release."));
      void statusQuery.refetch();
    },
    onError: (error: unknown) => {
      setCheck(null);
      toast.error(messageOf(error, t("The release channel could not be reached.")));
    },
  });

  const { mutate: runDownload, loading: isDownloading } = aos.client.update.download.useMutation({
    onSuccess: (result: any) => {
      // A signature or checksum failure lands here too, and it is the one
      // message in this screen that must not be softened.
      const refused = refusalOf(result);
      if (refused) {
        toast.error(messageOf(refused, t("The download could not be verified.")));
        return;
      }
      setDownloaded((result?.data?.staged as Staged | undefined) ?? null);
      toast.success(t("Downloaded and verified."));
      void statusQuery.refetch();
    },
    onError: (error: unknown) => {
      toast.error(messageOf(error, t("The download could not be verified.")));
    },
  });

  const { mutate: runApply, loading: isApplying } = aos.client.update.apply.useMutation({
    onSuccess: (result: any) => {
      const refused = refusalOf(result);
      if (refused) {
        toast.error(messageOf(refused, t("The update could not be applied.")));
        void statusQuery.refetch();
        return;
      }
      toast.success(t("Installed. The daemon restarted on the new version."));
      setCheck(null);
      setDownloaded(null);
      void statusQuery.refetch();
    },
    onError: (error: unknown) => {
      toast.error(messageOf(error, t("The update could not be applied.")));
    },
  });

  const busy = isChecking || isDownloading || isApplying;
  const answer = checkLine(check, status);
  const offer = offerOf(check, downloaded, status);
  const page = offer ? releasePage(offer.release) : null;
  const reopen = offer ? reopenLine(offer) : null;
  const waiting = !check && status?.staged ? status.staged : null;

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
              <p className="text-sm text-muted-foreground">{statusLine(status)}</p>
            </div>

            <div className="flex items-center gap-2">
              <Badge variant="secondary">{status?.current ?? "—"}</Badge>
              {status?.channel ? <Badge variant="outline">{status.channel}</Badge> : null}
              {canCheck(status) ? (
                <Button type="button" size="sm" disabled={busy} onClick={() => void runCheck({})}>
                  {isChecking ? t("Checking…") : t("Check for updates")}
                </Button>
              ) : null}
            </div>
          </FormSectionItem>

          {answer ? (
            <FormSectionItem>
              <p className="text-sm text-muted-foreground">{answer}</p>
            </FormSectionItem>
          ) : null}

          {waiting ? (
            <FormSectionItem>
              <p className="text-sm text-muted-foreground">
                {t("{{version}} is downloaded and verified. Check for updates to install it.", {
                  version: waiting.version,
                })}
              </p>
            </FormSectionItem>
          ) : null}
        </FormSectionContent>
      </FormSection>

      {offer ? (
        <FormSection>
          <FormSectionHeader>
            <FormSectionTitle>
              {t("{{version}} is available", { version: offer.release.version })}
            </FormSectionTitle>
            <FormSectionDescription>{offerLine(offer)}</FormSectionDescription>
          </FormSectionHeader>

          <FormSectionContent>
            {offer.install.method === "terminal" && offer.staged && offer.install.command ? (
              <FormSectionItem>
                <div className="min-w-0 space-y-2">
                  <code className="block select-all break-all rounded bg-muted px-2 py-1 font-mono text-xs text-foreground">
                    {offer.install.command}
                  </code>
                  {reopen ? <p className="text-sm text-muted-foreground">{reopen}</p> : null}
                </div>
              </FormSectionItem>
            ) : null}
            <FormSectionItem>
              <div className="min-w-0">
                <p className="text-sm font-medium text-foreground">{t("Release notes")}</p>
                {offer.release.notes ? (
                  <pre className="mt-1 max-h-40 overflow-auto whitespace-pre-wrap text-sm text-muted-foreground">
                    {offer.release.notes}
                  </pre>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    {t("This release carries no notes.")}
                  </p>
                )}
              </div>
              <Badge variant="outline">{offer.release.channel}</Badge>
            </FormSectionItem>
          </FormSectionContent>

          <FormSectionFooter className="flex justify-end gap-2">
            {offer.install.method === "reinstall" ? (
              page ? (
                <Button type="button" size="sm" variant="secondary" onClick={() => void openExternal(page)}>
                  {t("Open the release page")}
                </Button>
              ) : null
            ) : (
              <>
                <Button
                  type="button"
                  size="sm"
                  variant="secondary"
                  disabled={busy || Boolean(offer.staged)}
                  onClick={() => void runDownload({ body: { release: offer.release } })}
                >
                  {isDownloading ? t("Downloading…") : t("Download and verify")}
                </Button>
                {offer.install.method === "here" ? (
                  <Button
                    type="button"
                    size="sm"
                    disabled={busy || !offer.staged}
                    onClick={() => void runApply({ body: { version: offer.staged?.version } })}
                  >
                    {isApplying ? t("Installing…") : t("Install and restart")}
                  </Button>
                ) : null}
              </>
            )}
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
