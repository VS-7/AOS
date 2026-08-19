import { useMemo } from "react";
import * as z from "zod";
import {
  ComputerIcon,
  Moon02Icon,
  Sun01Icon,
} from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";

import { aos } from "@/app/aos";
import { SettingsSectionShell } from "../../../section-shell";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import {
  FormControl,
  FormField,
  FormItem,
  Form,
} from "@/components/ui/form";
import { toast } from "sonner";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { ButtonGroup, ButtonGroupText } from "@/components/ui/button-group";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  FormSection,
  FormSectionHeader,
  FormSectionTitle,
  FormSectionDescription,
  FormSectionContent,
  FormSectionItem,
} from "@/components/ui/form-section";
import { ThemeRadiusSelector } from "@/components/ui/radius-selector";
import { Slider } from "@/components/ui/slider";
import { ColorPickerPopover } from "@/components/ui/color-picker";
import {
  FontFamilySelector,
  SYSTEM_FONT_VALUE,
} from "@/components/ui/font-family-selector";
import { ThemeSelector } from "@/components/ui/theme-selector";
import { cn } from "@/lib/utils";
import { SettingsFieldLabel } from "./components/settings-field-label";
import { getThemePreviewColors } from "@/components/ui/theme-preview-swatch";
import type { FractalTheme } from "@/features/theme/interfaces/theme.interfaces";

const SETTINGS_CONTROL_WIDTH = "h-8 w-48 px-2 shadow-none";
const SETTINGS_SELECT_TRIGGER = cn(
  SETTINGS_CONTROL_WIDTH,
  "rounded-md py-0",
);

const appearanceFormSchema = z.object({
  mode: z.enum(["light", "dark", "system"]),
  preset: z.string(),
  accent: z.string(),
  surface: z.string(),
  ink: z.string(),
  contrast: z.number().min(0).max(100),
  windows: z.enum(["solid", "blur"]),
  uiFont: z.string().optional(),
  codeFont: z.string().optional(),
  radius: z.enum(["none", "sm", "md", "lg"]),
  uiFontSize: z.number(),
  codeFontSize: z.number(),
  iconsSet: z.enum(["minimal", "standard", "complete", "none"]),
  iconsColored: z.boolean(),
});

type AppearanceFormValues = z.infer<typeof appearanceFormSchema>;

function buildAppearanceFormValues(values: AppearanceFormValues) {
  return {
    mode: values.mode,
    preset: values.preset,
    accent: values.accent,
    surface: values.surface,
    ink: values.ink,
    contrast: values.contrast,
    windows: values.windows,
    uiFont: values.uiFont,
    codeFont: values.codeFont,
    radius: values.radius,
    uiFontSize: values.uiFontSize,
    codeFontSize: values.codeFontSize,
    iconsSet: values.iconsSet,
    iconsColored: values.iconsColored,
  };
}

