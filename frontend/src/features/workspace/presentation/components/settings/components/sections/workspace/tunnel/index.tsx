import * as React from "react";
import { useRouter } from "@tanstack/react-router";
import * as z from "zod";
import { Copy, Check, ExternalLink, Loader2, CircleCheck, CircleDot, Circle } from "lucide-react";
import { toast } from "sonner";
import { openExternal } from "@/lib/wails";

import { aos } from "@/app/aos";
import { api } from "@/lib/aos-facade";
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

const tunnelEnableFormSchema = z.object({
  enabled: z.boolean(),
});

const tunnelCredentialsFormSchema = z.object({
  hostname: z.string().optional().or(z.literal("")),
  token: z.string().optional().or(z.literal("")),
});

export function WorkspaceTunnelSection() {
  const router = useRouter();
  
  // `aos.useContext()` is AOS's global route context (`withContext(...)`),
  // which this port's `app/aos.tsx` never wires -- `DefaultContext` (`app/
  // builders/types.ts`) is deliberately loose (`Record<string, any>`) for
  // exactly this unset case, so no per-call-site cast is needed here.
  const context = aos.useContext();
  const tunnelStatusQuery = aos.client.tunnel.getStatus.useQuery();
  const tunnelStatus = tunnelStatusQuery.data;
  const [copied, setCopied] = React.useState(false);

  const activationForm = aos.useForm({
    schema: tunnelEnableFormSchema,
    mode: "onChange",
    values: {
      enabled: context.config?.tunnel?.enabled ?? false,
    },
    onSubmit: async (values) => {
      const previousEnabled = context.config?.tunnel?.enabled ?? false;

      try {
        await api.config.update.mutate({
          body: {
            tunnel: {
              enabled: values.enabled,
            },
          },
        });

        if (values.enabled) {
          const result = await api.tunnel.start.mutate();
          const publicUrl = result.data?.url ?? tunnelPublicUrl;
          toast.success(`Tunnel started! URL: ${publicUrl}`);
        } else {
          await api.tunnel.stop.mutate();
          toast.success(t("Tunnel stopped."));
        }

        await tunnelStatusQuery.refetch();
        router.invalidate();
      } catch (error) {
        try {
          await api.config.update.mutate({
            body: {
              tunnel: {
                enabled: previousEnabled,
              },
            },
          });
        } catch {
          // Ignore rollback failures so the original error can surface.
        }

        activationForm.reset({ enabled: previousEnabled });
        toast.error(error instanceof Error ? error.message : "Failed to update tunnel settings");
        throw error;
      }
    },
  });

  const credentialsForm = aos.useForm({
    schema: tunnelCredentialsFormSchema,
    values: {
      hostname: context.config?.tunnel?.hostname ?? "",
      token: context.config?.tunnel?.token ?? "",
    },
    onSubmit: async (values) => {
      const enabled = activationForm.getValues("enabled");
      const hostname = values.hostname?.trim() ?? "";
      const token = values.token?.trim() ?? "";

      try {
        await api.config.update.mutate({
          body: {
            tunnel: {
              hostname,
              token,
            },
          },
        });

        if (enabled && hostname && token) {
          const result = await api.tunnel.start.mutate();
          const publicUrl = result.data?.url ?? tunnelPublicUrl;
          toast.success(`Tunnel settings saved and started! URL: ${publicUrl}`);
        } else {
          toast.success(t("Tunnel settings saved."));
        }

        await tunnelStatusQuery.refetch();
        router.invalidate();
      } catch (error) {
        toast.error(error instanceof Error ? error.message : "Failed to save tunnel credentials");
        throw error;
      }
    },
  });

  const isTunnelEnabled = activationForm.watch("enabled");
  const tunnelHostname = credentialsForm.watch("hostname") ?? "";
  const tunnelToken = credentialsForm.watch("token") ?? "";
  const tunnelPublicUrl = TunnelUrlHelper.buildPublicUrl(tunnelHostname);
  const isTunnelConfigured = !!tunnelHostname.trim() && !!tunnelToken.trim();
  const isTunnelOnline = tunnelStatus?.online ?? false;
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
                      ? "Applying tunnel settings..."
                      : isTunnelOnline
                        ? "Your instance is accessible from the internet."
                        : isTunnelEnabled && isTunnelConfigured
                          ? "Tunnel is enabled and waiting to connect."
                          : isTunnelEnabled
                            ? "Tunnel is enabled, but hostname or token is missing."
                            : "Tunnel is disabled. Your instance is only accessible locally."}
                  </FormDescription>
                </div>
                {isBusy ? (
                  <Badge variant="outline" className="gap-1">
                    <Loader2 className="size-3 animate-spin text-blue-600" />
                    {t("Connecting")}
                  </Badge>
                ) : isTunnelOnline ? (
                  <Badge variant="outline" className="gap-1">
                    <CircleCheck className="size-3 text-emerald-400" />
                    {t("Online")}
                  </Badge>
                ) : isTunnelEnabled ? (
                  <Badge variant="outline" className="gap-1">
                    <CircleDot className="size-3 text-blue-600" />
                    {t("Connecting")}
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
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={isBusy}
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
                    >
                      <ExternalLink className="size-4" />
                    </Button>
                  </div>
                ) : (
                  <div className="flex items-center gap-2 text-muted-foreground">
                    <Loader2 className="size-4 animate-spin" />
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
                {credentialsForm.isLoading ? "Saving..." : "Save Changes"}
              </Button>
            </FormSectionFooter>
          </FormSection>
        </Form>
      </SettingsSectionShell>
    </div>
  );
}
