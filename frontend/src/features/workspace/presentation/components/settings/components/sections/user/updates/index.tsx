import * as React from "react";
import { toast } from "sonner";

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

interface UpdateStatus {
  current: string;
  channel: string;
  latestKnown?: string;
  checkedAt?: string;
}

/**
 * A release, shaped exactly as `update_check` returns it and `update_download`
 * takes it back.
 *
 * The interface reads two of these fields and passes the rest through
 * untouched — it never constructs one. The full shape is spelled out anyway
 * because the generated command types check it, and a looser type here would
 * only mean a cast that stops checking anything.
 */
interface Release {
  version: string;
  channel: string;
  checksumsUrl: string;
  signatureUrl: string;
  publishedAt: string;
  assets: unknown;
  notes?: string;
}

/** What `update_download` staged, handed straight back to `update_apply`. */
interface Staged {
  version: string;
  dir: string;
  binaries: Record<string, string>;
}

interface CheckResult {
  upToDate: boolean;
  current: string;
  channel: string;
  release?: Release;
}


interface GatewayMeta {
  pid: number;
  port: number;
  host: string;
  version?: string;
}

interface GatewayState {
  status: string;
  healthy: boolean;
  meta?: GatewayMeta;
}

type Phase = "idle" | "checking" | "downloading" | "applying";

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
  const [status, setStatus] = React.useState<UpdateStatus | null>(null);
  const [check, setCheck] = React.useState<CheckResult | null>(null);
  const [staged, setStaged] = React.useState<Staged | null>(null);
  const [phase, setPhase] = React.useState<Phase>("idle");

  const readStatus = React.useCallback(async () => {
    try {
      const answer = (await client.invoke("update_status", {
        _reasoning: "the settings screen is showing which version this installation runs",
      })) as UpdateStatus | undefined;
      setStatus(answer ?? null);
    } catch {
      // An installation whose release feed is switched off (AOS_UPDATE_BASE_URL
      // unset) answers nothing useful here; the section says what it knows.
      setStatus(null);
    }
  }, []);

  React.useEffect(() => {
    void readStatus();
  }, [readStatus]);

  const runCheck = async () => {
    setPhase("checking");
    setStaged(null);
    try {
      const answer = (await client.invoke("update_check", {
        _reasoning: "a person asked whether a newer release exists",
      })) as CheckResult | undefined;
      setCheck(answer ?? null);
      if (answer?.upToDate) toast.success(t("You are on the newest release."));
      await readStatus();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("The release channel could not be reached."));
    } finally {
      setPhase("idle");
    }
  };

  const runDownload = async () => {
    if (!check?.release) return;
    setPhase("downloading");
    try {
      const answer = (await client.invoke("update_download", {
        release: check.release,
        _reasoning: "a person asked to fetch and verify the release that was found",
      })) as { staged?: Staged } | undefined;
      setStaged(answer?.staged ?? null);
      toast.success(t("Downloaded and verified."));
    } catch (error) {
      // A signature or checksum failure lands here, and it is the one message
      // in this screen that must not be softened.
      toast.error(error instanceof Error ? error.message : t("The download could not be verified."));
    } finally {
      setPhase("idle");
    }
  };

  const runApply = async () => {
    if (!staged) return;
    setPhase("applying");
    try {
      await client.invoke("update_apply", {
        staged,
        _reasoning: "a person asked to install the staged release",
      });
      toast.success(t("Installed. The daemon restarted on the new version."));
      setCheck(null);
      setStaged(null);
      await readStatus();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("The update could not be applied."));
    } finally {
      setPhase("idle");
    }
  };

  const busy = phase !== "idle";

  return (
    <FormSection>
      <FormSectionHeader>
        <FormSectionTitle>{t("Updates")}</FormSectionTitle>
        <FormSectionDescription>
          {t("Check the release channel, verify what it offers, and install it.")}
        </FormSectionDescription>
      </FormSectionHeader>
      <FormSectionContent>
        <div className="space-y-4">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm">{t("Running")}</span>
            <Badge variant="secondary">{status?.current ?? "—"}</Badge>
            {status?.channel ? <Badge variant="outline">{status.channel}</Badge> : null}
            <Button
              size="sm"
              className="ml-auto"
              disabled={busy}
              onClick={runCheck}
            >
              {phase === "checking" ? t("Checking…") : t("Check for updates")}
            </Button>
          </div>

          {check && !check.upToDate && check.release ? (
            <div className="space-y-3 rounded-lg border border-border/60 p-3">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium">
                  {t("{{version}} is available").replace("{{version}}", check.release.version)}
                </span>
                <Badge>{check.channel}</Badge>
              </div>
              {check.release.notes ? (
                <pre className="max-h-40 overflow-auto whitespace-pre-wrap rounded-md bg-muted/40 p-2 text-xs">
                  {check.release.notes}
                </pre>
              ) : null}
              <div className="flex gap-2">
                <Button size="sm" variant="secondary" disabled={busy || Boolean(staged)} onClick={runDownload}>
                  {phase === "downloading" ? t("Downloading…") : t("Download and verify")}
                </Button>
                <Button size="sm" disabled={busy || !staged} onClick={runApply}>
                  {phase === "applying" ? t("Installing…") : t("Install and restart")}
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                {staged
                  ? t("Verified and staged. Installing restarts the daemon; in-flight work finishes first.")
                  : t("Nothing is installed until you download it and the signature checks out.")}
              </p>
            </div>
          ) : null}

          {check?.upToDate ? (
            <p className="text-sm text-muted-foreground">{t("You are on the newest release.")}</p>
          ) : null}
        </div>
      </FormSectionContent>
    </FormSection>
  );
}

