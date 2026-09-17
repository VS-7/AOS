import { useEffect, useState } from "react";
import {
  BrainCircuit,
  CalendarClock,
  ListChecks,
  RadioTower,
  Settings2,
} from "lucide-react";
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { ImageUpload } from "@/components/ui/image-upload";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  TabsSubtle,
  TabsSubtleItem,
  TabsSubtlePanel,
} from "@/components/ui/tabs-subtle";
import type { Agent } from "@/features/agent/interfaces/agent.interfaces";
import { AgentMemoriesTab } from "./memories";
import { AgentTasksTab } from "./tasks";
import { AgentChannelsTab } from "./channels";
import { AgentRoutinesTab } from "./routines";
import { t } from "@/lib/i18n";
import { agentSlug } from "../../../../contexts/agent-form";
import { useAgents } from "../../../../contexts/agents.context";

/**
 * The id a new agent will get, shown while its name is typed.
 *
 * The id is made from the name and is the agent's identity from then on —
 * its directory, every reference to it. A form that never showed it could
 * only find out it was unusable, or taken, from the daemon.
 */
function NewAgentId({ name }: { name: string }) {
  const { agents } = useAgents();
  if (!name.trim()) return null;
  const id = agentSlug(name);
  if (!id) {
    return (
      <p className="text-xs text-destructive">
        {t("Use a name with letters or digits: the agent's id is made from it.")}
      </p>
    );
  }
  const taken = agents.find((agent) => agent.id === id);
  return (
    <p className={taken ? "text-xs text-destructive" : "text-xs text-muted-foreground"}>
      {taken
        ? t("{{name}} already uses the id {{id}}. Choose another name.", { name: taken.name || taken.id, id })
        : t("Id: {{id}}", { id })}
    </p>
  );
}

interface AgentContentTabsProps {
  agent?: Agent;
  form: any;
  isCreateMode: boolean;
  isLoadingInstructions: boolean;
  /** The instructions could not be read: there is nothing to edit. */
  instructionsFailed?: boolean;
  onRetryInstructions?: () => void;
}

export function AgentContentTabs({
  agent,
  form,
  isCreateMode,
  isLoadingInstructions,
  instructionsFailed = false,
  onRetryInstructions,
}: AgentContentTabsProps) {
  const [selectedIndex, setSelectedIndex] = useState(0);
  const idPrefix = `agent-content-${agent?.id ?? "new"}`;

  useEffect(() => {
    setSelectedIndex(0);
  }, [agent?.id, isCreateMode]);

  return (
    <div className="grid h-full grid-rows-[auto_1fr] overflow-hidden">
      <div className="px-4 pt-2">
        <TabsSubtle
          selectedIndex={selectedIndex}
          onSelect={setSelectedIndex}
          idPrefix={idPrefix}
          activeLabel
        >
          <TabsSubtleItem index={0} icon={Settings2} label={t("Overview")} />
          {!isCreateMode && agent ? (
            <>
              <TabsSubtleItem index={1} icon={BrainCircuit} label={t("Memories")} />
              <TabsSubtleItem index={2} icon={ListChecks} label={t("Tasks")} />
              <TabsSubtleItem index={3} icon={CalendarClock} label={t("Routines")} />
              <TabsSubtleItem index={4} icon={RadioTower} label={t("Channels")} />
            </>
          ) : null}
        </TabsSubtle>
      </div>

      <div className="overflow-auto">
        <TabsSubtlePanel
          index={0}
          selectedIndex={selectedIndex}
          idPrefix={idPrefix}
          className="h-full"
        >
          <div className="container mx-auto max-w-3xl space-y-6 px-6 py-6 pb-10">
            <FormField
              control={form.control}
              name="image"
              render={({ field }) => (
                <FormItem className="flex flex-row items-center justify-between gap-4">
                  <div className="space-y-0.5">
                    <FormLabel className="opacity-60">{t("Avatar")}</FormLabel>
                    <FormDescription>
                      {t("Photo shown wherever this agent appears.")}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <ImageUpload
                      value={field.value}
                      fallback={form.watch("name") || "A"}
                      onChange={field.onChange}
                      onRemove={() => field.onChange("")}
                    />
                  </FormControl>
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel className="opacity-60">{t("Name")}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t("Atlas")}
                      className="h-auto rounded-none border-0 bg-transparent px-0 py-0 text-2xl font-semibold shadow-none focus-visible:ring-0"
                      {...field}
                    />
                  </FormControl>
                  {isCreateMode ? <NewAgentId name={field.value ?? ""} /> : null}
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="description"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel className="opacity-60">{t("Description")}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={t("Describe when this agent should be used.")}
                      className="min-h-10 max-h-48 resize-none rounded-none border-0 bg-transparent px-0 py-0 text-sm shadow-none focus-visible:ring-0"
                      {...field}
                      value={field.value ?? ""}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="role"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel className="opacity-60">{t("Role")}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t("Frontend Systems Specialist")}
                      className="rounded-none border-0 bg-transparent px-0 py-0 text-sm shadow-none focus-visible:ring-0"
                      {...field}
                      value={field.value ?? ""}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="content"
              render={({ field }) => (
                <FormItem>
                  <FormLabel className="opacity-60">{t("Instructions")}</FormLabel>
                  <FormControl>
                    {/* Plain text, not MarkdownEditor: these are the words the
                        model reads. The rich editor re-serialised whatever it
                        loaded — `<rules>` saved as `\<rules>`, `-` bullets as
                        `*`, HTML comments dropped, trailing spaces as
                        `&#x20;` — so a one-character edit rewrote the whole
                        AGENT.md and changed the instructions themselves. */}
                    <Textarea
                      {...field}
                      value={field.value ?? ""}
                      spellCheck={false}
                      // Until the agent's own instructions arrive the field
                      // holds nothing; typing into it then would save that
                      // nothing over AGENT.md. The same holds when they could
                      // not be read at all.
                      disabled={isLoadingInstructions || instructionsFailed}
                      placeholder={
                        isLoadingInstructions
                          ? t("Loading instructions…")
                          : instructionsFailed
                            ? t("The instructions could not be loaded.")
                            : t("Write the system instructions for this agent...")
                      }
                      className="min-h-64 resize-y font-mono text-sm leading-relaxed"
                    />
                  </FormControl>
                  {instructionsFailed && onRetryInstructions ? (
                    <Button type="button" variant="secondary" size="sm" className="w-fit" onClick={onRetryInstructions}>
                      {t("Try again")}
                    </Button>
                  ) : null}
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </TabsSubtlePanel>

        {!isCreateMode && agent ? (
          <>
            <TabsSubtlePanel
              index={1}
              selectedIndex={selectedIndex}
              idPrefix={idPrefix}
              className="h-full"
            >
              <AgentMemoriesTab agent={agent} />
            </TabsSubtlePanel>
            <TabsSubtlePanel
              index={2}
              selectedIndex={selectedIndex}
              idPrefix={idPrefix}
              className="h-full"
            >
              <AgentTasksTab agent={agent} />
            </TabsSubtlePanel>
            <TabsSubtlePanel
              index={3}
              selectedIndex={selectedIndex}
              idPrefix={idPrefix}
              className="h-full"
            >
              <AgentRoutinesTab agent={agent} />
            </TabsSubtlePanel>
            <TabsSubtlePanel
              index={4}
              selectedIndex={selectedIndex}
              idPrefix={idPrefix}
              className="h-full"
            >
              <AgentChannelsTab key={agent.id} agent={agent} />
            </TabsSubtlePanel>
          </>
        ) : null}
      </div>
    </div>
  );
}
