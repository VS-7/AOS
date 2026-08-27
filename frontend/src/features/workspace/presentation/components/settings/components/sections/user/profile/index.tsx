import { useRouter } from "@tanstack/react-router";
import * as z from "zod";

import { aos } from "@/app/aos";
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
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
} from "@/components/ui/form";
import { ImageUpload } from "@/components/ui/image-upload";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { AppError } from "@/core/errors/aos.error";
import { api } from "@/lib/aos-facade";
import { t, LOCALES, LOCALE_NAMES, normalizeLocale, setLocale } from "@/lib/i18n";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

const profileFormSchema = z.object({
  name: z.string().min(2),
  email: z.string().email(),
  image: z.string().optional().or(z.literal("")),
  timezone: z.string(),
  language: z.string(),
  city: z.string().optional(),
  country: z.string().optional(),
});

const passwordFormSchema = z
  .object({
    currentPassword: z.string().optional().or(z.literal("")),
    newPassword: z.string().optional().or(z.literal("")),
    verifyPassword: z.string().optional().or(z.literal("")),
  })
  .superRefine((data, ctx) => {
    const wantsPasswordChange = Boolean(
      data.currentPassword || data.newPassword || data.verifyPassword,
    );

    if (!wantsPasswordChange) return;

    if (!data.currentPassword) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "Current password is required",
        path: ["currentPassword"],
      });
    }

    if (!data.newPassword || data.newPassword.length < 6) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "New password must be at least 6 characters",
        path: ["newPassword"],
      });
    }

    if (data.newPassword !== data.verifyPassword) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "Passwords do not match",
        path: ["verifyPassword"],
      });
    }
  });

/**
 * Account identity, region, and password — AuthStore + config region.
 */
