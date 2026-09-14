import * as React from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { aos } from "@/app/aos";
import { errorMessage } from "@/lib/aos-facade";
import { t } from "@/lib/i18n";
import { ArtifactHelper } from "../helpers/artifact.helper";
import {
  addressWithPassword,
  checkArtifactPassword,
  closeArtifactAccess,
  useArtifactAccessRequest,
} from "../helpers/artifact-access";

/**
 * The shortest password a by_password artifact takes — the daemon's own
 * minimum (artifact.minPasswordLength), checked here too so the button says
 * so before a request does.
 */
export const MIN_ARTIFACT_PASSWORD_LENGTH = 8;

/**
 * Asks for a by_password artifact's password, or sets one.
 *
 * Three cases, one field:
 * - opening an artifact that has a password: the password is checked with the
 *   daemon, then the artifact opens with it;
 * - opening one that has none (an agent created it that way, and it refuses
 *   everybody): a first password is set, then it opens;
 * - "Set password" from the row menu: the password is set or changed, and
 *   nothing opens.
 *
 * Mounted once, by the workspace layout; `ArtifactHelper.openInBrowserTab`
 * and the Surfaces row menu ask for it through `requestArtifactAccess`.
 */
export function ArtifactAccessDialog() {
  const request = useArtifactAccessRequest();
  const [password, setPassword] = React.useState("");
  const [problem, setProblem] = React.useState<string | null>(null);
  const [busy, setBusy] = React.useState(false);

  const artifact = request?.artifact;
  const setting = request?.mode === "set" || artifact?.hasPassword === false;
  const opening = request?.mode === "open";

  React.useEffect(() => {
    setPassword("");
    setProblem(null);
    setBusy(false);
  }, [request]);

  const tooShort = setting && password.length < MIN_ARTIFACT_PASSWORD_LENGTH;

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!artifact || !password || tooShort) return;
    setBusy(true);
    setProblem(null);
    try {
      if (setting) {
        await aos.client.artifact.setPassword.mutateOrThrow({
          params: { artifact: artifact.id },
          body: { password },
        });
        await aos.stores.artifact.actions.refresh();
        // A tab already open on it carries the old password in its address,
        // and would answer the daemon's refusal the next time it loads.
        for (const tab of aos.stores.viewport.state.tabs.items) {
          if (tab.type === "browser" && tab.metadata?.artifactId === artifact.id) {
            aos.stores.viewport.actions.updateTab(tab.id, {
              url: addressWithPassword(artifact.urls.local, password),
            });
          }
        }
        if (!opening) toast.success(t("Password set."));
      } else {
        const accepted = await checkArtifactPassword(artifact.urls.local, password);
        if (accepted !== true) {
          setProblem(
            accepted === false
              ? t("That password is not the one this artifact is shared with.")
              : accepted,
          );
          return;
        }
      }
      closeArtifactAccess();
      if (opening) {
        ArtifactHelper.openInBrowserTab({ ...artifact, hasPassword: true }, { password });
      }
    } catch (error) {
      setProblem(errorMessage(error) ?? t("Unable to set the password."));
    } finally {
      setBusy(false);
    }
  }

  const title = !artifact
    ? ""
    : request?.mode === "set"
      ? t("Password for “{{name}}”", { name: artifact.name })
      : setting
        ? t("“{{name}}” has no password yet", { name: artifact.name })
        : t("“{{name}}” is protected by a password", { name: artifact.name });
  const description = request?.mode === "set"
    ? t("Anyone holding the password can open the artifact. Setting a new one stops the old one from working.")
    : setting
      ? t("It is shared by password, and refuses everybody until it has one. Set one to open it.")
      : t("Enter the password it is shared with to open it.");
  const action = request?.mode === "set"
    ? t("Set password")
    : setting
      ? t("Set password and open")
      : t("Open");

  return (
    <Dialog
      open={request != null}
      onOpenChange={(open) => {
        if (!open) closeArtifactAccess();
      }}
    >
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <form className="flex flex-col gap-4" onSubmit={(event) => void submit(event)}>
          <div className="flex flex-col gap-2">
            <Label htmlFor="artifact-access-password">{t("Password")}</Label>
            <Input
              id="artifact-access-password"
              type="password"
              autoFocus
              autoComplete={setting ? "new-password" : "current-password"}
              value={password}
              disabled={busy}
              aria-invalid={problem ? true : undefined}
              onChange={(event) => {
                setPassword(event.target.value);
                setProblem(null);
              }}
            />
            {problem ? (
              <p role="alert" className="text-xs text-destructive">
                {problem}
              </p>
            ) : setting ? (
              <p className="text-xs text-muted-foreground">
                {t("At least {{count}} characters. Share it only with who should open the artifact.", {
                  count: MIN_ARTIFACT_PASSWORD_LENGTH,
                })}
              </p>
            ) : null}
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" size="sm" onClick={() => closeArtifactAccess()}>
              {t("Cancel")}
            </Button>
            <Button type="submit" size="sm" disabled={busy || !password || tooShort}>
              {action}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
