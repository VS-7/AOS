import React from "react";
import { Bot, BrainCircuit, Crown, Gauge, Layers, Lock, Package, Shield } from "lucide-react";
import { useWatch } from "react-hook-form";
import {
  FormControl,
  FormField,
  FormItem,
  FormMessage,
} from "@/components/ui/form";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { AgentModelSelect, type AgentModelSelectProvider } from "@/components/ui/agent-model-select";
import type { Agent } from "@/features/agent/interfaces/agent.interfaces";
import { useModelProviders } from "@/features/model/services/model-provider.service";
import { useAgents } from "../../contexts/agents.context";
import { AGENT_REASONING_LEVELS } from "../../contexts/agent-form";
import { t } from "@/lib/i18n";

// Radix Select has no empty value; this stands for "not set on the agent".
const UNSET = "__unset__";

function reasoningLabel(level: string): string {
  switch (level) {
    case "none":
      return t("None");
    case "low":
      return t("Low");
    case "medium":
      return t("Medium");
    case "high":
      return t("High");
    default:
      return level;
  }
}

function permissionLabel(permission: string): string {
  switch (permission) {
    case "read":
      return t("Read");
    case "write":
      return t("Write");
    case "delete":
      return t("Delete");
    case "execute":
      return t("Execute");
    default:
      return permission;
  }
}