export function UserAppearanceSection() {
  const state = aos.stores.theme.useState();
  const themesQuery = aos.client.theme.list.useQuery();
  const themeList: FractalTheme[] = themesQuery.data?.themes ?? [];
  const mode = useMemo(
    () =>
      state.mode === "system"
        ? window.matchMedia("(prefers-color-scheme: dark)").matches
          ? "dark"
          : "light"
        : state.mode,
    [state.mode],
  );

  const form = aos.useForm({
    schema: appearanceFormSchema,
    mode: "onChange",
    values: (() => {
      const themeValues = aos.stores.theme.actions.get();
      return buildAppearanceFormValues({
        ...themeValues,
        iconsSet: themeValues.icons?.set ?? "complete",
        iconsColored: themeValues.icons?.colored ?? true,
      });
    })(),
    onSubmit: async (values) => {
      const next = await aos.stores.theme.actions.update({
        ...values,
        icons: {
          set: values.iconsSet,
          colored: values.iconsColored,
        },
      });

      return buildAppearanceFormValues({
        ...next,
        iconsSet: next.icons?.set ?? values.iconsSet,
        iconsColored: next.icons?.colored ?? values.iconsColored,
      });
    },
    onResponse: ({ error }) => {
      if (error) {
        toast.error(error.message || "Failed to update appearance");
        return;
      }

      toast.success("Appearance updated successfully!");
    },
  });

  const isNative = typeof window !== "undefined" && !!window.fractal;
  const currentValues = form.watch();
  const themePreviewById = useMemo(() => {
    const map = new Map<
      string,
      NonNullable<ReturnType<typeof getThemePreviewColors>>
    >();

    for (const theme of themeList) {
      const preview = getThemePreviewColors(theme, mode);
      if (preview) map.set(theme.id, preview);
    }

    return map;
  }, [mode, themeList]);

  const themeOptions = useMemo(
    () =>
      themeList.map((theme) => ({
        id: theme.id,
        name: theme.name,
        preview: themePreviewById.get(theme.id) ?? null,
      })),
    [themePreviewById, themeList],
  );

  return (
    <Form
      form={form}
      disableLoadingState
      className="flex h-full flex-1 flex-col overflow-y-auto"
    >
      <SettingsSectionShell>
        <FormSection>
          <FormSectionHeader>
            <FormSectionTitle>Theme</FormSectionTitle>
            <FormSectionDescription>
              Use light, dark, or device theme
            </FormSectionDescription>
          </FormSectionHeader>
          <FormSectionContent>
            <FormSectionItem className="border-y-0!">
              <SettingsFieldLabel
                label="Theme mode"
                description="Choose how Fractal adapts to your environment"
              />
              <FormField
                control={form.control}
                name="mode"
                render={({ field }) => (
                  <FormItem className="gap-0">
                    <FormControl>
                      <ToggleGroup
                        type="single"
                        className="overflow-hidden rounded-md border"
                        value={field.value}
                        onValueChange={field.onChange}
                      >
                        <ToggleGroupItem
                          value="light"
                          className="gap-1.5 px-3 shadow-none"
                        >
                          <HugeiconsIcon
                            icon={Sun01Icon}
                            className="size-3.5"
                          />
                          Light
                        </ToggleGroupItem>
                        <ToggleGroupItem
                          value="dark"
                          className="gap-1.5 px-3 shadow-none"
                        >
                          <HugeiconsIcon
                            icon={Moon02Icon}
                            className="size-3.5"
                          />
                          Dark
                        </ToggleGroupItem>
                        <ToggleGroupItem
                          value="system"
                          className="gap-1.5 px-3 shadow-none"
                        >
                          <HugeiconsIcon
                            icon={ComputerIcon}
                            className="size-3.5"
                          />
                          Device
                        </ToggleGroupItem>
                      </ToggleGroup>
                    </FormControl>
                  </FormItem>
                )}
              />
            </FormSectionItem>

            <div className="border-y-0! p-4">
              <div
                className="overflow-hidden rounded-md border border-border font-mono text-xs"
                style={{
                  backgroundColor: currentValues.surface,
                  color: currentValues.ink,
                }}
              >
                <div
                  className="flex-1 p-4"
                  style={{
                    backgroundColor:
                      currentValues.windows === "blur"
                        ? "transparent"
                        : "rgba(0,0,0,0.1)",
                  }}
                >
                  <div className="flex gap-4">
                    <span className="select-none opacity-50">1</span>
                    <span>
                      <span style={{ color: currentValues.accent }}>const</span>{" "}
                      themePreview: ThemeConfig = {"{"}
                    </span>
                  </div>
                  <div
                    className="mx-[-1rem] flex gap-4 border-l-2 bg-primary/10 px-4 py-0.5"
                    style={{ borderLeftColor: currentValues.accent }}
                  >
                    <span className="select-none opacity-50">2</span>
                    <span className="pl-6">
                      surface:{" "}
                      <span style={{ color: currentValues.accent }}>
                        "{currentValues.surface}"
                      </span>
                      ,
                    </span>
                  </div>
                  <div className="flex gap-4">
                    <span className="select-none opacity-50">3</span>
                    <span className="pl-6">
                      accent:{" "}
                      <span style={{ color: currentValues.accent }}>
                        "{currentValues.accent}"
                      </span>
                      ,
                    </span>
                  </div>
                  <div className="flex gap-4">
                    <span className="select-none opacity-50">4</span>
                    <span className="pl-6">
                      contrast:{" "}
                      <span style={{ color: currentValues.accent }}>
                        {currentValues.contrast}
                      </span>
                      ,
                    </span>
                  </div>
                  <div className="flex gap-4">
                    <span className="select-none opacity-50">5</span>
                    <span className="pl-6">
                      radius:{" "}
                      <span style={{ color: currentValues.accent }}>
                        "{currentValues.radius}"
                      </span>
                      ,
                    </span>
                  </div>
                  <div className="flex gap-4">
                    <span className="select-none opacity-50">6</span>
                    <span>{"};"}</span>
                  </div>
                </div>
              </div>
            </div>

            <FormSectionItem className="border-t-0!">
              <h4 className="text-sm font-medium capitalize">
                {mode} theme settings
              </h4>
            </FormSectionItem>

            <FormField
              control={form.control}
              name="preset"
              render={({ field }) => (
                <FormItem className="gap-0">
                  <FormSectionItem className="bg-background/50">
                    <SettingsFieldLabel
                      label="Theme"
                      description="Pick a color preset for the active mode"
                    />
                    <FormControl>
                      <ThemeSelector
                        value={field.value}
                        onValueChange={async (val) => {
                          field.onChange(val);
                          const next = await aos.stores.theme.actions.update({ preset: val });
                          if (next) {
                            form.reset(buildAppearanceFormValues({
                              ...next,
                              iconsSet: next.icons?.set ?? currentValues.iconsSet,
                              iconsColored: next.icons?.colored ?? currentValues.iconsColored,
                            }));
                          }
                        }}
                        themes={themeOptions}
                      />
                    </FormControl>
                  </FormSectionItem>
                </FormItem>
              )}
            />

            {[
              {
                name: "accent",
                label: "Accent",
                description: "Primary brand and interactive color",
              },
              {
                name: "surface",
                label: "Background",
                description: "Main surface color behind content",
              },
              {
                name: "ink",
                label: "Foreground",
                description: "Default text and icon color",
              },
            ].map((item) => (
              <FormField
                key={item.name}
                control={form.control}
                name={item.name as "accent" | "surface" | "ink"}
                render={({ field }) => (
                  <FormItem className="gap-0">
                    <FormSectionItem className="bg-background/50">
                      <SettingsFieldLabel
                        label={item.label}
                        description={item.description}
                      />
                      <FormControl>
                        <ColorPickerPopover
                          triggerClassName={cn(SETTINGS_CONTROL_WIDTH)}
                          onTriggerRemove={() => field.onChange(null)}
                          value={field.value}
                          onValueChange={(v) => field.onChange(v)}
                        />
                      </FormControl>
                    </FormSectionItem>
                  </FormItem>
                )}
              />
            ))}

            <FormField
              control={form.control}
              name="contrast"
              render={({ field }) => (
                <FormItem className="gap-0">
                  <FormSectionItem className="bg-background/50">
                    <SettingsFieldLabel
                      label="Contrast"
                      description="Adjust the contrast level of the interface"
                    />
                    <FormControl>
                      <div
                        className={cn(
                          "flex items-center gap-3",
                          SETTINGS_CONTROL_WIDTH,
                        )}
                      >
                        <Slider
                          min={0}
                          max={100}
                          step={1}
                          value={field.value}
                          onChange={(val) => {
                            const numVal = Array.isArray(val) ? val[0] : val;
                            field.onChange(numVal);
                            void aos.stores.theme.actions.update({ contrast: numVal });
                          }}
                        />
                        <span className="w-8 text-right text-xs tabular-nums text-muted-foreground">
                          {field.value}
                        </span>
                      </div>
                    </FormControl>
                  </FormSectionItem>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="uiFont"
              render={({ field }) => (
                <FormItem className="gap-0">
                  <FormSectionItem className="bg-background/50">
                    <SettingsFieldLabel
                      label="UI font"
                      description="Typeface used across the interface"
                    />
                    <FormControl>
                      <FontFamilySelector
                        className={SETTINGS_CONTROL_WIDTH}
                        value={field.value ?? SYSTEM_FONT_VALUE}
                        onValueChange={field.onChange}
                        systemLabel="System font"
                      />
                    </FormControl>
                  </FormSectionItem>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="codeFont"
              render={({ field }) => (
                <FormItem className="gap-0">
                  <FormSectionItem className="bg-background/50">
                    <SettingsFieldLabel
                      label="Code font"
                      description="Typeface used in editors and code blocks"
                    />
                    <FormControl>
                      <FontFamilySelector
                        className={SETTINGS_CONTROL_WIDTH}
                        value={field.value ?? SYSTEM_FONT_VALUE}
                        onValueChange={field.onChange}
                        systemLabel="Default monospace"
                        mono
                      />
                    </FormControl>
                  </FormSectionItem>
                </FormItem>
              )}
            />
            {isNative && (
              <FormField
                control={form.control}
                name="windows"
                render={({ field }) => (
                  <FormItem className="gap-0">
                    <FormSectionItem className="bg-background/50">
                      <SettingsFieldLabel
                        label="Translucent sidebar"
                        description="Apply a frosted glass effect to the sidebar"
                      />
                      <FormControl>
                        <Switch
                          checked={field.value === "blur"}
                          onCheckedChange={(checked) =>
                            field.onChange(checked ? "blur" : "solid")
                          }
                        />
                      </FormControl>
                    </FormSectionItem>
                  </FormItem>
                )}
              />
            )}
            <FormField
              control={form.control}
              name="radius"
              render={({ field }) => (
                <FormItem className="gap-0">
                  <FormSectionItem className="bg-background/50">
                    <SettingsFieldLabel
                      label="Radius"
                      description="Choose how rounded the interface should feel"
                    />
                    <FormControl>
                      <ThemeRadiusSelector
                        value={field.value}
                        onValueChange={field.onChange}
                      />
                    </FormControl>
                  </FormSectionItem>
                </FormItem>
              )}
            />
          </FormSectionContent>
        </FormSection>

        <FormSection>
          <FormSectionHeader>
            <FormSectionTitle>File icons</FormSectionTitle>
            <FormSectionDescription>
              Control the icon set used by the Files explorer and file tabs
            </FormSectionDescription>
          </FormSectionHeader>
          <FormSectionContent>
            <FormField
              control={form.control}
              name="iconsSet"
              render={({ field }) => (
                <FormItem className="gap-0">
                  <FormSectionItem>
                    <SettingsFieldLabel
                      label="Icon set"
                      description="Choose how detailed file icons should be"
                    />
                    <FormControl>
                      <Select value={field.value} onValueChange={field.onChange}>
                        <SelectTrigger
                          size="sm"
                          className={SETTINGS_SELECT_TRIGGER}
                        >
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="complete">Complete</SelectItem>
                          <SelectItem value="standard">Standard</SelectItem>
                          <SelectItem value="minimal">Minimal</SelectItem>
                          <SelectItem value="none">None</SelectItem>
                        </SelectContent>
                      </Select>
                    </FormControl>
                  </FormSectionItem>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="iconsColored"
              render={({ field }) => (
                <FormItem className="gap-0">
                  <FormSectionItem>
                    <SettingsFieldLabel
                      label="Colored icons"
                      description="Use semantic colors for file type icons"
                    />
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormSectionItem>
                </FormItem>
              )}
            />
          </FormSectionContent>
        </FormSection>

        <FormSection>
          <FormSectionHeader>
            <FormSectionTitle>Typography</FormSectionTitle>
            <FormSectionDescription>
              Adjust the font sizes used throughout the application
            </FormSectionDescription>
          </FormSectionHeader>
          <FormSectionContent>
            {[
              {
                name: "uiFontSize" as const,
                label: "UI font size",
                description: "Adjust the base size used for the UI",
              },
              {
                name: "codeFontSize" as const,
                label: "Code font size",
                description: "Adjust the base size used for code",
              },
            ].map((item) => (
              <FormField
                key={item.name}
                control={form.control}
                name={item.name}
                render={({ field }) => (
                  <FormItem className="gap-0">
                    <FormSectionItem>
                      <SettingsFieldLabel
                        label={item.label}
                        description={item.description}
                      />
                      <FormControl>
                        <ButtonGroup>
                          <Input
                            type="number"
                            className="h-8 w-16 text-center focus-visible:ring-0"
                            {...field}
                            onChange={(event) =>
                              field.onChange(Number(event.target.value))
                            }
                          />
                          <ButtonGroupText className="h-8 px-3 text-xs font-normal text-muted-foreground">
                            px
                          </ButtonGroupText>
                        </ButtonGroup>
                      </FormControl>
                    </FormSectionItem>
                  </FormItem>
                )}
              />
            ))}
          </FormSectionContent>
        </FormSection>
      </SettingsSectionShell>
    </Form>
  );
}
