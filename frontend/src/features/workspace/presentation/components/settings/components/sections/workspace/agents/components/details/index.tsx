import React from "react";
import { Bot, BrainCircuit, Cpu, Layers, Shield } from "lucide-react";
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { useAgents } from "../../contexts/agents.context";
import { t } from "@/lib/i18n";

export function SelectedAgentDetail() {
  const { selectedAgent, selectedAgentId, isCreateMode, form } = useAgents();

  if (!selectedAgentId && !isCreateMode) return null;

  return (
    <SplitPageLayout.DetailTabs defaultValue="overview">
      <SplitPageLayout.DetailTab value="overview" label={t("Overview")}>
        <SplitPageLayout.Widget>
          <SplitPageLayout.WidgetHeader>
            <SplitPageLayout.WidgetTitle>
              {t("Configuration")}
            </SplitPageLayout.WidgetTitle>
          </SplitPageLayout.WidgetHeader>
          <SplitPageLayout.WidgetContent>
            <FormField
              control={form.control}
              name="provider"
              render={({ field }) => (
                <FormItem className="w-full space-y-2">
                  <SplitPageLayout.WidgetItem>
                    <Cpu className="size-3.5 shrink-0 text-muted-foreground" />
                    <span className="w-16 shrink-0 text-xs text-muted-foreground">
                      {t("Provider")}
                    </span>
                    <FormControl>
                      <Input
                        placeholder="openai"
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

            <FormField
              control={form.control}
              name="model"
              render={({ field }) => (
                <FormItem className="w-full space-y-2">
                  <SplitPageLayout.WidgetItem>
                    <Layers className="size-3.5 shrink-0 text-muted-foreground" />
                    <span className="w-16 shrink-0 text-xs text-muted-foreground">
                      {t("Model")}
                    </span>
                    <FormControl>
                      <Input
                        placeholder="gpt-5"
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

            <FormField
              control={form.control}
              name="skill"
              render={({ field }) => (
                <FormItem className="w-full space-y-2">
                  <SplitPageLayout.WidgetItem>
                    <BrainCircuit className="size-3.5 shrink-0 text-muted-foreground" />
                    <span className="w-16 shrink-0 text-xs text-muted-foreground">
                      {t("Skill")}
                    </span>
                    <FormControl>
                      <Input
                        placeholder="general"
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

            <FormField
              control={form.control}
              name="orchestrator"
              render={({ field }) => (
                <FormItem className="border-0 p-0">
                  <SplitPageLayout.WidgetItem className="justify-between">
                    <div className="flex items-center gap-2">
                      <Shield className="size-3.5 shrink-0 text-muted-foreground" />
                      <span className="text-xs text-muted-foreground">
                        {t("Orchestrator")}
                      </span>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </SplitPageLayout.WidgetItem>
                  <FormMessage />
                </FormItem>
              )}
            />

            {!isCreateMode && selectedAgent ? (
              <SplitPageLayout.WidgetItem>
                <Bot className="size-3.5 shrink-0 text-muted-foreground" />
                <span className="w-16 shrink-0 text-xs text-muted-foreground">
                  ID
                </span>
                <span className="font-mono text-xs text-muted-foreground">
                  {selectedAgent.id}
                </span>
              </SplitPageLayout.WidgetItem>
            ) : null}
          </SplitPageLayout.WidgetContent>
        </SplitPageLayout.Widget>
      </SplitPageLayout.DetailTab>
    </SplitPageLayout.DetailTabs>
  );
}
