import { useEffect, useMemo, type ComponentType, type SVGProps } from "react";
import { useFieldArray } from "react-hook-form";
import { Plus, Trash2 } from "lucide-react";
import { z } from "zod";
import { toast } from "sonner";

import { aos } from "@/app/aos";
import { SettingsSectionShell } from "@/features/workspace/presentation/components/settings/components/section-shell";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { TelegramIcon, WhatsAppIcon } from "@/components/icons/chat-providers";
import { DiscordIcon } from "@/components/icons/discord-icon";
import { FractalAppError } from "@/core/errors/fractal.error";
import type { FractalAgent } from "@/features/agent/interfaces/agent.interfaces";

const CHANNEL_PROVIDERS = ["telegram", "discord", "slack", "whatsapp"] as const;
type ChannelProvider = (typeof CHANNEL_PROVIDERS)[number];

type AllowedIdEntry = { value: string };

function SlackIcon({ className, ...props }: SVGProps<SVGSVGElement>) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={`h-4 w-4 ${className ?? ""}`.trim()}
      fill="currentColor"
      {...props}
    >
      <rect x="3" y="9" width="6" height="3" rx="1.5" />
      <rect x="9" y="3" width="3" height="6" rx="1.5" />
      <rect x="12" y="12" width="6" height="3" rx="1.5" />
      <rect x="12" y="15" width="3" height="6" rx="1.5" />
    </svg>
  );
}

const telegramFormSchema = z.object({
  token: z.string().optional().default(""),
  allowedIds: z
    .array(
      z.object({
        value: z.string().optional().default(""),
      }),
    )
    .default([]),
});

const CHANNELS = [
  {
    provider: "telegram" as const,
    label: "Telegram",
    description: "Connect a Telegram bot token and allow specific chat ids.",
    icon: TelegramIcon,
    comingSoon: false,
  },
  {
    provider: "discord" as const,
    label: "Discord",
    description: "Channel bindings for Discord will arrive in a later release.",
    icon: DiscordIcon,
    comingSoon: true,
  },
  {
    provider: "slack" as const,
    label: "Slack",
    description: "Slack support is planned, but not enabled yet.",
    icon: SlackIcon,
    comingSoon: true,
  },
  {
    provider: "whatsapp" as const,
    label: "WhatsApp",
    description: "WhatsApp support is planned, but not enabled yet.",
    icon: WhatsAppIcon,
    comingSoon: true,
  },
] satisfies Array<{
  provider: ChannelProvider;
  label: string;
  description: string;
  icon: ComponentType<SVGProps<SVGSVGElement>>;
  comingSoon: boolean;
}>;

interface AgentChannelsTabProps {
  agent: FractalAgent;
}

function normalizeAllowedIds(value: unknown): AllowedIdEntry[] {
  if (!Array.isArray(value)) return [];

  return value
    .map((entry) => {
      if (typeof entry === "string") {
        return { value: entry };
      }

      if (entry && typeof entry === "object") {
        const candidate = entry as Record<string, unknown>;
        const nextValue =
          typeof candidate.value === "string"
            ? candidate.value
            : typeof candidate.id === "string"
              ? candidate.id
              : "";

        return { value: nextValue };
      }

      return { value: "" };
    })
    .filter((entry) => entry.value.trim().length > 0);
}

function getTelegramConfig(agent: FractalAgent) {
  const telegram = agent.channels?.find((channel) => channel.provider === "telegram");
  const data = telegram?.data ?? {};

  return {
    token: typeof data.token === "string" ? data.token : "",
    allowedIds: normalizeAllowedIds(data.allowedIds),
  };
}

function buildChannelsPayload(agent: FractalAgent, token: string, allowedIds: AllowedIdEntry[]) {
  const telegramChannel = {
    provider: "telegram",
    data: {
      token: token.trim(),
      allowedIds: allowedIds
        .map((entry) => entry.value.trim())
        .filter((value) => value.length > 0),
    },
  };

  const otherChannels = (agent.channels ?? []).filter((channel) => channel.provider !== "telegram");
  return [telegramChannel, ...otherChannels];
}

