import * as z from "zod";

import { aos } from "@/app/aos";
import { SettingsSectionShell } from "../../../section-shell";
import {
  FormSection,
  FormSectionContent,
  FormSectionDescription,
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
import { AppError } from "@/core/errors/aos.error";
import type { Config } from "@/features/config/interfaces/config.interfaces";
import { toast } from "sonner";
import { t } from "@/lib/i18n";
import { changedSettings } from "../../../../helpers/changed-settings";
import { saveConfigSettings } from "../../../../helpers/save-config-settings";

const generalFormSchema = z.object({
  preventSleep: z.boolean(),
  enableNotifications: z.boolean(),
});

/** The configuration paths this form edits, with the values it shows for them. */
function generalSettings(config: Config | undefined) {
  return {
    "general.preventSleep": config?.general?.preventSleep ?? false,
    "notifications.enabled": config?.notifications?.enabled ?? true,
  };
}

export function UserGeneralSection() {
  // The configuration store, not the route context: the context is a copy
  // taken when the route loaded, and a save never reached it — a switch
  // turned off showed on again after visiting another section, and the next
  // toggle sent it back on.
  const config = aos.stores.config.useState();
  const shown = generalSettings(config);

  const form = aos.useForm({
    schema: generalFormSchema,
    mode: "onChange",
    values: {
      preventSleep: shown["general.preventSleep"],
      enableNotifications: shown["notifications.enabled"],
    },
    onSubmit: async (values) => {
      const next = {
        "general.preventSleep": values.preventSleep,
        "notifications.enabled": values.enableNotifications,
      };
      if (await saveConfigSettings(changedSettings("", next, generalSettings(aos.stores.config.state)))) {
        toast.success(t("Settings updated successfully!"));
      }
      return values;
    },
    onResponse: ({ error }) => {
      if (!error) return;
      if (error instanceof AppError) {
        toast.error(error.message);
        return;
      }
      toast.error(error.message || t("Failed to update general settings"));
    },
  });

  // `disableLoadingState`: this form saves itself while the person is still
  // typing, and disabling its fields for each save took the focus out of the
  // one being typed in — the keystrokes after it went nowhere.
  return (
    <Form form={form} disableLoadingState className="flex h-full flex-1 flex-col overflow-y-auto">
      <SettingsSectionShell>
        <FormSection>
          <FormSectionHeader>
            <FormSectionTitle>{t("Behavior")}</FormSectionTitle>
            <FormSectionDescription>
              {t("Configure execution behaviors.")}
            </FormSectionDescription>
          </FormSectionHeader>
          <FormSectionContent className="divide-y divide-border">
            <FormField
              control={form.control}
              name="preventSleep"
              render={({ field }) => (
                <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                  <div className="space-y-0.5">
                    <FormLabel>{t("Prevent sleep during execution")}</FormLabel>
                    <FormDescription>
                      {t("Keeps your computer awake while running long tasks.")}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch checked={field.value} onCheckedChange={field.onChange} />
                  </FormControl>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="enableNotifications"
              render={({ field }) => (
                <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                  <div className="space-y-0.5">
                    <FormLabel>{t("Enable permission notifications")}</FormLabel>
                    <FormDescription>
                      {t("Show OS notifications when a permission is required.")}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch checked={field.value} onCheckedChange={field.onChange} />
                  </FormControl>
                </FormItem>
              )}
            />
          </FormSectionContent>
        </FormSection>
      </SettingsSectionShell>
    </Form>
  );
}
