import { useCallback, useEffect, useState } from "react";
import type { JSX, ReactNode } from "react";
import { Loader2 } from "lucide-react";

import { client } from "@/lib/client";
import { DomainError } from "@/lib/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Logo } from "@/components/ui/logo";
import { FolderInput } from "@/components/ui/folder-input";
import { t } from "@/lib/i18n";

/**
 * What renders when somebody is signed in and the installation has no
 * workspace.
 *
 * Onboarding creates one now, so a fresh installation never reaches this. An
 * installation made before that fix did: the account existed, `workspaces/`
 * was empty, and every screen in the application was a workspace-scoped screen
 * with nothing behind it — an empty task list, an empty agent roster, a
 * sidebar reading "No Workspace" and no way anywhere in the interface to
 * create one, because the only button that does lives inside the workspace
 * switcher, which had nothing to switch between.
 *
 * It is a gate rather than a page for the same reason `AuthGate` is: the
 * answer is the same for every route, so it is asked once, ahead of the
 * router — and the workspace store's own preload, which runs inside the
 * router, can then count on there being a workspace to find.
 */
type Gate =
  | { state: "checking" }
  | { state: "missing" }
  | { state: "ready" };

export function WorkspaceGate({ children }: { children: ReactNode }): JSX.Element {
  const [gate, setGate] = useState<Gate>({ state: "checking" });

  const recheck = useCallback(() => {
    setGate({ state: "checking" });
    client
      .invoke("workspace_list", {
        _reasoning: "deciding whether this installation has a workspace to open",
      })
      .then((answer) => {
        const workspaces = (answer as { workspaces?: unknown[] } | undefined)?.workspaces ?? [];
        setGate({ state: workspaces.length > 0 ? "ready" : "missing" });
      })
      // A daemon that has not answered yet is not the same as an installation
      // with no workspace, and guessing wrong here would put a creation form
      // in front of somebody whose workspaces are fine. Let the application
      // through and let its own error handling say what is wrong.
      .catch(() => setGate({ state: "ready" }));
  }, []);

  useEffect(recheck, [recheck]);

  if (gate.state === "checking") return <></>;
  if (gate.state === "missing") return <FirstWorkspace onCreated={recheck} />;
  return <>{children}</>;
}

function FirstWorkspace({ onCreated }: { onCreated: () => void }): JSX.Element {
  const [name, setName] = useState("");
  const [path, setPath] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    const workspaceName = name.trim();
    if (!workspaceName) {
      setError(t("Name your workspace."));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const chosen = path.trim();
      await client.invoke("workspace_create", {
        name: workspaceName,
        ...(chosen ? { path: chosen } : {}),
        _reasoning: "the person is creating the installation's first workspace",
      });
      onCreated();
    } catch (err) {
      setError(err instanceof DomainError ? err.message : t("Could not create the workspace."));
      setBusy(false);
    }
  }

  return (
    <div className="flex h-screen items-center justify-center p-6">
      <div className="w-full max-w-md space-y-6">
        <Logo className="h-5" />

        <div>
          <h1 className="text-xl font-semibold tracking-tight">{t("Your first workspace")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t("A workspace is a folder your agents work in. Everything they write — agents, tasks, notes — is a file inside it.")}
          </p>
        </div>

        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="first-workspace-name">{t("Workspace name")}</Label>
            <Input
              id="first-workspace-name"
              autoFocus
              placeholder={t("e.g. Acme Corp")}
              value={name}
              disabled={busy}
              onChange={(event) => setName(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") void submit();
              }}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="first-workspace-path">{t("Folder (optional)")}</Label>
            <FolderInput
              value={path}
              onChange={setPath}
              disabled={busy}
              placeholder={t("Leave empty and AOS picks a folder for you")}
            />
            <p className="text-xs text-muted-foreground">
              {t("Point this at a Git repository to have your agents work directly on it.")}
            </p>
          </div>
        </div>

        {error ? <p className="text-sm text-destructive">{error}</p> : null}

        <Button className="w-full" disabled={busy} onClick={() => void submit()}>
          {busy ? <Loader2 className="size-4 animate-spin" /> : null}
          {t("Create workspace")}
        </Button>
      </div>
    </div>
  );
}
