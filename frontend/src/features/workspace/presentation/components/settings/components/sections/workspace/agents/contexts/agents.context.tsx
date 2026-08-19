import React, {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { z } from "zod";
import { toast } from "sonner";
import { aos } from "@/app/aos";
import type { FractalAgent } from "@/features/agent/interfaces/agent.interfaces";

const NEW_AGENT_ID = "__new_agent__";

const agentFormSchema = z.object({
  name: z.string().trim().min(1, "Name is required"),
  image: z.string().optional().or(z.literal("")),
  description: z.string().optional(),
  role: z.string().optional(),
  skill: z.string().optional(),
  provider: z.string().optional(),
  model: z.string().optional(),
  content: z.string().optional(),
  orchestrator: z.boolean().default(false),
});

type AgentFormValues = z.infer<typeof agentFormSchema>;

interface AgentsContextType {
  agents: FractalAgent[];
  filteredAgents: FractalAgent[];
  selectedAgentId: string | null;
  selectedAgent: FractalAgent | undefined;
  isCreateMode: boolean;
  isLoadingContent: boolean;
  isDeleting: boolean;
  searchQuery: string;
  form: any;
  setSelectedAgentId: (id: string | null) => void;
  setSearchQuery: (query: string) => void;
  startCreate: () => void;
  deleteSelectedAgent: () => void;
}

const AgentsContext = createContext<AgentsContextType | null>(null);

interface AgentsProviderProps {
  children: React.ReactNode;
  agents: FractalAgent[];
}

function buildAgentFormValues(agent?: FractalAgent | null): AgentFormValues {
  return {
    name: agent?.name ?? "",
    image: agent?.image ?? "",
    description: agent?.description ?? "",
    role: agent?.role ?? "",
    skill: agent?.skill ?? "",
    provider: agent?.provider ?? "",
    model: agent?.model ?? "",
    content: agent?.content ?? "",
    orchestrator: agent?.orchestrator ?? false,
  };
}

function buildAgentPayload(values: AgentFormValues) {
  return {
    name: values.name.trim(),
    // Empty string clears a previous avatar; omit only when truly unset on create.
    image: (values.image ?? "").trim(),
    description: values.description?.trim() || undefined,
    role: values.role?.trim() || undefined,
    skill: values.skill?.trim() || undefined,
    provider: values.provider?.trim() || undefined,
    model: values.model?.trim() || undefined,
    content: values.content?.trim() || undefined,
    orchestrator: values.orchestrator,
  };
}

function getAgentErrorMessage(error: unknown) {
  if (error instanceof Error) return error.message;
  return "Unable to save this agent.";
}

export function AgentsProvider({ children, agents }: AgentsProviderProps) {
  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null);
  const [selectedAgentFull, setSelectedAgentFull] = useState<FractalAgent | undefined>(
    undefined,
  );
  const [isLoadingContent, setIsLoadingContent] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");

  const isCreateMode = selectedAgentId === NEW_AGENT_ID;

  const form = aos.useForm({
    schema: agentFormSchema,
    values: buildAgentFormValues(null),
    onSubmit: async (values: AgentFormValues) => {
      const body = buildAgentPayload(values);

      if (isCreateMode) {
        const result = await aos.client.agent.create.mutate({ body });

        const createdAgent = result?.data;

        if (result?.error || !createdAgent?.id) {
          toast.error(getAgentErrorMessage(result?.error));
          return;
        }

        toast.success("Agent created.");
        await aos.stores.agent.actions.refresh();
        setSelectedAgentFull(createdAgent as FractalAgent);
        setSelectedAgentId(createdAgent.id);
        form.reset(buildAgentFormValues(createdAgent as FractalAgent));
        return;
      }

      if (!selectedAgentId) return;

      const result = await aos.client.agent.update.mutate({
        params: { agent: selectedAgentId },
        body,
      });

      const updatedAgent = result?.data;

      if (result?.error || !updatedAgent?.id) {
        toast.error(getAgentErrorMessage(result?.error));
        return;
      }

      toast.success("Agent updated.");
      await aos.stores.agent.actions.refresh();
      setSelectedAgentFull(updatedAgent as FractalAgent);
      form.reset(buildAgentFormValues(updatedAgent as FractalAgent));
    },
  });

  const { mutate: deleteAgent, loading: isDeleting } =
    aos.client.agent.delete.useMutation({
      onSuccess: async () => {
        toast.success("Agent deleted.");
        await aos.stores.agent.actions.refresh();
        setSelectedAgentId(null);
        setSelectedAgentFull(undefined);
        form.reset(buildAgentFormValues(null));
      },
      onError: (error) => {
        toast.error(getAgentErrorMessage(error));
      },
    });

  const filteredAgents = useMemo(() => {
    if (!searchQuery.trim()) return agents;
    const query = searchQuery.toLowerCase();

    return agents.filter(
      (agent) =>
        agent.name.toLowerCase().includes(query) ||
        agent.description?.toLowerCase().includes(query) ||
        agent.skill?.toLowerCase().includes(query) ||
        agent.provider?.toLowerCase().includes(query) ||
        agent.model?.toLowerCase().includes(query),
    );
  }, [agents, searchQuery]);

  useEffect(() => {
    if (!selectedAgentId) {
      setSelectedAgentFull(undefined);
      setIsLoadingContent(false);
      form.reset(buildAgentFormValues(null));
      return;
    }

    if (selectedAgentId === NEW_AGENT_ID) {
      setSelectedAgentFull(undefined);
      setIsLoadingContent(false);
      form.reset(buildAgentFormValues(null));
      return;
    }

    const baseAgent = agents.find((agent) => agent.id === selectedAgentId);

    if (baseAgent) {
      setSelectedAgentFull(baseAgent);
      form.reset(buildAgentFormValues(baseAgent));
    }

    setIsLoadingContent(true);

    aos.client.agent.getById
      .query({ params: { agent: selectedAgentId } })
      .then((response) => {
        if (response.data?.agent) {
          setSelectedAgentFull(response.data.agent);
          form.reset(buildAgentFormValues(response.data.agent));
        }
      })
      .finally(() => {
        setIsLoadingContent(false);
      });
  }, [selectedAgentId, agents]);

  return (
    <AgentsContext.Provider
      value={{
        agents,
        filteredAgents,
        selectedAgentId,
        selectedAgent: selectedAgentFull,
        isCreateMode,
        isLoadingContent,
        isDeleting,
        searchQuery,
        form,
        setSelectedAgentId,
        setSearchQuery,
        startCreate: () => setSelectedAgentId(NEW_AGENT_ID),
        deleteSelectedAgent: () => {
          if (!selectedAgentId || isCreateMode) return;
          deleteAgent({ params: { agent: selectedAgentId } });
        },
      }}
    >
      {children}
    </AgentsContext.Provider>
  );
}

export function useAgents() {
  const context = useContext(AgentsContext);

  if (!context) {
    throw new Error("useAgents must be used within AgentsProvider");
  }

  return context;
}
