import * as z from "zod";
import { useRouter } from "@tanstack/react-router";

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
import { toast } from "sonner";

const generalFormSchema = z.object({
  preventSleep: z.boolean(),
  enableNotifications: z.boolean(),
});

export function UserGeneralSection() {
  const router = useRouter();
  
  // `aos.useContext()` is AOS's global route context (`withContext(...)`),
  // which this port's `app/aos.tsx` never wires -- `DefaultContext` (`app/
  // builders/types.ts`) is deliberately loose (`Record<string, any>`) for
  // exactly this unset case, so no per-call-site cast is needed here.
  const context = aos.useContext();

  const form = aos.useForm({
    schema: generalFormSchema,
    mutation: "config.update",
    mode: "onChange",
    values: {
      preventSleep: context.config?.general?.preventSleep ?? false,
      enableNotifications: context.config?.notifications?.enabled ?? true,
    },
    // Two `Config` sections in one submit — built as `{set: {...}}}` here
    // directly rather than through `command-map.ts`'s `config.update`
    // `coerceIn` (see that entry's own comment): a coercion per field would
    // collide on the `set` key it produces, the same situation `workspace.
    // update` documents for its own `.../profile` call site.
    onSubmit: (values) => ({
      body: {
        set: {
          "general.preventSleep": values.preventSleep,
          "notifications.enabled": values.enableNotifications,
        },
      },
    }),
    onResponse: ({ error }) => {
      if (error) {
        if (error instanceof AppError) {
          toast.error(error.message);
          return;
        }

        console.error(error);
        toast.error(error.message || "Failed to update general settings");
        return;
      }

      toast.success("Settings updated succesfully!");
      router.invalidate();
    },
  });

  return (
    <Form form={form} className="flex h-full flex-1 flex-col overflow-y-auto">
      <SettingsSectionShell>
        <FormSection>
          <FormSectionHeader>
            <FormSectionTitle>Behavior</FormSectionTitle>
            <FormSectionDescription>
              Configure execution behaviors.
            </FormSectionDescription>
          </FormSectionHeader>
          <FormSectionContent className="divide-y divide-border">
            <FormField
              control={form.control}
              name="preventSleep"
              render={({ field }) => (
                <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                  <div className="space-y-0.5">
                    <FormLabel>Prevent sleep during execution</FormLabel>
                    <FormDescription>
                      Keeps your computer awake while running long tasks.
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
                    <FormLabel>Enable permission notifications</FormLabel>
                    <FormDescription>
                      Show OS notifications when a permission is required.
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
