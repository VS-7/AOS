import * as React from "react";
import * as z from "zod";
import { Copy, Check, ExternalLink, Info, Loader2, CircleCheck, CircleDot, Circle } from "lucide-react";
import { toast } from "sonner";
import { openExternal } from "@/lib/wails";

import { aos } from "@/app/aos";
import { api, errorMessage } from "@/lib/aos-facade";
import { SettingsSectionShell } from "../../../section-shell";
import {
  FormSection,
  FormSectionContent,
  FormSectionDescription,
  FormSectionFooter,
  FormSectionHeader,
  FormSectionTitle,
} from "@/components/ui/form-section";
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  Form,
} from "@/components/ui/form";
import { Switch } from "@/components/ui/switch";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { TunnelUrlHelper } from "@/features/tunnel/presentation/helpers/tunnel-url.helper";
import { t } from "@/lib/i18n";
import type { Config } from "@/features/config/interfaces/config.interfaces";
import { changedSettings } from "../../../../helpers/changed-settings";

const tunnelEnableFormSchema = z.object({
  enabled: z.boolean(),
});

const tunnelCredentialsFormSchema = z.object({
  hostname: z.string().optional().or(z.literal("")),
  token: z.string().optional().or(z.literal("")),
});

export function WorkspaceTunnelSection() {
  // The configuration store, not the route context: the context is a copy
  // taken when the route loaded, and no save reached it, so this page
  // reopened with the hostname and switch it had before the last save.
  const config = aos.stores.config.useState();
  const tunnelStatusQuery = aos.client.tunnel.getStatus.useQuery();
  const tunnelStatus = tunnelStatusQuery.data;
  const [copied, setCopied] = React.useState(false);

  const activationForm = aos.useForm({
    schema: tunnelEnableFormSchema,
    mode: "onChange",
    values: {
      enabled: config?.tunnel?.enabled ?? false,
    },
    onSubmit: async (values) => {
      const previousEnabled = aos.stores.config.state?.tunnel?.enabled ?? false;
      if (values.enabled === previousEnabled) return values;

      // `mutateOrThrow` throughout: this is written as try/catch, and `mutate`
      // resolves a refusal as a value. With `mutate` the catch — and the
      // rollback in it — never ran: a start the daemon refused
      // (AOS_TUNNEL_INSECURE_EXPOSURE) was announced as "Tunnel started!",
      // and `tunnel.enabled` stayed saved as true over a tunnel that was not
      // running.
      try {
        aos.stores.config.actions.adopt(
          await api.config.update.mutateOrThrow<Config>({
            body: { set: { "tunnel.enabled": values.enabled } },
          }),
        );

        if (values.enabled) {
          const started = await api.tunnel.start.mutateOrThrow<{ url?: string }>();
          toast.success(
            t("Tunnel started! URL: {{url}}", { url: started?.url ?? tunnelPublicUrl }),
          );
        } else {
          await api.tunnel.stop.mutateOrThrow();
          toast.success(t("Tunnel stopped."));
        }

        await tunnelStatusQuery.refetch();
        return values;
      } catch (error) {
        // Best effort: the refusal is what the person needs to read, and a
        // rollback that also fails must not replace it.
        const rolledBack = await api.config.update.mutate<Config>({
          body: { set: { "tunnel.enabled": previousEnabled } },
        });
        if (!rolledBack.error) aos.stores.config.actions.adopt(rolledBack.data);

        activationForm.reset({ enabled: previousEnabled });
        toast.error(
          values.enabled ? t("The tunnel could not be started") : t("The tunnel could not be stopped"),
          { description: errorMessage(error) },
        );
        await tunnelStatusQuery.refetch();
        throw error;
      }
    },
  });

  const credentialsForm = aos.useForm({
    schema: tunnelCredentialsFormSchema,
    values: {
      hostname: config?.tunnel?.hostname ?? "",
      token: config?.tunnel?.token ?? "",
    },
    onSubmit: async (values) => {
      const enabled = activationForm.getValues("enabled");
      const hostname = values.hostname?.trim() ?? "";
      const token = values.token?.trim() ?? "";

      // The same try/catch shape as the switch above, with the same trap:
      // `mutate` never throws, so a refused save or start reported success.
      try {
        // Only what changed. The token comes back from the daemon as a
        // fingerprint ("***…a1b2"), never in full, and that is what this
        // field holds until somebody types a new one: sending it back with
        // every hostname edit replaced the real token with its fingerprint.
        const saved = aos.stores.config.state?.tunnel;
        const set = changedSettings(
          "tunnel",
          { hostname, token },
          { hostname: saved?.hostname ?? "", token: saved?.token ?? "" },
        );
        if (Object.keys(set).length > 0) {
          aos.stores.config.actions.adopt(await api.config.update.mutateOrThrow<Config>({ body: { set } }));
        }

        if (enabled && hostname && token) {
          const started = await api.tunnel.start.mutateOrThrow<{ url?: string }>();
          toast.success(
            t("Tunnel settings saved and started! URL: {{url}}", {
              url: started?.url ?? tunnelPublicUrl,
            }),
          );
        } else {
          toast.success(t("Tunnel settings saved."));
        }

        await tunnelStatusQuery.refetch();
        return { hostname, token };
      } catch (error) {
        toast.error(t("Failed to save tunnel credentials"), { description: errorMessage(error) });
        await tunnelStatusQuery.refetch();
        throw error;
      }
    },
  });

  const isTunnelEnabled = activationForm.watch("enabled");
  const tunnelHostname = credentialsForm.watch("hostname") ?? "";
  const tunnelToken = credentialsForm.watch("token") ?? "";
  const tunnelPublicUrl = TunnelUrlHelper.buildPublicUrl(tunnelHostname);
  const isTunnelConfigured = !!tunnelHostname.trim() && !!tunnelToken.trim();
  // Go's `tunnel.State` is `{status, url, pid, startedAt, error}`: there is
  // no `online` field, so reading one kept "Online" from ever appearing.
  const tunnelState = (tunnelStatus as { status?: string } | undefined)?.status;
  const isTunnelOnline = tunnelState === "running";
  const isTunnelStarting = tunnelState === "starting";
  const tunnelError =
    tunnelState === "failed" ? (tunnelStatus as { error?: string } | undefined)?.error : undefined;
  // The daemon refuses to publish an API that asks for no credential
  // (AOS_TUNNEL_INSECURE_EXPOSURE), and reports that same guard in the status
  // it answers, so the switch can say so before it is flipped rather than
  // after a refusal. Read from there and not from config.security.apiToken:
  // nothing authenticates against that field, so this screen used to demand a
  // token no one could make count, while the one Settings > Developers issues
  // — the credential REST and MCP callers present — did not clear the
  // warning. Unknown (the first fetch) is not "unauthenticated": the switch
  // is disabled while the query runs anyway.
  const lacksAuthentication =
    tunnelStatus !== undefined &&
    !(tunnelStatus as { authenticated?: boolean } | undefined)?.authenticated;
  const isBusy = activationForm.isLoading || credentialsForm.isLoading || tunnelStatusQuery.isFetching;

  const handleCopyUrl = async () => {
    if (!tunnelPublicUrl) return;

    try {
      await navigator.clipboard.writeText(tunnelPublicUrl);
      setCopied(true);
      toast.success(t("URL copied to clipboard!"));
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error(t("Failed to copy URL"));
    }
  };

  const handleOpenUrl = () => {
    if (tunnelPublicUrl) {
      void openExternal(tunnelPublicUrl);
    }
  };

  return (
    <div className="flex h-full flex-1 flex-col overflow-y-auto">
      <SettingsSectionShell>
        <Form form={activationForm} className="flex flex-1 flex-col">
          <FormSection>
            <FormSectionHeader>
              <FormSectionTitle>{t("Tunnel")}</FormSectionTitle>
              <FormSectionDescription>
                {t("Expose your local AOS instance through a Cloudflare Tunnel.")}
              </FormSectionDescription>
            </FormSectionHeader>
            <FormSectionContent className="divide-y divide-border">
              <div className="flex items-center justify-between gap-4 p-4">
                <div className="space-y-0.5">
                  <FormLabel>{t("Status")}</FormLabel>
                  <FormDescription>
                    {isBusy
                      ? t("Applying tunnel settings...")
                      : isTunnelOnline
                        ? t("Your instance is accessible from the internet.")
                        : tunnelError
                          ? tunnelError
                          : isTunnelStarting
                            ? t("Tunnel is enabled and waiting to connect.")
                            : isTunnelEnabled && isTunnelConfigured
                              ? t("Tunnel is enabled but not running.")
                              : isTunnelEnabled
                                ? t("Tunnel is enabled, but hostname or token is missing.")
                                : t("Tunnel is disabled. Your instance is only accessible locally.")}
                  </FormDescription>
                </div>
                {isBusy || isTunnelStarting ? (
                  <Badge variant="outline" className="gap-1">
                    <Loader2 className="size-3 animate-spin text-blue-600" />
                    {t("Connecting")}
                  </Badge>
                ) : isTunnelOnline ? (
                  <Badge variant="outline" className="gap-1">
                    <CircleCheck className="size-3 text-emerald-400" />
                    {t("Online")}
                  </Badge>
                ) : tunnelError ? (
                  <Badge variant="outline" className="gap-1">
                    <CircleDot className="size-3 text-destructive" />
                    {t("Failed")}
                  </Badge>
                ) : (
                  <Badge variant="outline" className="gap-1">
                    <Circle className="size-3 text-muted-foreground" />
                    {t("Offline")}
                  </Badge>
                )}
              </div>

              <FormField
                control={activationForm.control}
                name="enabled"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                    <div className="space-y-0.5">
                      <FormLabel>{t("Enable Tunnel")}</FormLabel>
                      <FormDescription>
                        {t("Start cloudflared automatically with the saved hostname and token.")}
                      </FormDescription>
                      {lacksAuthentication && !field.value ? (
                        <FormDescription className="text-destructive">
                          {t("The daemon will not expose an API that asks for no credential. Turn authentication on, then generate an API token in Settings > Developers.")}
                        </FormDescription>
                      ) : null}
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        // Turning it off stays possible; turning it on without
                        // a credential can only be refused.
                        disabled={isBusy || (lacksAuthentication && !field.value)}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            </FormSectionContent>
          </FormSection>
        </Form>

        <Form form={credentialsForm} className="flex flex-1 flex-col">
          <FormSection>
            <FormSectionHeader>
              <FormSectionTitle>{t("Connection")}</FormSectionTitle>
              <FormSectionDescription>
                {t("Configure the fixed hostname and Cloudflare tunnel token for this desktop.")}
              </FormSectionDescription>
            </FormSectionHeader>
            <FormSectionContent className="divide-y divide-border">
              <FormField
                control={credentialsForm.control}
                name="hostname"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                    <div className="space-y-0.5">
                      <FormLabel>{t("Hostname")}</FormLabel>
                      <FormDescription>
                        {t("The public URL for this tunnel, like `workspace.example.com`.")}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <div className="w-1/2">
                        <Input
                          placeholder="workspace.example.com"
                          {...field}
                        />
                      </div>
                    </FormControl>
                  </FormItem>
                )}
              />

              <FormField
                control={credentialsForm.control}
                name="token"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                    <div className="space-y-0.5">
                      <FormLabel>{t("Tunnel Token")}</FormLabel>
                      <FormDescription>
                        {t("Cloudflare token used to start the named tunnel.")}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <div className="w-1/2">
                        <Input
                          type="password"
                          placeholder={t("Cloudflare tunnel token")}
                          autoComplete="off"
                          {...field}
                        />
                      </div>
                    </FormControl>
                  </FormItem>
                )}
              />

              <div className="p-4 space-y-3">
                <FormLabel>{t("Public URL")}</FormLabel>
                {tunnelPublicUrl ? (
                  <div className="flex items-center gap-2">
                    <Input
                      value={tunnelPublicUrl}
                      readOnly
                      className="font-mono text-sm"
                    />
                    <Button
                      type="button"
                      variant="outline"
                      size="icon"
                      onClick={handleCopyUrl}
                      className="shrink-0"
                      aria-label={t("Copy URL")}
                    >
                      {copied ? (
                        <Check className="size-4" />
                      ) : (
                        <Copy className="size-4" />
                      )}
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      size="icon"
                      onClick={handleOpenUrl}
                      className="shrink-0"
                      aria-label={t("Open URL")}
                    >
                      <ExternalLink className="size-4" />
                    </Button>
                  </div>
                ) : (
                  // A static icon: this is an empty state waiting on the person,
                  // and a spinner here read as something loading forever.
                  <div className="flex items-center gap-2 text-muted-foreground">
                    <Info className="size-4" />
                    <span className="text-sm">{t("Add a hostname to generate the public URL.")}</span>
                  </div>
                )}
                <FormDescription>
                  {t("Share this URL to allow remote access to your AOS instance.")}
                </FormDescription>
              </div>

              <div className="flex items-center justify-between p-4">
                <div className="space-y-0.5">
                  <FormLabel>{t("Provider")}</FormLabel>
                  <FormDescription>
                    {t("Cloudflare Tunnel provides stable remote access for the desktop app.")}
                  </FormDescription>
                </div>
                <Badge variant="outline">{t("Cloudflare")}</Badge>
              </div>
            </FormSectionContent>
            <FormSectionFooter className="flex justify-end">
              <Button
                type="submit"
                disabled={isBusy}
              >
                {credentialsForm.isLoading ? t("Saving...") : t("Save changes")}
              </Button>
            </FormSectionFooter>
          </FormSection>
        </Form>
      </SettingsSectionShell>
    </div>
  );
}
