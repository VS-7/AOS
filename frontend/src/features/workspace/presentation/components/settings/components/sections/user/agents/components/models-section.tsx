"use client";

import * as React from "react";

import {
  AgentModelSelect,
  type AgentModelReasoning,
  type AgentModelSelectProvider,
  type AgentModelSelectValue,
} from "@/components/ui/agent-model-select";
import type { ModelProvider } from "@/features/model/interfaces/model.interfaces";
import { useProviderLogo } from "../hooks/use-provider-logo";

type SlotKey = "default" | "subconscious" | "realtime" | "voice" | "image" | "video";

const SLOT_META: Record<
  SlotKey,
  { title: string; description: string; capability: "fast" | "reasoning" | "vision" | "realtime" | "voice" | "image" | "video" }
> = {
  default: {
    title: "Default",
    description: "The agent's core reasoning — used for every task.",
    capability: "fast",
  },
  subconscious: {
    title: "Subconscious",
    description: "Background work: context compaction and memory dreaming.",
    capability: "fast",
  },
  realtime: {
    title: "Realtime",
    description: "Live voice and streaming sessions with the agent.",
    capability: "realtime",
  },
  voice: {
    title: "Voice",
    description: "Outbound voice messages from the agent to the user.",
    capability: "voice",
  },
  image: {
    title: "Image",
    description: "Image generation from text prompts with the agent.",
    capability: "image",
  },
  video: {
    title: "Video",
    description: "Video generation from text prompts with the agent.",
    capability: "video",
  },
};

interface ModelsSectionProps {
  providers: ModelProvider[];
  value: Partial<Record<SlotKey, AgentModelSelectValue>>;
  onChange: (slot: SlotKey, next: AgentModelSelectValue) => void;
}

function modelHasCapability(
  model: ModelProvider["models"][number],
  capability: "fast" | "reasoning" | "vision" | "realtime" | "voice" | "image" | "video",
): boolean {
  if (capability === "fast") return true;
  if (capability === "vision") return !!model.capabilities?.vision;
  if (capability === "reasoning") return !!model.capabilities?.reasoning;
  if (capability === "realtime") return !!model.capabilities?.realtime;
  if (capability === "voice") return !!model.capabilities?.voice;
  if (capability === "image") return !!model.capabilities?.image;
  if (capability === "video") return !!model.capabilities?.video;
  return true;
}

function toSelectProviders(
  providers: ModelProvider[],
  capability: "fast" | "reasoning" | "vision" | "realtime" | "voice" | "image" | "video",
): AgentModelSelectProvider[] {
  return providers
    .filter((p) => p.configured)
    .map((p) => {
      const supportedModels = p.models.filter((m) =>
        modelHasCapability(m, capability),
      );
      // Hide providers that don't expose any model for this capability.
      if (capability !== "fast" && supportedModels.length === 0) {
        return null;
      }
      return {
        id: p.id,
        name: p.name,
        configured: p.configured,
        models: supportedModels.map((m) => ({ id: m.id, name: m.name })),
        renderLogo: () => <ProviderLogo provider={p} />,
      };
    })
    .filter((p): p is NonNullable<typeof p> => p !== null);
}

function ProviderLogo({ provider }: { provider: ModelProvider }) {
  const src = useProviderLogo(provider);
  if (!src) return null;
  return (
    <img
      src={src}
      alt={`${provider.name} logo`}
      className="size-3.5 shrink-0 rounded"
    />
  );
}

/**
 * What this slot should show: the saved value, or nothing.
 *
 * This used to invent a value — first configured provider, its first
 * model — whenever the slot had none saved, and return it as if it were
 * the current selection. Nothing persisted it: `onChange` only fires when
 * somebody actually picks. So connecting any provider made all six slots
 * *look* configured while `agents.models` stayed `{}` in the config, and
 * the one slot that matters (`default`, the only one the runtime reads to
 * answer a chat) was empty. The agent then failed every message with
 * `AOS_AGENT_PROVIDER_NOT_ENABLED` while the screen showed a model
 * sitting right there. A control has to show what is saved, or a person
 * cannot tell configured from unconfigured.
 *
 * `setModelProviderKey` now seeds `default` on connect, so the common
 * path arrives here with a real stored value. A slot whose saved
 * provider has since been disconnected also reads as unset: the stored
 * pick is no longer usable, and saying so is what prompts a new one.
 */
function resolveSlotValue(
  providers: ModelProvider[],
  current: AgentModelSelectValue | undefined,
): AgentModelSelectValue {
  if (current?.provider) {
    const provider = providers.find((p) => p.id === current.provider);
    if (provider?.configured) {
      return current;
    }
  }
  return { provider: "", model: "", reasoning: current?.reasoning };
}

export function ModelsSection({ providers, value, onChange }: ModelsSectionProps) {
  return (
    <div className="rounded-xl border border-border bg-secondary/50 overflow-hidden divide-y divide-border">
      {(Object.keys(SLOT_META) as SlotKey[]).map((slot) => {
        const meta = SLOT_META[slot];
        const selectProviders = toSelectProviders(providers, meta.capability);
        const current = resolveSlotValue(providers, value[slot]);

        const currentModel = providers
          .find((p) => p.id === current.provider)
          ?.models.find((m) => m.id === current.model);
        const hasReasoning =
          meta.capability === "reasoning" ||
          currentModel?.capabilities?.reasoning === true;

        return (
          <div
            key={slot}
            className="flex items-center gap-3 p-4 min-h-16"
          >
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium leading-tight">{meta.title}</p>
              <p className="text-xs text-muted-foreground leading-tight">
                {meta.description}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <AgentModelSelect
                providers={selectProviders}
                value={current}
                onChange={(next: AgentModelSelectValue) => onChange(slot, next)}
                showReasoning={hasReasoning}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}

export type { SlotKey };
export type ModelsValue = Partial<Record<SlotKey, AgentModelSelectValue>>;

export function emptyModelsValue(): ModelsValue {
  return {};
}
