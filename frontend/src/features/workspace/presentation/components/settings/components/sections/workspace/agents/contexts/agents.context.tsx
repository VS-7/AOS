import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { toast } from "sonner";
import { aos } from "@/app/aos";
import type { Agent } from "@/features/agent/interfaces/agent.interfaces";
import { t } from "@/lib/i18n";
import {
  agentFormSchema,
  agentSlug,
  buildAgentFormValues,
  buildCreatePayload,
  buildUpdatePayload,
  EMPTY_AGENT_FORM,
  type AgentFormValues,
} from "./agent-form";

const NEW_AGENT_ID = "__new_agent__";

interface AgentsContextType {
  agents: Agent[];
  filteredAgents: Agent[];
  selectedAgentId: string | null;
  selectedAgent: Agent | undefined;
  isCreateMode: boolean;
  /** The selected agent's instructions have not arrived yet. */
  isLoadingContent: boolean;
  /**
   * The selected agent's instructions could not be read, so the form holds
   * none: editing them would save that nothing over AGENT.md.
   */
  contentLoadFailed: boolean;
  /** Reads the selected agent again after a failed load. */
  retryContentLoad: () => void;
  isDeleting: boolean;
  /** Whether the form holds edits that have not been saved. */
  isDirty: boolean;
  searchQuery: string;
  form: any;
  /** Selects an agent, or asks first when the form holds unsaved edits. */
  setSelectedAgentId: (id: string | null) => void;
  /** The selection waiting on "discard unsaved changes?", if any. */
  pendingSelection: string | null;
  confirmPendingSelection: () => void;
  cancelPendingSelection: () => void;
  setSearchQuery: (query: string) => void;
  startCreate: () => void;
  deleteSelectedAgent: () => void;
}

const AgentsContext = createContext<AgentsContextType | null>(null);

interface AgentsProviderProps {
  children: React.ReactNode;
  agents: Agent[];
  /**
   * An agent to open, named by whatever led here (the chat sidebar's "View
   * details" sends `?agent=luara`). The page reads it from the URL; the
   * provider only acts on it, so it does not need a router to exist.
   */
  requestedAgentId?: string;
  /**
   * Which request this is. The same agent asked for twice reaches the same
   * URL, so the id alone cannot tell a second "View details" from the first.
   */
  requestKey?: string;
}

function getAgentErrorMessage(error: unknown, fallback = t("Unable to save this agent.")) {
  if (error && typeof error === "object" && "message" in error) {
    const message = (error as { message?: unknown }).message;
    if (typeof message === "string" && message) return message;
  }
  return fallback;
}

/** Thrown out of `onSubmit` so `aos.useForm` leaves the typed values alone. */
class SubmitRefused extends Error {}

