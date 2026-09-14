import { useState } from "react";
import { useFieldArray } from "react-hook-form";
import { PencilIcon, PlusIcon, SearchIcon, Trash2Icon } from "lucide-react";
import { z } from "zod";

import { aos } from "@/app/aos";
import { SettingsSectionShell } from "../../../section-shell";
import { Form } from "@/components/ui/form";
import { AppError } from "@/core/errors/aos.error";
import { WorkspaceTaskType } from "@/features/workspace/interfaces/workspace.interfaces";
import { WorkspaceTaskTypeSchema } from "@/features/workspace/schemas/workspace.schema";
import { toast } from "sonner";
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { useAlert } from "@/components/ui/alert-provider";
import { cn } from "@/lib/utils";
import { UpsertTaskTypeView } from "./views/upsert-task-type.view";
import { SETTINGS_CONTENT_MAX_WIDTH } from "../../../../constants";
import { changedSettings } from "../../../../helpers/changed-settings";
import { saveWorkspaceSettings } from "../../../../helpers/save-workspace-settings";
import { t } from "@/lib/i18n";

const taskTypesFormSchema = z.object({
  tasks: z.array(WorkspaceTaskTypeSchema).min(1, "At least one task type is required."),
});

type WorkspaceSnapshot = typeof aos.stores.workspace.state.current;

/** What the form shows for a workspace — also what a change is measured against. */
function taskTypesFormValues(workspace: WorkspaceSnapshot): { tasks: WorkspaceTaskType[] } {
  // Go's `workspace_get` does not always carry a label or colour, and this
  // form's schema requires both, so an entry missing either gets the same
  // fallback an empty workspace starts from.
  const tasks = (workspace?.tasks ?? []) as WorkspaceTaskType[];
  return {
    tasks: tasks.length
      ? tasks.map((task) => ({ ...task, label: task.label || "Task", color: task.color || "#64748b" }))
      : [{ id: "task", label: "Task", color: "#64748b", instructions: "" }],
  };
}

