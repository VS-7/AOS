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
import { Input } from "@/components/ui/input";
import { MarkdownEditor } from "@/components/ui/markdown-editor";
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

interface AgentContentTabsProps {
  agent?: Agent;
  form: any;
  isCreateMode: boolean;
  isLoadingInstructions: boolean;
}

export function AgentContentTabs({
  agent,
  form,
  isCreateMode,
  isLoadingInstructions,
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
          <TabsSubtleItem index={0} icon={Settings2} label="Overview" />
          {!isCreateMode && agent ? (
            <>
              <TabsSubtleItem index={1} icon={BrainCircuit} label="Memories" />
              <TabsSubtleItem index={2} icon={ListChecks} label="Tasks" />
              <TabsSubtleItem index={3} icon={CalendarClock} label="Routines" />
              <TabsSubtleItem index={4} icon={RadioTower} label="Channels" />
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
                    <FormLabel className="opacity-60">Avatar</FormLabel>
                    <FormDescription>
                      Photo shown wherever this agent appears.
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
                  <FormLabel className="opacity-60">Name</FormLabel>
                  <FormControl>
                    <Input
                      placeholder="Atlas"
                      className="h-auto rounded-none border-0 bg-transparent px-0 py-0 text-2xl font-semibold shadow-none focus-visible:ring-0"
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="description"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel className="opacity-60">Description</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder="Describe when this agent should be used."
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
                  <FormLabel className="opacity-60">Role</FormLabel>
                  <FormControl>
                    <Input
                      placeholder="Frontend Systems Specialist"
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
                  <FormLabel className="opacity-60">Instructions</FormLabel>
                  <FormControl>
                    <MarkdownEditor
                      value={field.value ?? ""}
                      onValueChange={field.onChange}
                      placeholder="Write the system instructions for this agent..."
                    />
                  </FormControl>
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
