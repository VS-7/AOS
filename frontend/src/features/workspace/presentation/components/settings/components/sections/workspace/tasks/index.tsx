import { useState } from "react";
import { useFieldArray } from "react-hook-form";
import { PencilIcon, PlusIcon, SearchIcon, Trash2Icon } from "lucide-react";
import { z } from "zod";

import { aos } from "@/app/aos";
import { SettingsSectionShell } from "../../../section-shell";
import { Form } from "@/components/ui/form";
import { FractalAppError } from "@/core/errors/fractal.error";
import { FractalWorkspaceTaskType } from "@/features/workspace/interfaces/workspace.interfaces";
import { FractalWorkspaceTaskTypeSchema } from "@/features/workspace/schemas/workspace.schema";
import { toast } from "sonner";
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { UpsertTaskTypeView } from "./views/upsert-task-type.view";

const taskTypesFormSchema = z.object({
  tasks: z.array(FractalWorkspaceTaskTypeSchema).min(1, "At least one task type is required."),
});

export function WorkspaceTasksSection() {
  
  // `aos.useContext()` is Fractal's global route context (`withContext(...)`),
  // which this port's `app/aos.tsx` never wires -- `DefaultContext` (`app/
  // builders/types.ts`) is deliberately loose (`Record<string, any>`) for
  // exactly this unset case, so no per-call-site cast is needed here.
  const context = aos.useContext();
  const currentWorkspace = context.workspaces?.current;
  const [search, setSearch] = useState("");
  const [upsertOpen, setUpsertOpen] = useState(false);
  const [selectedTaskType, setSelectedTaskType] = useState<
    { data: FractalWorkspaceTaskType; index: number } | undefined
  >();

  const form = aos.useForm({
    schema: taskTypesFormSchema,
    mode: "onChange",
    mutation: "workspace.update",
    values: {
      // Task 10: `currentWorkspace.tasks` (`app/stores.ts`'s own
      // `WorkspaceTaskType`) types `label`/`color` optional — Go's
      // `workspace_get` doesn't always populate them (see that file's own
      // doc comment on the honest-empty-state policy). This form's Zod
      // schema (`FractalWorkspaceTaskTypeSchema`, pristine) requires both
      // as non-empty strings, so entries missing either get the same
      // fallback the empty-workspace default below already uses, rather
      // than widening the schema itself to accept what Go's shape allows.
      tasks: currentWorkspace?.tasks?.length
        ? currentWorkspace.tasks.map((task) => ({
            ...task,
            label: task.label || "Task",
            color: task.color || "#64748b",
          }))
        : [{ id: "task", label: "Task", color: "#64748b", instructions: "" }],
    },
    onSubmit: (values) => ({
      body: { tasks: values.tasks },
      params: { id: currentWorkspace?.id },
    }),
    onResponse: ({ error }) => {
      if (error) {
        if (error instanceof FractalAppError) {
          toast.error(error.message);
          return;
        }

        console.error(error);
        toast.error(error.message || "Failed to update task settings");
        return;
      }

      toast.success("Task settings updated successfully!");
    },
  });

  // `aos.useForm`'s `TValues` generic doesn't propagate the array item
  // shape from `taskTypesFormSchema` through to `form.control` here, so
  // `useFieldArray` falls back to react-hook-form's default (untyped)
  // field shape without this explicit type parameter.
  const taskTypesFieldArray = useFieldArray<{ tasks: FractalWorkspaceTaskType[] }, "tasks">({
    control: form.control as any,
    name: "tasks",
  });

  const filteredTasks = taskTypesFieldArray.fields
    .map((field, index) => ({ field, index }))
    .filter(
      (item) =>
        item.field.label.toLowerCase().includes(search.toLowerCase()) ||
        item.field.id.toLowerCase().includes(search.toLowerCase()),
    );

  function handleSave(data: FractalWorkspaceTaskType, index?: number) {
    if (index !== undefined) {
      taskTypesFieldArray.update(index, data);
    } else {
      taskTypesFieldArray.append(data);
    }

    form.handleSubmit((values) => form.submit())();
  }

  function handleEdit(index: number) {
    setSelectedTaskType({
      data: taskTypesFieldArray.fields[index],
      index,
    });
    setUpsertOpen(true);
  }

  function handleDelete(index: number) {
    if (taskTypesFieldArray.fields.length === 1) {
      toast.error("At least one task type is required.");
      return;
    }

    taskTypesFieldArray.remove(index);
    form.handleSubmit((values) => form.submit())();
  }

  return (
    <Form form={form} className="flex h-full flex-1 flex-col overflow-y-auto">
      <SettingsSectionShell className="relative">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-1">
            <h1 className="text-sm font-semibold tracking-tight">Task types</h1>
            <p className="text-sm text-muted-foreground">
              Define the task taxonomy used across the workspace, including colors and agent instructions.
            </p>
          </div>

          <div className="flex items-center justify-between gap-4">
            <div className="flex max-w-sm flex-1 items-center gap-2">
              <InputGroup className="border-0 p-0 has-[[data-slot=input-group-control]:focus-visible]:bg-transparent">
                <InputGroupAddon className="p-0">
                  <SearchIcon />
                </InputGroupAddon>
                <InputGroupInput
                  placeholder="Filter by name..."
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
              New task type
            </Button>
          </div>

          <div className="rounded-md">
            <div className="grid grid-cols-[1.5fr_2fr_100px] items-center gap-4 px-4 py-2 text-xs font-medium uppercase tracking-wider text-muted-foreground">
              <div className="flex items-center gap-2">Name</div>
              <div>Description</div>
              <div className="text-right">Actions</div>
            </div>

            <div className="flex flex-col divide-y rounded-md border">
              {filteredTasks.length === 0 ? (
                <div className="flex h-32 flex-col items-center justify-center gap-2 text-muted-foreground">
                  <p className="text-sm">No task types found</p>
                </div>
              ) : (
                filteredTasks.map(({ field, index }) => (
                  <div
                    key={field.id}
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
                          <span className="italic opacity-40">No description</span>
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
                      >
                        <PencilIcon className="h-4 w-4" />
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-muted-foreground hover:text-destructive"
                        onClick={() => handleDelete(index)}
                        disabled={taskTypesFieldArray.fields.length === 1}
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

        <UpsertTaskTypeView
          open={upsertOpen}
          onOpenChange={setUpsertOpen}
          taskType={selectedTaskType?.data}
          index={selectedTaskType?.index}
          onSave={handleSave}
        />
      </SettingsSectionShell>
    </Form>
  );
}
