"use client";

import * as React from "react";
import { useRouter } from "@tanstack/react-router";

import { SettingsSectionShell } from "../../../section-shell";
import {
  FormSection,
  FormSectionContent,
  FormSectionDescription,
  FormSectionHeader,
  FormSectionTitle,
} from "@/components/ui/form-section";
import { aos } from "@/app/aos";
import { toast } from "sonner";
import type { ModelProvider } from "@/features/model/interfaces/model.interfaces";
import type { ConfigAgentModels } from "@/features/config/interfaces/config.interfaces";
import {
  ModelsSection,
  emptyModelsValue,
  type ModelsValue,
  type SlotKey,
} from "./components/models-section";
import { ProvidersSection } from "./components/providers-section";
import type { AgentModelSelectValue } from "@/components/ui/agent-model-select";

const DEFAULT_AGENT_MODELS: ConfigAgentModels = {
  default: { provider: "", model: "", reasoning: "medium" },
  subconscious: { provider: "", model: "", reasoning: "medium" },
  realtime: { provider: "", model: "", reasoning: "medium" },
  voice: { provider: "", model: "", reasoning: "medium" },
  image: { provider: "", model: "", reasoning: "medium" },
  video: { provider: "", model: "", reasoning: "medium" },
};

export function UserAgentsSection() {
  const router = useRouter();
  
  // `aos.useContext()` is AOS's global route context (`withContext(...)`),
  // which this port's `app/aos.tsx` never wires -- `DefaultContext` (`app/
  // builders/types.ts`) is deliberately loose (`Record<string, any>`) for
  // exactly this unset case, so no per-call-site cast is needed here.
  const context = aos.useContext();
  const config = context.config;

  const providersQuery = aos.client.model.list.useQuery({
    query: {
      mode: "available"
    }
  });
  const providers: ModelProvider[] =
    (providersQuery.data as ModelProvider[] | undefined) ?? [];

  const [models, setModels] = React.useState<ModelsValue>(() => {
    const result: ModelsValue = {};
    const modelBuckets = config?.agents?.models;
    if (modelBuckets) {
      (["default", "subconscious", "realtime", "voice", "image", "video"] as const).forEach(
        (key) => {
          const entry = modelBuckets[key];
          if (entry) {
            result[key] = {
              provider: entry.provider ?? "",
              model: entry.model ?? "",
              reasoning: entry.reasoning,
            };
          }
        },
      );
    }
    return Object.keys(result).length > 0 ? result : emptyModelsValue();
  });

  React.useEffect(() => {
    const modelBuckets = config?.agents?.models;
    if (!modelBuckets) return;
    const updated: ModelsValue = {};
    (["default", "subconscious", "realtime", "voice", "image", "video"] as const).forEach(
      (key) => {
        const entry = modelBuckets[key];
        if (entry) {
          updated[key] = {
            provider: entry.provider ?? "",
            model: entry.model ?? "",
            reasoning: entry.reasoning,
          };
        }
      },
    );
    setModels(updated);
  }, [config?.agents?.models]);

  const handleSlotChange = React.useCallback(
    async (slot: SlotKey, next: AgentModelSelectValue) => {
      setModels((prev) => ({ ...prev, [slot]: next }));

      const currentBucket = config?.agents?.models?.[slot];

      const updatedBucket = {
        provider: next.provider,
        model: next.model,
        reasoning: next.reasoning ?? currentBucket?.reasoning ?? "medium",
      } as const;

      // Persist only the slot we touched to avoid stomping the other bucket.
      try {
        const result = await aos.stores.config.actions.update({
          agents: {
            models: {
              ...(config?.agents?.models ?? DEFAULT_AGENT_MODELS),
              [slot]: updatedBucket,
            },
          },
        });
        if (!result) {
          toast.error("Failed to save model preference.");
          return;
        }
        router.invalidate();
      } catch (error) {
        console.error(error);
        toast.error(
          error instanceof Error
            ? error.message
            : "Failed to save model preference.",
        );
      }
    },
    [config?.agents?.models, router],
  );

  return (
    <SettingsSectionShell>
      <FormSection>
        <FormSectionHeader>
          <FormSectionTitle>Providers</FormSectionTitle>
          <FormSectionDescription>
            Connect the AI providers you want to use in AOS.
          </FormSectionDescription>
        </FormSectionHeader>
        <FormSectionContent>
          <ProvidersSection providers={providers} onRefresh={() => { void providersQuery.refetch(); }} />
        </FormSectionContent>
      </FormSection>

      <FormSection>
        <FormSectionHeader>
          <FormSectionTitle>Models</FormSectionTitle>
          <FormSectionDescription>
            Pick the model AOS uses for each kind of work.
          </FormSectionDescription>
        </FormSectionHeader>
        <FormSectionContent>
          <ModelsSection
            providers={providers}
            value={models}
            onChange={handleSlotChange}
          />
        </FormSectionContent>
      </FormSection>
    </SettingsSectionShell>
  );
}

export default UserAgentsSection;
