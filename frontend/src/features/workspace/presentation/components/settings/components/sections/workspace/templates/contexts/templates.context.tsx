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
import type { Template } from "@/features/template/interfaces/template.interfaces";
import { t } from "@/lib/i18n";

const NEW_TEMPLATE_ID = "__new_template__";

const templateFormSchema = z.object({
  name: z.string().trim().min(1, "Name is required"),
  skill: z.string().optional(),
  description: z.string().trim().min(1, "Description is required"),
  output: z.string().optional(),
  schemaText: z.string().optional(),
  content: z.string().optional(),
});

type TemplateFormValues = z.infer<typeof templateFormSchema>;

interface TemplatesContextType {
  templates: Omit<Template, "schema" | "content">[];
  filteredTemplates: Omit<Template, "schema" | "content">[];
  selectedTemplateId: string | null;
  selectedTemplate: Template | undefined;
  isCreateMode: boolean;
  isLoadingContent: boolean;
  isDeleting: boolean;
  searchQuery: string;
  form: any;
  setSelectedTemplateId: (id: string | null) => void;
  setSearchQuery: (query: string) => void;
  startCreate: () => void;
  deleteSelectedTemplate: () => void;
}

const TemplatesContext = createContext<TemplatesContextType | null>(null);

interface TemplatesProviderProps {
  children: React.ReactNode;
  templates: Omit<Template, "schema" | "content">[];
  refreshTemplates: () => Promise<void>;
}

function buildTemplateFormValues(
  template?: Template | null,
): TemplateFormValues {
  return {
    name: template?.name ?? "",
    skill: template?.skill ?? "",
    description: template?.description ?? "",
    output: template?.output ?? "",
    schemaText: template?.schema
      ? JSON.stringify(template.schema, null, 2)
      : "",
    content: template?.content ?? "",
  };
}

function parseTemplateSchema(schemaText?: string) {
  const trimmed = schemaText?.trim();

  if (!trimmed) return undefined;

  return JSON.parse(trimmed);
}

function getTemplatePayload(values: TemplateFormValues) {
  return {
    name: values.name.trim(),
    skill: values.skill?.trim() || undefined,
    description: values.description.trim(),
    output: values.output?.trim() || undefined,
    schema: parseTemplateSchema(values.schemaText),
    content: values.content?.trim() || undefined,
  };
}

function getTemplateErrorMessage(error: unknown) {
  if (error instanceof Error) return error.message;
  return "Unable to save this template.";
}

export function TemplatesProvider({
  children,
  templates,
  refreshTemplates,
}: TemplatesProviderProps) {
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(
    null,
  );
  const [selectedTemplateFull, setSelectedTemplateFull] = useState<
    Template | undefined
  >(undefined);
  const [isLoadingContent, setIsLoadingContent] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");

  const isCreateMode = selectedTemplateId === NEW_TEMPLATE_ID;

  const form = aos.useForm({
    schema: templateFormSchema,
    values: buildTemplateFormValues(null),
    onSubmit: async (values: TemplateFormValues) => {
      let body: ReturnType<typeof getTemplatePayload>;

      try {
        body = getTemplatePayload(values);
      } catch {
        toast.error(t("Template schema must be valid JSON."));
        return;
      }

      if (isCreateMode) {
        const result = await aos.client.template.create.mutate({ body });

        const createdTemplate = result?.data;

        if (result?.error || !createdTemplate?.id) {
          toast.error(getTemplateErrorMessage(result?.error));
          return;
        }

        toast.success(t("Template created."));
        await refreshTemplates();
        setSelectedTemplateFull(createdTemplate as Template);
        setSelectedTemplateId(createdTemplate.id);
        form.reset(buildTemplateFormValues(createdTemplate as Template));
        return;
      }

      if (!selectedTemplateId) return;

      const result = await aos.client.template.update.mutate({
        params: { template: selectedTemplateId },
        body: {
          skill: body.skill,
          description: body.description,
          output: body.output,
          schema: body.schema,
          content: body.content,
        },
      });

      const updatedTemplate = result?.data;

      if (result?.error || !updatedTemplate?.id) {
        toast.error(getTemplateErrorMessage(result?.error));
        return;
      }

      toast.success(t("Template updated."));
      await refreshTemplates();
      setSelectedTemplateFull(updatedTemplate as Template);
      form.reset(buildTemplateFormValues(updatedTemplate as Template));
    },
  });

  const { mutate: deleteTemplate, loading: isDeleting } =
    aos.client.template.delete.useMutation({
      onSuccess: async () => {
        toast.success(t("Template deleted."));
        await refreshTemplates();
        setSelectedTemplateId(null);
        setSelectedTemplateFull(undefined);
        form.reset(buildTemplateFormValues(null));
      },
      onError: (error) => {
        toast.error(getTemplateErrorMessage(error));
      },
    });

  const filteredTemplates = useMemo(() => {
    if (!searchQuery.trim()) return templates;
    const query = searchQuery.toLowerCase();

    return templates.filter(
      (template) =>
        template.name.toLowerCase().includes(query) ||
        template.description?.toLowerCase().includes(query) ||
        template.skill?.toLowerCase().includes(query),
    );
  }, [templates, searchQuery]);

  useEffect(() => {
    if (!selectedTemplateId) {
      setSelectedTemplateFull(undefined);
      setIsLoadingContent(false);
      form.reset(buildTemplateFormValues(null));
      return;
    }

    if (selectedTemplateId === NEW_TEMPLATE_ID) {
      setSelectedTemplateFull(undefined);
      setIsLoadingContent(false);
      form.reset(buildTemplateFormValues(null));
      return;
    }

    const baseTemplate = templates.find(
      (template) => template.id === selectedTemplateId,
    );

    if (baseTemplate) {
      setSelectedTemplateFull(baseTemplate as Template);
      form.reset(buildTemplateFormValues(baseTemplate as Template));
    }

    setIsLoadingContent(true);

    aos.client.template.getById
      .query({ params: { template: selectedTemplateId } })
      .then((response) => {
        if (response.data?.id) {
          setSelectedTemplateFull(response.data as Template);
          form.reset(buildTemplateFormValues(response.data as Template));
        }
      })
      .finally(() => {
        setIsLoadingContent(false);
      });
  }, [selectedTemplateId, templates]);

  return (
    <TemplatesContext.Provider
      value={{
        templates,
        filteredTemplates,
        selectedTemplateId,
        selectedTemplate: selectedTemplateFull,
        isCreateMode,
        isLoadingContent,
        isDeleting,
        searchQuery,
        form,
        setSelectedTemplateId,
        setSearchQuery,
        startCreate: () => setSelectedTemplateId(NEW_TEMPLATE_ID),
        deleteSelectedTemplate: () => {
          if (!selectedTemplateId || isCreateMode) return;
          deleteTemplate({ params: { template: selectedTemplateId } });
        },
      }}
    >
      {children}
    </TemplatesContext.Provider>
  );
}

export function useTemplates() {
  const context = useContext(TemplatesContext);

  if (!context) {
    throw new Error("useTemplates must be used within TemplatesProvider");
  }

  return context;
}