export function SelectedAgentDetail() {
  const { selectedAgent, selectedAgentId, isCreateMode, form, agents } = useAgents();
  const providers = useModelProviders();

  const provider = useWatch({ control: form.control, name: "provider" }) as string | undefined;
  const model = useWatch({ control: form.control, name: "model" }) as string | undefined;

  if (!selectedAgentId && !isCreateMode) return null;

  // Only connected providers can answer, so only those are offered. An
  // agent already saved with another one still shows it, with the reason it
  // will not work — a free-text box accepted any string, connected or not.
  const selectable: AgentModelSelectProvider[] = providers
    .filter((p) => p.configured)
    .map((p) => ({ id: p.id, name: p.name, configured: true, models: p.models }));
  const known = providers.find((p) => p.id === provider);
  const notConnected = !!provider && !known?.configured;
  // Only a provider that authenticates with a key this installation holds
  // is unusable without being connected — the daemon refuses those turns
  // (AOS_AGENT_PROVIDER_NOT_CONNECTED). A login-file provider still reads the
  // other tool's login, so for those this is a note, not a warning.
  const needsKey = known?.auth.mode === "api-key" && known.auth.required;

  const setModel = (next: { provider: string; model: string }) => {
    form.setValue("provider", next.provider, { shouldDirty: true });
    form.setValue("model", next.model, { shouldDirty: true });
  };

  const leaders = agents.filter((agent) => agent.id !== selectedAgent?.id);

  return (
    <SplitPageLayout.DetailTabs defaultValue="overview">
      <SplitPageLayout.DetailTab value="overview" label={t("Overview")}>
        <SplitPageLayout.Widget>
          <SplitPageLayout.WidgetHeader>
            <SplitPageLayout.WidgetTitle>{t("Configuration")}</SplitPageLayout.WidgetTitle>
          </SplitPageLayout.WidgetHeader>
          <SplitPageLayout.WidgetContent>
            <div className="flex flex-col gap-1.5 p-3">
              <div className="flex items-center gap-2">
                <Layers className="size-3.5 shrink-0 text-muted-foreground" />
                <span className="w-16 shrink-0 text-xs text-muted-foreground">{t("Model")}</span>
                <AgentModelSelect
                  providers={selectable}
                  value={{ provider: provider ?? "", model: model ?? "" }}
                  onChange={setModel}
                  showReasoning={false}
                  placeholder={t("Workspace default")}
                  onClear={() => setModel({ provider: "", model: "" })}
                  clearLabel={t("Use the workspace default")}
                  className="min-w-0"
                />
              </div>
              {notConnected && !needsKey ? (
                <p className="text-xs text-muted-foreground">
                  {t("{{provider}} is not connected in AI providers; this agent relies on the login already on this machine.", {
                    provider: known?.name ?? provider,
                  })}
                </p>
              ) : notConnected ? (
                <p className="text-xs text-destructive" role="alert">
                  {t("{{provider}} is not connected, so this agent cannot answer until it is. Connect it in AI providers, or choose another model.", {
                    provider: known?.name ?? provider,
                  })}
                </p>
              ) : !provider && model ? (
                <p className="text-xs text-muted-foreground">
                  {t("{{model}} on the workspace default provider.", { model })}
                </p>
              ) : null}
            </div>

            <FormField
              control={form.control}
              name="reasoning"
              render={({ field }) => (
                <FormItem className="w-full space-y-0">
                  <SplitPageLayout.WidgetItem>
                    <Gauge className="size-3.5 shrink-0 text-muted-foreground" />
                    <span className="w-16 shrink-0 text-xs text-muted-foreground">{t("Reasoning")}</span>
                    <Select
                      value={field.value || UNSET}
                      onValueChange={(next) => field.onChange(next === UNSET ? "" : next)}
                    >
                      <FormControl>
                        <SelectTrigger size="sm" className="h-7 min-w-0 text-xs" aria-label={t("Reasoning")}>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value={UNSET}>{t("Workspace default")}</SelectItem>
                        {AGENT_REASONING_LEVELS.map((level) => (
                          <SelectItem key={level} value={level}>
                            {reasoningLabel(level)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </SplitPageLayout.WidgetItem>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="leader"
              render={({ field }) => (
                <FormItem className="w-full space-y-0">
                  <SplitPageLayout.WidgetItem>
                    <Crown className="size-3.5 shrink-0 text-muted-foreground" />
                    <span className="w-16 shrink-0 text-xs text-muted-foreground">{t("Leader")}</span>
                    <Select
                      value={field.value || UNSET}
                      onValueChange={(next) => field.onChange(next === UNSET ? "" : next)}
                    >
                      <FormControl>
                        <SelectTrigger size="sm" className="h-7 min-w-0 text-xs" aria-label={t("Leader")}>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value={UNSET}>{t("No leader")}</SelectItem>
                        {leaders.map((agent) => (
                          <SelectItem key={agent.id} value={agent.id}>
                            {agent.name || agent.id}
                          </SelectItem>
                        ))}
                        {field.value && !leaders.some((agent) => agent.id === field.value) ? (
                          // A leader that is not (or no longer) in the roster
                          // is allowed by the daemon; show it rather than a
                          // blank trigger.
                          <SelectItem value={field.value}>{field.value}</SelectItem>
                        ) : null}
                      </SelectContent>
                    </Select>
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
                      <span className="text-xs text-muted-foreground">{t("Orchestrator")}</span>
                    </div>
                    <FormControl>
                      <Switch checked={field.value} onCheckedChange={field.onChange} />
                    </FormControl>
                  </SplitPageLayout.WidgetItem>
                  <FormMessage />
                </FormItem>
              )}
            />

            {!isCreateMode && selectedAgent ? (
              <>
                {selectedAgent.skill ? (
                  // Where the agent file lives, not a setting: the daemon
                  // derives it from the path and neither input accepts it.
                  <SplitPageLayout.WidgetItem>
                    <Package className="size-3.5 shrink-0 text-muted-foreground" />
                    <span className="w-16 shrink-0 text-xs text-muted-foreground">{t("Skill")}</span>
                    <span className="font-mono text-xs text-muted-foreground">{selectedAgent.skill}</span>
                  </SplitPageLayout.WidgetItem>
                ) : null}
                <SplitPageLayout.WidgetItem>
                  <Bot className="size-3.5 shrink-0 text-muted-foreground" />
                  <span className="w-16 shrink-0 text-xs text-muted-foreground">{t("ID")}</span>
                  <span className="font-mono text-xs text-muted-foreground">{selectedAgent.id}</span>
                </SplitPageLayout.WidgetItem>
              </>
            ) : null}
          </SplitPageLayout.WidgetContent>
        </SplitPageLayout.Widget>

        {!isCreateMode && selectedAgent ? <SandboxWidget agent={selectedAgent} /> : null}
      </SplitPageLayout.DetailTab>
    </SplitPageLayout.DetailTabs>
  );
}

/**
 * What the agent may reach, read-only.
 *
 * The policy is a decision written into AGENT.md (ADR-0006) and it was
 * nowhere on screen: a person could not see that an agent may run `git` but
 * not `git push --force` without opening the file.
 */
function SandboxWidget({ agent }: { agent: Agent }) {
  const sandbox = agent.sandbox;
  const permissions = sandbox?.permissions ?? [];
  const exec = sandbox?.exec;

  return (
    <SplitPageLayout.Widget>
      <SplitPageLayout.WidgetHeader>
        <SplitPageLayout.WidgetTitle>{t("Sandbox")}</SplitPageLayout.WidgetTitle>
      </SplitPageLayout.WidgetHeader>
      <SplitPageLayout.WidgetContent>
        {!sandbox ? (
          <SplitPageLayout.WidgetItem>
            <Lock className="size-3.5 shrink-0 text-muted-foreground" />
            <span className="text-xs text-muted-foreground">
              {t("No sandbox declared: this agent can read, and nothing else.")}
            </span>
          </SplitPageLayout.WidgetItem>
        ) : (
          <>
            <SplitPageLayout.WidgetItem className="items-start">
              <Lock className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
              <span className="w-16 shrink-0 text-xs text-muted-foreground">{t("Files")}</span>
              <span className="text-xs">
                {permissions.length > 0 ? permissions.map(permissionLabel).join(", ") : t("Read")}
              </span>
            </SplitPageLayout.WidgetItem>
            <SplitPageLayout.WidgetItem className="items-start">
              <BrainCircuit className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
              <span className="w-16 shrink-0 text-xs text-muted-foreground">{t("Programs")}</span>
              <div className="flex min-w-0 flex-col gap-1 text-xs">
                {exec?.policy === "allowlist" && (exec.allow?.length ?? 0) > 0 ? (
                  <span className="wrap-anywhere font-mono">{exec.allow!.join(", ")}</span>
                ) : (
                  <span className="text-muted-foreground">{t("None")}</span>
                )}
                {(exec?.denyArgs?.length ?? 0) > 0 ? (
                  <span className="wrap-anywhere text-muted-foreground">
                    {t("Refused: {{patterns}}", { patterns: exec!.denyArgs!.join(", ") })}
                  </span>
                ) : null}
                <span className="text-muted-foreground">
                  {exec?.allowShell ? t("Shell allowed") : t("No shell")}
                </span>
              </div>
            </SplitPageLayout.WidgetItem>
          </>
        )}
      </SplitPageLayout.WidgetContent>
    </SplitPageLayout.Widget>
  );
}
