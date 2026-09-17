import React, { useEffect, useMemo } from "react";
import { z } from "zod";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { WorkspaceTaskType } from "@/features/workspace/interfaces/workspace.interfaces";
import { WorkspaceTaskTypeSchema } from "@/features/workspace/schemas/workspace.schema";
import { Slug } from "@/core/helpers/slug.helper";
import { Button } from "@/components/ui/button";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { SettingsStackedView } from "../../../../stacked-view";
import { aos } from "@/app/aos";
import { t } from "@/lib/i18n";

// The label is what the id is slugged from, so an empty one saved a task
// type with no label and an empty id — which the daemon accepted — the
// first time "Create task type" actually submitted. Built when the view
// renders, so the message is in the language on screen.
//
// `takenIds` are the ids of the other types: the id is slugged from the label,
// so a second "Docs" asked the daemon to store two types named "docs" and
// only its refusal said so.
function buildTaskTypeFormSchema(takenIds: string[]) {
  return WorkspaceTaskTypeSchema.extend({
    label: z.string().trim().min(1, t("Label is required")),
  }).superRefine((values, ctx) => {
    const id = values.id || Slug.generate(values.label);
    if (id && takenIds.includes(id)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["label"],
        message: t("Another task type already uses the id {{id}}.", { id }),
      });
    }
  });
}

interface UpsertTaskTypeViewProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  taskType?: WorkspaceTaskType;
  index?: number;
  onSave: (data: WorkspaceTaskType, index?: number) => void;
  /** The ids of every other task type, which this one may not reuse. */
  takenIds: string[];
}

export function UpsertTaskTypeView({
  open,
  onOpenChange,
  taskType,
  index,
  onSave,
  takenIds,
}: UpsertTaskTypeViewProps) {
  const takenKey = takenIds.join("\n");
  // eslint-disable-next-line react-hooks/exhaustive-deps -- keyed on the ids' content, not the array's identity
  const taskTypeFormSchema = useMemo(() => buildTaskTypeFormSchema(takenIds), [takenKey]);
  const form = aos.useForm({
    schema: taskTypeFormSchema,
    values: {
      id: "",
      label: "",
      color: "#64748b",
      description: "",
      instructions: "",
    },
    onSubmit(values) {
      const finalData = {
        ...values,
        id: values.id || Slug.generate(values.label),
      };
      onSave(finalData, index);
      onOpenChange(false);
    },
  });

  useEffect(() => {
    if (open) {
      if (taskType) {
        form.reset({
          ...taskType,
          description: taskType.description || "",
          instructions: taskType.instructions || "",
        });
      } else {
        form.reset({
          id: "",
          label: "",
          color: "#64748b",
          description: "",
          instructions: "",
        });
      }
    }
  }, [open, taskType, form]);

  return (
    <SettingsStackedView
      open={open}
      onBack={() => onOpenChange(false)}
      title={taskType ? t("Edit Task Type") : t("Create Task Type")}
      description={
        taskType
          ? t("Update the details for this task type.")
          : t("Define a new task type for your workspace.")
      }
      contentClassName="p-6"
    >
      <Form form={form}>
        <div className="grid gap-4">
          <div className="grid grid-cols-[1fr_auto] items-end gap-4">
            <FormField
              control={form.control}
              name="label"
              render={({ field }) => (
                <FormItem className="flex-1">
                  <FormLabel>{t("Label")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("Bug")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="color"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("Color")}</FormLabel>
                  <FormControl>
                    <div className="flex items-center gap-2">
                      <Input
                        type="color"
                        className="h-9 w-9 rounded-md p-1 cursor-pointer"
                        {...field}
                      />
                    </div>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name="description"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t("Description")}</FormLabel>
                <FormControl>
                  <Input placeholder={t("Briefly describe what this task type is for...")} {...field} value={field.value || ""} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="instructions"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t("Specific instructions")}</FormLabel>
                <FormControl>
                  <Textarea
                    placeholder={t("Guide agents on how to handle this task type.")}
                    className="min-h-24"
                    {...field}
                    value={field.value || ""}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        <div className="flex justify-end gap-2 border-t pt-4">
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
            {t("Cancel")}
          </Button>
          <Button type="submit">
            {taskType ? t("Save changes") : t("Create task type")}
          </Button>
        </div>
      </Form>
    </SettingsStackedView>
  );
}
