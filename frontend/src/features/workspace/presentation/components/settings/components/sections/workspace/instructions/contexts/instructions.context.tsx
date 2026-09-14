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
import type { Instruction } from "@/features/instruction/interfaces/instruction.interfaces";
import { errorMessage } from "@/lib/aos-facade";
import { t } from "@/lib/i18n";
import {
  instructionCreatePayload,
  instructionUpdatePayload,
  type InstructionFormValues,
} from "../helpers/instruction-payload";

const NEW_INSTRUCTION_ID = "__new_instruction__";

/** Built when the provider renders, so its messages are in the language on screen. */
function buildInstructionFormSchema() {
  return z.object({
    name: z.string().trim().min(1, t("Name is required")),
    type: z.string().trim().min(1, t("Type is required")),
    description: z.string().optional(),
    content: z.string().optional(),
    pathsText: z.string().optional(),
  });
}

interface InstructionsContextType {
  instructions: Instruction[];
  filteredInstructions: Instruction[];
  selectedInstructionId: string | null;
  selectedInstruction: Instruction | undefined;
  isCreateMode: boolean;
  isLoadingContent: boolean;
  isDeleting: boolean;
  searchQuery: string;
  form: any;
  setSelectedInstructionId: (id: string | null) => void;
  setSearchQuery: (query: string) => void;
  startCreate: () => void;
  deleteSelectedInstruction: () => void;
}

const InstructionsContext = createContext<InstructionsContextType | null>(null);

interface InstructionsProviderProps {
  children: React.ReactNode;
  instructions: Instruction[];
  refreshInstructions: () => Promise<void>;
}

function buildInstructionFormValues(
  instruction?: Instruction | null,
): InstructionFormValues {
  return {
    name: instruction?.name ?? "",
    type: instruction?.type ?? "standards",
    description: instruction?.description ?? "",
    content: instruction?.content ?? "",
    pathsText: instruction?.paths?.join("\n") ?? "",
  };
}

function getErrorMessage(error: unknown) {
  return errorMessage(error) ?? t("Unable to save this instruction.");
}

export function InstructionsProvider({
  children,
  instructions,
  refreshInstructions,
}: InstructionsProviderProps) {
  const [selectedInstructionId, setSelectedInstructionId] = useState<
    string | null
  >(null);
  const [selectedInstructionFull, setSelectedInstructionFull] = useState<
    Instruction | undefined
  >(undefined);
  const [isLoadingContent, setIsLoadingContent] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");

  const isCreateMode = selectedInstructionId === NEW_INSTRUCTION_ID;

  const instructionFormSchema = useMemo(buildInstructionFormSchema, []);
  const form = aos.useForm({
    schema: instructionFormSchema,
    values: buildInstructionFormValues(null),
    onSubmit: async (values: InstructionFormValues) => {
      if (isCreateMode) {
        const result = await aos.client.instruction.create.mutate({ body: instructionCreatePayload(values) });

        const createdInstruction = result?.data?.instruction;

        if (result?.error || !createdInstruction) {
          toast.error(getErrorMessage(result?.error));
          return;
        }

        toast.success(t("Instruction created."));
        await refreshInstructions();
        setSelectedInstructionFull(createdInstruction);
        setSelectedInstructionId(createdInstruction.id);
        form.reset(buildInstructionFormValues(createdInstruction));
        return;
      }

      if (!selectedInstructionId) return;

      const result = await aos.client.instruction.update.mutate({
        params: { instruction: selectedInstructionId },
        body: instructionUpdatePayload(values),
      });

      const updatedInstruction = result?.data?.instruction;

      if (result?.error || !updatedInstruction) {
        toast.error(getErrorMessage(result?.error));
        return;
      }

      toast.success(t("Instruction updated."));
      await refreshInstructions();
      setSelectedInstructionFull(updatedInstruction);
      form.reset(buildInstructionFormValues(updatedInstruction));
    },
  });

  const { mutate: deleteInstruction, loading: isDeleting } =
    aos.client.instruction.delete.useMutation({
      onSuccess: async () => {
        toast.success(t("Instruction deleted."));
        await refreshInstructions();
        setSelectedInstructionId(null);
        setSelectedInstructionFull(undefined);
        form.reset(buildInstructionFormValues(null));
      },
      onError: (error) => {
        toast.error(getErrorMessage(error));
      },
    });

  const filteredInstructions = useMemo(() => {
    if (!searchQuery.trim()) return instructions;
    const query = searchQuery.toLowerCase();

    return instructions.filter(
      (instruction) =>
        instruction.name.toLowerCase().includes(query) ||
        instruction.type.toLowerCase().includes(query) ||
        instruction.description?.toLowerCase().includes(query),
    );
  }, [instructions, searchQuery]);

  useEffect(() => {
    if (!selectedInstructionId) {
      setSelectedInstructionFull(undefined);
      setIsLoadingContent(false);
      form.reset(buildInstructionFormValues(null));
      return;
    }

    if (selectedInstructionId === NEW_INSTRUCTION_ID) {
      setSelectedInstructionFull(undefined);
      setIsLoadingContent(false);
      form.reset(buildInstructionFormValues(null));
      return;
    }

    const baseInstruction = instructions.find(
      (instruction) => instruction.id === selectedInstructionId,
    );

    if (baseInstruction) {
      setSelectedInstructionFull(baseInstruction);
      form.reset(buildInstructionFormValues(baseInstruction));
    }

    setIsLoadingContent(true);

    aos.client.instruction.getById
      .query({ params: { instruction: selectedInstructionId } })
      .then((response) => {
        if (response.data?.instruction) {
          setSelectedInstructionFull(response.data.instruction);
          form.reset(buildInstructionFormValues(response.data.instruction));
        }
      })
      .finally(() => {
        setIsLoadingContent(false);
      });
  }, [selectedInstructionId, instructions]);

  return (
    <InstructionsContext.Provider
      value={{
        instructions,
        filteredInstructions,
        selectedInstructionId,
        selectedInstruction: selectedInstructionFull,
        isCreateMode,
        isLoadingContent,
        isDeleting,
        searchQuery,
        form,
        setSelectedInstructionId,
        setSearchQuery,
        startCreate: () => setSelectedInstructionId(NEW_INSTRUCTION_ID),
        deleteSelectedInstruction: () => {
          if (!selectedInstructionId || isCreateMode) return;
          deleteInstruction({ params: { instruction: selectedInstructionId } });
        },
      }}
    >
      {children}
    </InstructionsContext.Provider>
  );
}

export function useInstructions() {
  const context = useContext(InstructionsContext);

  if (!context) {
    throw new Error("useInstructions must be used within InstructionsProvider");
  }

  return context;
}
