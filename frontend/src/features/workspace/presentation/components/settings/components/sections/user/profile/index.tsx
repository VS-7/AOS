import { useMemo, useState } from "react";
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
  FormMessage,
} from "@/components/ui/form";
import { ImageUpload } from "@/components/ui/image-upload";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { AppError } from "@/core/errors/aos.error";
import type { Config } from "@/features/config/interfaces/config.interfaces";
import { errorMessage } from "@/lib/aos-facade";
import { t, LOCALES, LOCALE_NAMES, LOCALE_TAGS, useTranslation, type Locale } from "@/lib/i18n";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { changedSettings } from "../../../../helpers/changed-settings";
import { saveConfigSettings } from "../../../../helpers/save-config-settings";

/**
 * Built when the section renders rather than when the module loads: the
 * messages go through `t()`, which answers in the locale in force at the
 * moment it runs.
 */
function buildProfileFormSchema() {
  return z.object({
    // Judged trimmed — "   " passed `min(2)` and went to the daemon, which
    // refuses a name that is only spaces — but not *transformed*: the form
    // resets itself with what a save returns, and a trimmed value took the
    // space out from under somebody still typing "Vitor Sergio".
    name: z.string().refine((value) => value.trim().length >= 2, t("Name must be at least 2 characters.")),
    email: z.string().email(t("Enter a valid email address.")),
    image: z.string().optional().or(z.literal("")),
    timezone: z.string(),
    city: z.string().optional(),
    country: z.string().optional(),
  });
}

function buildPasswordFormSchema() {
  return z
    .object({
      currentPassword: z.string().optional().or(z.literal("")),
      newPassword: z.string().optional().or(z.literal("")),
      verifyPassword: z.string().optional().or(z.literal("")),
    })
    // Always checked. This form has its own "Update password" button and
    // nothing else to save, so an empty submission is a mistake to point at,
    // not a no-op to congratulate with "Password updated successfully!".
    .superRefine((data, ctx) => {
      if (!data.currentPassword) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: t("Current password is required"),
          path: ["currentPassword"],
        });
      }

      // Twelve, as the daemon enforces (auth.MinPasswordLen). Six here let a
      // password through the form that the daemon then refused.
      if (!data.newPassword || data.newPassword.length < 12) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: t("New password must be at least 12 characters"),
          path: ["newPassword"],
        });
      }

      if (data.newPassword !== data.verifyPassword) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: t("Passwords do not match"),
          path: ["verifyPassword"],
        });
      }
    });
}

/** The region paths this page edits, with the values it shows for them. */
function regionSettings(config: Config | undefined) {
  return {
    "region.timezone": config?.region?.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone,
    "region.city": config?.region?.city || "",
    "region.country": config?.region?.country || "",
  };
}

/**
 * Account identity, region, and password — AuthStore + config region.
 */