export function AgentChannelsTab({ agent }: AgentChannelsTabProps) {
  const telegramConfig = useMemo(() => getTelegramConfig(agent), [agent.channels]);

  const form = aos.useForm({
    schema: telegramFormSchema,
    mutation: "agent.update",
    values: telegramConfig,
    onSubmit: (values) => ({
      params: { agent: agent.id },
      body: {
        channels: buildChannelsPayload(agent, values.token, values.allowedIds),
      },
    }),
    onResponse: ({ error }) => {
      if (!error) {
        toast.success("Telegram channel updated successfully.");
        void aos.stores.agent.actions.refresh();
        return;
      }

      if (error instanceof FractalAppError) {
        toast.error(error.message);
        return;
      }

      console.error(error);
      toast.error(error.message || "Failed to update Telegram channel");
    },
  });

  // Same `aos.useForm` generic-inference gap as `tasks/index.tsx`'s
  // `useFieldArray` — `form.control`'s array-item shape doesn't propagate
  // through to `useFieldArray` without help.
  const allowedIdsFieldArray = useFieldArray({
    control: form.control as any,
    name: "allowedIds",
  });

  useEffect(() => {
    form.reset(telegramConfig);
  }, [form, telegramConfig, agent.id]);

  const telegramToken = form.watch("token") ?? "";
  const telegramStatus = telegramToken.trim().length > 0 ? "Configured" : "Not configured";

  return (
    <Form form={form} className="flex h-full flex-1 flex-col overflow-y-auto">
      <SettingsSectionShell className="relative" contentClassName="max-w-4xl">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-1">
            <h1 className="text-sm font-semibold tracking-tight">Channels</h1>
            <p className="text-sm text-muted-foreground">
              Configure how agents can be reached across messaging platforms. Telegram is the only
              provider enabled for now.
            </p>
          </div>

          <Accordion type="single" collapsible defaultValue="telegram" className="w-full">
            {CHANNELS.map((channel) => {
              const Icon = channel.icon;
              const isTelegram = channel.provider === "telegram";

              return (
                <AccordionItem
                  key={channel.provider}
                  value={channel.provider}
                  disabled={!isTelegram}
                  className="rounded-lg border border-border/70 bg-card px-1 data-[state=open]:bg-muted/20"
                >
                  <AccordionTrigger className="rounded-md px-3 py-3 hover:no-underline">
                    <span className="flex w-full items-center gap-3">
                      <span className="flex size-8 shrink-0 items-center justify-center rounded-md border border-border bg-background">
                        <Icon className="size-4" />
                      </span>
                      <span className="flex min-w-0 flex-1 flex-col items-start gap-0.5 text-left">
                        <span className="text-sm font-medium text-foreground">{channel.label}</span>
                        <span className="line-clamp-1 text-xs text-muted-foreground">
                          {channel.description}
                        </span>
                      </span>
                      <Badge
                        variant={isTelegram && telegramStatus === "Configured" ? "secondary" : "outline"}
                        className="shrink-0"
                      >
                        {isTelegram ? telegramStatus : "Coming soon"}
                      </Badge>
                    </span>
                  </AccordionTrigger>

                  <AccordionContent className="px-3 pb-3 pt-1">
                    {isTelegram ? (
                      <div className="space-y-5">
                        <FormField
                          control={form.control}
                          name="token"
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>Bot token</FormLabel>
                              <FormControl>
                                <Input
                                  {...field}
                                  type="password"
                                  autoComplete="off"
                                  placeholder="123456:ABC-DEF..."
                                  value={field.value ?? ""}
                                />
                              </FormControl>
                              <FormDescription>
                                Paste the Telegram bot token that will authenticate this agent.
                              </FormDescription>
                            </FormItem>
                          )}
                        />

                        <div className="space-y-3">
                          <div className="flex items-center justify-between gap-3">
                            <div className="space-y-1">
                              <h3 className="text-sm font-medium text-foreground">Allowed ids</h3>
                              <p className="text-xs text-muted-foreground">
                                Add the Telegram user, chat, or group ids that can message this agent.
                              </p>
                            </div>

                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              onClick={() => allowedIdsFieldArray.append({ value: "" })}
                            >
                              <Plus className="size-4" />
                              Add id
                            </Button>
                          </div>

                          <div className="space-y-3">
                            {allowedIdsFieldArray.fields.length === 0 ? (
                              <div className="rounded-lg border border-dashed border-border/70 px-4 py-5 text-sm text-muted-foreground">
                                No ids added yet. Start with the first allowed Telegram id.
                              </div>
                            ) : (
                              allowedIdsFieldArray.fields.map((field, index) => (
                                <div
                                  key={field.id}
                                  className="flex items-end gap-2 rounded-lg border border-border/70 bg-muted/10 px-3 py-3"
                                >
                                  <FormField
                                    control={form.control}
                                    name={`allowedIds.${index}.value` as const}
                                    render={({ field: allowedField }) => (
                                      <FormItem className="flex-1">
                                        <FormLabel className="sr-only">Allowed id</FormLabel>
                                        <FormControl>
                                          <Input
                                            {...allowedField}
                                            placeholder="123456789"
                                            value={allowedField.value ?? ""}
                                          />
                                        </FormControl>
                                      </FormItem>
                                    )}
                                  />

                                  <Button
                                    type="button"
                                    variant="ghost"
                                    size="icon"
                                    className="size-9 shrink-0 text-muted-foreground hover:text-destructive"
                                    onClick={() => allowedIdsFieldArray.remove(index)}
                                  >
                                    <Trash2 className="size-4" />
                                    <span className="sr-only">Remove allowed id</span>
                                  </Button>
                                </div>
                              ))
                            )}
                          </div>
                        </div>

                        <div className="flex items-center justify-end border-t border-border/60 pt-4">
                          <Button type="submit" disabled={form.isLoading}>
                            {form.isLoading ? "Saving..." : "Save Telegram"}
                          </Button>
                        </div>
                      </div>
                    ) : (
                      <div className="rounded-lg border border-dashed border-border/70 px-4 py-5 text-sm text-muted-foreground">
                        This channel is coming soon.
                      </div>
                    )}
                  </AccordionContent>
                </AccordionItem>
              );
            })}
          </Accordion>
        </div>
      </SettingsSectionShell>
    </Form>
  );
}
