import * as React from "react";
import {
  ArrowLeft,
  BadgeCheck,
  Braces,
  Database,
  FileCode2,
  Hash,
  ListChecks,
  PencilLine,
  PlusCircle,
  Save,
  Sparkles,
  Trash2,
} from "lucide-react";
import { useNavigate, useRouter } from "@tanstack/react-router";
import { toast } from "sonner";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Page, PageBody } from "@/components/ui/page";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { Textarea } from "@/components/ui/textarea";
import { aos } from "@/app/aos";
import { stores } from "@/app/lib/stores";
import { isDormant } from "@/lib/command-map";
import { DormantGate } from "@/components/DormantDomain";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { SchemaForm } from "../../../../components/schema-form";
import {
  FormSchemaHelper,
  type CollectionUpsertFormValues,
} from "../../../../helpers/form-schema.helper";
import { ButtonGroup } from "@/components/ui/button-group";

function getErrorMessage(error: unknown) {
  if (error instanceof Error) {
    return error.message;
  }

  return "Unable to save this record.";
}

function isRecordObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function extractRecordData(record: Record<string, unknown> | null) {
  if (!record) {
    return {};
  }

  const { id: _id, content: _content, path: _path, ...data } = record;
  return data;
}

function resolveRecord(result: unknown) {
  if (Array.isArray(result)) {
    return result[0] as Record<string, unknown> | undefined;
  }

  if (result && typeof result === "object") {
    return result as Record<string, unknown>;
  }

  return null;
}

function getPrimaryLabel(
  recordId: string,
  data: Record<string, unknown> | null | undefined,
) {
  const candidates = ["name", "title", "label", "email"];

  for (const candidate of candidates) {
    const value = data?.[candidate];

    if (typeof value === "string" && value.trim().length > 0) {
      return value.trim();
    }
  }

  return recordId === "new"
    ? "New entry"
    : FormSchemaHelper.prettifyLabel(recordId);
}

function getRequiredCount(schema: Record<string, unknown>) {
  const required = schema.required;
  return Array.isArray(required) ? required.length : 0;
}