export function UserProfileSection() {
  // The stores, not the route context: the context is a copy taken when the
  // route loaded, and a save never reached it — a city saved here came back
  // as the old one after visiting another section, and the next edit sent
  // the old one again.
  const authUser = aos.stores.auth.useState((state) => state.user);
  const config = aos.stores.config.useState();
  const { locale } = useTranslation();
  const [switchingLanguage, setSwitchingLanguage] = useState(false);

  const profileFormSchema = useMemo(buildProfileFormSchema, []);
  const passwordFormSchema = useMemo(buildPasswordFormSchema, []);
  const region = regionSettings(config);

  const profileForm = aos.useForm({
    schema: profileFormSchema,
    mode: "onChange",
    values: {
      name: authUser?.name || "",
      email: authUser?.email || "",
      image: authUser?.image || "",
      timezone: region["region.timezone"],
      city: region["region.city"],
      country: region["region.country"],
    },
    // Each half is sent only when something in it changed: the account and
    // the configuration are two writes, and a city edit has no reason to
    // rewrite the account (or a name edit the region).
    onSubmit: async (values) => {
      const account = aos.stores.auth.state.user;
      const name = values.name.trim();
      const image = values.image || "";
      let saved = false;
      if (name !== (account?.name ?? "") || values.email !== (account?.email ?? "") || image !== (account?.image ?? "")) {
        const profileResult = await aos.stores.auth.actions.updateProfile({ name, email: values.email, image });
        if (profileResult.error) throw profileResult.error;
        saved = true;
      }

      const nextRegion = {
        "region.timezone": values.timezone,
        "region.city": values.city ?? "",
        "region.country": values.country ?? "",
      };
      if (await saveConfigSettings(changedSettings("", nextRegion, regionSettings(aos.stores.config.state)))) {
        saved = true;
      }

      if (saved) toast.success(t("Profile updated successfully!"));
      return values;
    },
    onResponse: ({ error }) => {
      if (!error) return;
      if (error instanceof AppError) {
        toast.error(error.message);
        return;
      }
      toast.error(error instanceof Error ? error.message : t("Failed to update profile"));
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

        toast.error(error instanceof Error ? error.message : t("Failed to update password"));
        return;
      }

      toast.success(t("Password updated successfully!"));
    },
  });

  /**
   * Saves the language to the configuration, which then puts the interface
   * in it — `region.language` is where the language lives (see
   * `applyConfiguredLocale`).
   *
   * It used to switch the interface first and leave the save to the form's
   * autosave, which the switch cancelled: changing language remounts the
   * whole tree, taking the pending save with it. The language was never
   * saved, the select went back to the old one, and picking that one again
   * did nothing because the select already showed it — there was no way back.
   * A pending edit elsewhere on the page is saved first for the same reason.
   */
  const chooseLanguage = async (next: Locale) => {
    if (next === locale || switchingLanguage) return;
    setSwitchingLanguage(true);
    try {
      // Unconditionally: the save sends only what differs from what is
      // stored, so with nothing pending it sends nothing.
      await profileForm.submit();
      await saveConfigSettings({ "region.language": LOCALE_TAGS[next] });
    } catch (error) {
      toast.error(t("The language could not be saved"), { description: errorMessage(error) });
      setSwitchingLanguage(false);
    }
  };

  return (
    <div className="flex h-full flex-1 flex-col overflow-y-auto">
      {/* `disableLoadingState`: this form saves itself while the person is
          still typing, and disabling its fields for each save took the
          focus out of the one being typed in — the keystrokes after it went
          nowhere. */}
      <Form form={profileForm} disableLoadingState>
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
                      {/* A thumbnail: the daemon keeps it with the account and
                          sends it with every session read, so it takes no
                          more than an avatar needs (see auth.MaxImageBytes). */}
                      <ImageUpload
                        value={field.value}
                        fallback={authUser?.name || "U"}
                        onChange={field.onChange}
                        onRemove={() => field.onChange("")}
                        maxEdgePx={256}
                        maxBytes={150_000}
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
                      <FormMessage />
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
                      <FormMessage />
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
              <div className="flex flex-row items-center justify-between gap-4 p-4">
                <div className="flex-1 space-y-0.5">
                  <Label htmlFor="profile-language">{t("Language")}</Label>
                  <p className="text-sm text-muted-foreground">
                    {t("The language the interface is shown in.")}
                  </p>
                </div>
                {/*
                  * A select, not the free-text box this used to be: the field
                  * decides which of two catalogues the interface renders from,
                  * so a typo here used to mean silently getting English back
                  * with no way to tell why. It shows the language on screen,
                  * and applies on change — a language you cannot see until you
                  * submit is a language you cannot check.
                  */}
                <Select
                  value={locale}
                  disabled={switchingLanguage}
                  onValueChange={(next) => void chooseLanguage(next as Locale)}
                >
                  <SelectTrigger id="profile-language" className="max-w-50">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {LOCALES.map((option) => (
                      <SelectItem key={option} value={option}>
                        {LOCALE_NAMES[option]}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
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
                      <FormMessage />
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
                        {t("At least 12 characters.")}
                      </FormDescription>
                      <FormMessage />
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
                      <FormMessage />
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
