import React from "react";
import { BookOpen, FolderOutput, Puzzle } from "lucide-react";
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { useTemplates } from "../../contexts/templates.context";
import { t } from "@/lib/i18n";

export function SelectedTemplateDetail() {
  const { selectedTemplate, selectedTemplateId, isCreateMode, form } =
    useTemplates();

  if (!selectedTemplateId && !isCreateMode) return null;

  return (
    <SplitPageLayout.DetailTabs defaultValue="overview">
      <SplitPageLayout.DetailTab value="overview" label={t("Overview")}>
        <SplitPageLayout.Widget>
          <SplitPageLayout.WidgetHeader>
            <SplitPageLayout.WidgetTitle>
              {t("Properties")}
            </SplitPageLayout.WidgetTitle>
          </SplitPageLayout.WidgetHeader>
          <SplitPageLayout.WidgetContent>
            {/* Editable only while creating: templates_update takes no skill,
                so an edit here used to be silently dropped by the save. */}
            {isCreateMode ? (
              <FormField
                control={form.control}
                name="skill"
                render={({ field }) => (
                  <FormItem className="w-full space-y-2">
                    <SplitPageLayout.WidgetItem>
                      <Puzzle className="size-3.5 shrink-0 text-muted-foreground" />
                      <span className="w-14 shrink-0 text-xs text-muted-foreground">
                        {t("Skill")}
                      </span>
                      <FormControl>
                        <Input
                          placeholder={t("global")}
                          aria-label={t("Skill")}
                          className="h-7 border-0 bg-transparent px-0 py-0 text-xs shadow-none focus-visible:ring-0"
                          {...field}
                          value={field.value ?? ""}
                        />
                      </FormControl>
                    </SplitPageLayout.WidgetItem>
                    <FormMessage />
                  </FormItem>
                )}
              />
            ) : (
              <SplitPageLayout.WidgetItem>
                <Puzzle className="size-3.5 shrink-0 text-muted-foreground" />
                <span className="w-14 shrink-0 text-xs text-muted-foreground">
                  {t("Skill")}
                </span>
                <span className="text-xs text-foreground">
                  {selectedTemplate?.skill || t("global")}
                </span>
              </SplitPageLayout.WidgetItem>
            )}

            <FormField
              control={form.control}
              name="output"
              render={({ field }) => (
                <FormItem className="w-full space-y-2">
                  <SplitPageLayout.WidgetItem>
                    <FolderOutput className="size-3.5 shrink-0 text-muted-foreground" />
                    <span className="w-14 shrink-0 text-xs text-muted-foreground">
                      {t("Output")}
                    </span>
                    <FormControl>
                      <Input
                        placeholder=".aos/artifacts/{{name}}.md"
                        className="h-7 border-0 bg-transparent px-0 py-0 text-xs shadow-none focus-visible:ring-0"
                        {...field}
                        value={field.value ?? ""}
                      />
                    </FormControl>
                  </SplitPageLayout.WidgetItem>
                  <FormMessage />
                </FormItem>
              )}
            />

            {!isCreateMode && selectedTemplate ? (
              <SplitPageLayout.WidgetItem>
                <BookOpen className="size-3.5 shrink-0 text-muted-foreground" />
                <span className="w-14 shrink-0 text-xs text-muted-foreground">
                  ID
                </span>
                <span className="font-mono text-xs text-muted-foreground">
                  {selectedTemplate.id}
                </span>
              </SplitPageLayout.WidgetItem>
            ) : null}
          </SplitPageLayout.WidgetContent>
        </SplitPageLayout.Widget>

        <SplitPageLayout.Widget>
          <SplitPageLayout.WidgetHeader>
            <SplitPageLayout.WidgetTitle>{t("Variables")}</SplitPageLayout.WidgetTitle>
          </SplitPageLayout.WidgetHeader>
          <SplitPageLayout.WidgetContent>
            <FormField
              control={form.control}
              name="variablesText"
              render={({ field }) => (
                // Padded like every other widget row: straight in the unpadded
                // widget body, the label sat on the card's border.
                <FormItem className="w-full space-y-2 p-3">
                  <FormLabel className="text-xs text-muted-foreground">
                    {t("The variables this template declares, as JSON")}
                  </FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={
                        '[\n  {\n    "name": "title",\n    "type": "string",\n    "description": "What it is called",\n    "required": true\n  }\n]'
                      }
                      className="min-h-40 resize-none font-mono text-xs"
                      {...field}
                      value={field.value ?? ""}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SplitPageLayout.WidgetContent>
        </SplitPageLayout.Widget>
      </SplitPageLayout.DetailTab>
    </SplitPageLayout.DetailTabs>
  );
}