export function UserProfileSection() {
  const router = useRouter();
  
  // `aos.useContext()` is AOS's global route context (`withContext(...)`),
  // which this port's `app/aos.tsx` never wires -- `DefaultContext` (`app/
  // builders/types.ts`) is deliberately loose (`Record<string, any>`) for
  // exactly this unset case, so no per-call-site cast is needed here.
  const context = aos.useContext();
  const authUser = aos.stores.auth.useState((state) => state.user);
  const config = context.config;

  const profileForm = aos.useForm({
    schema: profileFormSchema,
    mode: "onChange",
    values: {
      name: authUser?.name || "",
      email: authUser?.email || "",
      image: authUser?.image || "",
      timezone:
        config?.region?.timezone ||
        Intl.DateTimeFormat().resolvedOptions().timeZone,
      language: config?.region?.language || "en-US",
      city: config?.region?.city || "",
      country: config?.region?.country || "",
    },
    onSubmit: async (values) => {
      const profileResult = await aos.stores.auth.actions.updateProfile({
        name: values.name,
        email: values.email,
        image: values.image || undefined,
      });

      if (profileResult.error) {
        throw profileResult.error;
      }

      const regionResult = await api.config.update.mutate({
        body: {
          region: {
            timezone: values.timezone,
            language: values.language,
            city: values.city,
            country: values.country,
          },
        },
      });

      if (regionResult.error) {
        throw regionResult.error;
      }

      return values;
    },
    onResponse: ({ error }) => {
      if (error) {
        if (error instanceof AppError) {
          toast.error(error.message);
          return;
        }

        toast.error(
          error instanceof Error ? error.message : "Failed to update profile",
        );
        return;
      }

      toast.success(t("Profile updated successfully!"));
      router.invalidate();
    },
  });

  const passwordForm = aos.useForm({
    schema: passwordFormSchema,
    values: {
      currentPassword: "",
      newPassword: "",
      verifyPassword: "",
    },
    onSubmit: async (values) => {
      const wantsPasswordChange = Boolean(
        values.currentPassword || values.newPassword || values.verifyPassword,
      );

      if (!wantsPasswordChange) {
        return {
          currentPassword: "",
          newPassword: "",
          verifyPassword: "",
        };
      }

      // `verifyPassword` is the form's own confirmation field, checked by
      // the schema above; the daemon takes the two it acts on.
      const passwordResult = await aos.stores.auth.actions.updatePassword({
        currentPassword: values.currentPassword!,
        newPassword: values.newPassword!,
      });

      if (passwordResult.error) {
        throw passwordResult.error;
      }

      return {
        currentPassword: "",
        newPassword: "",
        verifyPassword: "",
      };
    },
    onResponse: ({ error }) => {
      if (error) {
        if (error instanceof AppError) {
          toast.error(error.message);
          return;
        }

        toast.error(
          error instanceof Error
            ? error.message
            : "Failed to update password",
        );
        return;
      }

      toast.success(t("Password updated successfully!"));
      router.invalidate();
    },
  });

  return (
    <div className="flex h-full flex-1 flex-col overflow-y-auto">
      <Form form={profileForm}>
        <SettingsSectionShell>
          <FormSection>
            <FormSectionHeader>
              <FormSectionTitle>{t("Basic Info")}</FormSectionTitle>
              <FormSectionDescription>
                {t("Your name and how you appear in AOS.")}
              </FormSectionDescription>
            </FormSectionHeader>
            <FormSectionContent className="divide-y divide-border">
              <FormField
                control={profileForm.control}
                name="image"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                    <div className="flex-1 space-y-0.5">
                      <FormLabel>{t("Avatar")}</FormLabel>
                      <FormDescription>
                        {t("Your profile photo.")}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <ImageUpload
                        value={field.value}
                        fallback={authUser?.name || "U"}
                        onChange={field.onChange}
                        onRemove={() => field.onChange("")}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
              <FormField
                control={profileForm.control}
                name="name"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                    <div className="flex-1 space-y-0.5">
                      <FormLabel>{t("Name")}</FormLabel>
                      <FormDescription>
                        {t("How your name is shown in the app.")}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Input
                        className="max-w-50"
                        placeholder={t("Your name")}
                        {...field}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
              <FormField
                control={profileForm.control}
                name="email"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                    <div className="flex-1 space-y-0.5">
                      <FormLabel>{t("Email")}</FormLabel>
                      <FormDescription>
                        {t("Email used to sign in.")}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Input
                        className="max-w-50"
                        type="email"
                        placeholder={t("you@example.com")}
                        {...field}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            </FormSectionContent>
          </FormSection>

          <FormSection>
            <FormSectionHeader>
              <FormSectionTitle>{t("Region")}</FormSectionTitle>
              <FormSectionDescription>
                {t("Timezone and language preferences.")}
              </FormSectionDescription>
            </FormSectionHeader>
            <FormSectionContent className="divide-y divide-border">
              <FormField
                control={profileForm.control}
                name="timezone"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                    <div className="flex-1 space-y-0.5">
                      <FormLabel>{t("Timezone")}</FormLabel>
                      <FormDescription>
                        {t("Your local timezone.")}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Input
                        className="max-w-50"
                        placeholder={t("America/Sao_Paulo")}
                        {...field}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
              <FormField
                control={profileForm.control}
                name="language"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                    <div className="flex-1 space-y-0.5">
                      <FormLabel>{t("Language")}</FormLabel>
                      <FormDescription>
                        {t("The language the interface is shown in.")}
                      </FormDescription>
                    </div>
                    <FormControl>
                      {/*
                        * A select, not the free-text box this used to be: the
                        * field decides which of two catalogues the interface
                        * renders from, so a typo here used to mean silently
                        * getting English back with no way to tell why.
                        *
                        * It applies on change rather than on save — a language
                        * you cannot see until you submit is a language you
                        * cannot check.
                        */}
                      <Select
                        value={normalizeLocale(field.value) ?? "en"}
                        onValueChange={(next) => {
                          field.onChange(next);
                          setLocale(next as (typeof LOCALES)[number]);
                        }}
                      >
                        <SelectTrigger className="max-w-50">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {LOCALES.map((locale) => (
                            <SelectItem key={locale} value={locale}>
                              {LOCALE_NAMES[locale]}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </FormControl>
                  </FormItem>
                )}
              />
            </FormSectionContent>
          </FormSection>

          <FormSection>
            <FormSectionHeader>
              <FormSectionTitle>{t("Location")}</FormSectionTitle>
              <FormSectionDescription>
                {t("Where you are based.")}
              </FormSectionDescription>
            </FormSectionHeader>
            <FormSectionContent className="divide-y divide-border">
              <FormField
                control={profileForm.control}
                name="city"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                    <div className="flex-1 space-y-0.5">
                      <FormLabel>{t("City")}</FormLabel>
                      <FormDescription>
                        {t("Optional.")}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Input
                        className="max-w-50"
                        placeholder={t("São Paulo")}
                        {...field}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
              <FormField
                control={profileForm.control}
                name="country"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                    <div className="flex-1 space-y-0.5">
                      <FormLabel>{t("Country")}</FormLabel>
                      <FormDescription>
                        {t("Optional.")}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Input
                        className="max-w-50"
                        placeholder={t("Brazil")}
                        {...field}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            </FormSectionContent>
          </FormSection>
        </SettingsSectionShell>
      </Form>

      <Form form={passwordForm}>
        <SettingsSectionShell>
          <FormSection>
            <FormSectionHeader>
              <FormSectionTitle>{t("Password")}</FormSectionTitle>
              <FormSectionDescription>
                {t("Update your account password.")}
              </FormSectionDescription>
            </FormSectionHeader>
            <FormSectionContent className="divide-y divide-border">
              <FormField
                control={passwordForm.control}
                name="currentPassword"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                    <div className="flex-1 space-y-0.5">
                      <FormLabel>{t("Current password")}</FormLabel>
                      <FormDescription>
                        {t("Your current password.")}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Input
                        className="max-w-50"
                        type="password"
                        placeholder="••••••••"
                        autoComplete="current-password"
                        {...field}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
              <FormField
                control={passwordForm.control}
                name="newPassword"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                    <div className="flex-1 space-y-0.5">
                      <FormLabel>{t("New password")}</FormLabel>
                      <FormDescription>
                        {t("At least 6 characters.")}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Input
                        className="max-w-50"
                        type="password"
                        placeholder="••••••••"
                        autoComplete="new-password"
                        {...field}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
              <FormField
                control={passwordForm.control}
                name="verifyPassword"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                    <div className="flex-1 space-y-0.5">
                      <FormLabel>{t("Confirm new password")}</FormLabel>
                      <FormDescription>
                        {t("Type it again to confirm.")}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Input
                        className="max-w-50"
                        type="password"
                        placeholder="••••••••"
                        autoComplete="new-password"
                        {...field}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            </FormSectionContent>
            <FormSectionFooter className="flex justify-end">
              <Button type="submit">{t("Update password")}</Button>
            </FormSectionFooter>
          </FormSection>
        </SettingsSectionShell>
      </Form>
    </div>
  );
}
