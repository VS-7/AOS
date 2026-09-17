import { t } from "@/lib/i18n";
import * as React from "react"
import { z } from "zod"

import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  FormControl,
  FormField,
  Field,
  FieldGroup,
  Form,
  FormMessage,
} from "@/components/ui/form"
import { Textarea } from "@/components/ui/textarea"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { aos } from "@/app/aos"
import { toast } from "sonner"
import type { Todo } from "@/features/task/interfaces/todo.interfaces"

interface TodoDialogUpsertProps {
  taskId: string
  todo?: Todo
  onCreated?: () => void
  /** The trigger. Left out, the dialog is driven by `open`/`onOpenChange`. */
  children?: React.ReactNode
  open?: boolean
  onOpenChange?: (open: boolean) => void
}

export function TodoDialogUpsert({ taskId, todo, onCreated, children, open: controlledOpen, onOpenChange }: TodoDialogUpsertProps) {
  const [uncontrolledOpen, setUncontrolledOpen] = React.useState(false)
  const open = controlledOpen ?? uncontrolledOpen
  const setOpen = (next: boolean) => {
    if (controlledOpen === undefined) setUncontrolledOpen(next)
    onOpenChange?.(next)
  }

  const isEdit = !!todo

  // Built per render rather than at module load, so the validation message is
  // in the language the interface is in when the dialog opens.
  const formSchema = React.useMemo(() => z.object({
    description: z.string().trim().min(1, t("Description is required")),
    instructions: z.string().optional(),
    evidence: z.string().optional(),
  }), [])

  // Reads `todo.title`/`todo.content` — Go's real `Todo` field names (see
  // `interfaces/task.interfaces.ts`'s `TaskTodoSchema` doc comment)
  // — while keeping this form's own field names (`description`/
  // `instructions`) unchanged; the outgoing rename back to `title`/
  // `content` happens in `command-map.ts`'s `todo.create`/`todo.update`
  // entries, not here.
  const initialValues = React.useMemo(() => ({
    description: todo?.title || "",
    instructions: todo?.content || "",
    evidence: todo?.evidence || "",
  }), [todo])

  const form = aos.useForm({
    schema: formSchema,
    mutation: isEdit ? "todo.update" : "todo.create",
    values: initialValues,
    onSubmit: (values) => {
      if (isEdit) {
        // Every field goes, emptied ones as "": todos_update reads a missing
        // key as "leave it", so clearing the notes used to save nothing.
        return {
          params: { taskId, id: todo!.id },
          body: {
            description: values.description,
            instructions: values.instructions ?? "",
            evidence: values.evidence ?? "",
          },
        }
      }

      // The todo agent field is gone: Go's `Todo` has no agent, and the
      // disabled input only said so.
      const body: Record<string, unknown> = { description: values.description }
      if (values.instructions) body.instructions = values.instructions
      return { params: { taskId }, body }
    },
    onResponse: ({ error }) => {
      // Create/Update never submitted before, so a refusal had nowhere to go.
      if (error) {
        toast.error(error.message || t("Could not save the todo."))
        return
      }
      setOpen(false)
      form.reset()
      onCreated?.()
    },
  })

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {children && <DialogTrigger asChild>{children}</DialogTrigger>}
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{isEdit ? t("Edit Todo") : t("Create Todo")}</DialogTitle>
        </DialogHeader>
        <Form form={form}>
          <FieldGroup>
            <FormField
              control={form.control}
              name="description"
              render={({ field }) => (
                <Field>
                  <Label htmlFor="description">{t("Description")}</Label>
                  <FormControl>
                    <Textarea
                      {...field}
                      id="description"
                      placeholder={t("What needs to be done?")}
                      className="min-h-20 resize-none"
                    />
                  </FormControl>
                  <FormMessage />
                </Field>
              )}
            />
            <FormField
              control={form.control}
              name="instructions"
              render={({ field }) => (
                <Field>
                  <Label htmlFor="instructions">{t("Instructions (optional)")}</Label>
                  <FormControl>
                    <Textarea
                      {...field}
                      id="instructions"
                      placeholder={t("Instructions for the agent...")}
                      className="min-h-20 resize-none"
                    />
                  </FormControl>
                </Field>
              )}
            />
            {isEdit && (
              <FormField
                control={form.control}
                name="evidence"
                render={({ field }) => (
                  <Field>
                    <Label htmlFor="evidence">{t("Evidence (optional)")}</Label>
                    <FormControl>
                      <Textarea
                        {...field}
                        id="evidence"
                        placeholder={t("What was verified, concretely.")}
                        className="min-h-16 resize-none"
                      />
                    </FormControl>
                  </Field>
                )}
              />
            )}
          </FieldGroup>
          <DialogFooter className="mt-6">
            <Button variant="outline" type="button" onClick={() => {
              form.reset()
              setOpen(false)
            }}>
              {t("Cancel")}
            </Button>
            <Button type="submit" disabled={form.isLoading}>
              {form.isLoading ? t("Saving...") : isEdit ? t("Update") : t("Create")}
            </Button>
          </DialogFooter>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