export function AgentsProvider({ children, agents, requestedAgentId, requestKey }: AgentsProviderProps) {
  const [selectedAgentId, setSelectedAgentIdState] = useState<string | null>(null);
  const [selectedAgentFull, setSelectedAgentFull] = useState<Agent | undefined>(undefined);
  const [isLoadingContent, setIsLoadingContent] = useState(false);
  const [contentLoadFailed, setContentLoadFailed] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [pendingSelection, setPendingSelection] = useState<string | null>(null);

  const isCreateMode = selectedAgentId === NEW_AGENT_ID;

  // The effects below react to one thing each — the selection, or the
  // roster — and read the rest through refs. Depending on both is what made
  // every roster refresh behave like a new selection.
  const agentsRef = useRef(agents);
  agentsRef.current = agents;
  const selectedIdRef = useRef(selectedAgentId);
  selectedIdRef.current = selectedAgentId;
  const fullRef = useRef(selectedAgentFull);
  fullRef.current = selectedAgentFull;
  // A load answered after the person picked someone else must not land.
  const loadSeq = useRef(0);
  // The record a save or create already handed back, so selecting it does
  // not ask the daemon for what it just said.
  const knownRecord = useRef<Agent | null>(null);
  // Whether the selected id has been seen in a roster, which is what makes
  // its absence from a later one mean "deleted" rather than "not listed yet".
  const seenInRoster = useRef<string | null>(null);
  // The roster version a re-read is already fetching: two refreshes that
  // announce the same change ask the daemon once.
  const rereading = useRef<string | null>(null);
  // Whether the form holds the selected agent's own instructions, rather than
  // the roster entry's empty ones. Read by the save, outside any render.
  const contentLoaded = useRef(false);

  const form = aos.useForm({
    schema: agentFormSchema,
    values: EMPTY_AGENT_FORM,
    onSubmit: async (values: AgentFormValues) => {
      if (selectedIdRef.current === NEW_AGENT_ID) {
        const id = agentSlug(values.name);
        if (!id) {
          form.setError("name", {
            message: t("Use a name with letters or digits: the agent's id is made from it."),
          });
          throw new SubmitRefused();
        }
        const taken = agentsRef.current.find((agent) => agent.id === id);
        if (taken) {
          form.setError("name", {
            message: t("{{name}} already uses the id {{id}}. Choose another name.", {
              name: taken.name || taken.id,
              id,
            }),
          });
          throw new SubmitRefused();
        }

        const result = await aos.client.agent.create.mutate({ body: buildCreatePayload(values) });
        const created = result?.data as Agent | undefined;
        if (result?.error || !created?.id) {
          toast.error(getAgentErrorMessage(result?.error));
          throw new SubmitRefused();
        }

        toast.success(t("Agent created."));
        knownRecord.current = created;
        setSelectedAgentFull(created);
        setSelectedAgentIdState(created.id);
        void aos.stores.agent.actions.refresh();
        return buildAgentFormValues(created);
      }

      const id = selectedIdRef.current;
      if (!id) throw new SubmitRefused();

      const body = buildUpdatePayload(values, form.formState.dirtyFields);
      // Instructions that never loaded are not an edit: whatever the field
      // holds would replace the whole of AGENT.md.
      if (!contentLoaded.current) delete body.content;
      if (Object.keys(body).length === 0) return values;

      const result = await aos.client.agent.update.mutate({ params: { agent: id }, body });
      const updated = result?.data as Agent | undefined;
      if (result?.error || !updated?.id) {
        toast.error(getAgentErrorMessage(result?.error));
        throw new SubmitRefused();
      }

      toast.success(t("Agent updated."));
      // The update answers with the whole agent, so there is nothing to
      // read back; the roster refresh below finds the same updatedAt and
      // leaves the form alone.
      setSelectedAgentFull(updated);
      void aos.stores.agent.actions.refresh();
      return buildAgentFormValues(updated);
    },
  });

  // Read during render on purpose: react-hook-form only keeps `isDirty` and
  // `dirtyFields` current once something has subscribed to them, and the
  // effects and handlers below read them outside any render.
  const isDirty = form.formState.isDirty;
  void form.formState.dirtyFields;

  /**
   * Reads the agent, instructions included.
   *
   * Only a load that has nothing to show yet marks the content as loading —
   * that is what disables Instructions. A re-read of an agent already on
   * screen (changed elsewhere) leaves the field alone: disabling it took the
   * caret out mid-sentence and dropped the next keys.
   */
  const loadAgent = useCallback(
    async (id: string) => {
      const seq = ++loadSeq.current;
      const first = !contentLoaded.current;
      if (first) {
        setIsLoadingContent(true);
        setContentLoadFailed(false);
      }
      const failed = (message: string) => {
        toast.error(message);
        // A failed re-read keeps the instructions already on screen; a failed
        // first read has none to keep.
        if (first) setContentLoadFailed(true);
      };
      try {
        const response = await aos.client.agent.getById.query({ params: { agent: id } });
        if (seq !== loadSeq.current) return;
        const agent = response?.data?.agent as Agent | undefined;
        if (response?.error || !agent) {
          failed(getAgentErrorMessage(response?.error, t("Could not load this agent.")));
          return;
        }
        contentLoaded.current = true;
        setContentLoadFailed(false);
        setSelectedAgentFull(agent);
        // Whatever the person typed while this was in flight stays; only
        // the untouched fields take the daemon's values.
        form.reset(buildAgentFormValues(agent), { keepDirtyValues: true });
      } catch (error) {
        if (seq !== loadSeq.current) return;
        // A transport failure, not a refusal.
        console.error("[AgentsContext] failed to load agent", error);
        failed(getAgentErrorMessage(error, t("Could not load this agent.")));
      } finally {
        if (seq === loadSeq.current) {
          setIsLoadingContent(false);
          rereading.current = null;
        }
      }
    },
    [form],
  );

  // A new selection — and only a new selection — replaces the form.
  useEffect(() => {
    loadSeq.current++;
    rereading.current = null;
    setContentLoadFailed(false);

    if (!selectedAgentId || selectedAgentId === NEW_AGENT_ID) {
      // A new agent's instructions are whatever is typed: nothing to lose.
      contentLoaded.current = true;
      setSelectedAgentFull(undefined);
      setIsLoadingContent(false);
      form.reset(EMPTY_AGENT_FORM);
      return;
    }

    const known = knownRecord.current;
    knownRecord.current = null;
    if (known?.id === selectedAgentId) {
      contentLoaded.current = true;
      seenInRoster.current = null;
      setIsLoadingContent(false);
      form.reset(buildAgentFormValues(known));
      return;
    }

    contentLoaded.current = false;

    const listed = agentsRef.current.find((agent) => agent.id === selectedAgentId);
    seenInRoster.current = listed ? selectedAgentId : null;
    setSelectedAgentFull(listed);
    form.reset(buildAgentFormValues(listed));
    void loadAgent(selectedAgentId);
  }, [selectedAgentId, form, loadAgent]);

  // A roster refresh follows the selected agent without clobbering it.
  useEffect(() => {
    const id = selectedIdRef.current;
    if (!id || id === NEW_AGENT_ID) return;

    const listed = agents.find((agent) => agent.id === id);
    if (!listed) {
      // Gone from a roster it was already in: deleted, here or elsewhere.
      // Asking the daemon for it would only fetch "not found".
      if (seenInRoster.current === id) setSelectedAgentIdState(null);
      return;
    }
    seenInRoster.current = id;

    const current = fullRef.current;
    if (!current || listed.updatedAt === current.updatedAt) return;
    if (rereading.current === (listed.updatedAt ?? "")) return;
    // Changed elsewhere. Unsaved edits win: the person is looking at them,
    // and a reload under their cursor is the defect this replaced.
    if (form.formState.isDirty) return;
    rereading.current = listed.updatedAt ?? "";
    void loadAgent(id);
  }, [agents, form, loadAgent]);

  const { mutate: deleteAgent, loading: isDeleting } =
    aos.client.agent.delete.useMutation({
      onSuccess: async () => {
        toast.success(t("Agent deleted."));
        // Let go of the agent first. Refreshing while it was still selected
        // sent the reload after a record that no longer existed.
        setSelectedAgentIdState(null);
        setSelectedAgentFull(undefined);
        form.reset(EMPTY_AGENT_FORM);
        await aos.stores.agent.actions.refresh();
      },
      onError: (error) => {
        toast.error(getAgentErrorMessage(error));
      },
    });

  const selectAgent = useCallback(
    (id: string | null) => {
      if (id === selectedIdRef.current) return;
      if (selectedIdRef.current && form.formState.isDirty) {
        setPendingSelection(id ?? "");
        return;
      }
      setSelectedAgentIdState(id);
    },
    [form],
  );

  // The requested agent is the one to open: the chat sidebar's "View details"
  // landed here with both agents listed and nothing selected. Applied once the
  // roster has it, and again only for a new request — another agent, or the
  // same one asked for again (`requestKey`) — so it never fights a selection
  // the person makes afterwards. It goes through selectAgent, like a click,
  // so an unsaved edit to the agent already open is asked about rather than
  // discarded.
  const requestedAgentListed =
    typeof requestedAgentId === "string" &&
    agents.some((agent) => agent.id === requestedAgentId);
  useEffect(() => {
    if (requestedAgentListed) selectAgent(requestedAgentId as string);
  }, [requestedAgentId, requestKey, requestedAgentListed, selectAgent]);

  const filteredAgents = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    if (!query) return agents;

    return agents.filter((agent) =>
      [agent.name, agent.id, agent.role, agent.description, agent.skill, agent.provider, agent.model]
        .some((value) => value?.toLowerCase().includes(query)),
    );
  }, [agents, searchQuery]);

  return (
    <AgentsContext.Provider
      value={{
        agents,
        filteredAgents,
        selectedAgentId,
        selectedAgent: selectedAgentFull,
        isCreateMode,
        isLoadingContent,
        contentLoadFailed,
        retryContentLoad: () => {
          const id = selectedIdRef.current;
          if (id && id !== NEW_AGENT_ID && !contentLoaded.current) void loadAgent(id);
        },
        isDeleting,
        isDirty,
        searchQuery,
        form,
        setSelectedAgentId: selectAgent,
        pendingSelection,
        confirmPendingSelection: () => {
          if (pendingSelection === null) return;
          setPendingSelection(null);
          setSelectedAgentIdState(pendingSelection || null);
        },
        cancelPendingSelection: () => setPendingSelection(null),
        setSearchQuery,
        startCreate: () => selectAgent(NEW_AGENT_ID),
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
