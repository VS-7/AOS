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
import { FractalAppError } from "@/core/errors/fractal.error";
import { api } from "@/lib/aos-facade";

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
  
  // `aos.useContext()` is Fractal's global route context (`withContext(...)`),
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
        if (error instanceof FractalAppError) {
          toast.error(error.message);
          return;
        }

        toast.error(
          error instanceof Error ? error.message : "Failed to update profile",
        );
        return;
      }

      toast.success("Profile updated successfully!");
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

      const passwordResult = await aos.stores.auth.actions.updatePassword({
        currentPassword: values.currentPassword!,
        newPassword: values.newPassword!,
        verifyPassword: values.verifyPassword!,
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
        if (error instanceof FractalAppError) {
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

      toast.success("Password updated successfully!");
      router.invalidate();
    },
  });

  return (
    <div className="flex h-full flex-1 flex-col overflow-y-auto">
      <Form form={profileForm}>
        <SettingsSectionShell>
          <FormSection>
            <FormSectionHeader>
              <FormSectionTitle>Basic Info</FormSectionTitle>
              <FormSectionDescription>
                Your name and how you appear in Fractal.
              </FormSectionDescription>
            </FormSectionHeader>
            <FormSectionContent className="divide-y divide-border">
              <FormField
                control={profileForm.control}
                name="image"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                    <div className="flex-1 space-y-0.5">
                      <FormLabel>Avatar</FormLabel>
                      <FormDescription>
                        Your profile photo.
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
                      <FormLabel>Name</FormLabel>
                      <FormDescription>
                        How your name is shown in the app.
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Input
                        className="max-w-50"
                        placeholder="Your name"
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
                      <FormLabel>Email</FormLabel>
                      <FormDescription>
                        Email used to sign in.
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Input
                        className="max-w-50"
                        type="email"
                        placeholder="you@example.com"
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
              <FormSectionTitle>Region</FormSectionTitle>
              <FormSectionDescription>
                Timezone and language preferences.
              </FormSectionDescription>
            </FormSectionHeader>
            <FormSectionContent className="divide-y divide-border">
              <FormField
                control={profileForm.control}
                name="timezone"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                    <div className="flex-1 space-y-0.5">
                      <FormLabel>Timezone</FormLabel>
                      <FormDescription>
                        Your local timezone.
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Input
                        className="max-w-50"
                        placeholder="America/Sao_Paulo"
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
                      <FormLabel>Language</FormLabel>
                      <FormDescription>
                        Preferred language, e.g. en-US or pt-BR.
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Input
                        className="max-w-50"
                        placeholder="en-US"
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
              <FormSectionTitle>Location</FormSectionTitle>
              <FormSectionDescription>
                Where you are based.
              </FormSectionDescription>
            </FormSectionHeader>
            <FormSectionContent className="divide-y divide-border">
              <FormField
                control={profileForm.control}
                name="city"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                    <div className="flex-1 space-y-0.5">
                      <FormLabel>City</FormLabel>
                      <FormDescription>
                        Optional.
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Input
                        className="max-w-50"
                        placeholder="São Paulo"
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
                      <FormLabel>Country</FormLabel>
                      <FormDescription>
                        Optional.
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Input
                        className="max-w-50"
                        placeholder="Brazil"
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
              <FormSectionTitle>Password</FormSectionTitle>
              <FormSectionDescription>
                Update your account password.
              </FormSectionDescription>
            </FormSectionHeader>
            <FormSectionContent className="divide-y divide-border">
              <FormField
                control={passwordForm.control}
                name="currentPassword"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between gap-4 p-4">
                    <div className="flex-1 space-y-0.5">
                      <FormLabel>Current password</FormLabel>
                      <FormDescription>
                        Your current password.
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
                      <FormLabel>New password</FormLabel>
                      <FormDescription>
                        At least 6 characters.
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
                      <FormLabel>Confirm new password</FormLabel>
                      <FormDescription>
                        Type it again to confirm.
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
              <Button type="submit">Update password</Button>
            </FormSectionFooter>
          </FormSection>
        </SettingsSectionShell>
      </Form>
    </div>
  );
}