export const CollectionRecordUpsertPage = aos
  .page("/collections/$id/records/$record")
  .withMetadata({
    title: "Collection Record",
    description: "Create and edit collection records",
  })
  .use(WorkspacePageMiddleware())
  .withLoader(async ({ client, request, response }) => {
    // Task 10: the `collection` domain is dormant — no Go backend to call
    // yet. Short-circuits before any client call so the dormant command's
    // empty envelope never reaches the `!collection` check below, which
    // would otherwise call `response.notFound()`. `collection` here is a
    // stub, not a real empty record — see the `isDormant` early return in
    // `withComponent` below, which bails before this stub's fields (e.g.
    // `collection.format`) are ever dereferenced.
    if (isDormant("collection")) {
      return { collection: {} as any, mode: "create" as const, record: null };
    }

    const collectionResult = await client.collection.getById.query({
      params: { collection: request.params.id },
    });

    const collection = collectionResult.data?.collection;

    if (!collection) {
      return response.notFound();
    }

    const isCreate = request.params.record === "new";

    if (isCreate) {
      return {
        collection,
        mode: "create" as const,
        record: null,
      };
    }

    const recordResult = await client.collection.getRecordById.query({
      params: {
        collection: request.params.id,
        record: request.params.record,
      },
    });

    const record = resolveRecord(recordResult.data);

    if (!record) {
      return response.notFound();
    }

    return {
      collection,
      mode: "edit" as const,
      record,
    };
  })
  .withComponent(({ route }) => {
    const navigate = useNavigate();
    const router = useRouter();
    const { collection, mode, record } = route.useLoaderData();
    const collectionId = route.useParams().id;
    const recordParam = route.useParams().record;
    const schema = isRecordObject(collection.schema) ? collection.schema : {};
    const requiredCount = getRequiredCount(schema);
    const isEditMode = mode === "edit";
    const recordId = typeof record?.id === "string" ? record.id : recordParam;
    const formSchema = React.useMemo(
      () => FormSchemaHelper.createUpsertFormSchema(schema),
      [schema],
    );
    const formValues = React.useMemo(
      () =>
        FormSchemaHelper.buildUpsertFormValues(
          schema,
          extractRecordData(record),
          typeof record?.content === "string" ? record.content : "",
        ),
      [record, schema],
    );

    const { mutate: deleteRecord, loading: isDeleting } =
      aos.client.collection.deleteRecord.useMutation({
        onSuccess: async () => {
          toast.success("Record deleted.");
          await router.invalidate();
          await navigate({
            to: "/collections/$id",
            params: { id: collectionId },
          });
        },
        onError: (error) => {
          toast.error(getErrorMessage(error));
        },
      });

    const form = aos.useForm({
      schema: formSchema,
      values: formValues,
      preventNavigation: false,
      onSubmit: async (values: CollectionUpsertFormValues) => {
        const body = {
          data: values.data,
          ...(collection.format === "md"
            ? { content: values.content ?? "" }
            : {}),
        };

        if (isEditMode) {
          const result = await aos.client.collection.updateRecord.mutate({
            params: { collection: collectionId, record: recordId },
            body,
          });

          if (result?.error || !result.data?.id) {
            toast.error(getErrorMessage(result?.error));
            return;
          }

          toast.success(isEditMode ? "Record updated." : "Record created.");

          router.invalidate();
          navigate({ to: "/collections/$id", params: { id: collectionId } });

          return;
        }

        const result = await aos.client.collection.createRecord.mutate({
          params: { collection: collectionId },
          body,
        });

        if (result?.error || !result.data?.id) {
          toast.error(getErrorMessage(result?.error));
          return;
        }

        toast.success(isEditMode ? "Record updated." : "Record created.");

        router.invalidate();
        navigate({ to: "/collections/$id", params: { id: collectionId } });
      },
    });

    const liveData = form.watch("data");
    const displayTitle = getPrimaryLabel(
      recordId,
      FormSchemaHelper.isPlainObject(liveData)
        ? liveData
        : extractRecordData(record),
    );
    const editorStateLabel = form.isLoading
      ? isEditMode
        ? "Saving changes..."
        : "Creating record..."
      : form.formState.isDirty
        ? "Unsaved changes"
        : "Everything saved";

    function handleBack() {
      void navigate({ to: "/collections/$id", params: { id: collectionId } });
    }

    // Every hook above has already run unconditionally, in the same order
    // on every render — `isDormant("collection")` is a build-time
    // constant, not state, so this bail does not change hook order across
    // renders. It runs *before* the JSX below, which dereferences
    // `collection.name`/`collection.format` as non-null fields (the real
    // loader guarantees that; the dormant stub above does not) — building
    // that tree with the stub would throw, not just render emptily.
    if (isDormant("collection")) {
      return <DormantGate feature="collection">{null}</DormantGate>;
    }

    return (
      <Page className="h-full overflow-hidden">
        <PageBody className="overflow-hidden">
          <Form form={form} className="flex h-full flex-1 flex-col">
            <SplitPageLayout>
              <SplitPageLayout.Content>
                <div className="grid h-full grid-rows-[auto_1fr]">
                  <SplitPageLayout.ContentHeader>
                    <SplitPageLayout.ContentHeaderMain className="items-center">
                      <SplitPageLayout.ContentTitle>
                        {displayTitle}
                      </SplitPageLayout.ContentTitle>

                      <ButtonGroup className="bg-secondary/30 rounded-full">
                        <Badge variant="outline">
                          <Database />
                          {collection.name}
                        </Badge>
                        <Badge variant="outline">
                          {isEditMode ? <PencilLine /> : <PlusCircle />}
                          {isEditMode ? "Editing" : "New"}
                        </Badge>
                        <Badge variant="outline">
                          <FileCode2 />
                          {collection.format.toUpperCase()}
                        </Badge>
                        <Badge variant="outline">
                          <BadgeCheck />
                          {requiredCount} required{" "}
                          {requiredCount === 1 ? "field" : "fields"}
                        </Badge>
                      </ButtonGroup>
                    </SplitPageLayout.ContentHeaderMain>

                    <SplitPageLayout.ContentHeaderActions>
                      <span className="hidden text-xs text-muted-foreground md:block">
                        {editorStateLabel}
                      </span>

                      <ButtonGroup>
                        {isEditMode ? (
                          <AlertDialog>
                            <AlertDialogTrigger asChild>
                              <Button
                                type="button"
                                size="sm"
                                variant="destructive"
                                disabled={isDeleting}
                              >
                                <Trash2 />
                                Delete
                              </Button>
                            </AlertDialogTrigger>
                            <AlertDialogContent size="sm">
                              <AlertDialogHeader>
                                <AlertDialogTitle>
                                  Delete this record?
                                </AlertDialogTitle>
                                <AlertDialogDescription>
                                  This action removes{" "}
                                  <strong>{recordId}</strong> from{" "}
                                  <strong>{collection.name}</strong>.
                                </AlertDialogDescription>
                              </AlertDialogHeader>
                              <AlertDialogFooter>
                                <AlertDialogCancel disabled={isDeleting}>
                                  Cancel
                                </AlertDialogCancel>
                                <AlertDialogAction
                                  variant="destructive"
                                  disabled={isDeleting}
                                  onClick={() =>
                                    deleteRecord({
                                      params: {
                                        collection: collectionId,
                                        record: recordId,
                                      },
                                    })
                                  }
                                >
                                  {isDeleting ? "Deleting..." : "Delete record"}
                                </AlertDialogAction>
                              </AlertDialogFooter>
                            </AlertDialogContent>
                          </AlertDialog>
                        ) : null}

                        <Button
                          type="button"
                          variant="secondary"
                          size="sm"
                          onClick={() => void form.submit()}
                          disabled={form.isLoading}
                        >
                          <Save />
                          {form.isLoading
                            ? "Saving..."
                            : isEditMode
                              ? "Save changes"
                              : "Create record"}
                        </Button>
                      </ButtonGroup>
                    </SplitPageLayout.ContentHeaderActions>
                  </SplitPageLayout.ContentHeader>

                  <SplitPageLayout.ContentBody>
                    <div className="container mx-auto max-w-3xl py-6 pb-10">
                      <SchemaForm
                        form={form}
                        schema={schema}
                        disabled={form.isLoading}
                      />

                      {collection.format === "md" ? (
                        <div className="mt-10">
                          <FormField
                            control={form.control}
                            name={"content"}
                            render={({ field }) => (
                              <FormItem className="grid gap-3 sm:grid-cols-[minmax(0,190px)_minmax(0,1fr)] sm:gap-8">
                                <div className="space-y-1 pt-1">
                                  <FormLabel className="text-[0.95rem] font-medium text-foreground">
                                    Content
                                  </FormLabel>
                                  <FormDescription className="max-w-xs text-[0.82rem] leading-5 text-muted-foreground">
                                    Stored separately from the structured
                                    fields.
                                  </FormDescription>
                                </div>

                                <div className="space-y-2">
                                  <FormControl>
                                    <Textarea
                                      value={
                                        typeof field.value === "string"
                                          ? field.value
                                          : ""
                                      }
                                      onChange={(event) =>
                                        field.onChange(event.target.value)
                                      }
                                      placeholder="Write the record body..."
                                      className="min-h-72 rounded-[24px] border-border/70 bg-background/70 px-4 py-3 font-mono shadow-none"
                                    />
                                  </FormControl>
                                  <FormMessage className="text-xs" />
                                </div>
                              </FormItem>
                            )}
                          />
                        </div>
                      ) : null}
                    </div>
                  </SplitPageLayout.ContentBody>
                </div>
              </SplitPageLayout.Content>

              <SplitPageLayout.Detail>
                <SplitPageLayout.DetailTabs defaultValue="overview">
                  <SplitPageLayout.DetailTab
                    value="overview"
                    label="Overview"
                    icon={ListChecks}
                  >
                    <SplitPageLayout.Widget>
                      <SplitPageLayout.WidgetContent>
                        <SplitPageLayout.WidgetItem>
                          <Sparkles className="size-3.5 shrink-0 text-muted-foreground" />
                          <span className="w-16 shrink-0 text-xs text-muted-foreground">
                            State
                          </span>
                          <span className="text-xs">{editorStateLabel}</span>
                        </SplitPageLayout.WidgetItem>
                        <SplitPageLayout.WidgetItem>
                          {isEditMode ? (
                            <PencilLine className="size-3.5 shrink-0 text-muted-foreground" />
                          ) : (
                            <PlusCircle className="size-3.5 shrink-0 text-muted-foreground" />
                          )}
                          <span className="w-16 shrink-0 text-xs text-muted-foreground">
                            Mode
                          </span>
                          <span className="text-xs">
                            {isEditMode ? "Editing" : "Creating"}
                          </span>
                        </SplitPageLayout.WidgetItem>
                        <SplitPageLayout.WidgetItem>
                          <FileCode2 className="size-3.5 shrink-0 text-muted-foreground" />
                          <span className="w-16 shrink-0 text-xs text-muted-foreground">
                            Format
                          </span>
                          <Badge variant="outline">
                            <FileCode2 />
                            {collection.format}
                          </Badge>
                        </SplitPageLayout.WidgetItem>
                        <SplitPageLayout.WidgetItem>
                          <Braces className="size-3.5 shrink-0 text-muted-foreground" />
                          <span className="w-16 shrink-0 text-xs text-muted-foreground">
                            Schema
                          </span>
                          <span className="text-xs">
                            {requiredCount} required
                          </span>
                        </SplitPageLayout.WidgetItem>
                      </SplitPageLayout.WidgetContent>
                    </SplitPageLayout.Widget>

                    <SplitPageLayout.Widget>
                      <SplitPageLayout.WidgetHeader>
                        <SplitPageLayout.WidgetTitle>
                          Collection
                        </SplitPageLayout.WidgetTitle>
                      </SplitPageLayout.WidgetHeader>
                      <SplitPageLayout.WidgetContent>
                        <SplitPageLayout.WidgetItem>
                          <Database className="size-3.5 shrink-0 text-muted-foreground" />
                          <span className="w-16 shrink-0 text-xs text-muted-foreground">
                            Name
                          </span>
                          <span className="truncate text-xs">
                            {collection.name}
                          </span>
                        </SplitPageLayout.WidgetItem>
                        <SplitPageLayout.WidgetItem>
                          <Hash className="size-3.5 shrink-0 text-muted-foreground" />
                          <span className="w-16 shrink-0 text-xs text-muted-foreground">
                            Record
                          </span>
                          <span className="truncate text-xs">{recordId}</span>
                        </SplitPageLayout.WidgetItem>
                      </SplitPageLayout.WidgetContent>
                    </SplitPageLayout.Widget>
                  </SplitPageLayout.DetailTab>
                </SplitPageLayout.DetailTabs>
              </SplitPageLayout.Detail>
            </SplitPageLayout>
          </Form>
        </PageBody>
      </Page>
    );
  })
  .build();