/**
 * What the daemon says about itself.
 *
 * Only status and restart. Starting a daemon that is already answering this
 * very call is meaningless, and stopping it would have the window cut the
 * connection it is speaking over — supervision belongs to whatever launched
 * the daemon (`aos gateway`, or the desktop's own supervisor), not to a panel
 * inside the thing being supervised.
 *
 * Restart earns its place: it is what a hung worker or a configuration change
 * needs, and until now the only way to ask for it was the terminal.
 */
export function DaemonStatusPanel(): React.JSX.Element {
  const [state, setState] = React.useState<GatewayState | null>(null);
  const [busy, setBusy] = React.useState(false);

  const read = React.useCallback(async () => {
    try {
      const answer = (await client.invoke("gateway_status", {
        _reasoning: "the settings screen is showing whether the daemon is healthy",
      })) as GatewayState | undefined;
      setState(answer ?? null);
    } catch {
      setState(null);
    }
  }, []);

  React.useEffect(() => {
    void read();
  }, [read]);

  const restart = async () => {
    setBusy(true);
    try {
      await client.invoke("gateway_restart", {
        _reasoning: "a person asked to restart the daemon",
      });
      toast.success(t("The daemon is restarting."));
      // Deliberately unawaited-then-read: the daemon drops this connection
      // while it comes back, so the first status read after a restart is
      // expected to fail. Giving it a beat is the difference between showing
      // "stopped" for a second and showing the truth.
      setTimeout(() => void read(), 2500);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("The daemon could not be restarted."));
    } finally {
      setBusy(false);
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
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant={state?.healthy ? "secondary" : "destructive"}>
            {state?.healthy ? t("Healthy") : (state?.status ?? t("Unknown"))}
          </Badge>
          {state?.meta?.version ? <Badge variant="outline">{state.meta.version}</Badge> : null}
          {state?.meta ? (
            <span className="text-xs text-muted-foreground">
              {state.meta.host}:{state.meta.port} · pid {state.meta.pid}
            </span>
          ) : null}
          <div className="ml-auto flex gap-2">
            <Button size="sm" variant="secondary" disabled={busy} onClick={read}>
              {t("Refresh")}
            </Button>
            <Button size="sm" variant="secondary" disabled={busy} onClick={restart}>
              {t("Restart daemon")}
            </Button>
          </div>
        </div>
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
    <div className="space-y-6">
      <UpdatesPanel />
      <DaemonStatusPanel />
    </div>
  );
}