export function WorkspaceTasksSection() {
  // The store, not the route context: the context is a copy taken when the
  // route loaded, and a save never reached it — a deleted type came back on
  // returning here, and the next save wrote it back.
  const currentWorkspace = aos.stores.workspace.useState((state) => state.current);
  const { confirm } = useAlert();
  const [search, setSearch] = useState("");
  const [upsertOpen, setUpsertOpen] = useState(false);
  const [selectedTaskType, setSelectedTaskType] = useState<
    { data: WorkspaceTaskType; index: number } | undefined
  >();

  // No autosave mode: the list changes only through handleSave and
  // handleDelete, which submit themselves. With "onChange" as well, the
  // field-array write they make fired the autosave too, and every task type
  // was saved twice, with two "updated" toasts.
  const form = aos.useForm({
    schema: taskTypesFormSchema,
    values: taskTypesFormValues(currentWorkspace),
    onSubmit: async (values) => {
      const saved = aos.stores.workspace.state.current;
      if (await saveWorkspaceSettings(saved?.id, changedSettings("", values, taskTypesFormValues(saved)))) {
        toast.success(t("Task settings updated successfully!"));
      }
      return values;
    },
    onResponse: ({ error }) => {
      if (!error) return;
      // What the daemon refused — a type with no id or label, or two types
      // sharing an id — is put back on screen as it is saved, not as the
      // refused edit left it.
      form.reset(taskTypesFormValues(aos.stores.workspace.state.current));
      if (error instanceof AppError) {
        toast.error(error.message);
        return;
      }
      toast.error(error.message || t("Failed to update task settings"));
    },
  });

  // `aos.useForm`'s `TValues` generic doesn't propagate the array item
  // shape from `taskTypesFormSchema` through to `form.control` here, so
  // `useFieldArray` falls back to react-hook-form's default (untyped)
  // field shape without this explicit type parameter.
  //
  // `keyName: "key"`: react-hook-form writes its own generated row key into
  // each item under `keyName`, "id" by default — the task type's own id. The
  // editor was handed that uuid as the type's id, so saving an edit renamed
  // "bug" to "f6c08556-…" in the workspace config.
  const taskTypesFieldArray = useFieldArray<{ tasks: WorkspaceTaskType[] }, "tasks", "key">({
    control: form.control as any,
    name: "tasks",
    keyName: "key",
  });

  const filteredTasks = taskTypesFieldArray.fields
    .map((field, index) => ({ field, index }))
    .filter(
      (item) =>
        item.field.label.toLowerCase().includes(search.toLowerCase()) ||
        item.field.id.toLowerCase().includes(search.toLowerCase()),
    );

  function handleSave(data: WorkspaceTaskType, index?: number) {
    if (index !== undefined) {
      taskTypesFieldArray.update(index, data);
    } else {
      taskTypesFieldArray.append(data);
    }

    void form.submit();
  }

  function handleEdit(index: number) {
    // The form's value, not the field-array row: a row also carries the
    // array's own `key`, which is not part of a task type.
    setSelectedTaskType({
      data: form.getValues(`tasks.${index}`) as WorkspaceTaskType,
      index,
    });
    setUpsertOpen(true);
  }

  async function handleDelete(index: number) {
    if (taskTypesFieldArray.fields.length === 1) {
      toast.error(t("At least one task type is required."));
      return;
    }

    // Asked first, like every other delete in these settings: one click on
    // the trash icon used to remove the type, and the tasks that use it lose
    // their type and its instructions.
    const label = taskTypesFieldArray.fields[index]?.label ?? "";
    const confirmed = await confirm({
      title: t("Delete this task type?"),
      description: t("Tasks of type {{label}} keep their records but lose the type and its instructions.", { label }),
      confirmText: t("Delete"),
      cancelText: t("Cancel"),
      variant: "destructive",
    });
    if (!confirmed) return;

    taskTypesFieldArray.remove(index);
    void form.submit();
  }

  return (
    // The task-type editor is a <Form> of its own, so it sits beside the
    // list's form rather than inside it: a <form> nested in another makes
    // Blink and WebKit stop its submit event at the outer one, and "Create
    // task type" used to reload the window with the fields in the URL. This
    // wrapper is what the editor's `absolute inset-0` covers and clips to.
    <div className="relative flex h-full flex-1 flex-col overflow-hidden">
      <Form form={form} className="flex h-full flex-1 flex-col overflow-y-auto">
        <SettingsSectionShell>
          <div className="flex flex-col gap-6">
            <div className="flex flex-col gap-1">
              <h1 className="text-sm font-semibold tracking-tight">{t("Task types")}</h1>
              <p className="text-sm text-muted-foreground">
                {t("Define the task taxonomy used across the workspace, including colors and agent instructions.")}
              </p>
            </div>

            <div className="flex items-center justify-between gap-4">
              <div className="flex max-w-sm flex-1 items-center gap-2">
                <InputGroup className="border-0 p-0 has-[[data-slot=input-group-control]:focus-visible]:bg-transparent">
                  <InputGroupAddon className="p-0">
                    <SearchIcon />
                  </InputGroupAddon>
                  <InputGroupInput
                    placeholder={t("Filter by name...")}
                    className="border-0 p-0 focus:bg-transparent"
                    value={search}
                    onChange={(event) => setSearch(event.target.value)}
                  />
                </InputGroup>
              </div>

              <Button
                type="button"
                variant="secondary"
                onClick={() => {
                  setSelectedTaskType(undefined);
                  setUpsertOpen(true);
                }}
              >
                {!form.isLoading ? <PlusIcon /> : <Spinner />}
                {t("New task type")}
              </Button>
            </div>

            <div className="rounded-md">
              <div className="grid grid-cols-[1.5fr_2fr_100px] items-center gap-4 px-4 py-2 text-xs font-medium uppercase tracking-wider text-muted-foreground">
                <div className="flex items-center gap-2">{t("Name")}</div>
                <div>{t("Description")}</div>
                <div className="text-right">{t("Actions")}</div>
              </div>

              <div className="flex flex-col divide-y rounded-md border">
                {filteredTasks.length === 0 ? (
                  <div className="flex h-32 flex-col items-center justify-center gap-2 text-muted-foreground">
                    <p className="text-sm">{t("No task types found")}</p>
                  </div>
                ) : (
                  filteredTasks.map(({ field, index }) => (
                    <div
                      key={field.key}
                      className="group grid grid-cols-[1.5fr_2fr_100px] items-center gap-4 px-4 py-3 text-sm transition-colors hover:bg-muted/30"
                    >
                      <div className="flex items-center gap-3">
                        <div
                          className="h-2 w-2 shrink-0 rounded-full"
                          style={{ backgroundColor: field.color || "#64748b" }}
                        />
                        <span className="font-medium text-foreground">{field.label}</span>
                      </div>
                      <div className="min-w-0">
                        <span className="line-clamp-1 text-xs text-muted-foreground">
                          {field.description || (
                            <span className="italic opacity-40">{t("No description")}</span>
                          )}
                        </span>
                      </div>
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-muted-foreground hover:text-foreground"
                          onClick={() => handleEdit(index)}
                          aria-label={t("Edit {{label}}", { label: field.label })}
                        >
                          <PencilIcon className="h-4 w-4" />
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-muted-foreground hover:text-destructive"
                          onClick={() => void handleDelete(index)}
                          disabled={taskTypesFieldArray.fields.length === 1}
                          aria-label={t("Delete {{label}}", { label: field.label })}
                        >
                          <Trash2Icon className="h-4 w-4" />
                        </Button>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>
        </SettingsSectionShell>
      </Form>

      {/* The editor slides over the list's own centred column rather than
          the whole content area, so its fields line up with the rows they
          edit. Neither layer takes clicks of its own; the open editor does. */}
      <div className="pointer-events-none absolute inset-0 overflow-hidden">
        <div className={cn("relative mx-auto h-full w-full", SETTINGS_CONTENT_MAX_WIDTH)}>
          <UpsertTaskTypeView
            open={upsertOpen}
            onOpenChange={setUpsertOpen}
            taskType={selectedTaskType?.data}
            index={selectedTaskType?.index}
            onSave={handleSave}
            takenIds={taskTypesFieldArray.fields
              .filter((_, index) => index !== selectedTaskType?.index)
              .map((field) => field.id)}
          />
        </div>
      </div>
    </div>
  );
}
