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
import { useModelProviders } from "@/features/model/services/model-provider.service";
import type { ConfigAgentModels } from "@/features/config/interfaces/config.interfaces";
import { ModelsSection } from "./components/models-section";
import { ProvidersSection } from "./components/providers-section";
import type { AgentModelSelectValue } from "@/components/ui/agent-model-select";
import { useModelSlots, type SlotKey } from "./hooks/use-model-slots";
import { t } from "@/lib/i18n";

export function UserAgentsSection() {
  const router = useRouter();

  // `aos.useContext()` is AOS's global route context (`withContext(...)`),
  // which this port's `app/aos.tsx` never wires -- `DefaultContext` (`app/
  // builders/types.ts`) is deliberately loose (`Record<string, any>`) for
  // exactly this unset case, so no per-call-site cast is needed here.
  const context = aos.useContext();
  const config = context.config;
  const savedModels = config?.agents?.models as ConfigAgentModels | undefined;

  const providers = useModelProviders();

  const persist = React.useCallback(
    async (slot: SlotKey, next: AgentModelSelectValue) => {
      const currentBucket = savedModels?.[slot];
      // Persist only the slot we touched to avoid stomping the others: the
      // map is one patch leaf, so it is written whole from what is saved.
      await aos.stores.config.actions.update({
        agents: {
          models: {
            ...(savedModels ?? {}),
            [slot]: {
              provider: next.provider,
              model: next.model,
              reasoning: next.reasoning ?? currentBucket?.reasoning ?? "medium",
            },
          } as ConfigAgentModels,
        },
      });
      await router.invalidate();
    },
    [savedModels, router],
  );

  const slots = useModelSlots(savedModels, persist);

  const handleSlotChange = React.useCallback(
    (slot: SlotKey, next: AgentModelSelectValue) => {
      slots.change(slot, next).catch((error: unknown) => {
        toast.error(error instanceof Error ? error.message : t("Failed to save model preference."));
      });
    },
    [slots],
  );

  return (
    <SettingsSectionShell>
      <FormSection>
        <FormSectionHeader>
          <FormSectionTitle>{t("Providers")}</FormSectionTitle>
          <FormSectionDescription>
            {t("Connect the AI providers you want to use in AOS.")}
          </FormSectionDescription>
        </FormSectionHeader>
        <FormSectionContent>
          <ProvidersSection providers={providers} models={savedModels} />
        </FormSectionContent>
      </FormSection>

      <FormSection>
        <FormSectionHeader>
          <FormSectionTitle>{t("Models")}</FormSectionTitle>
          <FormSectionDescription>
            {t("Pick the model AOS uses for each kind of work.")}
          </FormSectionDescription>
        </FormSectionHeader>
        <FormSectionContent>
          <ModelsSection
            providers={providers}
            value={slots.value}
            onChange={handleSlotChange}
          />
        </FormSectionContent>
      </FormSection>
    </SettingsSectionShell>
  );
}

export default UserAgentsSection;
