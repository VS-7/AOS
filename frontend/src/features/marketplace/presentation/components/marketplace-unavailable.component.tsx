import * as React from "react";
import { useRouter } from "@tanstack/react-router";
import { toast } from "sonner";

import { aos } from "@/app/aos";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { errorMessage } from "@/lib/aos-facade";
import { isDesktop, system } from "@/lib/client";
import { useTranslation } from "@/lib/i18n";

/** Go's code for "the marketplace has nothing configured to search". */
export const NO_REGISTRIES_CODE = "AOS_MARKETPLACE_NO_REGISTRIES_CONFIGURED";

interface RegistryEntry {
  id: string;
  type: "http" | "git";
  url: string;
}

/** A registry id from its address, when the person does not name one. */
function registryIdFor(url: string): string {
  try {
    return new URL(url).hostname.replace(/[^a-z0-9]+/gi, "-").replace(/^-|-$/g, "").toLowerCase() || "registry";
  } catch {
    return "registry";
  }
}

/**
 * What the marketplace says when it cannot search.
 *
 * It used to say "No plugins matched your search. Try another query" — for
 * a search nobody had typed, when marketplace_discovery had answered that no
 * registry is configured at all. With no registry this names the state and
 * offers to add one; any other refusal is shown as what it is.
 */
export function MarketplaceUnavailable({ code, message }: { code: string; message: string }) {
  const { t } = useTranslation();

  if (code !== NO_REGISTRIES_CODE) {
    return (
      <div className="rounded-md border border-dashed p-6">
        <p className="text-sm font-medium text-foreground">{t("The marketplace could not be searched")}</p>
        <p className="mt-1 text-sm text-muted-foreground">{message || t("Something went wrong.")}</p>
      </div>
    );
  }

  return (
    <div className="rounded-md border border-dashed p-6">
      <p className="text-sm font-medium text-foreground">{t("No marketplace registry is configured")}</p>
      <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
        {t("The marketplace searches the registries listed under marketplace.registries in AOS's config.json. Add one below; the daemon reads them when it starts.")}
      </p>
      <RegistryForm />
    </div>
  );
}

function RegistryForm() {
  const { t } = useTranslation();
  const router = useRouter();
  const [url, setUrl] = React.useState("");
  const [type, setType] = React.useState<RegistryEntry["type"]>("http");
  const [id, setId] = React.useState("");
  const [saving, setSaving] = React.useState(false);
  const [saved, setSaved] = React.useState(false);
  const [restarting, setRestarting] = React.useState(false);

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    const address = url.trim();
    if (!address) return;
    setSaving(true);
    try {
      // The whole list, as it is now plus this one: a list is a leaf of
      // config_update, replaced wholesale.
      const current = await aos.client.config.get.query();
      if (current.error) throw current.error;
      const existing: RegistryEntry[] = current.data?.marketplace?.registries ?? [];
      const entry: RegistryEntry = { id: id.trim() || registryIdFor(address), type, url: address };
      const next = [...existing.filter((registry) => registry.id !== entry.id), entry];
      await aos.client.config.update.mutateOrThrow({ body: { set: { "marketplace.registries": next } } });
      setSaved(true);
    } catch (error) {
      toast.error(t("The registry could not be saved"), { description: errorMessage(error) });
    } finally {
      setSaving(false);
    }
  };

  const restart = async () => {
    setRestarting(true);
    try {
      await system.restartDaemon();
      toast.success(t("The daemon is restarting."));
      // The connection drops while it comes back; reading again at once would
      // only report that.
      setTimeout(() => void router.invalidate(), 3000);
    } catch (error) {
      toast.error(t("The daemon could not be restarted."), { description: errorMessage(error) });
    } finally {
      setRestarting(false);
    }
  };

  if (saved) {
    return (
      <div className="mt-4 flex flex-wrap items-center gap-3">
        <p className="text-sm text-muted-foreground">
          {t("Registry saved. Restart the daemon so the marketplace searches it.")}
        </p>
        {isDesktop() ? (
          <Button size="sm" onClick={() => void restart()} disabled={restarting} aria-busy={restarting}>
            {restarting ? <Spinner /> : null}
            {t("Restart the daemon")}
          </Button>
        ) : null}
      </div>
    );
  }

  return (
    <form onSubmit={(event) => void save(event)} className="mt-4 grid max-w-2xl gap-3 sm:grid-cols-[1fr_9rem]">
      <div className="grid gap-1.5">
        <Label htmlFor="registry-url">{t("Registry address")}</Label>
        <Input
          id="registry-url"
          value={url}
          onChange={(event) => setUrl(event.target.value)}
          placeholder={type === "git" ? "https://github.com/acme/registry.git" : "https://registry.example.com"}
          required
        />
      </div>
      <div className="grid gap-1.5">
        <Label htmlFor="registry-type">{t("Kind")}</Label>
        <Select value={type} onValueChange={(value) => setType(value as RegistryEntry["type"])}>
          <SelectTrigger id="registry-type">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="http">{t("HTTP index")}</SelectItem>
            <SelectItem value="git">{t("Git repository")}</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="grid gap-1.5">
        <Label htmlFor="registry-id">{t("Name (optional)")}</Label>
        <Input id="registry-id" value={id} onChange={(event) => setId(event.target.value)} placeholder={url ? registryIdFor(url) : "acme"} />
      </div>
      <div className="flex items-end">
        <Button type="submit" className="w-full" disabled={saving || !url.trim()} aria-busy={saving}>
          {saving ? <Spinner /> : null}
          {t("Add registry")}
        </Button>
      </div>
    </form>
  );
}
